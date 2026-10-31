# Next up (handoff, updated 2026-10-31)

Short on purpose: read this first when resuming, then only the files it points to.

## Where things stand

- `main` (origin): Sprint 3 US-10 borrowing, US-11 returns (and follow-ups), US-13, US-16 and the iOS-style skin. Migrations up to `00009_loan_returns.sql`, applied on the dev DB. Branches are cleaned up.
- The US-11 click-through in the browser is done. It found one bug: the return dialog's Condition showed blank instead of "Fine — back to Available". The fix is on branch `fix/return-condition-default`.
- Dev data: DSK-0004 (Under repair) and DSK-0002 (Available) were returned during walk-throughs; LAP-0001 is still on loan to the officer. `make seed` restores the demo data.

## To do, in order

1. Land `fix/return-condition-default` (PR → checks → merge).
2. **US-12 — View borrowing history.** You said lookups are mostly **by period**. Design in progress; see below.
3. **VPS deploy** (you): migrations 00008 and 00009 run on deploy; `backup.sh` still open.

## US-12 brief (to start the design quickly)

Acceptance criteria: `docs/StoreIT-Sprint3-Requirements-VI.md`, US12-AC1..AC5. In short:
- AC1: authorized users open a borrowing history screen.
- AC2: each record shows asset, borrower, borrowed on, return by, returned on and current state.
- AC3: active loans are distinguishable from returned ones.
- AC4: records stay visible after the return.
- AC5: no access without permission, in both the UI and the API.

What already exists:
- Data: `inventory.loans` keeps every loan forever, with `asset_id`, `borrower_id`,
  `ticket_id`, `lent_by`, `started_on`, `return_by`, `returned_on`, `returned_by` and
  `return_note`. Nothing new is needed in the database for AC2–AC4.
- API: `GET /loans` lists **active loans only**. It takes `mine`, `sort`
  (`started_on|return_by`, `-` for descending), `page` and `page_size`. It needs
  `borrow.manage`, and `mine=true` needs only `borrow.request`.
  Code path: `handler/borrowing.go ListLoans` → `service/borrowing.go ListLoans` →
  `repository ListActiveLoans` → SQL `ListActiveLoans`/`CountActiveLoans` in
  `repository/queries/borrowing.sql`.
- UI: `features/borrowing/pages/LoansPage.vue` (On loan; the `mine` prop gives My loans) and
  `MyLoansPage.vue`. Use `SegmentedFilter` the way `TicketTable.vue` does, for an
  Active / Returned / All switch.

Likely shape (my guess, to confirm with you):
- Add a `state` filter to `GET /loans` (`active` by default, `returned`, `all`), plus `q`
  (asset tag or name, borrower) and `asset_id`.
- Return `returned_on`, `returned_by` and `return_note` per loan.
- Turn On loan into a "Borrowing history" page with the Active / Returned / All filter;
  returned rows show "Returned on …".
- Possibly a per-asset history panel on the asset page.
- Permission: `borrow.manage` for everyone's loans; borrowers only see their own
  (`mine`), and AC5 is tested at the HTTP level.

Questions to settle first:
1. What does the inventory team mainly look things up by: an asset, a person, or a period?
2. Should the asset page show that asset's loan history?
3. Should My loans also show a borrower's past (returned) loans?
4. Does it need an Excel export (the asset export exists in `service/export.go`)?

## Working rules (from earlier sessions)

- **Commits and pushes**
  - Branch before committing.
  - Commits end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
  - Never commit `backend/docs/tasks/`.
  - Ask before pushing, merging or restarting containers.
- **Seed data and email**
  - Sign in as seed `@storeit.test` accounts, with `SEED_PASSWORD` read from `.env` and
    never printed.
  - Don't use the admin account in `.env`.
  - Dev mail goes through real Resend, but the mail guard drops `storeit.test` addresses.
- **Code**
  - PrimeVue components and PrimeIcons only.
  - Code comments are in Vietnamese; UI text is in English.
- **Process**
  - Subagents only when asked.
  - Specs and plans go in `docs/superpowers/specs|plans/`. US-10 and US-11 are good
    templates: `2026-10-30-borrowing-design.md` and `2026-10-30-loan-returns-design.md`.
