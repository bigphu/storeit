# Loan history (US-12) — design

Acceptance criteria: `docs/StoreIT-Sprint3-Requirements-VI.md`, US-12 (US12-AC1..AC5).
Builds on US-10 / US-11: `inventory.loans` keeps every loan with `started_on`, `return_by`,
`returned_on`, `returned_by` and `return_note`. No new tables.

## Intent

The inventory team looks back at borrowing **by period** (what was out last month), and
sometimes by asset or person. Today they only see what is out now (On loan) and lose
sight of a loan once it is returned.

## Goals

- One **Loans** page (replaces On loan, same route `/borrowing/loans`) with:
  - **Show:** Out now (default, today's On loan) / Returned / All (AC1, AC3, AC4).
  - **Period:** Any time (default) / This month / Last month / Last 30 days / Custom range.
  - **Search:** asset tag or name, or borrower name or email.
- Each row shows asset, type, borrower, borrowed on, return by, returned on and a state tag
  (AC2):
  - **On loan**
  - **Overdue N days**
  - **Returned**
  - **Returned late**: returned after its return-by date.
- The asset page gets a **Loans** tab with that asset's loan history (managers).
- **My loans** gets the same Show switch, so borrowers see their past loans too.
- AC5: everyone's loans and the asset tab need `inventory.borrow.manage` in the API and the UI.
  Borrowers only ever see their own (`mine=true`).

## Non-goals

- Excel export of the history (later, on top of the existing export).
- Showing who recorded the return. `returned_by` is stored, not displayed.
- Any change to tickets.

## Decisions

- **Period = out at any point in it** (my call, approved in chat only as "good"):
  - A loan matches `[from, to]` when `started_on <= to` and it was not back before `from`
    (`returned_on IS NULL OR returned_on >= from`).
  - "Last month" therefore includes a September loan returned on 2 October.
  - Presets are computed from the business day (`GET /borrowing/today`):
    - This month: 1st of the month to today.
    - Last month: the full previous calendar month.
    - Last 30 days: today − 29 to today.
- **Default sort:**
  - Out now keeps overdue first (as today).
  - Returned and All are newest first (`-started_on`).
  - Sortable columns: borrowed on, return by, returned on.
- **"Returned late"** is derived (`returned_on > return_by`). It isn't stored.

## API

- `GET /loans` gains these parameters (`mine`, `sort`, `page`, `page_size` stay as they are):
  - `state`: `active` (default) | `returned` | `all`
  - `from`, `to`: dates, both optional; `from > to` → 422
  - `q`: up to 100 characters; matches asset tag or name, or the borrower's name or email
    through the identity search (up to 1000 people, as ticket search)
  - `asset_id`: managers only
  - `sort`: also accepts `returned_on` / `-returned_on`
- Each `Loan` gains `returned_on` (date | null) and `return_note`.
- `overdue_days` stays 0 for returned loans.
- Permissions:
  - `mine=true` needs `borrow.request` or `borrow.manage`.
  - Anything else, `asset_id` included, needs `borrow.manage`; without it the API returns 403.

## Left to the builder

- Layout of the period picker (Select of presets plus a range DatePicker for Custom),
  following the existing toolbars.
- Empty-state wording, for example "No loans in this period."
