# Asset replacement (US-13) — design

Date: 2026-10-29. Status: draft for review.

## Intent

> As an inventory officer, I want to record an asset replacement so that the relationship between the old and new assets remains traceable. (US-13, FR-08)

The user's need is to **trace an asset back to its origin**. From any asset you can follow "Replaces" links back through the assets it took over from, and from an old asset you can see what replaced it.

## Goals

- **Recording a replacement.** Someone who manages assets (`inventory.asset.manage`) records that asset A (old) was replaced by asset B (new), with a **date** and a **reason**. Both are required.
- **The old asset is retired automatically** as part of the same action. Its retire reason reads "Replaced by B-TAG: <reason>". (User's decision.)
- **Which assets qualify:** any asset that isn't retired can be replaced, including broken or in-repair ones. The replacement must be another asset that isn't retired.
- **Both pages show the link:**
  - the old asset: "Replaced by B-TAG";
  - the new asset: "Replaces A-TAG", one row per asset it replaced.

  Following "Replaces" walks back through an asset's origins.
- **Both Activity histories (US-16) record it:** the old asset's "replaced by B-TAG: reason", the new asset's "replaces A-TAG".
- **Undo** (user's decision):
  - The success message after recording offers **Undo**, like other StoreIt actions.
  - Later, the old asset's page offers **Undo replacement**.
  - Undoing restores the old asset to active and removes the link.
  - History is never edited, so both histories keep the "replaced" entry and add a "replacement undone" entry.

## Non-goals

- Creating the new asset inside the replace dialog. It must already exist.
- Replacing one old asset with several new ones.
- Editing a recorded replacement. Undo it and record it again.
- Replacing from the asset list, the row menu or bulk actions. Only the asset page has these actions.
- A tree or graph view of whole chains. The links on each page are enough to walk them.
- Showing replacements in Excel exports.

## Anti-goals

- A replacement whose two assets disagree. For example, the old one is not retired while the link exists, or the link points at a missing asset.
- Losing history on undo.
- A chain that loops.

## Rules

| Rule | Detail |
|---|---|
| Old asset | Exists and is not retired. It has no current replacement: an asset can be replaced only once unless that replacement is undone. |
| New asset | Exists, is not retired, and is not the old asset. One new asset may replace several old ones. |
| No loops | A retired asset can't be a replacement, and the old asset becomes retired, so no chain can loop. No extra graph check is needed. |
| Date | `replaced_on` is a calendar date that is not in the future. It defaults to today in the UI. "Today" is the user's day: the server accepts any date up to one day after its own UTC date, so a Vietnamese user (UTC+7) early in the morning isn't refused. *(My call.)* |
| Reason | 1–500 characters after trimming, the same limit as retire. |
| Retirement | The old asset goes to the default retired status, as retire does. Its retire reason is "Replaced by <new tag>: <reason>", cut to 500 characters. |
| Plain Restore on a replaced asset | Same effect as Undo replacement: restores it and removes the link. The asset page shows **Undo replacement** in place of **Restore** while a link exists. |
| Concurrency | Recording and undoing take the old asset's `version`, like retire and restore, and lock the rows involved, so two people replacing the same asset at once can't both succeed. A stale version is the usual 409. |
| Undo when the new asset has since been retired | Allowed. The link is removed and the old asset restored; the new asset's state is untouched. |

## The design

### Data

- **New table.** A new migration, **00007**, adds `inventory.asset_replacements` (the app is deployed, so earlier migrations are never edited). Columns:

| Column | Type |
|---|---|
| `old_asset_id` | uuid PK, FK → assets |
| `new_asset_id` | uuid NOT NULL, FK → assets, CHECK `<> old_asset_id` |
| `replaced_on` | date NOT NULL |
| `reason` | text NOT NULL |
| `created_by` | uuid, nullable: the system actor has no account |
| `created_at` | timestamptz NOT NULL |

  Plus an index on `new_asset_id` for the "Replaces" list.
- **Undo** deletes the row. The history lives in the events.

### Events

All new, on `contract`, appended in the same transaction as the change:

| Event | Aggregate | Payload |
|---|---|---|
| `inventory.asset_replaced` | old asset | `asset_id`, `replacement_id`, `replacement_tag`, `replaced_on`, `reason` |
| `inventory.asset_replaces` | new asset | `asset_id`, `replaced_id`, `replaced_tag`, `replaced_on`, `reason` |
| `inventory.asset_replacement_undone` | **both** assets (one event each) | `asset_id`, `other_id`, `other_tag`, `role` (`old` or `new`) |

- **No separate retire or restore events.** Recording does not also emit `asset_retired`, and undo does not emit `asset_restored`. The replacement events carry that meaning, so each history shows one entry per action.

### API

| Method | Path | Body | Returns |
|---|---|---|---|
| `POST` | `/assets/{assetID}/replacement` | `{ replacement_asset_id, replaced_on, reason, version }` | the old asset's `AssetDetail` |
| `POST` | `/assets/{assetID}/replacement/undo` | `{ version }` | the old asset's `AssetDetail` |

- **Permission:** `inventory.asset.manage` for both.
- **Errors** are domain problems with clear titles:
  - the old asset is retired or already replaced;
  - the new asset is missing or retired, or is the same asset;
  - the date is in the future;
  - the reason is empty;
  - an undo on an asset with no replacement;
  - plus the usual 404s and the version 409.
- **`AssetDetail` gets two fields:**
  - `replaced_by`: `{ id, tag, name, replaced_on, reason }` or null;
  - `replaces`: a list of `{ id, tag, name, replaced_on }`, newest first.
- **`POST /assets/{assetID}/restore`** on a replaced asset behaves like undo.

### Activity history (US-16)

New kinds:

| Kind | Shown on | Wording |
|---|---|---|
| `replaced` | Old asset | "**Name** replaced this asset with B-TAG: reason" |
| `replaces` | New asset | "**Name** recorded that this asset replaces A-TAG" |
| `replacement_undone` | Both | "**Name** undid the replacement by B-TAG" or "… of A-TAG" |

- Each kind gets an icon (PrimeIcons, checked to exist), e.g. `pi-sync` for replaced, replaces and undone.
- The service fills a new entry field, `related: { id, tag }`. The frontend links the tag to that asset's page.

### Asset page

- **Actions** (with permission):
  - an asset that isn't retired: **Replace…** next to Retire;
  - a replaced asset: **Undo replacement** in place of Restore.
- **Replace dialog** (the project's `FormDialog`):
  - **Replacement asset:** a PrimeVue `AutoComplete` that searches tag and name through `GET /assets?q=`, showing tag · name · type. It excludes this asset and retired ones; retired assets are already excluded by default.
  - **Date:** `DatePicker`, defaults to today, no future dates.
  - **Reason:** `Textarea`, required, 500 characters max.
  - **Button:** "Replace asset". Replacing retires the asset, so the dialog says so in one line: "LAP-0017 will be retired."
- **After saving:** the success message "LAP-0017 replaced by LAP-0042." with **Undo** (the existing `runAction`/undo pattern). Lists, details and the activity history refresh through the existing asset invalidation.
- **Details panel**, a "Replacement" block shown when either side exists:
  - "Replaced by **B-TAG** · name · date · reason";
  - "Replaces" followed by one line per old asset: **A-TAG** · name · date.

  Tags link to the asset pages.
- **Asset list:** no new column. Retired replaced assets are hidden by default as they are today, and their retire reason explains why.

## Constraints

- Code comments in Vietnamese; UI text in English; PrimeVue v4 and PrimeIcons only.
- Transactions, outbox appends and locking live in repositories. Services never see `pgx.Tx`.
- Regenerate code with `make generate` and `npm run gen:api`.
- Platform docs don't change; `backend/docs/inventory.md` gets the new endpoints, rules and events.

## Testing

- **Domain:** each rule as a table test.
  - Old asset retired or already replaced.
  - New asset missing, retired, or the same as the old one.
  - A future date.
  - An empty or too-long reason.
  - The retire reason cut to 500 characters.
- **Repository** (database tests):
  - Recording writes the row, retires the old asset with the composed reason, and appends both events.
  - Undo deletes the row, restores the old asset, and appends two `replacement_undone` events.
  - Restore on a replaced asset does the same as undo.
  - Two concurrent replacements of the same old asset: exactly one succeeds.
- **HTTP:**
  - Record, read `replaced_by`/`replaces` on both assets, then undo.
  - 403 without manage, the 404s, the 409 on a stale version.
  - The activity history of both assets shows the new kinds with tags.
- **Frontend:** the summary wording for the new kinds; the dialog's validation helpers (pure functions).
- **Browser walk-through:**
  - Replace a broken asset from its page and check both pages and both histories.
  - Undo from the message.
  - Record again, then undo later from the page.

## Left to the builder

- Exact component split.
- Error type names.
- The migration's index names.
- Whether `related` is one field or part of each entry's change list, as long as the activity API stays backward compatible (new optional field).
