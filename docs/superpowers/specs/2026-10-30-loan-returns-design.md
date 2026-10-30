# Loan returns (US-11) — design

Acceptance criteria: `docs/StoreIT-Sprint3-Requirements-VI.md`, US-11 (US11-AC1..AC6).
Builds on US-10 borrowing (`2026-10-30-borrowing-design.md`): `inventory.loans` already has
`returned_on`, and the On loan status (`…0206`) is set by hand-over and direct lend.

## Intent

An asset that comes back is recorded as returned, so it can be lent again (AC4), while the
loan stays on record (AC5). A return is a physical hand-back to the inventory team, so the
team records it and checks the condition at the same time.

## Goals

- Inventory officers (`inventory.borrow.manage`) record a return for any active loan (AC1, AC2).
- The loan gets its actual return date and reads as **Returned** (AC3); nothing is deleted.
- The asset leaves On loan for Available, or for another status if it came back damaged (AC4).
- The return shows in the asset's activity (AC6) and in the loan record (US-12 builds the history views).
- The borrower gets a short receipt email.
- Borrowers see what they currently hold in a read-only **My loans** list.

## Non-goals

- Borrowers marking their own returns (the record would say an asset is back before anyone has
  seen it; it could be lent to someone else while still in their bag).
- Undo. A mistaken return is fixed by lending the asset again, as in US-10.
- Partial returns, accessories, extending a loan, damage charges.
- Borrowing history views (US-12).

## Behaviour

### Recording a return

- Where: **Record return…** on the asset page when the asset is on loan (next to the on-loan
  notice, managers only), and in the row menu of the **On loan** page.
- Dialog fields:
  - **Returned on**: date, defaults to the business day (`GET /borrowing/today`). Not after
    today, not before the loan's start date.
  - **Condition**: **Fine** (→ the default Available status) or one of the manual statuses
    that are not available-kind and not retired (for example In repair, Lost). Shown as a
    status select whose first option is "Fine — back to Available". Archived statuses are left
    out. Reserved / On loan are never offered.
  - **Note**: optional, up to 500 characters.
- On success: toast "LAP-0042 returned." The asset page refreshes to its new status.
- Errors: no active loan (already returned by someone else) → 409, shown as "This loan was
  already closed." and the lists refetch. Stale asset version → 409 as elsewhere. Bad
  date / status / note → 422 with fields.

### Backend

- `POST /assets/{assetID}/return {returned_on, status_id?, note?, version}` →
  the asset (same shape as lend). `inventory.borrow.manage`.
  - `status_id` omitted means the default available status.
  - Allowed `status_id`: not archived, kind not `retired`, not Reserved / On loan.
- One transaction in the repository: lock the asset (must be On loan, version matches), lock
  its active loan, set `returned_on`, `returned_by` (actor), `return_note`, set the asset
  status, append event `inventory.asset_returned` on the asset aggregate, and enqueue the
  `loan_returned` email with `EnqueueTx`.
- Migration `00009_loan_returns.sql`: `loans` gains `returned_by uuid`, `return_note text`,
  and `CHECK (returned_on IS NULL OR returned_on >= started_on)`. New migration; 00008 is not
  edited.
- The overdue scan already skips loans with `returned_on`, so reminders stop by themselves.
- Event payload `AssetReturned {asset_id, loan_id, borrower_id, ticket_id?, ticket_number?,
  returned_on, status_id, note?}`.

### Activity (US-16)

- New kind `returned`: "took this asset back from **Phạm Thu Hà** (ticket #12)" or "… · now
  In repair" when the condition was not Fine. The status change it makes does not also show
  as a plain status change (same rule as lend).

### Email

- `loan_returned` to the borrower: subject "LAP-0042 return recorded", body "We recorded
  LAP-0042 · Dell Latitude 7440 as returned on 3 Nov 2026. Thanks." Idempotency key
  `inventory/borrowing/loan_returned/<loan id>`. Skipped if the loan is no longer returned
  when the job runs (cannot happen today; same stale check as other kinds).

### Ticket page

- A handed-over ticket shows its loan's state under the asset: "On loan since 1 Nov" or
  "Returned on 3 Nov". `GET /borrow-tickets/{id}` gains `loan {started_on, return_by?,
  returned_on?}` for handed-over tickets (one extra read, detail page only).

### My loans

- `GET /loans?mine=true` returns the caller's active loans; `inventory.borrow.request` is
  enough for `mine=true`, everything else still needs `borrow.manage`.
- UI: **My loans** tab next to My tickets (`/borrowing/mine/loans`), same columns as the On
  loan page minus Borrower, read-only, overdue in red.

## Left to the builder

- Exact wording of the dialog's helper lines and empty states, following existing dialogs.
- Whether "Record return…" on the On loan page is also an inline row button (match how
  Tickets shows Approve / Hand over).
