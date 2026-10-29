# Asset Activity History (US-16) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn on the asset page's Activity tab: a newest-first timeline of everything done to the asset, built from the events StoreIt already keeps in `platform.events`.

**Architecture:**
- **Reading events:** `platform/events` gets a generic, read-only `ListForAggregate`.
- **Building entries:** the inventory service turns asset events into entries. It resolves IDs into names (statuses, types, attribute labels, option labels, account names) and drops location and holder changes.
- **Serving them:** a new endpoint, `GET /assets/{assetID}/activity`, returns them a page at a time.
- **Frontend:** an infinite query feeds a PrimeVue `Timeline` on the Activity tab. The pure formatting helpers are unit-tested.

**Tech Stack:** Go (chi, oapi-codegen strict server, pgx, sqlc, goose), Vue 3 + TypeScript + PrimeVue 4 + Vue Query, Vitest (node environment).

**Spec:** `docs/superpowers/specs/2026-10-29-asset-activity-history-design.md`

## Global Constraints

- Branch `feat/us16-activity-history`. Stage explicit paths. Never commit `backend/docs/tasks/`, or the user's unrelated working-tree changes (`frontend/.vscode`, `src/app/pages/*`, `src/lib/api/client.ts`, `src/lib/auth/*`, `src/lib/errors*`, `frontend/tsconfig*`).
- End commit messages with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Code comments in Vietnamese; UI text in English. LF endings; Python edits open files with `newline=''`.
- PrimeVue v4 components; PrimeIcons only. Check icon names exist in `frontend/node_modules/primeicons/primeicons.css`.
- Production runs migrations 00001–00005, so schema changes are a **new** migration file. Never edit an applied one.
- Changing a platform API means updating `backend/docs/platform.md` in the same change.
- Never hand-edit generated code. Regenerate with `make generate` (backend) and `npm run gen:api` (frontend).
- Nothing in this feature writes to `platform.events`.
- History visibility = `inventory.asset.read`, including retired assets.

## Review Focus

1. **An attribute from the asset's previous type** (the asset later changed type) still shows its label. The lookup is: current type first, then any type with that key. Pinned in Task 2 (`TestResolveActivity_LabelFromOtherType`).
2. **An option removed from its type** still shows its label, not its ID. Pinned in Task 2 (`TestResolveActivity_RemovedOption`).
3. **A page whose events are only location or holder changes** must still return `next`, so "Load more" keeps advancing instead of stopping early. Pinned in Task 3 (`TestAssetActivityOverHTTP`, paging case).
4. **A retired asset's history** stays readable. Pinned in Task 3.
5. **A deleted account** shows "Unknown user"; the system actor is `null`, shown as "System". Pinned in Task 2 (`TestResolveActivity_Actors`) and Task 4 (`activitySummary`).

---

### Task 1: `events.ListForAggregate` and the timeline index

**Files:**
- Create: `backend/migrations/00006_events_aggregate_index.sql`
- Modify: `backend/internal/platform/events/queries/events.sql` (the file holding `InsertEvent`)
- Create: `backend/internal/platform/events/read.go`
- Test: `backend/internal/platform/events/read_test.go` (database test, uses `dbtest`, like `outbox_test.go`)
- Modify: `backend/docs/platform.md` (the `### events` section)

**Interfaces:**
- Produces: `func ListForAggregate(ctx context.Context, q db.DBTX, aggregateType string, id uuid.UUID, before *uuid.UUID, limit int) ([]Event, error)`.
  - Returns events newest first, ordered by `(occurred_at DESC, id DESC)`.
  - `before` is the ID of the last event on the previous page; only events strictly older than that event come back.
  - `ActorID` is `uuid.Nil` for the system, matching `fromRow`.
  - `q` is the sqlc `DBTX` interface, so a pool and a transaction both work.

- [ ] **Step 1: Write the failing test** `TestListForAggregate_NewestFirstAndPaging`:
  - Insert 5 events for aggregate (`"asset"`, A), one at a time, through `Outbox.Append`, with distinct `occurred_at`. Also insert 1 event for another aggregate.
  - `ListForAggregate(…, "asset", A, nil, 3)` returns the newest 3, newest first.
  - Calling it again with `before = &page1[2].ID` returns the remaining 2.
  - Calling it with `before = &page2[1].ID` returns 0.
  - The other aggregate's event never appears.
  - An event appended with `auth.SystemActor` has `ActorID == uuid.Nil`.
- [ ] **Step 2: Run it:** `cd backend && CI=true go test -p 1 ./internal/platform/events/ -run TestListForAggregate -v`. Expected: build failure, `undefined: ListForAggregate`.
- [ ] **Step 3: Write the migration and the query.**
  - Migration `00006`, goose format:
    - Up: `CREATE INDEX events_aggregate_timeline_idx ON platform.events (aggregate_type, aggregate_id, occurred_at DESC, id DESC);`
    - Down: drops the index.
    - Include a Vietnamese header comment.
  - sqlc query `ListAggregateEvents :many`. It selects the same columns as `GetEvent`, filters by `aggregate_type` and `aggregate_id`, and takes an optional `before` (`sqlc.narg('before')::uuid`). When `before` is given, the filter is `(occurred_at, id) < (SELECT occurred_at, id FROM platform.events WHERE id = before)`. Order by `occurred_at DESC, id DESC`, `LIMIT @lim`.
  - Then run `make generate`.
- [ ] **Step 4: Implement `ListForAggregate`** in `read.go`, mapping rows the same way `fromRow` does.
- [ ] **Step 5: Run it:** the same command passes. Then `go test ./internal/platform/...` → `ok`.
- [ ] **Step 6: Document it.** Add one bullet to `docs/platform.md` `### events`: read a timeline with `events.ListForAggregate(ctx, q, aggregateType, id, before, limit)` (newest first; `before` = last ID of the previous page; read-only).
- [ ] **Step 7: Commit:** `feat(events): read an aggregate's events newest first, with a timeline index`.

---

### Task 2: Inventory service — build the activity entries

**Files:**
- Create: `backend/internal/inventory/service/activity.go`
- Test: `backend/internal/inventory/service/activity_test.go` (pure, no database)
- Modify: `backend/internal/inventory/domain/repository.go`, adding to `AssetRepository`:
  `Events(ctx context.Context, id uuid.UUID, before *uuid.UUID, limit int) ([]events.Event, error)`
- Modify: `backend/internal/inventory/repository/asset_repository.go`, implementing it with `events.ListForAggregate(ctx, r.pool, contract.AggregateAsset, id, before, limit)`. Use whatever field the repository already calls its pool.

**Interfaces:**
- Consumes: `events.ListForAggregate` and `events.Decode` (Task 1); the `contract.Asset*` payloads; `contract.FieldChange`.
- Produces:

```go
type ActivityEntry struct {
	ID         uuid.UUID
	Kind       string // "created" | "updated" | "retired" | "restored" | "other"
	Type       string // event type gốc (để hiện "other")
	OccurredAt time.Time
	Actor      *ActivityActor // nil = hệ thống
	Changes    []ActivityChange
	Reason     string // chỉ retired
}
type ActivityActor struct{ ID uuid.UUID; Name string }
type ActivityChange struct{ Field, Label string; From, To *string }

func (s *Service) AssetActivity(ctx context.Context, assetID uuid.UUID, before *uuid.UUID, limit int) (entries []ActivityEntry, next *uuid.UUID, err error)

// thuần, để test: dựng entry từ event với dữ liệu tra cứu đã nạp
func resolveActivity(evts []events.Event, lk activityLookups) []ActivityEntry
type activityLookups struct {
	statuses    map[uuid.UUID]domain.Status
	types       []domain.AssetType // mọi loại, kể cả đã lưu trữ; thuộc tính và option đã gỡ vẫn có
	currentType uuid.UUID          // loại hiện tại của tài sản
	accounts    map[uuid.UUID]idcontract.Account
}
```

**Rules `resolveActivity` must follow (from the spec):**
- **Kind by event type:**

| Event type | `Kind` |
|---|---|
| `inventory.asset_created` | `created` |
| `inventory.asset_updated` | `updated` |
| `inventory.asset_retired` | `retired`, plus `Reason` from the payload |
| `inventory.asset_restored` | `restored` |
| anything else | `other`, keeping `Type` |

- **Actor:**
  - `uuid.Nil` gives `nil`.
  - An ID missing from `accounts` gives `{ID, "Unknown user"}`.
- **Dropped fields:** changes to `location_id` and `holder_member_id` are left out. An `updated` entry left with no changes is left out of the result entirely.
- **Labels:**

| Field | Label |
|---|---|
| `name` | "Name" |
| `description` | "Description" |
| `asset_type_id` | "Type" |
| `status_id` | "Status" |
| `purchase_date` | "Purchase date" |
| `attributes.<key>` | The attribute's label: current type first, then the first type in `types` with that key; else the key itself |

- **Values:**
  - An empty or `nil` `From`/`To` becomes `nil`.
  - Status and type IDs become their names; an unknown ID becomes `"(deleted)"`.
  - Attribute values are formatted by data type:

| Data type | Shown as |
|---|---|
| number | `"<n> <unit>"` when the attribute has a unit, else `"<n>"` |
| boolean | `"Yes"` / `"No"` |
| select | The option's label (removed options included); an unknown option ID becomes `"(deleted)"` |
| date, text | As stored |

  - Other fields are shown as stored.

**`AssetActivity`:**
- Requires `domain.PermAssetRead`.
- `s.assets.Get` first, so an unknown asset returns `ErrAssetNotFound`.
- Loads `limit+1` events. If more than `limit` come back, `next` is the ID of the `limit`-th **raw** event. Raw, so that dropped entries don't stop the paging.
- Resolves names through the existing repositories: statuses `List(ctx, true)` and types `List(ctx, true)`. If `List` leaves out removed attributes, call `Get` for each type ID referenced. Check what `TypeRepository` returns.
- Fetches account names with one `GetAccounts` call per page.

- [ ] **Step 1: Write the failing tests** in `activity_test.go`. Build events with `events.New`, then set `ActorID` and `OccurredAt` by hand.
  - **`TestResolveActivity_Kinds`:**
    - created gives `created` with no changes;
    - retired with reason `"Broken screen"` gives `Reason == "Broken screen"`;
    - restored gives `restored`;
    - `"inventory.something_new"` gives `Kind == "other"` and `Type == "inventory.something_new"`.
  - **`TestResolveActivity_FieldValues`:** one `updated` event with these changes:

| Field | From | To | Expected |
|---|---|---|---|
| `status_id` | S1 ("Available") | S2 ("In use") | `{Label "Status", From "Available", To "In use"}` |
| `attributes.ram_gb` (number, unit `GB`) | `"8"` | `"16"` | `"8 GB"` → `"16 GB"` |
| `attributes.has_dock` (boolean) | `"true"` | `"false"` | `"Yes"` → `"No"` |
| `purchase_date` | `nil` | `"2026-01-10"` | From `nil`, To `"2026-01-10"` |
| `asset_type_id` | unknown ID | — | `"(deleted)"` |

  - **`TestResolveActivity_RemovedOption`:** select attribute `os` whose option `windows` has `RemovedAt` set. Changing from the `windows` ID to the `macos` ID gives `"Windows"` → `"macOS"`.
  - **`TestResolveActivity_LabelFromOtherType`:** the asset is now type T2; the change is `attributes.serial`, which exists only on T1 (label "Serial number"). Expected label: "Serial number".
  - **`TestResolveActivity_HiddenFields`:** an `updated` event with only `location_id` and `holder_member_id` is dropped. One with `location_id` plus `name` keeps only `name`.
  - **`TestResolveActivity_Actors`:**

| Actor | Expected |
|---|---|
| `uuid.Nil` | `nil` |
| A known account | `{ID, "Lê Văn Hùng"}` |
| An unknown ID | `{ID, "Unknown user"}` |

- [ ] **Step 2: Run them:** `cd backend && go test ./internal/inventory/service/ -run TestResolveActivity -v`. Expected: build failure, `undefined: resolveActivity`.
- [ ] **Step 3: Implement** `activity.go`, the repository method and the domain interface method.
- [ ] **Step 4: Run them:** they pass. Then `go build ./... && go test ./internal/inventory/...` (no Docker) → `ok`.
- [ ] **Step 5: Commit:** `feat(inventory): build an asset's activity entries with readable names`.

---

### Task 3: API endpoint and HTTP test

**Files:**
- Modify: `backend/internal/inventory/handler/openapi.yaml`
- Create: `backend/internal/inventory/handler/activity.go`
- Test: `backend/internal/inventory/http_db_test.go`, adding `TestAssetActivityOverHTTP`
- Regenerate: `handler/api/api.gen.go` (`make generate`), `frontend/src/lib/api/inventory.d.ts` (`npm run gen:api`)
- Modify: `backend/docs/inventory.md` (API table and a short "Activity" paragraph)

**Interfaces:**
- Consumes: `Service.AssetActivity` (Task 2).
- Produces:
  - `GET /assets/{assetID}/activity`, operationId `getAssetActivity`.
  - Query `before` (`ID`, optional) and `limit` (integer, 1–100, default 50 applied in Go; the validator doesn't apply defaults).
  - Response `AssetActivityPage { items: AssetActivityEntry[], next?: ID }`, where:
    - `AssetActivityEntry { id, kind (enum created|updated|retired|restored|other), type, occurred_at, actor: ActivityActor (nullable: true), changes: ActivityChange[], reason?: string }`;
    - `ActivityActor { id, name }`;
    - `ActivityChange { field, label, from (string, nullable: true), to (string, nullable: true) }`.
  - Errors as `GET /assets/{assetID}`: 404 for an unknown asset, 403 without `inventory.asset.read`.
  - Frontend types: `components['schemas']['AssetActivityEntry']` etc. in `inventory.d.ts`.

- [ ] **Step 1: Write the failing test** `TestAssetActivityOverHTTP`, using the existing `newApp`/`token`/`decode` helpers.
  - **Setup:** create a type with a number attribute `ram_gb` (unit `GB`). Create an asset with `ram_gb: 8`. Then, in order:
    - update it, changing `ram_gb` to 16 and the status to another status;
    - retire it with reason `"Broken"`;
    - restore it.
  - **Full history:** `GET /api/v1/assets/{id}/activity` returns 200 with kinds `[restored, retired, updated, created]` in that order.
    - The `updated` entry has `{label "RAM", from "8 GB", to "16 GB"}` and a `Status` change with status **names**.
    - `actor.name` is the token's account name.
  - **Paging:** `?limit=2` returns 2 items and a `next`. `?limit=2&before=<next>` returns the last 2 with no `next`.
  - **Retired asset readable:** retire again and read the activity: 200.
  - **Unknown asset:** a random UUID returns 404.
  - **No permission:** a token without `inventory.asset.read` returns 403.
  - **Paging across hidden events:** make 3 updates that only change `location_id` (`PUT` with `location_id` set to new UUIDs, if the API accepts any UUID; otherwise skip this case and record why in the commit message). Then `?limit=2`:
    - the hidden-only entries are absent;
    - `next` is still returned while older raw events exist;
    - following `next` eventually reaches `created`.
- [ ] **Step 2: Run it:** `cd backend && CI=true go test -p 1 ./internal/inventory/ -run TestAssetActivityOverHTTP -v`. Expected: FAIL, because the route doesn't exist yet.
- [ ] **Step 3: Implement.** OpenAPI path and schemas (follow the file's existing `ID` reference and `nullable: true` conventions), then `make generate`, then a handler that maps service entries to API types. If `limit` is absent, use 50.
- [ ] **Step 4: Run it:** the test passes. Then `make check` → no issues. Then `cd frontend && npm run gen:api && npx vue-tsc --noEmit -p tsconfig.app.json` → clean.
- [ ] **Step 5: Document it** in `docs/inventory.md`:
  - the endpoint and its permission;
  - newest first, with the `before`/`next` cursor;
  - `kind` values;
  - names resolved on the server;
  - location and holder changes dropped.
- [ ] **Step 6: Commit:** `feat(inventory): GET /assets/{id}/activity` (stage `frontend/src/lib/api/inventory.d.ts` only, not `client.ts`).

---

### Task 4: Frontend data and formatting helpers

**Files:**
- Create: `frontend/src/features/assets/activity.ts`
- Test: `frontend/src/features/assets/activity.test.ts`
- Modify: `frontend/src/features/assets/api.ts`

**Interfaces:**
- Consumes: the generated `AssetActivityEntry` type (Task 3).
- Produces:
  - `type ActivityEntry = components['schemas']['AssetActivityEntry']`. Re-export it from `activity.ts` through the project's `@/lib/api/types` pattern.
  - `activitySummary(e: ActivityEntry): { who: string; text: string }`
  - `relativeTime(iso: string, now: Date): string`
  - `isLong(text: string | null | undefined): boolean` (more than 160 characters or more than 3 lines)
  - `useAssetActivity(id: MaybeRefOrGetter<string>, enabled: MaybeRefOrGetter<boolean>)`: a Vue Query `useInfiniteQuery` with key `[...assetKeys.one(id), 'activity']`. This key sits under `assetKeys.all`, so the existing invalidation after edit, retire and restore refreshes it. `getNextPageParam` returns `next`; pages are 50.

- [ ] **Step 1: Write the failing tests:**
  - **`activitySummary`:**

| Entry | `who` | `text` |
|---|---|---|
| created, actor "Trần Thị Mai" | "Trần Thị Mai" | "created this asset" |
| updated with 2 changes | | "edited 2 fields" |
| updated with 1 change labelled "Status" | | "changed Status" |
| retired, reason "Broken screen", `actor: null` | "System" | "retired this asset: Broken screen" |
| retired, no reason | | "retired this asset" |
| restored | | "restored this asset" |
| other, type `"inventory.x"` | | "recorded inventory.x" |

  - **`relativeTime`**, with now = `2026-10-29T12:00:00Z`:

| Input | Expected |
|---|---|
| 20 s earlier | "just now" |
| 5 min earlier | "5 minutes ago" |
| 3 h earlier | "3 hours ago" |
| 1 day earlier | "yesterday" |
| 10 days earlier | "10 days ago" |
| 40 days earlier | `formatDate(iso)` from `@/lib/dates` |

    Use `Intl.RelativeTimeFormat('en', { numeric: 'auto' })`.
  - **`isLong`:** a 200-character string → true; 4 short lines → true; `"short"` → false; `null` → false.
- [ ] **Step 2: Run them:** `cd frontend && npx vitest run src/features/assets/activity.test.ts`. Expected: FAIL, module not found.
- [ ] **Step 3: Implement** `activity.ts`, and add `useAssetActivity` to `api.ts`.
- [ ] **Step 4: Run them:** they pass. Then `npx vue-tsc --noEmit -p tsconfig.app.json` is clean.
- [ ] **Step 5: Commit:** `feat(frontend): activity data and formatting helpers`.

---

### Task 5: Activity tab on the asset page

**Files:**
- Create: `frontend/src/features/assets/components/AssetActivity.vue`
- Modify: `frontend/src/features/assets/pages/AssetDetailPage.vue`, changing the `Tabs` block (around lines 115–120)
- Modify: `frontend/README.md` (one line on `AssetActivity`)

**Interfaces:**
- Consumes: `useAssetActivity`, `activitySummary`, `relativeTime`, `isLong` (Task 4); `formatDateTime` (`@/lib/dates`).
- Produces: `<AssetActivity :asset-id="asset.id" />`.

**What the component shows:**
- **Timeline:** a PrimeVue `Timeline` of the flattened pages.
- **Icons by kind:**

| Kind | Icon |
|---|---|
| created | `pi pi-plus-circle` |
| updated | `pi pi-pencil` |
| retired | `pi pi-ban` |
| restored | `pi pi-replay` |
| other | `pi pi-circle` |

- **Each entry:** the summary line, `who` in bold. The time shows `relativeTime(...)`, with `formatDateTime` as its `title`.
- **Changes:** for `updated`, one row per change: label, then `from → to`, with `null` shown as "—". A long value (`isLong`) is clamped to 3 lines, with a "Show more" / "Show less" text button.
- **Load more:** a "Load more" button while `hasNextPage`, showing loading while it fetches the next page.
- **States:**
  - Loading: `TableSkeleton` or a small skeleton list.
  - Empty: `EmptyState` with icon `pi pi-history` (check it exists) and the text "No activity recorded yet."
  - Error: the standard error message with a Retry button that calls `refetch`.

**Changes to the page:**
- `Tabs` gets a `ref` value (`'details' | 'activity'`).
- The Activity tab is enabled, and the `later` tag and its comment are removed.
- `<AssetActivity>` is mounted with `v-if="tab === 'activity'"`, so it loads on first open only.

- [ ] **Step 1: Build the component and the page change.** This is UI wiring; its logic was tested in Task 4.
- [ ] **Step 2: Check:** `cd frontend && npx vue-tsc --noEmit -p tsconfig.app.json && npx vitest run` → clean, all tests pass.
- [ ] **Step 3: Check in the browser** with the headless Edge script pattern on the dev stack, as the officer seed account (password from `.env`, never printed):
  - On an asset, change its status and one attribute. The Activity tab shows "edited 2 fields" with names, not IDs.
  - Retire it with a reason, then restore it. The tab shows both entries at the top without a page reload.
  - Do a bulk status change on two assets. Each asset's tab shows it.
  - An asset with more than 50 events shows "Load more", and clicking it appends older entries. If none exists, create one by script.

  Take one screenshot of the tab and look at it.
- [ ] **Step 4: Commit:** `feat(frontend): Activity tab on the asset page` (with the README line).

---

### Task 6: Full checks

- [ ] `cd backend && make check` → no issues; `CI=true go test -p 1 ./...` → all `ok`.
- [ ] `cd frontend && npm run check` → typecheck clean, tests pass, build succeeds.
