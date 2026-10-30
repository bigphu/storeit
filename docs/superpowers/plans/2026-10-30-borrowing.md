# Borrowing an Asset (US-10) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Employees ask for an asset type through a borrow ticket; asset managers approve it by reserving a specific asset, hand it over (starting a loan), or lend directly; an asset can never be reserved or lent twice; borrowers get email on approval, rejection, manager cancellation and once when overdue.

**Architecture:**
- **Storage:** inventory migration 00008 adds `asset_types.can_borrow`, two system statuses (Reserved, On loan), `inventory.borrow_tickets` and `inventory.loans` (partial unique indexes stop a second approved ticket or active loan per asset), and the two new permissions.
- **Consistency:** a new `BorrowingRepository` does every state change in one transaction: lock the ticket, then the asset; check; write; set the asset status quietly (no `asset_updated` event); append outbox events; `EnqueueTx` the email job. Existing asset write paths refuse to touch Reserved/On loan assets, under the same asset lock.
- **Email:** inventory joins the worker (`RegisterWorkers`, `PeriodicJobs`) with its own templates; an hourly scan queues one overdue email per loan after 08:00 business time; the platform `mail` package drops reserved test domains.
- **Frontend:** a Borrowing sidebar section (My tickets, Tickets, On loan), ticket detail page, New ticket / Approve / Reject / Lend dialogs, a Borrowing panel and actions on the asset page and list.

**Tech Stack:** Go (chi, oapi-codegen strict server, pgx, sqlc, goose, River), Vue 3 + TypeScript + PrimeVue 4 + Vue Query, Vitest.

**Spec:** `docs/superpowers/specs/2026-10-30-borrowing-design.md`

## Global Constraints

- Branch `feat/us10-borrowing`, linear history (rebase, never `--no-ff` merges; see `main` ruleset). Stage explicit paths. Never stage `backend/docs/tasks/`, the deleted `docs/superpowers/{plans,specs}/2026-10-0*` files, or the user's unrelated changes (`.gitignore`, `frontend/.vscode`, `src/app/pages/*`, `src/lib/api/client.ts`, `src/lib/auth/tokens*`, `src/lib/errors*`, `frontend/tsconfig*`, `devdocs.toml`, `docs/guides`, `docs/index.mdx`, `docs/StoreIT-*`).
- End commit messages with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Code comments in Vietnamese; UI and email text in English. LF line endings; Python edits use `newline=''`.
- PrimeVue v4 components; PrimeIcons only, checked to exist in `node_modules/primeicons/primeicons.css`.
- New migration **00008**; never edit applied migrations. It grants the new permissions to Administrator.
- Transactions, outbox appends, row locks and `EnqueueTx` live in repositories; services never see `pgx.Tx`; `domain` may not import `platform/events`.
- Lock order everywhere: **ticket row, then asset rows (ID order)**. No path locks an asset and then a ticket.
- Regenerate with `make generate` (backend) and `npm run gen:api` (frontend); never hand-edit generated code.
- Never send real email to seed addresses; tests use the `log` transport or fakes.
- **Values verbatim from the spec:** note and reject/cancel reason ≤ 500 characters; requirements max 10, form `<key>:<op>:<value>`; Reserved = kind `unavailable`, ID `00000000-0000-7000-8000-000000000205`; On loan = kind `in_use`, ID `…0206`; `can_borrow` default false; seed turns it on for Laptop, Monitor, Máy chiếu, Điện thoại; business time zone default `Asia/Ho_Chi_Minh`; overdue email once, not before 08:00 business time.

## Review Focus

1. **Requester's account disabled between approval and hand-over:** hand-over is refused (422 field `requester`), the reservation stays so a manager can cancel it. Pinned in Task 6 (`TestBorrowing_HandOverDisabledRequester`).
2. **Approved ticket cancelled before its "approved" email job runs:** no approval email goes out. Pinned in Task 7 (`TestSendBorrowingEmail_SkipsStale`).
3. **Business day at 00:30 in Vietnam (17:30 UTC the day before):** "needed from" = the Vietnamese date is accepted and a loan started then gets that date. Pinned in Task 6 (`TestService_BusinessDay`).
4. **Changing the type of a reserved or on-loan asset:** refused with 409 like a status change. Pinned in Task 4 (`TestAssetGuards_BorrowingAsset`).
5. **"Can be borrowed" turned off after approval:** hand-over still works; new approvals and lends of that type get 409. Pinned in Task 5 (`TestBorrowing_SwitchOffAfterApprove`).

---

### Task 1: Mail guard for reserved test domains

**Files:**
- Modify: `backend/internal/platform/mail/mail.go` (or a new `guard.go`), `mail_test.go`/`guard_test.go`
- Modify: `backend/docs/platform.md` (`### mail`)

**Interfaces — Produces:** `mail.New` returns a `Sender` that, for a recipient whose domain ends in `.test`, `.example`, `.invalid` or `.localhost` (or equals one of those labels), logs at WARN (`mail dropped: reserved test domain`, with the domain only) and returns nil without calling the transport. `func IsReservedTestDomain(email string) bool`.

- [ ] **Step 1: Failing tests** `TestIsReservedTestDomain` (true: `an@storeit.test`, `x@a.example`, `y@foo.invalid`, `z@localhost`; false: `an@storeit.vn`, `a@test.com`, `b@example.com`) and `TestSenderDropsReservedDomains` (wrap a fake transport: `Send` to `an@storeit.test` returns nil and the fake saw no message; to `an@storeit.vn` the fake saw one).
- [ ] **Step 2:** `cd backend && go test ./internal/platform/mail/` → FAIL (undefined).
- [ ] **Step 3:** Implement as a wrapper inside `mail.New` so every transport is guarded; one line in `platform.md`: "Recipients in reserved test domains (.test, .example, .invalid, .localhost) are logged and dropped, whatever the transport."
- [ ] **Step 4:** Same command → PASS.
- [ ] **Step 5:** Commit `feat(mail): never deliver to reserved test domains`.

### Task 2: Identity — account search for other modules

**Files:**
- Modify: `backend/internal/identity/contract/contract.go`, `identity/service/contract.go`, identity repository (reuse the `ListAccounts` query's search if it filters by name/email; else add `SearchActiveAccounts` to `queries/accounts.sql` + `make generate`)
- Modify: every fake `AccountReader` in other modules' tests (`inventory/service/fakes_test.go` and any other the compiler finds)
- Test: `backend/internal/identity/module_db_test.go` (or the repository db test)

**Interfaces — Produces:**

```go
// AccountReader (thêm)
// SearchAccounts: account đang hoạt động có tên hay email chứa q (không phân biệt hoa
// thường), sắp theo tên, tối đa limit (1..50); q rỗng trả các account đầu tiên theo tên
SearchAccounts(ctx context.Context, q string, limit int) ([]Account, error)
```

- [ ] **Step 1: Failing test** `TestAccountReader_SearchAccounts`: three accounts ("Phạm Thu Hà" ha@…, "Nguyễn Văn An" an@…, a disabled "Hà Disabled"); `SearchAccounts(ctx, "hà", 10)` returns only Phạm Thu Hà; `"AN@"` matches An by email; limit 1 returns one.
- [ ] **Step 2:** `cd backend && CI=true go test ./internal/identity/ -run SearchAccounts` → FAIL.
- [ ] **Step 3:** Implement; update `backend/docs/identity.md` contract list.
- [ ] **Step 4:** Same → PASS; `go build ./...` passes (fakes updated).
- [ ] **Step 5:** Commit `feat(identity): search active accounts for other modules`.

### Task 3: Inventory domain, events, migration and queries

**Files:**
- Create: `backend/migrations/00008_borrowing.sql`
- Create: `backend/internal/inventory/domain/borrowing.go`, `borrowing_test.go`
- Modify: `domain/permissions.go`, `domain/errors.go`, `domain/asset_type.go` (`CanBorrow bool`), `contract/events.go`
- Create: `backend/internal/inventory/repository/queries/borrowing.sql`; modify `queries/asset_types.sql` (`can_borrow` in select/insert/update), `queries/assets.sql` (`SetAssetStatusQuiet`); then `make generate` (commit regenerated models of other packages separately, as in US-13)

**Migration 00008 contains:** `ALTER TABLE inventory.asset_types ADD can_borrow boolean NOT NULL DEFAULT false`; statuses Reserved (`…0205`, `unavailable`, not default, system, position 5) and On loan (`…0206`, `in_use`, not default, system, position 6) — shift no existing positions, just append; `inventory.borrow_tickets` and `inventory.loans` with the columns in the spec's **Data** section (`number bigint GENERATED ALWAYS AS IDENTITY UNIQUE`; `state` CHECK; `return_by >= needed_from` CHECK; FK `asset_type_id`, `asset_id`, `suggested_asset_id` to inventory tables; account columns are plain uuid like `created_by`); partial unique indexes `borrow_tickets(asset_id) WHERE state = 'approved'` and `loans(asset_id) WHERE returned_on IS NULL`; indexes for `borrow_tickets(requester_id, created_at DESC)`, `borrow_tickets(state, created_at DESC)`; permissions `inventory.borrow.request` ("Ask to borrow assets and follow own tickets") granted to roles …0001–…0004, `inventory.borrow.manage` ("Approve borrow tickets, hand over and lend assets") granted to …0001 and …0003. Down: drop in reverse.

**Interfaces — Produces:**

```go
// domain
const (
	PermBorrowRequest = "inventory.borrow.request"
	PermBorrowManage  = "inventory.borrow.manage"
	MaxTicketNote         = 500 // cũng là giới hạn lý do từ chối / huỷ
	MaxTicketRequirements = 10
)
var ReservedStatusID, OnLoanStatusID uuid.UUID // …0205, …0206
func IsBorrowingStatus(id uuid.UUID) bool
func Borrowable(t AssetType, st Status) bool // t.CanBorrow && st.Kind == KindAvailable && !archived type

type TicketState string // TicketWaiting "waiting", TicketApproved "approved", TicketRejected "rejected", TicketCancelled "cancelled", TicketHandedOver "handed_over"
type BorrowTicket struct {
	ID uuid.UUID; Number int64; RequesterID, TypeID uuid.UUID; Requirements []string
	SuggestedAssetID, AssetID *uuid.UUID; NeededFrom time.Time; ReturnBy *time.Time // chỉ ngày
	Note, Reason string; State TicketState
	DecidedBy, CancelledBy *uuid.UUID; DecidedAt, CancelledAt *time.Time
	Version int32; CreatedAt, UpdatedAt time.Time
}
type NewBorrowTicket struct { RequesterID, TypeID uuid.UUID; Requirements []string; SuggestedAssetID *uuid.UUID; NeededFrom time.Time; ReturnBy *time.Time; Note string }
type Loan struct {
	ID, AssetID, BorrowerID uuid.UUID; TicketID *uuid.UUID; TicketNumber int64 // 0: cho mượn trực tiếp
	LentBy *uuid.UUID; StartedOn time.Time; ReturnBy, ReturnedOn *time.Time; CreatedAt time.Time
}
func (l Loan) OverdueDays(today time.Time) int // 0 khi chưa quá hạn, chưa có hạn hay đã trả
type TicketFilter struct { States []TicketState; RequesterID *uuid.UUID; Q string; Sort string /* "-number" (mặc định) | "number" | "needed_from" | "-needed_from" */; Page, PageSize int }

// CheckTicketDates: needed_from không trước today; return_by (nếu có) không trước needed_from
func CheckTicketDates(neededFrom time.Time, returnBy *time.Time, today time.Time) error // ErrInvalidTicket, field needed_from | return_by
// CheckLoanReturnBy: return_by (nếu có) không trước today
func CheckLoanReturnBy(returnBy *time.Time, today time.Time) error // ErrInvalidLoan, field return_by

// errors (errs.*)
ErrBorrowingStatus  = 422 "/errors/status-set-by-borrowing" (field status_id) "Reserved and On loan are set by borrowing"
ErrAssetInBorrowing = 409 "/errors/asset-reserved-or-on-loan"
ErrNotBorrowable    = 409 "/errors/asset-not-borrowable"
ErrTicketNotFound   = 404 "/errors/borrow-ticket-not-found"
ErrTicketState      = 409 "/errors/borrow-ticket-state"
ErrTicketChanged    = 409 "/errors/borrow-ticket-changed"
ErrInvalidTicket    = 422 "/errors/invalid-borrow-ticket"
ErrInvalidLoan      = 422 "/errors/invalid-loan"

// contract (asset aggregate)
EventAssetReserved            = "inventory.asset_reserved"             // AssetReserved{AssetID, TicketID uuid.UUID; TicketNumber int64; AccountID uuid.UUID}
EventAssetReservationCancelled = "inventory.asset_reservation_cancelled" // AssetReservationCancelled{AssetID, TicketID; TicketNumber; AccountID; ByManager bool}
EventAssetLent                = "inventory.asset_lent"                 // AssetLent{AssetID, LoanID, BorrowerID uuid.UUID; TicketID *uuid.UUID; TicketNumber int64; ReturnBy string /* YYYY-MM-DD or "" */}
// contract (aggregate "borrow_ticket", AggregateBorrowTicket)
EventBorrowTicketCreated/Approved/Rejected/Cancelled/HandedOver = "inventory.borrow_ticket_<state>" // BorrowTicketChanged{TicketID uuid.UUID; Number int64; AssetID *uuid.UUID; Reason string}
```

- [ ] **Step 1: Failing tests** in `borrowing_test.go`: `TestCheckTicketDates` (today 2026-10-30: needed_from 10-29 → field `needed_from`; 10-30 ok; return_by 10-31 with needed_from 11-01 → field `return_by`; nil return_by ok), `TestCheckLoanReturnBy` (10-29 → field `return_by`, 10-30 ok, nil ok), `TestIsBorrowingStatus` (…0205, …0206 true; Available, In use false), `TestBorrowable` (CanBorrow false → false; available kind + CanBorrow → true; in_use kind → false), `TestLoanOverdueDays` (return_by 10-25, today 10-30 → 5; returned → 0; nil return_by → 0; return_by today → 0).
- [ ] **Step 2:** `go test ./internal/inventory/domain/` → FAIL (undefined).
- [ ] **Step 3:** Implement domain, errors, events; write migration and queries (tickets: insert, get, get-for-update, list+count with the filter, counts by state, update state columns; loans: insert, active by asset, list active with asset/type join ordered overdue-first then `return_by` then `started_on`, overdue-unnotified (active, `return_by < $today`, `overdue_notified_on IS NULL`, limit), mark notified (`WHERE overdue_notified_on IS NULL` returning row); `SetAssetStatusQuiet(id, status_id)` bumps `version` and `updated_at`). `make generate`.
- [ ] **Step 4:** `go test ./internal/inventory/domain/` → PASS; `CI=true go test ./internal/inventory/repository/ -run Schema` → PASS (migration applies).
- [ ] **Step 5:** Commit `feat(inventory): borrowing rules, events, tables and queries` (+ `chore(sqlc): regenerate models` if other packages changed).

### Task 4: Asset guards, `can_borrow` on types, "Borrowable now" filter

**Files:**
- Modify: `repository/asset_repository.go` (`Replace`, `Retire`), `repository/replacement_repository.go` (`RecordReplacement`, old asset), `repository/type_repository.go`, `queries/assets.sql` (list/count filter `borrowable`)
- Modify: `service/assets.go` (`status()`), `service/bulk.go`, `service/asset_types.go`, `domain/repository.go` (`AssetFilter.Borrowable bool`, type create/update inputs carry `CanBorrow`)
- Modify: `handler/openapi.yaml` (`AssetType.can_borrow` required boolean; create/update bodies optional `can_borrow`; `GET /assets` query `borrowable` boolean), `handler/asset_types.go`, `handler/assets.go`, `convert.go`; `make generate`
- Test: `repository/asset_repository_db_test.go`, `http_db_test.go`

**Rules:** under the asset lock, `IsBorrowingStatus(cur.StatusID)` makes Retire, RecordReplacement (old asset) and Replace with a different `StatusID` or `TypeID` return `ErrAssetInBorrowing`; other field edits pass. `service.status()` returns `ErrBorrowingStatus` when a hand-picked status is Reserved or On loan (create, update, bulk status). Bulk paths report those assets as failed items with that error, as they do for retired ones. `borrowable=true` filters `type.can_borrow AND status.kind = 'available'`.

- [ ] **Step 1: Failing tests:** `TestAssetGuards_BorrowingAsset` (db: set an asset's status to Reserved with `SetAssetStatusQuiet`; Retire → `ErrAssetInBorrowing`; Replace changing status → same; Replace changing type → same; Replace changing only name → ok and status still Reserved; RecordReplacement with it as old → `ErrAssetInBorrowing`); HTTP `TestBorrowingStatusesNotPickable` (POST /assets with `status_id` Reserved → 422 `/errors/status-set-by-borrowing`; bulk-status to On loan → 422); HTTP `TestTypeCanBorrow` (PUT type `can_borrow: true` → GET shows true; omitted on create → false); HTTP `TestAssetsBorrowableFilter` (borrowable type with one available and one In use asset plus a non-borrowable type's available asset → `borrowable=true` returns only the first).
- [ ] **Step 2:** `CI=true go test ./internal/inventory/... -run 'AssetGuards|BorrowingStatuses|TypeCanBorrow|BorrowableFilter'` → FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Same → PASS; `CI=true go test ./internal/inventory/...` → all ok.
- [ ] **Step 5:** Commit `feat(inventory): reserved and on-loan assets are locked; can_borrow on types`.

### Task 5: Borrowing repository

**Files:**
- Create: `repository/borrowing_repository.go`, `repository/borrowing_repository_db_test.go`
- Create: `backend/internal/inventory/job/borrowing.go` (args only, used by `EnqueueTx`)
- Modify: `domain/repository.go` (`BorrowingRepository` interface), `service/fakes_test.go`

**Interfaces — Produces:**

```go
// job
type BorrowingEmailArgs struct {
	EmailKind string     `json:"kind"` // "ticket_approved" | "ticket_rejected" | "ticket_cancelled" | "loan_overdue"
	TicketID  *uuid.UUID `json:"ticket_id,omitempty"`
	LoanID    *uuid.UUID `json:"loan_id,omitempty"`
}
func (BorrowingEmailArgs) Kind() string // "inventory.send_borrowing_email"
type OverdueScanArgs struct{} // Kind() "inventory.scan_overdue_loans"

// repository
func NewBorrowingRepository(pool *pgxpool.Pool, outbox *events.Outbox, enq jobs.Enqueuer) *BorrowingRepository
func (r *BorrowingRepository) CreateTicket(ctx context.Context, t domain.NewBorrowTicket) (domain.BorrowTicket, error)
func (r *BorrowingRepository) GetTicket(ctx context.Context, id uuid.UUID) (domain.BorrowTicket, error) // ErrTicketNotFound
func (r *BorrowingRepository) ListTickets(ctx context.Context, f domain.TicketFilter) ([]domain.BorrowTicket, int64, error)
func (r *BorrowingRepository) TicketCounts(ctx context.Context) (waiting, approved int64, err error)
func (r *BorrowingRepository) Approve(ctx context.Context, ticketID, assetID uuid.UUID, version int32) (domain.BorrowTicket, error)
func (r *BorrowingRepository) Reject(ctx context.Context, ticketID uuid.UUID, reason string, version int32) (domain.BorrowTicket, error)
func (r *BorrowingRepository) Cancel(ctx context.Context, ticketID uuid.UUID, reason string, byManager bool, version int32) (domain.BorrowTicket, error)
func (r *BorrowingRepository) HandOver(ctx context.Context, ticketID uuid.UUID, startedOn time.Time, version int32) (domain.BorrowTicket, domain.Loan, error)
func (r *BorrowingRepository) Lend(ctx context.Context, assetID, borrowerID uuid.UUID, startedOn time.Time, returnBy *time.Time, version int32) (domain.Loan, error)
func (r *BorrowingRepository) ActiveLoan(ctx context.Context, assetID uuid.UUID) (*domain.Loan, error)
func (r *BorrowingRepository) ApprovedTicketFor(ctx context.Context, assetID uuid.UUID) (*domain.BorrowTicket, error)
func (r *BorrowingRepository) GetLoan(ctx context.Context, id uuid.UUID) (domain.Loan, error)
func (r *BorrowingRepository) ListActiveLoans(ctx context.Context, page, pageSize int) ([]domain.Loan, int64, error)
// QueueOverdueEmails: mỗi khoản mượn quá hạn chưa báo: đánh dấu overdue_notified_on = today và
// EnqueueTx một BorrowingEmailArgs{loan_overdue} trong cùng transaction; trả số đã xếp
func (r *BorrowingRepository) QueueOverdueEmails(ctx context.Context, today time.Time, limit int) (int, error)
```

**Each write, one transaction:** lock ticket (`FOR UPDATE`; wrong state → `ErrTicketState`, version → `ErrTicketChanged`), then the asset (`lockAssets`; missing → `ErrAssetNotFound`). Approve: asset type = ticket type, `Borrowable(type, status)` else `ErrNotBorrowable`; asset → Reserved; ticket approved with `asset_id`, `decided_by = actorID(ctx)`; events `asset_reserved` + `borrow_ticket_approved`; enqueue `ticket_approved`. Reject (waiting only): reason; event; enqueue `ticket_rejected`. Cancel (waiting or approved): approved → asset back to default Available + `asset_reservation_cancelled`; event `borrow_ticket_cancelled`; enqueue `ticket_cancelled` only when `byManager`. HandOver (approved only): asset must still be Reserved for this ticket; insert loan (`started_on`, `return_by` from ticket, `lent_by` actor); asset → On loan; ticket handed_over; events `asset_lent` + `borrow_ticket_handed_over`. Lend: asset version, `Borrowable`, insert loan, asset → On loan, `asset_lent`. A unique-index violation maps to `ErrNotBorrowable`.

- [ ] **Step 1: Failing db tests:** `TestBorrowing_TicketLifecycle` (create → approve: asset Reserved, events `asset_reserved` 1, `asset_updated` 0, one `inventory.send_borrowing_email` job in `river_job`; hand over: loan active with `started_on`, asset On loan, `asset_lent` 1); `TestBorrowing_RejectAndCancel` (reject waiting → rejected, job queued; cancel approved by manager → asset Available, `asset_reservation_cancelled` 1, job queued; cancel by requester → no job; cancel rejected → `ErrTicketState`); `TestBorrowing_ApproveRules` (wrong type, In use asset, type switch off → `ErrNotBorrowable`; stale version → `ErrTicketChanged`); `TestBorrowing_ConcurrentApproveSameAsset` (two waiting tickets, approve both with one asset at once, 20 rounds: exactly one ok per round, loser `ErrNotBorrowable`); `TestBorrowing_ApproveVsLend` (at once: one wins, loser `ErrNotBorrowable`); `TestBorrowing_LendVsRetire` (at once: never both; if lend won, retire got `ErrAssetInBorrowing`); `TestBorrowing_SwitchOffAfterApprove` (approve, then type `can_borrow` false: hand over ok; a new lend of that type → `ErrNotBorrowable`); `TestBorrowing_QueueOverdueEmails` (loans due 10-25 and 11-05, today 10-30: returns 1 and one job; second call returns 0); `TestBorrowing_AtomicOnEnqueueFailure` (repository built with an enqueuer whose `EnqueueTx` errors: Approve returns the error; ticket still waiting, asset still Available, no event).
- [ ] **Step 2:** `CI=true go test ./internal/inventory/repository/ -run Borrowing` → FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Same → PASS (run the concurrency tests 3 times: `-count=3`).
- [ ] **Step 5:** Commit `feat(inventory): borrow tickets and loans in one transaction each`.

*(Review Focus 1 is tested in Task 6: the requester's account state comes from identity, so it is a service check.)*

### Task 6: Borrowing service and HTTP API

**Files:**
- Create: `service/borrowing.go`, `service/borrowing_test.go`; `handler/borrowing.go`
- Modify: `service/service.go` (Deps: `Borrowing domain.BorrowingRepository`, `Location *time.Location`, `Now func() time.Time`), `service/assets.go` (`AssetView.Borrowing`), `service/activity.go`, `handler/activity.go`, `handler/convert.go`, `handler/openapi.yaml`, `make generate`
- Modify: `internal/inventory/config.go` (`AppURL string env:"INVENTORY_APP_URL" envDefault:"http://localhost:3000"`, `TimeZone string env:"INVENTORY_TIME_ZONE" envDefault:"Asia/Ho_Chi_Minh"`, validated like identity's AppURL and with `time.LoadLocation`), `module.go` (Deps `Jobs jobs.Enqueuer`; wiring), `cmd/server/main.go`, `cmd/seed/main.go`, `compose.yml` + `compose.prod.yml` (`INVENTORY_APP_URL` from `APP_URL` beside `IDENTITY_APP_URL`)
- Test: `service/borrowing_test.go` (fakes), `http_db_test.go`

**Interfaces — Produces:**

```go
type TicketInput struct { TypeID uuid.UUID; Requirements []string; SuggestedAssetID *uuid.UUID; NeededFrom time.Time; ReturnBy *time.Time; Note string }
type TicketView struct { domain.BorrowTicket; Requester idcontract.Account; TypeName string; Asset *AssetRef; SuggestedAsset *AssetRef }
type AssetRef struct { ID uuid.UUID; Tag, Name string }
type LoanView struct { domain.Loan; Asset AssetRef; TypeName string; Borrower idcontract.Account; OverdueDays int }
type BorrowingInfo struct { State string /* "reserved" | "on_loan" */; Account idcontract.Account; TicketID *uuid.UUID; TicketNumber int64; Since, ReturnBy *time.Time; OverdueDays int }
// AssetView thêm: Borrowing *BorrowingInfo
type TicketCounts struct { Waiting, Approved int64 }

func (s *Service) CreateTicket(ctx, in TicketInput) (TicketView, error)          // PermBorrowRequest; type CanBorrow else 422 field asset_type_id; ResolveAttrQuery on requirements (422 fields requirements[i]); max 10; note ≤ 500; CheckTicketDates with today(); suggested asset must be of the type (422 field suggested_asset_id)
func (s *Service) GetTicket(ctx, id uuid.UUID) (TicketView, error)               // manage, or own (else ErrTicketNotFound)
func (s *Service) ListTickets(ctx, f domain.TicketFilter, mine bool) ([]TicketView, int64, error) // without manage: RequesterID forced to actor
func (s *Service) TicketCounts(ctx) (TicketCounts, error)                        // manage
func (s *Service) ApproveTicket(ctx, id, assetID uuid.UUID, version int32) (TicketView, error)  // manage
func (s *Service) RejectTicket(ctx, id uuid.UUID, reason string, version int32) (TicketView, error) // manage; reason 1..500
func (s *Service) CancelTicket(ctx, id uuid.UUID, reason string, version int32) (TicketView, error) // requester: waiting|approved; manage: approved (+ own); reason ≤ 500 optional
func (s *Service) HandOverTicket(ctx, id uuid.UUID, version int32) (TicketView, error) // manage; requester account must be Active (else 422 field requester)
func (s *Service) LendAsset(ctx, assetID, borrowerID uuid.UUID, returnBy *time.Time, version int32) (AssetView, error) // manage; borrower Active (else 422 field borrower_id); CheckLoanReturnBy
func (s *Service) ListLoans(ctx, page, pageSize int) ([]LoanView, int64, error)   // manage
func (s *Service) SearchBorrowers(ctx, q string) ([]idcontract.Account, error)    // manage, limit 10
func (s *Service) today() time.Time // ngày hiện tại theo s.loc, ở dạng UTC 00:00 (khớp cột date)
```

**HTTP (openapi, all behind the access token; permission checks in the service):** `GET /borrow-tickets` (`state` repeatable, `mine`, `q`, `sort`, `page`, `page_size`) → `BorrowTicketPage`; `POST /borrow-tickets` → 201 `BorrowTicket`; `GET /borrow-tickets/summary` → `{waiting, approved}`; `GET /borrow-tickets/{ticketID}`; `POST /borrow-tickets/{ticketID}/approve {asset_id, version}`, `/reject {reason, version}`, `/cancel {reason?, version}`, `/hand-over {version}` → `BorrowTicket`; `POST /assets/{assetID}/lend {borrower_id, return_by?, version}` → `AssetDetail`; `GET /loans` (`page`, `page_size`) → `LoanPage`; `GET /borrowers?q=` → `[Borrower {id, name, email}]`. `BorrowTicket` carries `requester {id, name, email}`, `asset_type {id, name}`, `asset`/`suggested_asset` `{id, tag, name}` nullable, `requirements` (strings), dates as `date`. `AssetDetail.borrowing` nullable `{state, account {id, name}, ticket {id, number} nullable, since, return_by, overdue_days}`. `ActivityEntry` gains `person {id, name}` nullable, `ticket_number` (0 = none), `return_by` nullable; kinds `reserved`, `reservation_cancelled`, `lent`.

**Activity mapping:** `asset_reserved` → `reserved`; `asset_reservation_cancelled` → `reservation_cancelled`; `asset_lent` → `lent`; person names resolved with the actor-name lookup already in `activity.go`.

- [ ] **Step 1: Failing tests:** service `TestService_BusinessDay` (Now = 2026-10-29T17:30Z, loc HCM: `today()` = 2026-10-30; CreateTicket with needed_from 10-30 ok, 10-29 → 422 `needed_from`); service `TestBorrowing_HandOverDisabledRequester` (fake accounts: requester inactive → 422 field `requester`, repository HandOver not called); service `TestBorrowingPermissions` (no `request` → 403 on CreateTicket; requester can't GetTicket of another → `ErrTicketNotFound`; requester ListTickets returns only own even with `mine=false`; employee can cancel own approved; employee can't approve → 403); `TestResolveActivity_BorrowingKinds` (three events → kinds and `Person`, `TicketNumber`, `ReturnBy`); HTTP `TestBorrowingOverHTTP` (as employee create ticket with requirement `ram:gte:16` → 201; bad requirement → 422 `requirements[0]`; as officer approve with an available asset → 200, `GET /assets/{id}` shows `borrowing.state = reserved` with the requester's name; hand over → `on_loan`; `GET /loans` lists it; lend another asset directly → 200; `GET /borrow-tickets/summary` counts; every endpoint 401 without token).
- [ ] **Step 2:** `CI=true go test ./internal/inventory/... -run 'BusinessDay|HandOverDisabled|BorrowingPermissions|BorrowingKinds|BorrowingOverHTTP'` → FAIL.
- [ ] **Step 3:** Implement; update `backend/docs/inventory.md` (Borrowing section: rules, statuses, permissions table, endpoints, activity kinds, config).
- [ ] **Step 4:** Same → PASS; `cd backend && make check` → clean.
- [ ] **Step 5:** Commit `feat(inventory): borrowing API: tickets, approval, hand-over, direct lending`.

### Task 7: Borrowing emails and the overdue scan

**Files:**
- Create: `backend/internal/inventory/worker/borrowing.go`, `worker/borrowing_test.go`; `service/borrowing_mail.go`, `service/borrowing_mail_test.go`; `service/templates/{layout.html, ticket_approved.*, ticket_rejected.*, ticket_cancelled.*, loan_overdue.*}` (`.txt` + `.html`)
- Modify: `module.go` (`Deps.Mail mail.Sender`; `func (m *Module) RegisterWorkers(w *river.Workers) error`; `func PeriodicJobs() []*river.PeriodicJob` — hourly `OverdueScanArgs`, `RunOnStart: true`), `cmd/worker/main.go` and `cmd/worker/config.go` (`Inventory inventory.Config`), `backend/docs/inventory.md`

**Interfaces — Produces:**

```go
func (s *Service) SendBorrowingEmail(ctx context.Context, args job.BorrowingEmailArgs) error
func (s *Service) ScanOverdueLoans(ctx context.Context) error // before 08:00 business time: nothing; else QueueOverdueEmails(today, 100) until it returns < 100
```

**Content (subjects verbatim; bodies say the same in text and HTML, with a link to `<AppURL>/borrowing/mine`):**
- approved: "Ticket #N approved: TAG is ready for pickup" — "TAG · NAME is reserved for you. Contact the inventory team to pick it up."
- rejected: "Ticket #N was not approved" — "Reason: …"
- cancelled (by a manager): "Ticket #N was cancelled" — "Reason: …" when given.
- overdue: "TAG was due back on D MMM" — "Please return TAG · NAME to the inventory team. It was due back on D MMM YYYY."
- Idempotency keys `inventory/borrowing/<kind>/<ticket or loan id>`. Skip (return nil) when stale: approved only if the ticket is still approved with the same asset; rejected/cancelled only if still in that state; overdue only if the loan is still active. Recipient = requester (tickets) or borrower (loans), skipped if the account is inactive. Permanent mail errors → `river.JobCancel`.

- [ ] **Step 1: Failing tests:** `TestSendBorrowingEmail` (fake sender: approved ticket → one message to the requester, subject "Ticket #12 approved: LAP-0042 is ready for pickup", body contains "LAP-0042 · Dell Latitude 7440" and the link); `TestSendBorrowingEmail_SkipsStale` (ticket cancelled since → no message, nil error; loan returned → none); `TestScanOverdueLoans_BusinessHours` (Now 00:30 HCM → QueueOverdueEmails not called; 08:05 → called with today); worker `TestBorrowingEmailWork` (passes args through; permanent error → cancel, as identity's worker test does).
- [ ] **Step 2:** `go test ./internal/inventory/service/ ./internal/inventory/worker/ -run 'BorrowingEmail|ScanOverdue'` → FAIL.
- [ ] **Step 3:** Implement and wire the worker.
- [ ] **Step 4:** Same → PASS; `go build ./...`; `make check`.
- [ ] **Step 5:** Commit `feat(inventory): borrowing emails and the daily overdue reminder`.

### Task 8: Frontend data layer

**Files:**
- Modify: `frontend/src/lib/api/inventory.d.ts` (via `npm run gen:api`), `lib/api/types.ts` (`BorrowTicket`, `Loan`, `Borrower`, `AssetBorrowing`, `TicketState`), `lib/auth/permissions.ts` (`BorrowRequest`, `BorrowManage`)
- Create: `frontend/src/features/borrowing/api.ts`, `borrowing.ts`, `borrowing.test.ts`
- Modify: `features/assets/activity.ts`, `activity.test.ts`, `features/assets/components/AssetActivity.vue` (icons: `pi-bookmark` reserved, `pi-times-circle` reservation_cancelled, `pi-arrow-right-arrow-left` lent — verify each exists), `features/statuses` or a shared helper for status pickers

**Interfaces — Produces:**

```ts
// borrowing.ts
export const RESERVED_STATUS_ID = '00000000-0000-7000-8000-000000000205'
export const ON_LOAN_STATUS_ID = '00000000-0000-7000-8000-000000000206'
export function pickableStatuses<T extends { id: string }>(statuses: T[]): T[] // bỏ Reserved, On loan
export const TICKET_STATE_LABEL: Record<TicketState, string> // waiting "Waiting", approved "Ready for pickup", rejected "Rejected", cancelled "Cancelled", handed_over "Handed over"
export function overdueLabel(days: number): string // 1 → "Overdue 1 day", 5 → "Overdue 5 days", 0 → ""
export function requirementLabels(reqs: string[], attrs: Attribute[]): string[] // dùng attrFilterLabel; điều kiện không còn khớp thuộc tính hiện ra nguyên văn
export function ticketFormError(f: { typeId?: string; neededFrom?: string; returnBy?: string; note: string }, today: string): string | null
// "Pick what you need." | "Pick the date you need it from." | "That date has passed." | "Return by can't be before the date you need it from." | "Keep the note under 500 characters."
// api.ts (Vue Query; mutations meta toast false like assets): useTickets(params), useTicket(id), useTicketCounts(enabled), useCreateTicket, useApproveTicket, useRejectTicket, useCancelTicket, useHandOver, useLendAsset, useLoans(params), searchBorrowers(q)
```

Activity summary wording (verbatim): reserved "reserved this asset for {person} (ticket #{n})"; reservation_cancelled "cancelled the reservation for {person} (ticket #{n})"; lent with ticket "handed this asset over to {person} (ticket #{n})"; lent without "lent this asset to {person}"; any of these with `return_by` add " · return by {D MMM}".

- [ ] **Step 1: Failing tests** for every function above (exact strings) and the four activity summaries.
- [ ] **Step 2:** `cd frontend && npx vitest run src/features/borrowing src/features/assets/activity.test.ts` → FAIL.
- [ ] **Step 3:** `npm run gen:api`; implement; use `pickableStatuses` in the asset form, quick edit and bulk status pickers.
- [ ] **Step 4:** Same → PASS; `npx vue-tsc --noEmit -p tsconfig.app.json` clean.
- [ ] **Step 5:** Commit `feat(frontend): borrowing data, labels and activity wording`.

### Task 9: Borrowing pages

**Files:**
- Create: `features/borrowing/pages/{MyTicketsPage, TicketsPage, TicketDetailPage, LoansPage}.vue`, `features/borrowing/components/{NewTicketDialog, ApproveDialog, RejectDialog}.vue`
- Modify: `app/routes.ts` (`/borrowing/mine` perm BorrowRequest, `/borrowing/tickets` and `/borrowing/loans` perm BorrowManage, `/borrowing/tickets/:id` perm BorrowRequest; icons `pi-ticket`, `pi-inbox`, `pi-clock`), `app/layouts/AppSidebar.vue` (section "Borrowing": My tickets; Tickets with the waiting count from `useTicketCounts`; On loan), `features/assets/components/AttributeFilterPopover.vue` (prop `builtins?: boolean`, default true)

**Behaviour:** tables use `useListTable`, `useRowMenu`, `RowMenus`, `PersonCell` as the Accounts page does; columns as in the spec's Screens section; quick filters Waiting / Approved / All on Tickets (default Waiting), state tag tones from `lib/tones`. Row click opens the ticket (tickets) or the asset (loans). NewTicketDialog: type `Select` of types with `can_borrow`; spec chips via `AttributeFilterPopover :builtins="false"` with the type's attributes; live count from `GET /assets` (`type_id`, `attr`, `borrowable=true`, `page_size=1`) shown as "N {type} match and are free now"; needed from (default today, `minDate` today); return by optional (`minDate` needed from); note; `ticketFormError` gates submit; field errors from the API inside the form; success toast "Ticket #N sent." and navigate to it. ApproveDialog: two list queries (with and without the requirements, `borrowable=true`), a `DataTable` with a radio column, Tag, Name, one column per requirement attribute, Status, Purchased; matches first, others dimmed with the failing value struck through; preselects the suggested asset when present; button "Approve and reserve {TAG}"; 409 reloads both lists and shows the error. RejectDialog: reason required. TicketDetailPage: `DetailHeader` "Ticket #N", fields, requirement chips, actions by state and permission (Approve, Reject, Hand over, Cancel with a confirm). Toasts: "{TAG} reserved for {name}.", "Ticket #N rejected.", "Ticket #N cancelled.", "{TAG} handed over to {name}." No Undo (spec).

- [ ] **Step 1:** Add the route test cases to `app/routes.test.ts` (each route's `perm`) → run, FAIL.
- [ ] **Step 2:** Implement pages and dialogs.
- [ ] **Step 3:** `npx vitest run src/app` → PASS; `npm run check` → clean.
- [ ] **Step 4:** Browser check against the dev stack (ask before `make up`; sign in as `employee@storeit.test` and `officer@storeit.test`, password from `.env`, never printed): send a ticket with a spec, approve it, hand over, reject another, cancel an approved one.
- [ ] **Step 5:** Commit `feat(frontend): borrowing section with tickets, approval and loans`.

### Task 10: Borrowing on assets and types

**Files:**
- Create: `features/borrowing/components/LendDialog.vue`
- Modify: `features/assets/pages/AssetDetailPage.vue` (Borrowing panel; buttons Hand over / Cancel reservation when reserved (manage), Lend… and Borrow this… when available and the type can be borrowed), `features/assets/pages/AssetsPage.vue` (row menu Lend… / Borrow this…; quick filter chip "Borrowable now" ↔ `borrowable=true` in `listQuery.ts`), `features/assets/listQuery.ts` + test, `features/asset-types/pages/TypeSettingsPage.vue` ("Can be borrowed" `ToggleSwitch`, editable with TypeManage), `frontend/README.md`

**Behaviour:** panel text "Reserved for {name} · ticket #{n}" (ticket links to its page) or "On loan to {name} since {D MMM} · return by {D MMM}", overdue in the danger tone with `overdueLabel`. LendDialog: `AutoComplete` over `searchBorrowers` (name and email per option), optional return by (`minDate` today), "Lend {TAG}"; toast "{TAG} lent to {name}.". Borrow this… opens NewTicketDialog with the type preset and `suggested_asset_id`. Retire / Replace / Edit status are hidden while Reserved or On loan.

- [ ] **Step 1: Failing tests** in `listQuery.test.ts`: `borrowable` round-trips through `parseAssetQuery`/`serializeAssetQuery` and reaches `toApiParams` as `borrowable: true`.
- [ ] **Step 2:** `npx vitest run src/features/assets/listQuery.test.ts` → FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Same → PASS; `npm run check` clean; browser check: lend directly from the list, the asset page shows "On loan to …", Retire is gone; Borrow this… opens a prefilled ticket; the type switch saves.
- [ ] **Step 5:** Commit `feat(frontend): lend and borrow from asset pages; can-be-borrowed switch`.

### Task 11: Seed data and full checks

**Files:**
- Modify: `backend/internal/seed/data.go`, `seed.go`, `seed_db_test.go`

**Seed:** drop the "Cho mượn" status (assets that used it get In use); turn `can_borrow` on for Laptop, Monitor, Máy chiếu, Điện thoại; then, acting as accounts with the right permissions: three waiting tickets by employees (one with a requirement on the Laptop type), one approved, one handed over, one rejected (reason "No spare projector this week"), and one direct loan by the officer with `return_by` = today.

- [ ] **Step 1: Failing test** in `seed_db_test.go`: after `Run`, counts by state are waiting 3, approved 1, handed_over 1, rejected 1; active loans 2; no status named "Cho mượn"; the four types have `can_borrow`.
- [ ] **Step 2:** `CI=true go test ./internal/seed/` → FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** `cd backend && make check` and `CI=true go test -p 1 ./...` → all ok; `cd frontend && npm run check` → clean.
- [ ] **Step 5:** Commit `feat(seed): borrowable types, tickets and loans in the demo data`.
