# Loan history (US-12) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** One Loans page with Out now / Returned / All, a period filter and search; an asset Loans tab; My loans with the same switch.

**Architecture:** Generalise the loans query (repository `ListLoans(domain.LoanFilter)` replaces `ListActiveLoans`). The service maps `q` to people through identity search and checks permissions. `GET /loans` gains the filters. The frontend gets pure helpers for the period presets and the state tag, plus the toolbar on `LoansPage.vue` and a Loans tab on the asset page.

**Tech Stack:** Go (pgx, sqlc, oapi-codegen strict), Vue 3 + PrimeVue 4 + Vue Query.

**Spec:** `docs/superpowers/specs/2026-10-31-loan-history-design.md`

## Global Constraints

- Comments in Vietnamese; UI text in English; LF endings (Python edits with `newline=''`).
- Regenerate, never hand-edit: `make generate` (backend), `npm run gen:api` (frontend).
- No migration needed.
- PrimeVue components and PrimeIcons only.
- Copy:
  - Show options: "Out now", "Returned", "All".
  - Period options: "Any time", "This month", "Last month", "Last 30 days", "Custom range".
  - Tags: "On loan", "Overdue N days" (the existing `overdueLabel`), "Returned", "Returned late".
  - Page title: "Loans".
- Commits end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Review Focus

1. A loan that started before the period and was returned inside it → listed. A loan returned
   the day before `from` → not listed. (Task 1 `TestBorrowing_ListLoansPeriod`.)
2. `from` after `to` → 422 field `from`. (Task 2 `TestListLoans_Filters`.)
3. A borrower with `borrow.request` only: `asset_id`, `state=all` without `mine`, or no `mine`
   → 403. With `mine=true&state=returned` → only their own returned loans. (Task 3 HTTP test.)
4. A search matching both an asset tag and a person → the union, with no duplicate rows.
   (Task 1 test.)
5. Last month on 31 Oct → 2026-09-01..2026-09-30; on 1 Jan → previous December.
   (Task 4 `periodRange` test.)

---

### Task 1: Repository `ListLoans`

**Files:**
- `backend/internal/inventory/domain/borrowing.go`, `domain/repository.go`
- `repository/queries/borrowing.sql`, `repository/borrowing_repository.go`
- Test: `repository/borrowing_repository_db_test.go`
- Fake: `service/fakes_test.go`

**Interfaces:**
- Produces:
  - `domain.LoanFilter{State string /* "active"|"returned"|"all" */; From, To *time.Time; BorrowerID *uuid.UUID; AssetID *uuid.UUID; Q string; MatchBorrowers []uuid.UUID; Sort string; Page, PageSize int}`
  - `BorrowingRepository.ListLoans(ctx, today time.Time, f domain.LoanFilter) ([]domain.Loan, int64, error)` — replaces `ListActiveLoans`.
  - `Q` matches `a.tag ILIKE` or `a.name ILIKE` (`%q%`), or `borrower_id = ANY(MatchBorrowers)`.
  - Sorts: `started_on`, `return_by`, `returned_on`, each ±.
  - Default: `active` → overdue first (the existing order); otherwise `-started_on`.

- [ ] **Step 1: Write the failing tests**
  - `TestBorrowing_ListLoansPeriod`:
    - lend 3 assets: A started 10-01 returned 10-05; B started 09-20 returned 10-02; C started 09-01 returned 09-30.
    - `{State: "all", From: 10-01, To: 10-31}` → A and B, not C.
    - `{State: "returned"}` → all three.
    - `{State: "active"}` → none of them.
  - `TestBorrowing_ListLoansSearch`:
    - a `q` equal to A's tag plus `MatchBorrowers` = [C's borrower] → exactly A and C.
    - `AssetID` = B → only B.
- [ ] **Step 2: Run** `CI=true go test ./internal/inventory/repository/ -run ListLoans` → build fails (`ListLoans` undefined).
- [ ] **Step 3: Implement**
  - Replace `ListActiveLoans`/`CountActiveLoans` SQL with `ListLoans`/`CountLoans` (join `inventory.assets`).
  - Update callers and the fake (the fake records the last filter in `lastLoanFilter`).
  - Run `make generate`.
- [ ] **Step 4: Run** the same command plus `go build ./...` → pass.
- [ ] **Step 5: Commit** `feat(borrowing): list loans by state, period, search and asset`.

### Task 2: Service filters and permissions

**Files:**
- `service/borrowing.go`
- Test: `service/borrowing_return_test.go` (or a new `service/loan_history_test.go`)

**Interfaces:**
- Produces:
  - `service.LoanQuery{Mine bool; State string; From, To *time.Time; AssetID *uuid.UUID; Q, Sort string; Page, PageSize int}`
  - `(*Service).ListLoans(ctx, q LoanQuery) ([]LoanView, int64, error)`
- Rules:
  - `Mine` → `borrowActor` and `BorrowerID` = the actor; anything else needs manage.
  - `AssetID` with `Mine` → 403 unless the actor can manage.
  - `From > To` → `ErrInvalidLoan` field `from`.
  - `State` outside `active|returned|all` → field `state`; empty means `active`.
  - `Q` trimmed; non-empty → `SearchAccounts(q, maxTicketSearchPeople)` into `MatchBorrowers`.
  - `OverdueDays` stays 0 for returned loans.
- [ ] **Step 1: Failing tests** `TestListLoans_Filters`:
  - from > to → field `from`.
  - state "x" → field `state`.
  - mine + request-only → filter has `BorrowerID` = the actor.
  - no mine + request-only → 403.
  - asset_id + request-only + mine → 403.
  - q "nguyen" → `SearchAccounts` limit ≥ 1000 and the filter carries `MatchBorrowers`.
  - Update `TestListLoans_Mine` to the new signature.
- [ ] **Step 2: Run** `go test ./internal/inventory/service/` → build fails.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run** → pass.
- [ ] **Step 5: Commit** `feat(borrowing): loan history filters and permissions in the service`.

### Task 3: API, docs, HTTP test

**Files:**
- `handler/openapi.yaml`, `handler/borrowing.go`
- `backend/docs/inventory.md`
- Test: `internal/inventory/http_db_test.go` (`TestBorrowingOverHTTP`)

**Interfaces:**
- `GET /loans` parameters: `state` enum `[active, returned, all]`, `from`, `to` (date), `q` (maxLength 100), `asset_id` (ID).
- `sort` enum gains `returned_on`, `-returned_on`.
- `Loan` gains `returned_on` (date, nullable) and `return_note` (string).

- [ ] **Step 1: Failing test.** Extend `TestBorrowingOverHTTP`:
  - after the existing return: `GET /loans?state=returned` (officer) contains x with `returned_on`.
  - the employee: `GET /loans?mine=true&state=all` contains x.
  - the employee: `GET /loans?state=all` → 403; `GET /loans?mine=true&asset_id=<x>` → 403.
  - `GET /loans?from=2026-11-02&to=2026-11-01` → 422.
- [ ] **Step 2: Run** `CI=true go test ./internal/inventory/ -run TestBorrowingOverHTTP` → fails.
- [ ] **Step 3: Implement**
  - Update the spec and handler mapping, then run `make generate`.
  - Docs: the Returns/Loans bullet and the permission rows.
- [ ] **Step 4: Run** → pass; `make check`.
- [ ] **Step 5: Commit** `feat(borrowing): loan history API`.

### Task 4: Frontend Loans page and My loans

**Files:**
- `frontend/src/lib/api/inventory.d.ts` (generated)
- `features/borrowing/borrowing.ts`, `borrowing.test.ts`
- `pages/LoansPage.vue`
- `app/layouts/AppSidebar.vue`, `app/routes.ts` (title "Loans")

**Interfaces:**
- Produces:
  - `type Period = 'any' | 'this_month' | 'last_month' | 'last_30' | 'custom'`
  - `periodRange(p: Exclude<Period, 'any' | 'custom'>, today: string): { from: string; to: string }`
  - `loanStateTag(l: { overdue_days: number; return_by?: string | null; returned_on?: string | null }): { label: string; tone: Tone }`
    - On loan → `info`
    - Overdue → `danger` (via `overdueLabel`)
    - Returned → `success`
    - Returned late → `warn`
  - LoansPage URL state:
    - `show` = `active` | `returned` | `all` (default `active`)
    - `period`, `from`, `to`, `q`, `sort`, `page`
    - Sends `state`, `from`, `to`, `q`.
    - A "Returned on" column, sortable as `returned_on`.
  - Sidebar entry "Loans" (was "On loan"). My loans uses the same toolbar (`mine`), without Borrower.

- [ ] **Step 1: Failing tests**
  - `periodRange`, with today = 2026-10-31:
    - this_month → 10-01..10-31
    - last_month → 09-01..09-30
    - last_30 → 10-02..10-31
    - last_month with today = 2026-01-15 → 2025-12-01..2025-12-31
  - `loanStateTag`, four cases with exact labels and tones.
  - `routes.test.ts`: `/borrowing/loans` has title "Loans".
- [ ] **Step 2: Run** `npx vitest run src/features/borrowing src/app/routes.test.ts` → fail.
- [ ] **Step 3: Implement**
  - Run `npm run gen:api`.
  - Toolbar: `SegmentedFilter` (Show), a `Select` (Period), a range `DatePicker` shown only for Custom, and a search `InputText` with a 300 ms debounce, as in `TicketTable`.
- [ ] **Step 4: Run** `npm run check` → pass.
- [ ] **Step 5: Commit** `feat(borrowing): Loans page with period, state and search; My loans history`.

### Task 5: Asset Loans tab

**Files:**
- `features/assets/pages/AssetDetailPage.vue`
- Create: `features/borrowing/components/AssetLoans.vue`

**Interfaces:**
- `AssetLoans.vue` props `{ assetId: string }`:
  - calls `useLoans({ asset_id, state: 'all', sort: '-started_on', page_size: 50 })`;
  - columns: borrower, borrowed on, return by, returned on, state tag (`loanStateTag`), ticket link;
  - empty state "Never lent.".
- The tab "Loans" shows only with `borrow.manage`.

- [ ] **Step 1:** No component-test setup exists; the logic it relies on is already covered by Task 4. Verify with `npm run check`.
- [ ] **Step 2: Implement.**
- [ ] **Step 3:** Browser check:
  - Loans page: Returned + Last month; search by tag and by person.
  - The asset's Loans tab (manager).
  - Borrower: My loans with Returned.
- [ ] **Step 4: Commit** `feat(borrowing): loan history on the asset page`.
