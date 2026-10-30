# Loan returns (US-11) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Inventory officers record the return of a loaned asset (date, condition, note); the loan closes as Returned, the asset leaves On loan, the borrower gets a receipt, and borrowers see their active loans.

**Architecture:** Same layering as US-10: domain checks → repository transaction (lock asset, lock active loan, update, outbox event, `EnqueueTx` email) → service (permissions, field validation) → strict handler from `openapi.yaml` → Vue Query mutation + PrimeVue dialog.

**Tech Stack:** Go (chi, oapi-codegen strict, pgx, sqlc, goose, River), Vue 3 + PrimeVue 4 + Vue Query.

**Spec:** `docs/superpowers/specs/2026-10-30-loan-returns-design.md`

## Global Constraints

- Code comments in Vietnamese; UI text in English; LF line endings (Python edits: `newline=''`).
- Never hand-edit `*.gen.go`, `repository/db/*`, `frontend/src/lib/api/*.d.ts`: run `make generate` (backend) and `npm run gen:api` (frontend).
- Transactions, outbox `Append` and `EnqueueTx` live in the repository; services never see a `pgx.Tx`. Lock order: ticket → asset → loan.
- New migration `00009_loan_returns.sql`; do not edit `00008_borrowing.sql`.
- Only PrimeVue components and PrimeIcons that exist.
- Permission: recording a return is `inventory.borrow.manage`; `GET /loans?mine=true` needs only `inventory.borrow.request`.
- Copy pinned by the spec: toast "LAP-0042 returned."; conflict "This loan was already closed."; condition option "Fine — back to Available"; email subject "LAP-0042 return recorded", body "We recorded LAP-0042 · Dell Latitude 7440 as returned on 3 Nov 2026. Thanks."; activity "took this asset back from Phạm Thu Hà (ticket #12)" / "… · now In repair"; ticket page "On loan since 1 Nov" / "Returned on 3 Nov".
- Commits end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Never commit `backend/docs/tasks/`.

## Review Focus

1. Two officers record the same return at once → exactly one succeeds; the other gets 409 `ErrNoActiveLoan`; one email job, one event. (Task 1 test `TestBorrowing_ConcurrentReturn`.)
2. Returned on before the loan started, or after today → 422 with field `returned_on`, nothing changes. (Task 2 test `TestReturnLoan_Dates`.)
3. Condition status that is retired-kind, archived, Reserved, On loan, or unknown → 422 field `status_id`. (Task 2 test `TestReturnLoan_Condition`.)
4. Borrower's account disabled since the loan started → the return still succeeds; the email is skipped by the existing inactive check. (Task 2 test `TestReturnLoan_DisabledBorrower`.)
5. Return on an asset that is not on loan (available, reserved) → 409 `ErrNoActiveLoan`, status unchanged. (Task 1 test `TestBorrowing_ReturnRules`.)

---

### Task 1: Migration, domain and repository `Return`

**Files:**
- Create: `backend/migrations/00009_loan_returns.sql`
- Modify: `backend/internal/inventory/domain/borrowing.go`, `domain/errors.go`, `domain/repository.go`
- Modify: `backend/internal/inventory/repository/queries/borrowing.sql`, `repository/borrowing_repository.go`
- Modify: `backend/internal/inventory/contract/events.go`, `backend/internal/inventory/job/borrowing.go`
- Test: `backend/internal/inventory/repository/borrowing_repository_db_test.go`, `domain/borrowing_test.go`

**Interfaces:**
- Produces:
  - Migration: `loans` + `returned_by uuid`, `return_note text`, `CONSTRAINT loans_returned_on_check CHECK (returned_on IS NULL OR returned_on >= started_on)`. Down drops them.
  - `domain.Loan` gains `ReturnedBy *uuid.UUID`, `ReturnNote string`.
  - `domain.LoanReturn struct { ReturnedOn time.Time; StatusID uuid.UUID; Note string }`
  - `domain.ErrNoActiveLoan = errs.Conflict("/errors/no-active-loan", "Asset is not on loan", errs.WithDetail("The loan was already closed. Reload to see where the asset stands."))`
  - `domain.CheckReturnedOn(startedOn, returnedOn, today time.Time) error` → `ErrInvalidLoan` with field `returned_on` ("can't be before the loan started" / "can't be in the future"). Extend the `ErrInvalidLoan` field comment with `returned_on, status_id, note`.
  - `BorrowingRepository.Return(ctx, assetID uuid.UUID, r domain.LoanReturn, version int32) (domain.Loan, error)`
  - `BorrowingRepository.ListActiveLoans(ctx, today time.Time, borrowerID *uuid.UUID, sort string, page, pageSize int)` (new `borrowerID` filter; nil = everyone; also in `CountActiveLoans`).
  - `contract.EventAssetReturned = "inventory.asset_returned"`, `contract.AssetReturned{AssetID, LoanID, BorrowerID uuid.UUID; TicketID *uuid.UUID; TicketNumber int64; ReturnedOn string; StatusID uuid.UUID; Note string}` (json snake_case, omitempty on optional ones).
  - `job.EmailLoanReturned = "loan_returned"`.

- [ ] **Step 1: Write failing tests**
  - `domain`: `TestCheckReturnedOn` — started 2026-10-20, today 2026-10-30: 10-19 → field `returned_on`; 10-31 → field `returned_on`; 10-20 and 10-30 → nil.
  - DB `TestBorrowing_Return`: lend an asset (start 2026-10-20), `Return(ctx, id, LoanReturn{ReturnedOn: 2026-10-28, StatusID: AvailableStatusID, Note: "scratched lid"}, version)` → loan `ReturnedOn` = 10-28, `ReturnedBy` = actor, `ReturnNote` = note; asset status = Available; `ActiveLoan` = nil; one `loan_returned` email job for the loan (`emailJobs(t, r, "loan_returned", loan.ID) == 1`); one `inventory.asset_returned` event on the asset; the asset can be lent again.
  - DB `TestBorrowing_ReturnRules`: available asset → `ErrNoActiveLoan`; reserved asset → `ErrNoActiveLoan`; stale version → `ErrAssetChanged`; returning to a non-default status (In repair) sets that status.
  - DB `TestBorrowing_ConcurrentReturn`: two goroutines return the same loan → one nil, one `ErrNoActiveLoan`; email jobs = 1.
  - DB: extend the `ListActiveLoans` assertions with `borrowerID` set → only that borrower's loans, total matches.

- [ ] **Step 2: Run, expect FAIL** — `cd backend && CI=true go test ./internal/inventory/domain/ ./internal/inventory/repository/ -run 'Return|CheckReturnedOn|QueueOverdue'` → build errors (`Return` undefined).

- [ ] **Step 3: Implement**
  - SQL: `GetActiveLoanForUpdate :one` (`… WHERE asset_id = @asset_id AND returned_on IS NULL FOR UPDATE`), `ReturnLoan :one` (sets `returned_on`, `returned_by`, `return_note` (NULL if empty) `WHERE id = @id AND returned_on IS NULL RETURNING *`), `borrower_id` narg filter on `ListActiveLoans`/`CountActiveLoans`.
  - `Return`: `WithTx` → `lockAssets` (status must be `OnLoanStatusID`, else `ErrNoActiveLoan`; version mismatch → `ErrAssetChanged`) → `GetActiveLoanForUpdate` (no rows → `ErrNoActiveLoan`) → `ReturnLoan` → `setStatus(r.StatusID)` → append `AssetReturned` (ticket number via the loan's ticket, as `ActiveLoan` does) → `EnqueueTx` `BorrowingEmailArgs{EmailKind: EmailLoanReturned, LoanID: &loan.ID}` the same way Reject enqueues. Check-constraint violation maps to `ErrInvalidLoan` field `returned_on` in `mapBorrowErr`.
  - Update `toLoan`, fakes (`fakeBorrowing.Return`, new `ListActiveLoans` signature), callers of `ListActiveLoans` (pass `nil`).
  - `make generate`.

- [ ] **Step 4: Run, expect PASS** — same command plus `go build ./...` → ok.

- [ ] **Step 5: Commit** — `feat(borrowing): record a loan's return in the repository`.

### Task 2: Service `ReturnLoan`, email, activity

**Files:**
- Modify: `backend/internal/inventory/service/borrowing.go`, `service/borrowing_mail.go`, `service/activity.go`
- Create: `backend/internal/inventory/service/templates/loan_returned.txt`, `loan_returned.html`
- Test: `service/borrowing_test.go`, `service/borrowing_mail_test.go`, `service/activity_test.go`

**Interfaces:**
- Consumes: Task 1 `Return`, `LoanReturn`, `CheckReturnedOn`, `ErrNoActiveLoan`, `AssetReturned`, `EmailLoanReturned`.
- Produces:
  - `service.ReturnInput struct { ReturnedOn time.Time; StatusID *uuid.UUID; Note string }`
  - `(*Service).ReturnLoan(ctx, assetID uuid.UUID, in ReturnInput, version int32) (AssetView, error)`
  - `(*Service).ListLoans(ctx, mine bool, sort string, page, pageSize int)` — `mine` needs `borrow.request` or `borrow.manage` (`borrowActor`) and filters to the actor; `!mine` keeps `borrow.manage`.
  - `TicketView` gains `Loan *domain.Loan`, filled only in `GetTicket` for `handed_over` tickets via a new repo method `LoanForTicket(ctx, ticketID) (*domain.Loan, error)` (query `GetLoanByTicket :one`; add to the interface and fake in this task).
  - Activity entry kind `"returned"` with `Person` = borrower, `TicketNumber`, `StatusName` (new field on the entry, only for `returned`, empty when the new status is the default available one).

- [ ] **Step 1: Write failing tests**
  - `TestReturnLoan_Dates`: loan started 10-20, today 10-30 (HCM fixed clock) → `ReturnedOn` 10-19 and 10-31 give field `returned_on`; repository `Return` not called.
  - `TestReturnLoan_Condition`: `StatusID` of a retired-kind status, an archived status, `ReservedStatusID`, `OnLoanStatusID`, `uuid.New()` → field `status_id`; nil → repo receives the `Default(available)` status id; In repair → passed through.
  - `TestReturnLoan_Permissions`: `borrow.request` only → 403; note > 500 chars → field `note`.
  - `TestReturnLoan_DisabledBorrower`: borrower inactive → return succeeds.
  - `TestListLoans_Mine`: `borrow.request` + `mine=true` → repo got `borrowerID` = actor; `mine=false` without manage → 403.
  - `TestSendBorrowingEmail_Returned`: subject "LAP-0042 return recorded"; text and HTML contain "We recorded LAP-0042 · Dell Latitude 7440 as returned on 3 Nov 2026. Thanks."; idempotency key `inventory/borrowing/loan_returned/<loan id>`; a loan whose `ReturnedOn` is nil sends nothing.
  - Activity: an `asset_returned` event renders kind `returned`, person = borrower, ticket number, `StatusName` "In repair"; the status change it carries is not a separate `updated` entry.
  - `TestGetTicket_Loan`: handed-over ticket view has `Loan` with `StartedOn`; waiting ticket has nil.

- [ ] **Step 2: Run, expect FAIL** — `go test ./internal/inventory/service/` → undefined `ReturnLoan`.

- [ ] **Step 3: Implement** — validation order: permission → `ActiveLoan` (nil → `ErrNoActiveLoan`) → `CheckReturnedOn(loan.StartedOn, in.ReturnedOn, s.today())` → status rules → `cleanText(note, domain.MaxTicketNote)` → `Return`; then return `s.view` of the asset like `LendAsset`. Email case `EmailLoanReturned` in `SendBorrowingEmail`: stale when `ReturnedOn == nil`. Templates copy the `loan_overdue` pair's layout.

- [ ] **Step 4: Run, expect PASS** — `go test ./internal/inventory/...` (non-DB) → ok.

- [ ] **Step 5: Commit** — `feat(borrowing): return a loan in the service, with receipt email and activity`.

### Task 3: HTTP API and docs

**Files:**
- Modify: `backend/internal/inventory/handler/openapi.yaml`, `handler/borrowing.go`, `handler/activity.go` (or wherever activity entries map)
- Modify: `backend/docs/inventory.md`
- Test: `backend/internal/inventory/http_db_test.go` (`TestBorrowingOverHTTP`)

**Interfaces:**
- Produces:
  - `POST /assets/{assetID}/return` operationId `returnAsset`, body `ReturnAssetRequest {returned_on: date (required), status_id?: ID, note?: string ≤500, version: int32}` → 200 `Asset`.
  - `GET /loans` gains `mine: boolean`.
  - `BorrowTicket` gains optional `loan: TicketLoan {started_on: date, return_by?: date, returned_on?: date}`.
  - `AssetActivityEntry.kind` enum gains `returned`; entry gains optional `status_name: string`.
  - Docs: permission table rows for `POST /assets/{assetID}/return` (manage) and `GET /loans?mine=true` (request); event list gains `asset_returned`; email list gains `loan_returned`.

- [ ] **Step 1: Failing test** — extend `TestBorrowingOverHTTP`: after lending, `POST /assets/{id}/return {"returned_on": today, "version": v}` as manager → 200 with status Available; again → 409 `/errors/no-active-loan`; as a `borrow.request`-only user `GET /loans?mine=true` → 200 with only their loans, `GET /loans` → 403.
- [ ] **Step 2: Run, expect FAIL** — `CI=true go test ./internal/inventory/ -run TestBorrowingOverHTTP` → 404/405 on the new route.
- [ ] **Step 3: Implement** — spec, `make generate`, handlers mapping fields as `LendAsset` does; docs.
- [ ] **Step 4: Run, expect PASS** — same command; then `make check`.
- [ ] **Step 5: Commit** — `feat(borrowing): return endpoint, my loans, ticket loan`.

### Task 4: Frontend — Record return

**Files:**
- Modify: `frontend/src/lib/api/inventory.d.ts` (generated), `frontend/src/features/borrowing/api.ts`, `borrowing.ts`, `borrowing.test.ts`
- Create: `frontend/src/features/borrowing/components/ReturnDialog.vue`
- Modify: `frontend/src/features/assets/pages/AssetDetailPage.vue`, `features/borrowing/pages/LoansPage.vue`, `features/assets/activity.ts`, `activity.test.ts`, `components/AssetActivity.vue`

**Interfaces:**
- Consumes: Task 3 API.
- Produces:
  - `useReturnAsset()` — `useBorrowMutation(({ id, returnedOn, statusId, note, version }) => POST /assets/{assetID}/return)`.
  - `returnConditionOptions(statuses: Status[]): { label: string; value: string | null }[]` in `borrowing.ts` — first `{ label: 'Fine — back to Available', value: null }`, then statuses not archived, kind not `available` and not `retired`, not borrowing statuses, in list order.
  - `returnFormError(returnedOn: string, startedOn: string, today: string): string | null` — "Pick the return date." / "The loan started on <shortDay>; pick that day or later." / "That date hasn't come yet."
  - `ReturnDialog.vue` props `{ assetId: string; tag: string; version: number; startedOn: string }`, `v-model:visible`; fields Returned on (DatePicker, min startedOn, max today from `useBusinessDay`), Condition (Select of `returnConditionOptions`), Note (Textarea ≤500). Success toast "`${tag}` returned."; a 409 `/errors/no-active-loan` shows "This loan was already closed." as the general error.
  - Activity summary `returned`: "took this asset back from {person}" + " (ticket #N)" + " · now {status_name}" when present; icon `pi pi-replay`.

- [ ] **Step 1: Failing tests** (`borrowing.test.ts`, `activity.test.ts`): `returnConditionOptions` order and exclusions (available, retired, archived, Reserved, On loan left out; null first); `returnFormError` three messages and null for a valid date; `activitySummary` for `returned` with and without ticket / status.
- [ ] **Step 2: Run, expect FAIL** — `npx vitest run src/features/borrowing src/features/assets/activity.test.ts`.
- [ ] **Step 3: Implement** — `npm run gen:api` first; dialog modelled on `LendDialog.vue`; on the asset page, managers see **Record return…** next to the on-loan notice when `borrowing.state === 'on_loan'` (the loan's `since` is the `startedOn`); on LoansPage a row-menu item and an inline text button "Record return" like Tickets' Approve.
- [ ] **Step 4: Run, expect PASS** — same vitest command, then `npm run check`.
- [ ] **Step 5: Commit** — `feat(borrowing): record returns from the asset and On loan pages`.

### Task 5: Frontend — ticket loan line and My loans

**Files:**
- Modify: `frontend/src/features/borrowing/pages/TicketDetailPage.vue`, `pages/LoansPage.vue` (prop `mine`), `frontend/src/app/routes.ts`, `routes.test.ts`, `frontend/src/app/layouts/AppSidebar.vue`
- Create: `frontend/src/features/borrowing/pages/MyLoansPage.vue`

**Interfaces:**
- Produces: route `/borrowing/mine/loans` name `my-loans`, meta `{ title: 'My loans', icon: 'pi pi-clock', perm: Perm.BorrowRequest }`; sidebar entry "My loans" right after "My tickets". `LoansPage` takes `mine?: boolean`: sends `mine: true`, hides Borrower and row actions, header "My loans" / "What you have now, and when it's due back.", empty "You have nothing on loan."; `MyLoansPage` renders `<LoansPage mine />`. Ticket page under the asset: `On loan since {shortDay(started_on)}` or `Returned on {shortDay(returned_on)}`.

- [ ] **Step 1: Failing test** — `routes.test.ts`: `router.resolve('/borrowing/mine/loans').meta.perm` is `Perm.BorrowRequest` and name `my-loans`.
- [ ] **Step 2: Run, expect FAIL** — `npx vitest run src/app/routes.test.ts`.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run, expect PASS** — `npm run check`; backend `make check` and `CI=true go test -p 1 ./internal/inventory/...`.
- [ ] **Step 5: Commit** — `feat(borrowing): my loans and the loan line on tickets`. Then browser walk-through with seed accounts (manager returns a loan to In repair; borrower sees My loans and the ticket's "Returned on"); the receipt to the `@storeit.test` borrower must show as dropped by the mail guard in the worker log, not sent.
