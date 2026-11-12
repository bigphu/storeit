---
title: Borrowing
description: Requests, reservations, loans, returns, loan history and the emails borrowing sends.
---

Employees borrow assets for a while and bring them back. Only assets whose **type can be borrowed** take part:
turn on "Can be borrowed" in the type's settings (this needs the borrowing permission, so the inventory team can
do it without managing types).

## The flow

1. **Request.** An employee asks for a type (`Borrowing → My requests → New request`), optionally with specs
   such as RAM ≥ 16 GB, the day they need it, an optional return date, a note, or a specific asset they saw.
   Requests are numbered `REQ-0001`, `REQ-0002`, …
2. **Approve.** The inventory team (`Borrowing → Requests`) picks a free asset that matches; the approve dialog
   lists matching assets first. The asset becomes **Reserved** and the requester gets an email that it is ready
   for pickup. A request can also be rejected with a reason, which is emailed.
3. **Hand over.** When the person collects it, the reservation becomes a **loan** and the asset is **On loan**.
4. **Return.** The team records the return date, the condition (back to Available or another status) and a note.
   The loan is kept as history.

The team can also **lend directly** from an asset's page, without a request.

Requesters can cancel their own waiting or approved requests; the team can cancel approved ones (the requester
is emailed).

## Statuses set by borrowing

**Reserved** and **On loan** are system statuses. Nobody can pick them by hand, and while an asset has one it
can't be retired, replaced, or moved to another type or status. Its other fields stay editable.

An asset can be reserved or lent only when its type can be borrowed, the type isn't archived, and the asset's
status is an *available* kind. The asset list's **Borrowable now** filter shows exactly those.

## Loans and history

`Borrowing → Loans` lists every loan. Filter it by **status**, which uses the same words as the tags on each row:

| Status | Shows |
|---|---|
| Out now | Not returned yet, overdue first |
| Overdue | Not returned, and the return date has passed (due today is not overdue; a loan with no return date is never overdue) |
| Returned | Brought back |
| Returned late | Brought back after its return date (the same day, or no return date, is not late) |
| All | Everything |

Filters combine: pick an asset **type**, a **period** (a loan counts when it was out at any point in the period), and
search by asset tag or name, or by the borrower. Each asset's page has a Loans tab, and `My loans` shows a person's own
loans with the same status, type and period filters and a search by asset.

A loan is overdue from the day after its return-by date, in the app's time zone. A loan without a return-by date
shows **No due date** and is never overdue.

## Emails

The worker sends these, in the background, and skips them when they no longer apply (the request moved on, the
loan was returned, or the account was disabled):

| When | To | Subject |
|---|---|---|
| Request approved | Requester | REQ-0004 approved: TAG is ready for pickup |
| Request rejected | Requester | REQ-0004 was not approved (with the reason) |
| Approved request cancelled by the team | Requester | REQ-0004 was cancelled |
| Loan overdue (once, from 08:00 local time) | Borrower | TAG was due back on D MMM |
| Return recorded | Borrower | TAG return recorded |

Days follow `INVENTORY_TIME_ZONE` (default `Asia/Ho_Chi_Minh`).

## Permissions

| Permission | Allows |
|---|---|
| `inventory.borrow.request` | Make and cancel your own requests, see your own loans |
| `inventory.borrow.manage` | Approve, reject, hand over, lend, record returns, see all loans, turn "Can be borrowed" on or off |

The API and every rule are in `backend/docs/inventory.md` (section Borrowing).
