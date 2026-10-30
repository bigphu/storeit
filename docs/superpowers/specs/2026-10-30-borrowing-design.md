# Borrowing an asset (US-10): design

Status: draft for review · 2026-10-30 · Sprint 3, story US-10 (FR-06; BR-02, BR-03, BR-05; NFR-03, NFR-04).
Acceptance criteria: `docs/StoreIT-Sprint3-Planning.md` §1.4 (US10-AC1..AC7). US-11 (return) and
US-12 (borrowing history) build on this.

## Why

People in the office borrow laptops, projectors, phones and monitors for a few days. Today
StoreIt only knows an asset is "In use"; it can't say who has it, since when, or until when,
and nothing stops the same projector being promised to two people. US-10 makes borrowing a
recorded, checked flow: an employee asks, an asset manager decides and hands it over, and the
asset can't be lent twice.

## Goals

- Employees ask for **a kind of asset**, optionally with specs, through a **borrow ticket**.
- Asset managers approve a ticket by picking a **specific asset**, which is then **reserved**;
  the loan starts when it is **handed over**.
- Asset managers can also **lend directly**, without a ticket.
- An asset can never have two active reservations or loans (AC3, AC5), and a failed request
  leaves nothing half-done (AC6).
- Every step is visible later: on the ticket, on the loan, and in the asset's activity
  history (AC7).
- The borrower is emailed when a ticket is approved or rejected, and once when a loan runs
  past its return-by date.

## Non-goals (this story)

- Returning an asset (US-11). The statuses and loan record below are built for it.
- A history view of everyone's tickets and loans (US-12). Records are kept so it can be built.
- Editing a ticket after sending it (cancel and send a new one), extending a return date,
  several assets in one ticket, booking for future dates (the "needed from" date informs the
  manager; it does not schedule anything), reservations that expire on their own (a manager
  cancels one that is never picked up).
- Lending outside the company, QR codes, mobile.

## Anti-goals

- A reserved or lent asset that can still be lent, retired, replaced or have its status
  changed by hand.
- Deleting or overwriting ticket or loan records (BR-05). They are kept forever.
- Emails to the demo `@storeit.test` accounts going out through a real mail provider.
- A manager's inbox filling with one email per ticket.

## The flow (decided with the user)

Two ways a loan starts:

1. **Borrow ticket** (anyone with *request borrowing*): pick an asset type that can be
   borrowed, optional specs, "needed from", optional "return by", optional note.
2. **Direct lend** (asset managers): on an available, borrowable asset, pick the person and an
   optional return-by date. The loan starts immediately; there is no ticket.

Ticket states:

```
waiting ──approve (pick asset)──▶ approved ──hand over──▶ handed over  (a loan now exists)
   │                                │
   ├──reject (reason)──▶ rejected    └──cancel──▶ cancelled   (frees the reserved asset)
   └──cancel──▶ cancelled
```

- **Approve**: the manager picks one available asset of the ticket's type. The asset becomes
  **Reserved**; the requester is emailed that it's ready for pickup.
- **Hand over**: confirmed by the manager (on the ticket or on the asset's page). The loan
  starts that day; the asset becomes **On loan**.
- **Reject**: managers, while waiting, with a required reason (≤ 500 characters); the requester
  is emailed.
- **Cancel**: the requester, while waiting or approved; a manager, while approved (e.g. never
  picked up). Cancelling an approved ticket puts the asset back to **Available**. *My call:*
  when a manager cancels, the requester is emailed (as for a rejection).
- *My calls:* "needed from" can't be in the past when the ticket is sent; a person may have
  several open tickets at once; the approved asset must be of the ticket's type but doesn't
  have to meet every spec.

## Which assets can be lent (decided with the user)

- Asset types get a **"Can be borrowed"** switch (`can_borrow`, on the type's Settings page).
  The migration adds it **off** for every existing type, so nothing on a live server becomes
  lendable until someone turns it on. The demo seed turns it on for Laptop, Monitor, Máy chiếu
  and Điện thoại.
- An asset can be reserved or lent only if its type can be borrowed **and** its status is of
  kind `available`. Nothing per asset: to keep one specific asset in, give it an
  `unavailable` status as today.
- *My call:* the switch follows the type's existing permission, `inventory.type.manage`
  (Administrators and Authorized Managers), like every other type setting. Turning it off later
  doesn't touch existing reservations or loans; it only stops new ones.

## Statuses (decided with the user)

Two new **system** statuses, set and cleared only by borrowing:

| Status | Kind | Set when | Cleared when |
| --- | --- | --- | --- |
| Reserved | `unavailable` | a ticket is approved | the ticket is cancelled (→ default Available) or handed over (→ On loan) |
| On loan | `in_use` | hand-over or direct lend | the asset is returned (US-11, → default Available) |

- Fixed IDs like the other system statuses (next free: `…0205` Reserved, `…0206` On loan).
  Not the default of their kinds, can't be archived, and **can't be chosen by hand**: create,
  edit, quick edit and bulk status reject them (422 on the status field), and the status
  pickers in the UI leave them out.
- While an asset is Reserved or On loan, nobody can change its status by hand, retire it
  (single or bulk), or replace it (as the old asset). Editing its other fields stays allowed.
  Bulk actions report such assets as skipped, the way they already report retired ones.
- The demo seed drops its hand-made "Cho mượn" status and uses On loan. *My call:* it also
  creates a few tickets and loans in each state (waiting, approved, handed over, rejected,
  one overdue loan), written directly with no email jobs.

## Specs on a ticket (decided with the user)

- Specs are **attribute conditions** of the chosen type, picked with the same picker as the
  asset list's attribute filters, stored in the same `<key>:<op>:<value>` form (max 10) and
  checked with `domain.ResolveAttrQuery` when the ticket is created (422 per condition, like
  the list).
- The form shows a live count of assets that match and are free now (the existing
  `GET /assets` with `type_id`, `attr`, `status_kind=available`; employees can already read
  assets). Zero matches is allowed: the manager may still find something close.
- The approve dialog lists the type's available assets in a **table whose columns are the
  requested attributes**: assets meeting every condition first, then the others dimmed with
  the failing value struck through. Built from two list queries (with and without the
  conditions); no new matching code.
- If the type's attributes change after the ticket was sent, conditions that no longer resolve
  are shown as plain text and left out of matching.
- A ticket may name one **suggested asset** ("Borrow this…" on an asset's page). The approve
  dialog preselects it if it's still available; the manager can pick another.

## Data (my calls; names are the builder's)

In the inventory module (it already owns assets, types and statuses, and must lock them):

- `inventory.borrow_tickets`: id, `number` (sequence, shown as "#12"), `requester_id`
  (account), `asset_type_id`, `requirements text[]`, `suggested_asset_id` (null),
  `needed_from date`, `return_by date` (null, ≥ needed_from), `note` (≤ 500), `state`
  (`waiting|approved|rejected|cancelled|handed_over`), `asset_id` (set on approve),
  `decided_by` / `decided_at`, `reject_reason`, `cancelled_by` / `cancelled_at`, `version`,
  `created_at`, `updated_at`. Partial unique index on `asset_id WHERE state = 'approved'`.
- `inventory.loans`: id, `asset_id`, `borrower_id`, `ticket_id` (null for a direct lend),
  `lent_by`, `started_on date`, `return_by date` (null), `returned_on date` (null; US-11),
  `overdue_notified_on date` (null), `created_at`. Partial unique index on
  `asset_id WHERE returned_on IS NULL`: the database itself refuses a second active loan (AC5).
- Dates such as `started_on` and "overdue" use the business day in the app's time zone
  (Asia/Ho_Chi_Minh), not UTC; the builder decides where that zone is configured.
- Accounts are read through `identity/contract`. The Lend dialog needs a way to find people:
  add `SearchAccounts(ctx, q, limit)` (active accounts, name or email) to `AccountReader`.
  Any active account can borrow, including managers.

## Rules that keep it consistent (AC3, AC5, AC6)

- Approve, hand over, cancel and lend each run in **one transaction**: lock the ticket and the
  asset(s) involved (`FOR UPDATE`, assets in ID order as `lockAssets` already does), check
  state, version, the type switch and the status kind, write the ticket/loan, change the
  asset status, append outbox events and `EnqueueTx` the email job. Anything failing rolls
  back all of it.
- Retire, replace, edit, quick edit, bulk retire and bulk status take the same asset lock, so
  they can't slip between a check and a reservation.
- Two managers approving different tickets with the same asset, or approving while another
  lends it directly: exactly one wins; the other gets 409 "asset not available" and the dialog
  reloads the list.
- Version checks on tickets and assets as everywhere else (409 when stale).

## Permissions

| Permission | Who | Allows |
| --- | --- | --- |
| `inventory.borrow.request` (new) | all four system roles | send tickets, see and cancel **their own** tickets |
| `inventory.borrow.manage` (new) | Administrator, Inventory Officer | see all tickets and loans, approve, reject, cancel approved, hand over, lend directly |

- The migration grants both to Administrator (rule for new modules) and as above to the
  others. Someone without `manage` asking for another person's ticket gets 404, not 403, so
  ticket numbers don't leak. Every endpoint is checked server-side (US12-AC5 needs the same).

## API (shape; paths and error names are the builder's)

- `GET /borrow-tickets` (filters: state; `mine`; search person or type; sort; paging; managers
  see all, others only their own), `GET /borrow-tickets/{id}`.
- `POST /borrow-tickets` `{asset_type_id, requirements[], needed_from, return_by?, note?, suggested_asset_id?}`.
- `POST /borrow-tickets/{id}/approve {asset_id, version}`, `…/reject {reason, version}`,
  `…/cancel {version}`, `…/hand-over {version}`.
- `POST /assets/{id}/lend {borrower_id, return_by?, version}`.
- `GET /loans` (active; overdue first).
- A small count for the sidebar badge (waiting tickets), and the account search for Lend.
- `AssetDetail` gains `borrowing`: null, or `{state: reserved|on_loan, account {id, name},
  ticket {id, number}?, since?, return_by?, overdue}`. `AssetType` gains `can_borrow`.
- Errors: not borrowable (type off or status not available) → 409; wrong ticket state → 409;
  stale version → 409; invalid fields (dates, reason, requirements) → 422 with fields.

## Activity history and events

- Asset timeline (US-16) gets new kinds, worded like the existing ones:
  "reserved this asset for **Phạm Thu Hà** (ticket #12)", "cancelled the reservation for …",
  "handed this asset over to … · return by 7 Nov", "lent this asset to … · return by 7 Nov".
  Status changes made by borrowing don't also show as plain status changes (as replacement
  already does for retire).
- Ticket events (created, approved, rejected, cancelled, handed over) on a ticket aggregate,
  kept forever for US-12.

## Email

Sent by the worker from River jobs, with idempotency keys (platform `mail` rules):

- **Ticket approved**: the asset's tag and name, ready for pickup, contact the inventory team.
- **Ticket rejected**, or an approved ticket **cancelled by a manager**: the reason.
- **Overdue**: a daily periodic job (morning, app time zone) finds active loans past
  `return_by` with no `overdue_notified_on`, emails the borrower once and sets it. Direct
  loans included. The loan shows **Overdue** in the UI until returned.
- No email to managers per ticket; they see the waiting count in the sidebar.
- **Guard (platform `mail`)**: never deliver to reserved test domains (`.test`, `.example`,
  `.invalid`, `.localhost`); log and drop instead. Dev mail goes through real Resend, and the
  demo accounts are `@storeit.test`.

## Screens (agreed with mockups)

- Sidebar section **Borrowing**: **My tickets** (request permission), **Tickets** with a waiting
  count, and **On loan** (manage permission). All three are tables built from the shared list
  table pieces: sortable columns, ☰ row menu, click a row to open it.
  - Tickets: #, requested by, type, specs, needed from, return by, status, asset, actions
    (Approve · Reject, or Hand over); quick filters Waiting / Approved / All.
  - On loan: asset, type, borrower, since, return by, status; overdue first, in red.
- **New ticket** form: borrowable types only; spec chips with the filter picker; live match
  count; needed from (default today); return by (optional); note.
- **Approve** dialog: the table described above, "Approve and reserve TAG". **Reject** dialog:
  reason required.
- **Asset page**: a Borrowing panel ("Reserved for … · ticket #12" with Hand over and Cancel
  reservation, or "On loan to … since … · return by …", Overdue in red). **Lend…** for managers
  and **Borrow this…** for requesters on an available, borrowable asset. The same two actions
  in the asset list's row menu.
- *My call:* these actions don't use the toast **Undo** that other actions have. Reservations
  and loans send email and are kept as records, so going back is an explicit Cancel, not an
  undo. Approve, reject, hand over and lend run from dialogs; Cancel asks to confirm.
- AC1: the asset list gets a "Borrowable now" quick filter (type can be borrowed and status
  available), so employees can browse what's free.

## Tests the builder must include

- Two approvals with the same asset at once; approve against direct lend; lend against retire
  and bulk status: one winner, no deadlock, no second active loan.
- A failure halfway (e.g. the email job can't be queued) leaves no ticket change, no loan and
  no status change.
- 403/404 for every endpoint by role; one employee can't read or cancel another's ticket.
- The reserved and on-loan statuses can't be picked by hand anywhere.
- The overdue job emails once per loan, not on later runs; the reserved-domain guard drops
  `@storeit.test`.
- A browser walk-through as an employee and as an inventory officer: ticket → approve → hand
  over; reject; cancel after approve; direct lend; overdue shown.

## Docs to update in the same change

`backend/docs/inventory.md` (borrowing section, statuses, permissions, endpoints, activity
kinds), `backend/docs/platform.md` (mail guard), `frontend/README.md` (new pages and dialogs).

## Left to the builder

Exact table, column, endpoint and error names; the ticket number format; email wording within
the content above; where the app time zone comes from; how the sidebar count refreshes.
