# Asset Replacement (US-13) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Record that one asset replaced another. The old asset is retired automatically, both asset pages and both activity histories show the link, and the replacement can be undone.

**Architecture:**
- **Storage:** a new table, `inventory.asset_replacements`, holds the current links, one row per old asset. Recording and undoing are single repository transactions that lock both asset rows, write or delete the link, retire or restore the old asset, and append the new events.
- **API and pages:** the asset detail view carries `replaced_by` and `replaces`. The activity history learns three new kinds.
- **Frontend:** a Replace dialog and an Undo replacement action, wired through the existing `runAction` undo pattern.

**Tech Stack:** Go (chi, oapi-codegen strict server, pgx, sqlc, goose), Vue 3 + TypeScript + PrimeVue 4 + Vue Query, Vitest (node environment).

**Spec:** `docs/superpowers/specs/2026-10-29-asset-replacement-design.md`

## Global Constraints

- Branch `feat/us13-asset-replacement`. Stage explicit paths. Never commit `backend/docs/tasks/` or the user's unrelated working-tree changes (`frontend/.vscode`, `src/app/pages/*`, `src/lib/api/client.ts`, `src/lib/auth/*`, `src/lib/errors*`, `frontend/tsconfig*`).
- End commit messages with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Code comments in Vietnamese; UI text in English. LF line endings; Python edits use `newline=''`.
- PrimeVue v4 components; PrimeIcons only, checked to exist in `node_modules/primeicons/primeicons.css`.
- New migration **00007**; never edit applied migrations.
- Transactions, outbox appends and row locks live in repositories. Services never see `pgx.Tx`. `domain` may not import `platform/events` (depguard).
- Regenerate code with `make generate` (backend) and `npm run gen:api` (frontend); never hand-edit generated code.
- **Naming:** `useReplaceAsset` already means "full update (PUT)". The new frontend names are `useRecordReplacement` and `useUndoReplacement`.
- **Rules, verbatim from the spec:**
  - Reason: 1–500 characters after trimming.
  - Retire reason: "Replaced by <new tag>: <reason>", cut to 500 characters.
  - `replaced_on`: not after (server UTC date + 1 day).
  - The old asset must not be retired.
  - The new asset must exist, must not be retired, and must not be the old asset.
  - Restore on a replaced asset = undo.

## Review Focus

1. **Two people replace the same old asset at once:** exactly one succeeds; the other gets 409 (stale version, or old asset already retired). Pinned in Task 2 (`TestReplacement_Concurrent`).
2. **Two replacements locking the same pair in opposite roles at once** (A replaced by B while B is replaced by A): no deadlock, because locks are taken in ID order. Pinned in Task 2 (`TestReplacement_OppositeOrderNoDeadlock`).
3. **Plain Restore on a replaced asset** removes the link and writes the `replacement_undone` events, not `asset_restored`. Pinned in Task 2.
4. **Undo after the new asset was retired** still works and leaves the new asset retired. Pinned in Task 3 (HTTP).
5. **The activity entry for an undone replacement** reads correctly on both sides ("by B" on the old asset, "of A" on the new one). Pinned in Task 4 (unit) and Task 5 (`activitySummary`).

---

### Task 1: Domain rules, contract events, migration and queries

**Files:**
- Create: `backend/migrations/00007_asset_replacements.sql`
- Create: `backend/internal/inventory/domain/replacement.go`, `replacement_test.go`
- Modify: `backend/internal/inventory/domain/errors.go`
- Modify: `backend/internal/inventory/contract/events.go`
- Modify: `backend/internal/inventory/repository/queries/assets.sql`, or a new `queries/replacements.sql`; then `make generate`

**Interfaces — Produces:**

```go
// domain
type ReplacementLink struct {
	AssetID    uuid.UUID // tài sản ở đầu kia của liên kết
	Tag, Name  string
	ReplacedOn time.Time // chỉ ngày
	Reason     string
}
const MaxReplacementReason = 500
var ErrInvalidReplacement = errs.Unprocessable("/errors/invalid-replacement", "Invalid replacement") // field errors: replacement_asset_id, replaced_on, reason
var ErrAssetNotReplaced   = errs.Conflict("/errors/asset-not-replaced", "Asset not replaced")
func CheckReplacedOn(day time.Time, now time.Time) error   // ErrInvalidReplacement{replaced_on} if day > now.UTC() date + 1 day
func ReplacementRetireReason(newTag, reason string) string // "Replaced by <tag>: <reason>", ≤ 500 runes

// contract
const (
	EventAssetReplaced         = "inventory.asset_replaced"          // aggregate = old asset
	EventAssetReplaces         = "inventory.asset_replaces"          // aggregate = new asset
	EventAssetReplacementUndone = "inventory.asset_replacement_undone" // aggregate = each asset
)
type AssetReplaced struct { AssetID, ReplacementID uuid.UUID; ReplacementTag, ReplacedOn, Reason string } // ReplacedOn "YYYY-MM-DD"
type AssetReplaces struct { AssetID, ReplacedID uuid.UUID; ReplacedTag, ReplacedOn, Reason string }
type AssetReplacementUndone struct { AssetID, OtherID uuid.UUID; OtherTag, Role string } // Role: "old" | "new" = vai của AssetID
```

- **JSON tags:** snake_case, like the existing payloads.
- **Migration 00007** (goose Up and Down, Vietnamese header comment):
  - table `inventory.asset_replacements` with columns `old_asset_id uuid PRIMARY KEY REFERENCES inventory.assets(id)`, `new_asset_id uuid NOT NULL REFERENCES inventory.assets(id)`, `replaced_on date NOT NULL`, `reason text NOT NULL`, `created_by uuid`, `created_at timestamptz NOT NULL DEFAULT now()`;
  - a CHECK that `old_asset_id <> new_asset_id`;
  - an index on `new_asset_id`.
- **sqlc queries:**
  - `InsertReplacement`;
  - `GetReplacementByOld :one` (joined with the new asset's tag and name);
  - `ListReplacementsByNew :many` (joined with each old asset's tag and name, ordered `replaced_on DESC, created_at DESC`);
  - `DeleteReplacement`.

- [ ] **Step 1: Write the failing tests** in `replacement_test.go`:
  - **`TestCheckReplacedOn`**, with now = `2026-10-29T20:00:00Z`:

| `day` | Expected |
|---|---|
| 2026-10-29 | ok |
| 2026-10-30 | ok (the user's day in UTC+7) |
| 2026-10-31 | `ErrInvalidReplacement` with field `replaced_on` |

  - **`TestReplacementRetireReason`:**
    - `("LAP-0042", "screen broken")` gives `"Replaced by LAP-0042: screen broken"`;
    - a 600-rune reason gives exactly 500 runes.
- [ ] **Step 2: Run them:** `cd backend && go test ./internal/inventory/domain/ -run 'TestCheckReplacedOn|TestReplacementRetireReason'`. Expected: build failure, undefined symbols.
- [ ] **Step 3: Implement** the domain file, the errors, the contract types, the migration and the queries. Run `make generate`; it must succeed.
- [ ] **Step 4: Run them:** they pass; `go build ./...` is clean.
- [ ] **Step 5: Commit:** `feat(inventory): replacement rules, events and table`.

---

### Task 2: Repository — record, undo, restore = undo, read links

**Files:**
- Create: `backend/internal/inventory/repository/replacement_repository.go`. These are methods on `*AssetRepository`, so they share its pool and outbox.
- Modify: `backend/internal/inventory/repository/asset_repository.go` (`Restore` delegates to the undo path when a link exists)
- Modify: `backend/internal/inventory/domain/repository.go` (`AssetRepository` gains the methods below; also add stubs to `service/fakes_test.go`)
- Test: `backend/internal/inventory/repository/replacement_repository_db_test.go`

**Interfaces — Produces:**

```go
// đều trong một transaction; khoá hai tài sản theo thứ tự ID (tránh deadlock)
Replace(ctx context.Context, oldID, newID uuid.UUID, replacedOn time.Time, reason string, retiredStatus uuid.UUID, version int32) (domain.Asset, error)
UndoReplacement(ctx context.Context, oldID uuid.UUID, availableStatus uuid.UUID, version int32) (domain.Asset, error)
Replacements(ctx context.Context, id uuid.UUID) (by *domain.ReplacementLink, replaces []domain.ReplacementLink, err error)
```

**Behaviour:**
- **`Replace`:**
  1. Lock the old and new asset rows (`GetAssetForUpdate`) in ascending ID order.
  2. Check the old asset: missing gives `ErrAssetNotFound`; retired gives `ErrAssetRetired`; a version mismatch gives `ErrAssetChanged`.
  3. Check the new asset: missing, retired, or the same asset gives `ErrInvalidReplacement` with field `replacement_asset_id`.
  4. Insert the link row, with `created_by` the actor from ctx, or NULL for the system.
  5. `RetireAsset` with `domain.ReplacementRetireReason(new.Tag, reason)`.
  6. Append `AssetReplaced` (old) and `AssetReplaces` (new). **Not** `AssetRetired`.
- **`UndoReplacement`:**
  1. Lock the old asset, then the new asset, in ID order.
  2. A version mismatch gives `ErrAssetChanged`. No link row gives `ErrAssetNotReplaced`.
  3. Delete the link and `RestoreAsset` to `availableStatus`.
  4. Append two `AssetReplacementUndone` events (old with `Role "old"`, new with `Role "new"`). **Not** `AssetRestored`.
  5. Works even when the new asset is retired.
- **`Restore`:** inside its transaction, if the asset has a link row, it runs the same steps as `UndoReplacement` (share an unexported helper that takes the tx's queries).

- [ ] **Step 1: Write the failing tests:**
  - **`TestReplacement_RecordAndRead`:**
    - Old O and new N are both active; `Replace` succeeds.
    - O is retired, its `RetiredReason` is `"Replaced by <N.tag>: broken"`, and its status is the default retired status.
    - `Replacements(O)` gives `by.AssetID == N`. `Replacements(N)` gives `replaces[0].AssetID == O`.
    - Exactly one `inventory.asset_replaced` event exists on O and one `inventory.asset_replaces` on N, and no `asset_retired` on O. Use the existing `countEvents` helper.
  - **`TestReplacement_Rules`:**
    - Replacing O again gives `ErrAssetRetired`.
    - A retired N gives `ErrInvalidReplacement`.
    - O replacing itself gives `ErrInvalidReplacement`.
    - An unknown N gives `ErrInvalidReplacement`.
    - A stale version gives `ErrAssetChanged`.
  - **`TestReplacement_Undo`:**
    - Undo restores O to active and removes the link (`Replacements(O).by == nil`).
    - One `asset_replacement_undone` event lands on each of O and N.
    - Undoing again gives `ErrAssetNotReplaced`.
  - **`TestReplacement_RestoreUndoes`:** after `Replace`, a plain `Restore(O)` removes the link and writes the undone events, and no `asset_restored`.
  - **`TestReplacement_Concurrent`:** two goroutines `Replace(O, N1)` and `Replace(O, N2)` with the same version. Exactly one returns nil, and the other returns `ErrAssetChanged` or `ErrAssetRetired`.
  - **`TestReplacement_OppositeOrderNoDeadlock`:** with A and B active, run `Replace(A, B)` and `Replace(B, A)` concurrently with a 10-second context. Both return, and exactly one succeeds.
- [ ] **Step 2: Run them:** `cd backend && CI=true go test -p 1 ./internal/inventory/repository/ -run TestReplacement -v`. Expected: build failure, undefined methods.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run them:** they pass. Then `CI=true go test -p 1 ./internal/inventory/repository/` gives all `ok`, including the existing retire and restore tests.
- [ ] **Step 5: Commit:** `feat(inventory): record and undo asset replacements in one transaction`.

---

### Task 3: Service and API

**Files:**
- Create: `backend/internal/inventory/service/replacement.go`
- Modify: `backend/internal/inventory/service/assets.go` (`AssetView` gains the links; `view` loads them)
- Modify: `backend/internal/inventory/handler/openapi.yaml`, `handler/convert.go`; create `handler/replacement.go`; then `make generate`
- Test: `backend/internal/inventory/http_db_test.go`, adding `TestReplacementOverHTTP`

**Interfaces — Produces:**

```go
type ReplacementInput struct { ReplacementID uuid.UUID; ReplacedOn time.Time; Reason string }
func (s *Service) RecordReplacement(ctx context.Context, oldID uuid.UUID, in ReplacementInput, version int32) (AssetView, error) // PermAssetManage
func (s *Service) UndoReplacement(ctx context.Context, oldID uuid.UUID, version int32) (AssetView, error)                         // PermAssetManage
// AssetView thêm: ReplacedBy *domain.ReplacementLink; Replaces []domain.ReplacementLink
```

**Service details:**
- `RecordReplacement` cleans the reason with the existing `cleanText(…, domain.MaxReplacementReason)`. An empty reason gives `ErrInvalidReplacement` with field `reason`.
- It checks the date with `domain.CheckReplacedOn(in.ReplacedOn, time.Now())`.
- It uses `statuses.Default(ctx, domain.KindRetired)`, like `RetireAsset`. `UndoReplacement` uses `KindAvailable`.

**OpenAPI:**
- `POST /assets/{assetID}/replacement`: `ReplacementRequest { replacement_asset_id: ID, replaced_on: date, reason: string (1–500), version: int32 }`, returning `AssetDetail`.
- `POST /assets/{assetID}/replacement/undo`: `VersionRequest`, returning `AssetDetail`.
- `AssetDetail` gains:
  - `replaced_by: ReplacementLink` (nullable);
  - `replaces: ReplacementLink[]` (required, may be empty);
  - where `ReplacementLink { id, tag, name, replaced_on (date), reason }`.

- [ ] **Step 1: Write the failing test** `TestReplacementOverHTTP`. Create two assets, O and N, with the manage token.
  - **Record:** `POST /assets/{O}/replacement {replacement_asset_id: N, replaced_on: today, reason: "Broken screen", version}` returns 200.
    - The body has `retired_at` set, `replaced_by.tag == N.tag` and `replaced_by.reason == "Broken screen"`.
    - `GET /assets/{N}` has `replaces[0].tag == O.tag`.
  - **Undo after the new asset was retired:** retire N, then `POST /assets/{O}/replacement/undo {version}`. It returns 200: O is active, `replaced_by` is null, and N stays retired.
  - **Invalid date:** `replaced_on` = today + 3 days returns 422 with field `replaced_on`.
  - **Empty reason:** a reason of `"  "` returns 422.
  - **No permission:** a token without `inventory.asset.manage` returns 403.
  - **Unknown asset:** an unknown O returns 404.
  - **Not replaced:** undoing an asset that has no link returns 409 with `/errors/asset-not-replaced`.
  - **Stale version:** a stale version returns 409.
- [ ] **Step 2: Run it:** `cd backend && CI=true go test -p 1 ./internal/inventory/ -run TestReplacementOverHTTP -v`. Expected: FAIL (route not found).
- [ ] **Step 3: Implement** the service, the OpenAPI changes (follow the file's `nullable: true` and `ID` conventions), `make generate`, and the handlers. `toAPIAsset` maps the links; `replaces` is never null in JSON.
- [ ] **Step 4: Run it:** it passes. Then `make check` reports no issues, and `cd frontend && npm run gen:api && npx vue-tsc --noEmit -p tsconfig.app.json` is clean.
- [ ] **Step 5: Commit:** `feat(inventory): POST /assets/{id}/replacement and /undo` (stage `frontend/src/lib/api/inventory.d.ts` only, not `client.ts`).

---

### Task 4: Activity kinds for replacements

**Files:**
- Modify: `backend/internal/inventory/service/activity.go`, `activity_test.go`
- Modify: `backend/internal/inventory/handler/openapi.yaml` (the `kind` enum gains `replaced`, `replaces` and `replacement_undone`; `AssetActivityEntry` gains `related: ActivityRelated` (optional) `{ id, tag, role? }`), `handler/activity.go`; then `make generate` and `npm run gen:api`
- Test: extend `TestReplacementOverHTTP` (Task 3) with an activity check

**Interfaces:**
- Consumes: the Task 1 contract types.
- Produces: `ActivityEntry.Related *ActivityRelated` with `type ActivityRelated struct { ID uuid.UUID; Tag string; Role string }`. `Role` is set only for `replacement_undone`, and is this asset's role (`"old"` / `"new"`).

| Event | `Kind` | `Related` | Other |
|---|---|---|---|
| `inventory.asset_replaced` | `replaced` | `{ReplacementID, ReplacementTag}` | `Reason` = reason |
| `inventory.asset_replaces` | `replaces` | `{ReplacedID, ReplacedTag}` | |
| `inventory.asset_replacement_undone` | `replacement_undone` | `{OtherID, OtherTag, Role}` | |

- [ ] **Step 1: Write the failing tests:**
  - **`TestResolveActivity_ReplacementKinds`:** the three events resolve to the kinds, `Related` and `Reason` in the table above. `Role` is `"old"` / `"new"` as given.
  - **HTTP:** after record and undo, `GET /assets/{O}/activity` kinds start with `[replacement_undone, replaced, …]` with `related.tag == N.tag`. N's activity starts with `[replacement_undone, replaces, …]`.
- [ ] **Step 2: Run them:** they fail (the kinds come back as `other`).
- [ ] **Step 3: Implement, then regenerate the code.**
- [ ] **Step 4: Run them:** `go test ./internal/inventory/service/ -run TestResolveActivity` and the HTTP test pass; `make check` is clean.
- [ ] **Step 5: Commit:** `feat(inventory): replacement entries in the activity history`.

---

### Task 5: Frontend data and helpers

**Files:**
- Modify: `frontend/src/features/assets/api.ts` (`useRecordReplacement`, `useUndoReplacement` through the existing `useAssetMutation` helper, so lists, details and activity refresh)
- Modify: `frontend/src/features/assets/activity.ts`, `activity.test.ts` (new kinds)
- Create: `frontend/src/features/assets/replacement.ts`, `replacement.test.ts`

**Interfaces — Produces:**
- `useRecordReplacement()`: a mutation `({ id, replacementId, replacedOn, reason, version }) => AssetDetail`.
- `useUndoReplacement()`: a mutation `({ id, version }) => AssetDetail`.
- `todayLocal(now: Date): string`: the local `YYYY-MM-DD`, built with `toDateString` from `@/lib/dates`.
- `replacementFormError(f: { replacementId?: string; replacedOn?: string; reason: string }, today: string): string | null`. It returns the first problem in this order:
  - `"Pick the replacement asset."`
  - `"Pick the date."`
  - `"The date can't be in the future."` (when `replacedOn > today`)
  - `"Give a reason."` (when the trimmed reason is empty)
  - `"Keep the reason under 500 characters."`

  Otherwise it returns `null`.

- [ ] **Step 1: Write the failing tests:**
  - **`activitySummary`:**

| Entry | Expected `text` |
|---|---|
| `replaced` with `related.tag` "LAP-0042", reason "screen broken" | "replaced this asset with LAP-0042: screen broken" |
| `replaces` with tag "LAP-0017" | "recorded that this asset replaces LAP-0017" |
| `replacement_undone` with role `old`, tag "LAP-0042" | "undid the replacement by LAP-0042" |
| `replacement_undone` with role `new`, tag "LAP-0017" | "undid the replacement of LAP-0017" |

  - **`replacementFormError`:** each message above in order, and `null` for a complete form. A 500-character reason is fine; 501 isn't.
  - **`todayLocal`:** returns the date of a fixed `Date` built from local parts.
- [ ] **Step 2: Run them:** `cd frontend && npx vitest run src/features/assets/activity.test.ts src/features/assets/replacement.test.ts`. Expected: FAIL.
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run them:** they pass; `npx vue-tsc --noEmit -p tsconfig.app.json` is clean.
- [ ] **Step 5: Commit:** `feat(frontend): replacement data and helpers`.

---

### Task 6: Replace dialog, page actions, links

**Files:**
- Create: `frontend/src/features/assets/components/ReplaceDialog.vue`
- Modify: `frontend/src/features/assets/useAssetActions.ts` (`replace(asset, input)`, `undoReplacement(asset)` via `runAction`)
- Modify: `frontend/src/features/assets/pages/AssetDetailPage.vue` (buttons and the Replacement block)
- Modify: `frontend/src/features/assets/components/AssetActivity.vue` (link `related.tag` to `/assets/{related.id}`)
- Modify: `frontend/README.md` (one line), `backend/docs/inventory.md` (endpoints, rules, events, activity kinds)

**What each piece does:**
- **`ReplaceDialog`** (`FormDialog`, like `CreateAccountDialog`), with title "Replace <tag>":
  - **Replacement asset:** `AutoComplete` searching `GET /assets?q=` (page size 10). It excludes the current asset; retired ones are already excluded by default. Each option shows `tag · name · type`.
  - **Date:** `DatePicker`, default `todayLocal(new Date())`, `maxDate` = today.
  - **Reason:** `Textarea`, `maxlength` 500.
  - **Note line:** "<tag> will be retired."
  - **Submit button:** "Replace asset", disabled while `replacementFormError(...)` returns a message, which is shown under the form.
- **`useAssetActions.replace`:** `runAction` with:
  - `done`: `"<old tag> replaced by <new tag>."`
  - `undo`: `useUndoReplacement`
  - `undone`: `"<old tag> restored."`
  - `undoFailed`: `"Couldn't undo the replacement. Someone may have changed <old tag> since."`
- **`useAssetActions.undoReplacement`:** `runAction` with:
  - `done`: `"Replacement undone. <old tag> restored."`
  - `undo`: record the same replacement again, using the `replaced_by` data captured before.
  - `undone`: `"<old tag> replaced by <new tag> again."`
- **Asset page buttons:**
  - an active asset: **Replace…** (`pi pi-sync`, secondary outlined) next to Retire;
  - a retired asset with `replaced_by`: **Undo replacement** (`pi pi-undo`) in place of Restore.
- **Replacement panel:** shown when `replaced_by` or `replaces` is set:
  - "Replaced by" `RouterLink` tag · name · `formatDate(replaced_on)` · reason;
  - "Replaces" with one line per item (tag link · name · date).

- [ ] **Step 1: Build it.** It's UI wiring; the logic is tested in Task 5.
- [ ] **Step 2: Check:** `cd frontend && npx vue-tsc --noEmit -p tsconfig.app.json && npx vitest run` is clean.
- [ ] **Step 3: Check in the browser,** if the dev stack is running; otherwise ask the user before `make up`. Replace a "Under repair" asset from its page and check:
  - both pages show the links;
  - both Activity tabs show the entries, with tag links;
  - Undo from the message restores it;
  - recording again, then **Undo replacement** from the page, works.

  Take a screenshot of the dialog and of the page and look at them.
- [ ] **Step 4: Commit:** `feat(frontend): replace an asset from its page, with undo`.

---

### Task 7: Full checks

- [ ] `cd backend && make check` → no issues; `CI=true go test -p 1 ./...` → all `ok`.
- [ ] `cd frontend && npm run check` → typecheck clean, tests pass, build succeeds.
