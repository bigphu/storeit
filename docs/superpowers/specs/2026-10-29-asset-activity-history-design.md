# Asset activity history (US-16) — design

Date: 2026-10-29. Status: draft for review.

## Intent

> As an authorized user, I want to view an asset's activity history so that I can understand the important changes that have occurred throughout its lifecycle. (US-16, FR-10)

The main question people bring to it: "who changed this asset, when, and what did they change?" An asset's history is a log of **every activity done on it**.

## Goals

- Every asset page has a working **Activity** tab. It is a timeline of everything that happened to the asset, newest first.
- Each entry shows:
  - when it happened;
  - who did it;
  - what happened (created, edited, retired, restored);
  - for edits, each field that changed, with its old and new value written readably ("Status: Available → In use").
- The history covers the asset's whole past, not just changes made after this ships. StoreIt has recorded these events since its first release.
- Anyone who can view the asset (`inventory.asset.read`) can see its history. (User's decision.)
- Borrowing, returns and replacement (US-10 to US-13) will add their own entries to the same timeline later, without changing this design.

## Non-goals

- A company-wide activity page across all assets.
- Filtering or searching the history.
- Exporting the history.
- History for asset types, statuses or export profiles.

## Anti-goals (a failure even if it works)

- History that can be edited or deleted through the app.
- Rewriting or back-filling stored events.
- An entry that shows raw IDs where the user expects a name (status, type, choice attribute value).
- Leaking anything the viewer can't already see on the asset page.

## What already exists (the source of the history)

- **Events are kept for good.** `platform.events` keeps every event permanently. Each row has `type`, `aggregate_type`, `aggregate_id`, `actor_id` (NULL for the system), `occurred_at` and `payload`. Nothing prunes this table.
- **Asset events.** Those with `aggregate_type = 'asset'` are written in the same transaction as the change, so the log can't miss a committed change:

| Event type | Payload | Meaning |
|---|---|---|
| `inventory.asset_created` | `asset_id`, `tag`, `name`, `asset_type_id`, `status_id` | Asset created |
| `inventory.asset_updated` | `asset_id`, `changes: [{field, from, to}]` | Asset edited (full update, quick edit, bulk status change) |
| `inventory.asset_retired` | `asset_id`, `reason` | Retired (single or bulk) |
| `inventory.asset_restored` | `asset_id` | Restored |

- **What a change's `field` can be:** `name`, `description`, `asset_type_id`, `status_id`, `location_id`, `holder_member_id`, `purchase_date`, or `attributes.<key>`.
- **How values are stored:**
  - IDs (type, status, location, holder) are UUID strings.
  - Attribute values are strings: text as is; numbers canonical (`"16"`); dates `YYYY-MM-DD`; `"true"` / `"false"`; choice options as their option ID.
  - An empty value is `null` or `""`.
- **Bulk actions are covered.** They go through the per-asset path, so each asset gets its own event.

## The design

### API

`GET /api/v1/assets/{assetID}/activity?before=<cursor>&limit=<n>`

- **Permission:** `inventory.asset.read`. That covers retired assets too, because their page is viewable.
- **Unknown asset:** `404`, the same problem type as `GET /assets/{assetID}`.
- **Order and paging:** newest first, ordered by `(occurred_at, id)` descending. `limit` defaults to 50, max 100. `before` is an opaque cursor taken from the previous page's `next`. The response has `next` only when more entries exist.
- **Response:**

```json
{
  "items": [
    {
      "id": "…",
      "kind": "updated",
      "occurred_at": "2026-10-28T09:14:03Z",
      "actor": { "id": "…", "name": "Lê Văn Hùng" },
      "changes": [
        { "field": "status_id", "label": "Status", "from": "Available", "to": "In use" },
        { "field": "attributes.ram_gb", "label": "RAM", "from": "8 GB", "to": "16 GB" }
      ]
    },
    { "id": "…", "kind": "retired", "occurred_at": "…", "actor": null, "reason": "Broken screen" },
    { "id": "…", "kind": "created", "occurred_at": "…", "actor": { "id": "…", "name": "…" } }
  ],
  "next": "…"
}
```

- **`kind`:**

| Event type | `kind` |
|---|---|
| `inventory.asset_created` | `created` |
| `inventory.asset_updated` | `updated` |
| `inventory.asset_retired` | `retired` |
| `inventory.asset_restored` | `restored` |
| any other type on this asset | `other`, with the raw `type` |

  The `other` kind means a future event type shows up as a generic entry until it gets its own wording.
- **`actor`:**
  - `null` means the system (`actor_id` NULL).
  - A deleted or unknown account comes back as `{ "id": "…", "name": "Unknown user" }`.
  - Names come from `identity/contract.GetAccounts`, fetched in one call per page.
- **Values are resolved on the server, so the browser never sees raw IDs:**

| Field | Shown as |
|---|---|
| `status_id`, `asset_type_id` | The status or type name. Archived ones still resolve. An ID that no longer exists shows as "(deleted)". |
| `attributes.<key>` | `label` is the attribute's label, looked up in the asset's *current* type first, then in any type that has that key. Removed attributes and options still resolve. The value is formatted by data type: number + unit, date as is, booleans "Yes"/"No", choice options by their label. |
| `location_id`, `holder_member_id` | There is no directory for these yet. The label is "Location" / "Holder", and the entry says "set" (from empty), "changed" (one value to another) or "cleared" (to empty), with no from/to values. |
| `purchase_date`, `name`, `description` | As stored. Long text is not shortened by the API. |

  An empty `from` or `to` is `null`; the UI shows "—".
- **`reason`:** only on `retired`, and only when one was given.

### Where the code lives

My calls, which a builder may adjust within the platform rules (`backend/docs/platform.md`):

- **Reading events.** `platform/events` gets a read function: `events.ListForAggregate(ctx, db, aggregateType string, id uuid.UUID, before *Cursor, limit int)`. It returns events newest first. It reads only and stays generic, so other modules can reuse it.
- **Building the entries.** The inventory service builds the entries:
  - it checks the asset exists;
  - it decodes payloads with `events.Decode`;
  - it resolves names through the existing type and status repositories (including archived and removed items) and `identity/contract`.
- **API.** The endpoint goes in `inventory/handler/openapi.yaml` and is regenerated (`make generate`, then `npm run gen:api`).
- **Database.** An index on `platform.events (aggregate_type, aggregate_id, occurred_at DESC, id DESC)`, so the timeline stays fast as the table grows. Production has been deployed since v1.0.0, so this is a **new** migration file; earlier migrations are never edited again.

### Asset page: the Activity tab

- **Turning the tab on.** The `Activity` tab is enabled, and the "later" tag is removed. Data loads the first time the tab opens, not with the page.
- **Each entry, newest first, shows:**
  - an icon per kind (PrimeIcons only):

| Kind | Icon |
|---|---|
| created | `pi-plus-circle` |
| updated | `pi-pencil` |
| retired | `pi-ban` |
| restored | `pi-replay` |
| other | `pi-circle` |

  - a one-line summary: "**Lê Văn Hùng** edited 2 fields", "**System** retired this asset: Broken screen", "**Trần Thị Mai** created this asset";
  - the time, as relative time ("3 hours ago") with the exact local date and time on hover;
  - for edits, the changed fields as rows: label, then `from → to`. Long text is clipped to a few lines, with a "Show more" toggle.
- **"Load more":** shown at the bottom while `next` exists. It loads 50 at a time.
- **Empty and error states:**
  - Empty is unlikely, because every asset has a "created" event. If it happens, the tab says "No activity recorded yet."
  - On a load error, the tab shows the standard error message with a Retry button.
- **After a change.** When the asset is edited, retired or restored from this page, the tab's data refreshes, so the new entry appears at the top.
- **Copy and code style.** UI text in English; code comments in Vietnamese; components from PrimeVue v4; the project's other rules apply.

## Constraints

- Events are read only. Nothing in this feature writes to `platform.events`.
- History visibility follows asset visibility exactly.
- Future event kinds (borrowed, returned, replaced) must slot in by adding a `kind` and its wording. The API shape and the tab don't change.

## Testing

- **Backend:**
  - `ListForAggregate` ordering and cursor paging, against a real database.
  - The service resolving each field kind, including an archived status, a removed attribute and option, a deleted account, and the system actor.
  - HTTP:
    - 404 for an unknown asset;
    - 403 without `asset.read`;
    - a retired asset's history is readable;
    - paging with `next`.
- **Frontend:** the pure functions that turn an entry into its summary line and change rows, and the relative time format.
- **Browser walk-through:**
  - Edit an asset's status and an attribute: two rows appear with names, not IDs.
  - Retire with a reason, then restore.
  - A bulk status change appears on each asset touched.

## Left to the builder

- Exact component split for the timeline.
- Cursor encoding.
- How the index migration is numbered.
- Where the entry-formatting helpers live, as long as they are pure and tested.
