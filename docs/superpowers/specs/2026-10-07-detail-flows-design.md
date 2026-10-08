# Detail flows — design

Date: 2026-10-07. Status: approved in conversation. Two artifacts show the design:
- "StoreIt Detail Flows" (proposal, v2);
- "StoreIt Flow Demo" (clickable demo).

It builds on the UI patterns spec (`2026-10-07-ui-patterns-design.md`): `runAction`, `announce`, `confirmAction`, `FormDialog`, the toasts and the save bar all come from there.

## Goal

Asset types, statuses, accounts, roles and export profiles are opened, edited and changed the same way, with as few clicks as possible.

In scope:
- the detail page;
- editing details;
- creating an item;
- lifecycle actions (archive, restore, disable, enable, share, delete).

Also in scope: quick edit from the lists of all five areas and from the assets list.

Out of scope:
- **List layouts.** They stay as they are: type cards or table, status columns, the accounts table, role cards or compare view, the profiles table. Lists only gain quick edit and the row actions below.
- **Backend.** No changes. Every endpoint needed exists.

## Principles

1. **One detail page.** Every item opens the same page: header, tabs, Overview first.
2. **Fields are always editable.** There is no Edit button and no Edit dialog. Changes collect in a save bar.
3. **No question when Undo works.** Lifecycle actions run on the first click and offer Undo. Only sends, sign-outs and discarding unsaved work ask first.
4. **Keys finish the job.** Enter submits dialogs, Ctrl/⌘ S saves the save bar, Esc closes dialogs and menus.
5. **Ask for the minimum, prefill the rest, land where the next step is.**
6. **Rows mirror the header.** Every one-click header action is also on the list row, in the same order.
7. **Quick edits stay on the list.**
   - Hovering a row or card shows a small pencil next to its name (and an asset's status and purchase date); clicking it edits that value inline.
   - The pencil opens a side drawer with the item's Overview.
   - The detail page is for everything else.

## The detail page

### Header

`DetailHeader` is extended. In order, it shows:
- the area's filled icon (`AppIcon`) in a soft brand tile;
- the name;
- solid tags for the item's state;
- one line of facts (counts, code, email, owner);
- actions on the right.

Actions always appear in this order, left to right:
1. a related link or the main action ("View 87 assets", "Export now");
2. state changes (Make default, Archive or Restore, Disable or Enable, Share or Make private);
3. "More" (⋯), a menu for rare actions;
4. Delete, outlined in danger red, last.

### Tabs

- Overview is always first. A page with a single tab shows no tab bar.
- A tab with unsaved changes shows a dot. The app tab already shows one through `useTabDirty`.

### Overview

- Overview is a label/value grid.
- Editable fields are inputs from the start.
- A changed field gets a light warn fill (`--app-warn-soft`). A full warn-tinted border is allowed; there is no one-sided stripe.

### Locked fields

These show as plain text with a lock icon and the reason, not as disabled inputs:
- a type's code;
- a status's kind;
- an account's email;
- a built-in role's name;
- a profile's owner.

### Read only

Without the area's manage permission:
- every field shows as text;
- actions that need the permission are hidden;
- there is no save bar.

For export profiles, "manage" means `can_edit`. Someone else's shared profile shows read-only with "Duplicate to change it".

### Save bar

There is one save bar per tab (the existing `SaveBar`, extended). It shows:
- "N unsaved changes";
- Discard;
- Save, with a spinner while saving.

Behaviour:
- **Ctrl/⌘ S** saves when the bar is shown. Inside inputs too.
- **Save** runs through `runAction`. Its toast offers Undo, which writes the previous values back.
  - Areas with optimistic locking send the version the save returned (types, accounts, profiles).
  - Statuses and roles have no version.
- **Discard** clears the tab's edits at once. Its toast offers Undo, which puts the edits back as a draft. Discard never asks.
- **Switching between the page's own tabs** keeps the edits.
- **Leaving the page with unsaved edits asks "Discard changes?"** (`confirmDiscard`). This covers a link, the breadcrumb, the sidebar, or closing the app tab (already handled by `useTabDirty`).

### `useDetailDraft`

New `frontend/src/lib/detailDraft.ts`: a draft model with no Vue component code, so it can be unit-tested.
- It keeps the edits per tab: field to value for Overview, key to boolean for tick lists.
- It gives the change count per tab and whether any tab is dirty.
- It gives the changed values for a tab (what Save sends).
- `discard(tab)` returns what was removed, so Undo can restore it.
- After a save, the tab's edits are cleared against the new saved values.
- It feeds `useTabDirty`.

The existing role-permissions and account-roles drafts move onto it.

## Per area

| Area | Route | Tags | Header actions | Tabs |
|---|---|---|---|---|
| Asset type | `/types/:typeId/settings`, unchanged; the sidebar's type Settings link keeps working | Active or Archived, Built-in | View N assets · Archive or Restore (not built-in) | Overview (name, code 🔒, description) · Attributes |
| Status | **new** `/statuses/:id` | its kind, Default, Archived | View N assets · Make default · Archive or Restore (not the default) | Overview only (name, kind 🔒, its place in the column) |
| Account | `/accounts/:id` | Active, Invited or Disabled, You | Disable or Enable (not yourself) · More: Resend invitation (invited), Send reset link (active), Sign out everywhere | Overview (name, email 🔒, sign-in facts read-only) · Roles · Activity (later) |
| Role | `/roles/:id` | Custom or Built-in | Delete (custom; still blocked with a note while people hold it) | Overview (name, description; built-in name 🔒) · Permissions · People |
| Export profile | **new** `/export-profiles/:id` | Shared or Only you | Export now · Share or Make private · More: Duplicate · Delete | Overview (name, owner 🔒) · Columns · Format |

Notes:
- **Statuses** have no single-status endpoint. The page reads the status from the statuses list query, which is small and already cached.
- **Asset types:**
  - The "General" panel with its own Save becomes Overview.
  - The "Archive this type" panel goes; Archive moves to the header.
  - The Attributes table, its dialogs, reorder and remove (with Undo) are unchanged and move to their own tab.
- **Accounts:** the Profile panel's own Save goes; the name saves through the save bar.
- **Roles:** the "Edit details" dialog goes.
- **Export profiles:**
  - The Columns and Format tabs reuse the report dialog's configuration panes with the sheet preview beside them. Those panes are extracted into one `ReportEditor` component that both the dialog and the page use.
  - Duplicate creates "Name (copy)" and opens it.
  - The report dialog stays for one-off exports from an asset list, and its Save… still creates or updates a profile.
- **Undo for in-place saves:** PATCH the previous values back.
  - Name and description for types, roles and statuses; name for accounts; name for profiles.
  - Tick lists use their existing endpoints: role permissions, account roles, profile layout.

## Lists

- **Row click opens the detail page.** Statuses and export profiles gain this: today the status edit icon opens a dialog, and a profile row opens the report dialog. Ctrl-click opens a new app tab, as elsewhere.
- **Row icons and the right-click menu** offer the header's one-click actions in the same order:
  - **New:** Archive or Restore on type cards and table rows.
  - **New:** Delete on custom role cards.
  - **Kept:** status archive and restore, account disable and enable, profile share and delete.
- **The status edit icon** opens the status page with the name selected.
- **Profile rename** is the pencil next to the name; the right-click Rename… goes.
- **A pencil icon** opens the quick edit drawer (see Quick edit).

## Quick edit from lists

The commonest small edits never leave the list. There are two ways in, and the detail page stays the place for everything else.

### Inline cells (hover pencil)

**Behaviour:**
- Hovering a row or card, or moving keyboard focus into it, shows a small pencil next to each editable value. Touch screens show it all the time.
- Clicking the pencil turns the value into an input, a dropdown or a date picker. Pressing Enter or Space on the focused pencil does the same.
- Enter, or clicking away, saves through `runAction`, and the toast offers Undo. Esc cancels.
- Clicking the value itself opens the item as before (the click reaches the row or card), with no delay.

**Inline-editable cells:**

| List | Cells |
|---|---|
| Asset types (table rows, card titles) | Name |
| Statuses | Name |
| Accounts | Name |
| Roles (card titles) | Name |
| Export profiles | Name. Replaces the current right-click Rename… |
| Assets | Name, Status (dropdown of active, non-retired statuses), Purchase date (date picker) |

**Not inline:**
- retired assets;
- read-only viewers;
- built-in role names;
- profiles the viewer can't edit.

### Side drawer (pencil icon)

**What it is:**
- The row's pencil icon opens a PrimeVue Drawer on the right, so the list stays visible.
- It shows the same Overview form as the detail page: one `OverviewFields` component on the same `detailDraft` model.
- Save, or Ctrl/⌘ S, saves through `runAction` with Undo.
- "Open full page" goes to the detail page.

**Moving between rows:**
- ↑/↓, while focus is not in a field, move the drawer to the previous or next row of the current list page.
- Unsaved edits ask "Discard changes?" before moving or closing.

Lifecycle actions stay on the row icons. Accounts and export profiles also repeat them in an actions block under the drawer's fields; they run at once (asking first where the row does), not through Save.

| List | Drawer fields | Full page only |
|---|---|---|
| Asset types | Name, description (code 🔒) | Attributes |
| Statuses | Name (kind 🔒) | — |
| Accounts | Name, roles (tick list) | Sign-in facts, More actions |
| Roles | Name, description | Permissions, People |
| Export profiles | Name (owner 🔒) | Columns, Format |
| Assets | Name, status, purchase date, description, the type's attributes | Changing the type, history |

### Assets: full-record saves

An asset save replaces the whole record (`PUT /assets/{id}`) and needs its version, but list rows don't carry every field. So inline and drawer saves:
1. read the asset (`fetchAsset`);
2. apply the change to `assetBodyOf(asset)`;
3. PUT it with the asset's version.

Undo PUTs the previous body with the version the save returned.

A version conflict (`/errors/asset-changed`) shows "Someone else changed LAP-0012. Reloaded it; try again." and refreshes the row.

## Creating

| Area | Entry | Dialog | Then |
|---|---|---|---|
| Asset type | New type | Name, code (filled from the name) | Opens the type on Attributes with "Add attribute" focused |
| Status | "Add status" line in each column | none: the name is typed inline and the kind comes from the column | Appears in its column with focus kept for the next one; the toast offers Undo and "Open" |
| Account | Invite account | Name, email; Employee role preselected; other roles optional | Sends the invitation and opens the account |
| Role | New role | Name, "Start from" another role (Employee preselected) | Opens the role on Permissions |
| Export profile | New profile on the profiles page | Name | Creates it with the default layout (Tag, Name, Type, Status, Purchase date) and opens it on Columns |

All dialogs are `FormDialog`s. They open with the first field focused and its text selected, and Enter creates. The Save… popover in the report dialog keeps working as today.

## Click budget

The demo counts these tasks, from the area's list, typing and finishing keys not counted. They are the acceptance targets:

| Task | Clicks |
|---|---|
| Rename anything (role, status, type, account, profile, asset) | 0 page loads: hover, click the pencil, type, Enter |
| Change an asset's status | Hover, click the pencil, pick |
| Archive a type | 1 |
| Disable an account | 1 |
| Share a profile | 1 |
| Add a status | 1 |
| New type with its first attribute | 3 |
| New role from another role | 2 |
| Invite someone | 1 |
| Export with a profile | 2 |

## Testing

**Vitest:**
- **`detailDraft`:**
  - counts per tab;
  - changed values;
  - discard and restore;
  - clearing after save;
  - tick-list drafts.
- **Routes:** `/statuses/:id` and `/export-profiles/:id` resolve with titles and icons.
- **Undo payload helpers:** the previous values and the version for each area.
- **Facts line helpers** per area.
- **Asset quick-save helper:** a fetched asset plus a change gives the right PUT body and version; Undo gives the previous body.

**Browser:**
- walk every row of the click budget;
- check read-only, locked fields, leaving with unsaved edits, and Ctrl+S;
- check inline edits (Enter, Esc, click away) and the drawer's ↑/↓ moves.

`npm run check` passes.
