# UI interaction patterns — design

Date: 2026-10-07. Status: approved in conversation. The proposal artifact "StoreIt Interaction Patterns" (version 17) shows every pattern live.

## Goal

Every screen should handle three things the same way:
- asking before an action;
- reporting the result;
- letting the user take it back.

Concretely:
- Most actions that change data happen at once and offer Undo, instead of asking first.
- The few confirmations, the form dialogs, popovers, tables, tags, colours, fonts and scrollbars follow one design.
- On the server, deleting roles and export profiles becomes a soft delete, and attributes, options, roles and profiles each get a restore endpoint, so every delete can be undone.

Out of scope:
- a "recently deleted" or trash view;
- purging soft-deleted rows;
- undoing actions after their message has closed.

## Principles

1. **Act now, offer Undo.** If the server can reverse an action, it happens at once and its message offers Undo for 8 seconds. No question is asked first.
2. **Ask only before the irreversible:** signing someone out, sending an email, discarding unsaved work.
3. **Name the action.** Titles and buttons use a verb and an object ("Sign Lê Văn Hùng out everywhere?", "Send link"), never "Confirm" or "OK".
4. **Red means delete or discard.** Archive, disable and retire are never red.
5. **No coloured edges.** No component shows state with a border or stripe on one side. Thin grey dividers between areas stay.
6. **One voice for feedback.** Success reads "*Thing* *past verb*." ("Laptop archived.", "3 assets retired.").

## Colour

One palette, defined in `frontend/src/app/base.css` (`--app-*` variables). The PrimeVue Aura preset in `theme.ts` maps to it.

Each role has four shades:

| Role | bright | strong | ink (light / dark) | soft |
|---|---|---|---|---|
| brand (also success) | #10b981 | #059669 | #047857 / #34d399 | brand 14% (dark 20%) |
| info | #0ea5e9 | #0284c7 | #0369a1 / #38bdf8 | info 14% / 20% |
| warn | #f97316 | #ea580c | #c2410c / #fb923c | warn 14% / 20% |
| danger | #ef4444 | #dc2626 | #b91c1c / #f87171 | danger 12% / 20% |
| neutral | #94a3b8 | #64748b | #475569 / #cbd5e1 | neutral 12% / 20% |

Each shade has one job:
- **Bright:** icons, the confirmation bubble, progress, the scrollbar, focus outlines.
- **Strong:** surfaces with white text (tags, filled buttons) and coloured titles.
- **Ink:** coloured text on light backgrounds.
- **Soft:** message boxes and selection.

Bright and strong are the same in both themes. Row hover (brand 8%), selection (brand soft) and the scrollbar thumb (brand 38%, 70% on hover) are mixed from brand.

The primary colour stays Aura's emerald; brand is the same hue.

## Fonts

Bundled through npm, so the app never loads them from Google:
- **Be Vietnam Pro** (`@fontsource/be-vietnam-pro`): text, weights 400/500/600/700.
- **Bricolage Grotesque** (`@fontsource-variable/bricolage-grotesque`): page and dialog titles.
- **JetBrains Mono** (`@fontsource/jetbrains-mono`): codes, keys and tags.

They plug into the existing `--app-body`, `--app-display` and `--app-mono` variables.

## Scrollbars

Global CSS in `base.css`:
- 6px wide, no track, rounded thumb in faded brand.
- Chrome, Edge and Safari use `::-webkit-scrollbar`. The standard `scrollbar-width: thin` and `scrollbar-color` apply only where `::-webkit-scrollbar` is unsupported (Firefox), because Chrome ignores the WebKit styles when the standard properties are set.
- Panes that may start or stop scrolling (dialog bodies, the report dialog panes) use `scrollbar-gutter: stable`.

## Tags

PrimeVue Tag is themed solid: background in the role's strong shade, white bold text, pill shape.

| Meaning | Role | Examples |
|---|---|---|
| Good, usable | brand | Active, Available, default status |
| In use, shared | info | In use, Shared, Invited |
| Needs attention | warn | Unsaved, Not available, expiring invite |
| Out of service | danger | Retired, Disabled |
| Inactive | neutral | Archived, Only you, System |

## Toasts

`App.vue` renders PrimeVue Toast with a custom message slot, as today.

**Layout and timing:**
- Position: bottom-right, at most 3 visible, newest at the bottom.
- Light card, full 1px border, soft shadow.
- A solid round icon in the role's strong shade: a tick for success, "!" for errors.
- Success lasts 4s. With Undo it lasts 8s, and hovering pauses it.

**Undo toasts:**
- The card's border is the timer: an SVG outline in brand bright runs around the card and shortens as time passes.
- The Undo button is filled brand strong.
- Ctrl/⌘ Z triggers the newest visible Undo, except while typing in an input.

**Error toasts:**
- They never close on their own.
- They show a one-line summary. The explanation opens on hover or keyboard focus, and is always shown on touch screens (`hover: none`).
- Retry, a button filled danger strong, appears when the caller passes a retry function.

`notify` is extended:
- `notify.success(msg, { undo?, action? })`
- `notify.error(summary, { detail?, retry? })`

## `runAction`

New `frontend/src/lib/actions.ts`:

```ts
runAction({
  run: () => Promise<unknown>,    // the mutation
  done: string,                   // "Laptop archived."
  undo?: () => Promise<unknown>,  // reverse; absent = no Undo button
  undone?: string,                // "Laptop restored."
}): Promise<boolean>              // false when run failed (error already shown)
```

- It runs `run` and shows `done` with Undo when `undo` is given.
- Undo runs `undo`, then shows `undone`.
- If `undo` fails, the error toast says what is true now ("Couldn't restore Laptop. It is still archived.").
- Mutations used through it pass `meta: { toast: false }`, so errors aren't reported twice.
- Bulk actions pass one `run` and `undo` that cover the whole batch, and produce one message.

## Confirmations

New `frontend/src/lib/confirm.ts` exports `confirmAction({ title, body, impact?, action, danger, icon }): Promise<boolean>`. It is a wrapper over PrimeVue's ConfirmDialog with a custom template in `App.vue`.

**Layout**, everything centred:
- a white card, 26rem;
- a 4.8rem round bubble half above the top edge, filled in the tone's bright shade, with a 5px lighter ring and a soft glow, holding a filled icon in white;
- the title in the tone's strong shade;
- the body, then an optional impact list on a soft grey panel;
- buttons: Cancel (outlined) and the action (filled in tone strong), equal width.

**Tones and behaviour:**
- **danger** (red) is used for discarding unsaved work.
- **info** (blue) is used for actions that can't be undone but destroy nothing.
- Focus starts on Cancel for danger, otherwise on the action. Esc cancels.

## Form dialogs

New `frontend/src/components/FormDialog.vue`, wrapping PrimeVue Dialog:

```vue
<FormDialog v-model:visible size="s|m|l" icon="tag" title="Add status" action="Add status"
            :busy :error :dirty @submit>…fields…</FormDialog>
```

**Sizes:** S 26rem, M 34rem, L 60rem.

**Header:** a band in the soft surface shade holding:
- a filled icon in brand bright (the same subject icon as the sidebar);
- the title in Bricolage Grotesque, extra-bold, about 1.5rem;
- a round white close button with a shadow.

There is no divider line; the shade separates the header from the body.

**Body and footer:**
- The body shows `error` in an error message at the top; field errors appear under their fields.
- The footer has an optional hint slot on the left, then Cancel (text) and the action (primary, with a spinner while `busy`).

**Keyboard and closing:**
- Enter submits.
- Esc or ✕ with `dirty` asks "Discard changes?" through `confirmAction` (danger, alert icon).

Every existing dialog moves to it:
- Retire, Bulk action, Invite account
- Add/Edit attribute, Options (L), New asset type
- New role, Edit status
- Data export (L), Report (L)

## Filled icons

New `frontend/src/components/AppIcon.vue`: a small set of filled SVGs used by dialog headers, the confirmation bubble and toasts. The set:

- tag, trash, mail, logout, alert, tick
- user-plus, sitemap, shield, file, sliders, box

Row actions and menus keep PrimeIcons.

## Popovers

Popovers are used for one or two fields that belong to the thing clicked: Save… in the report dialog, rename, attribute filters.
- A short lead line explains what it does.
- Footer: Cancel, then the action. Enter submits.
- Esc or clicking outside closes it.
- Errors show inside it, and it stays open.

## Tables

The current behaviour stays:
- row click and Ctrl-click to a new tab;
- hover tint;
- icon row actions (open or edit, then the main action, then danger last) with tooltips and reasons;
- the right-click menu with the same items;
- the selection bar;
- the paginator layout.

New:
- `frontend/src/components/EmptyState.vue` (icon, sentence, optional action) on every list;
- a loading-rows slot (three skeleton rows) while the first page loads.

## Action map

The table below covers every action that changes data. **Undo** means the action runs at once and offers Undo. **Confirm** means `confirmAction` first, then a toast with no Undo.

| Action | Treatment | Undo via |
|---|---|---|
| Retire asset (dialog keeps the reason field) | Undo | restoreAsset |
| Restore asset | Undo | retireAsset with the previous reason |
| Retire selected | Undo | restore each |
| Change status of selected | Undo | setAssetsStatus back, grouped by previous status |
| Save asset changes | Undo | updateAsset with the previous fields and the new version |
| Archive asset type | Undo | restoreAssetType |
| Restore asset type | Undo | archiveAssetType |
| Reorder attributes / options / statuses | Undo | reorder to the previous order |
| Remove attribute | Undo | **new** restoreAttribute |
| Remove option | Undo | **new** restoreOption |
| Archive status | Undo | restoreStatus |
| Restore status | Undo | archiveStatus |
| Make status default | Undo | make the previous default the default |
| Disable account | Undo | enableAccount |
| Enable account | Undo | disableAccount |
| Change account roles | Undo | assignRoles with the previous roles |
| Remove person from role / add person to role | Undo | assignRoles back |
| Save role permissions | Undo | updateRolePermissions with the previous set |
| Delete role | Undo | **new** restoreRole |
| Delete export profile | Undo | **new** restoreExportProfile |
| Share or unshare a profile, rename a profile, save a profile layout | Undo | updateExportProfile back (new version) |
| Close a tab | Undo | reopen at its position with its path |
| Sign out everywhere | Confirm (info, logout icon) | |
| Resend invitation, send password reset | Confirm (info, mail icon) | |
| Invite account | the dialog is the confirmation | |
| Close a tab with unsaved work | Confirm (danger, alert icon) | |

## Backend

### Attributes and options

Both are already soft-deleted (`removed_at`).

**Restore attribute:** `POST /asset-types/{typeID}/attributes/{attributeID}/restore`
- Clears `removed_at`.
- Fails with 409 `/errors/attribute-key-taken` if an active attribute of the type now uses the same key.
- Writes the event `inventory.attribute_restored`.

**Restore option:** `POST /asset-types/{typeID}/attributes/{attributeID}/options/{optionID}/restore`
- Fails with 409 `/errors/option-label-taken` if an active option of that attribute uses the same label (case-insensitive).
- Writes `inventory.option_restored`.

**Shared rules:**
- Both need `inventory.type.manage`.
- Both return the restored item.
- Restoring something that isn't removed is a no-op that returns it.

### Roles

Migration: `identity.roles.deleted_at timestamptz NULL`. The unique name index becomes partial (`WHERE deleted_at IS NULL`).

Delete:
- `DeleteRole` keeps its rules: system roles can't be deleted, and a role with assignments returns `ErrRoleInUse`.
- It now sets `deleted_at` instead of deleting the row. Permissions are kept.

Deleted roles are excluded from:
- list, get (404), member counts;
- `AssignRoles` and `CreateAccount` (an unknown role is rejected as now);
- the permission matrix.

Restore: `POST /roles/{roleID}/restore`
- Needs `identity.role.manage`.
- 409 `/errors/role-name-taken` if an active role now has the name.
- Writes the event `identity.role_restored`.

### Export profiles

Migration: `inventory.export_profiles.deleted_at timestamptz NULL`. The unique `(owner_id, lower(name))` index becomes partial.

Delete:
- Delete sets `deleted_at`.
- List, get and export by `profile_id` treat deleted profiles as not found.

Restore: `POST /export-profiles/{profileID}/restore`
- Only the same people who could delete it may restore it: the owner, or someone with `inventory.export_profile.manage` for shared profiles.
- 409 `/errors/export-profile-name-taken` if the owner now has an active profile with that name.
- Writes `inventory.export_profile_restored`.

## Testing

**Backend:**
- repository DB tests for soft delete and restore of each of the four entities (row kept, hidden from lists, restored);
- the name, key or label conflict on restore;
- the partial unique indexes (a new item with a deleted item's name is allowed);
- deleted roles can't be assigned;
- HTTP tests for each restore endpoint, including permission checks.

**Frontend (Vitest):**
- `runAction`: success with Undo, Undo failure message, run failure;
- `confirmAction` resolves true or false;
- `notify` options;
- the action-map helpers for bulk undo (grouping by previous status).

`npm run check` and `make check` pass.

**Browser:** check each Undo flow in the dev app with the seeded data.
