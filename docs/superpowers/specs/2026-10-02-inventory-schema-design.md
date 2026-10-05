# Inventory module: schema design

Date: 2026-10-02. Status: all four sections approved in conversation; waiting for the
user's review of this written spec, then an implementation plan. Do not implement before
both.

Module: `backend/internal/inventory` (placeholders marked `TODO(M4)`), Postgres schema
`inventory`. Builds on `docs/platform.md` and the identity module (`docs/identity.md`).

## Scope

Sprint 2: asset types with custom attributes (US-17, US-18), assets that use them
(US-19), and the basic asset lifecycle they depend on (US-01 to US-05: create, view,
update, retire, search). Later features (borrowing, replacement, history, Excel, directory
links, parent/child, QR) are only sketched in section 3 so the Sprint 2 schema needs no
rework; their tables are **not** created now.

Reference project `C:\d-drive\assignments\asset-management` (`backend/db/init/001_schema.sql`)
keeps everything in one `assets` table with fixed columns and stores custom values in a
JSONB column validated only by the app. StoreIt deliberately does not copy that.

## Decisions (with the user)

| Topic | Decision |
|---|---|
| Custom value storage | One `assets` table with real columns for common fields; custom values as **typed rows** in one values table (approach A). Not JSONB, not a table per asset type, not one table per data type. |
| Location and assigned person | Nullable `location_id` and `holder_member_id` on `assets`, **no foreign key** (directory module not built; same pattern as identity's `member_id`). Inventory validates them through `directory/contract` once directory exists. |
| Scope | Sprint 2 tables in full; later parts sketched only. |
| Choice data type | Yes: `select` with an options table; values reference the option by id. |
| Units | One unit per attribute (number attributes only). Values are plain numbers in that unit. Unit cannot change while values exist. No per-value units. |
| Filtering by custom attribute | Not in Sprint 2 unless the user asks; would need indexes on the values table. |

## 1. Asset types and custom attributes

```sql
asset_types
  id           uuid PK
  code         text UNIQUE      -- 'LAPTOP': ^[A-Z0-9_-]+$, uppercase, immutable
  name         text             -- unique on lower(name)
  description  text DEFAULT ''
  is_system    boolean          -- seeded GENERAL type: cannot be archived
  archived_at  timestamptz NULL -- kept, not offered for new assets
  version, created_at, updated_at

asset_type_attributes
  id            uuid PK
  asset_type_id uuid -> asset_types
  key           text   -- 'ram_gb': ^[a-z][a-z0-9_]{0,31}$, immutable, never reused in the type
  label         text   -- 'RAM': editable, unique per type among active attributes
  data_type     text   CHECK IN ('text','number','date','boolean','select')
  unit          text NULL -- 'GB', 'kg', 'inch': only when data_type = 'number', <= 16 chars
  is_required   boolean
  position      int    -- display order
  removed_at    timestamptz NULL
  UNIQUE (asset_type_id, key)
  UNIQUE (id, asset_type_id, data_type)   -- target of the values FK

asset_attribute_options          -- for data_type 'select'
  id           uuid PK
  attribute_id uuid -> asset_type_attributes
  label        text  -- unique per attribute among active options
  position     int
  removed_at   timestamptz NULL
  UNIQUE (id, attribute_id)      -- target of the values FK

asset_attribute_values
  asset_id, attribute_id  PK
  asset_type_id, data_type        -- copies so foreign keys can check them
  value_text      text            -- non-blank, <= 1000 chars
  value_number    numeric         -- exact decimal, in the attribute's unit
  value_date      date
  value_bool      boolean
  value_option_id uuid
  FK (asset_id, asset_type_id)                -> assets (id, asset_type_id)
  FK (attribute_id, asset_type_id, data_type) -> asset_type_attributes (id, asset_type_id, data_type)
  FK (value_option_id, attribute_id)          -> asset_attribute_options (id, attribute_id)
  CHECK exactly the column for data_type is non-null
```

Guarantees from the database:

- A value's type matches its attribute's type.
- A value belongs to its asset's current type (changing the asset's type requires deleting
  the old values in the same transaction).
- An attribute's data type cannot change while values exist (FK; API answers 409).
- A select value can only be one of that attribute's own options.
- An empty optional attribute has no row (never an empty string).

Rules in the service:

- Required attributes are checked on asset create and on every asset update. Making an
  attribute required is allowed; assets missing it must supply it on their next edit.
- Removing an attribute sets `removed_at`: hidden from forms and validation, stored values
  kept for history, key never reused. Removing an option likewise: not offered for new
  entries; existing values keep pointing to it and are shown as removed.
- Editable on an attribute: label, required, position, unit (only while no values exist),
  data type (only while no values exist).
- Seed: one system type `GENERAL` ("General") with no attributes.

## 2. Assets and statuses

```sql
asset_statuses
  id          uuid PK
  name        text     -- unique on lower(name)
  kind        text CHECK IN ('available','in_use','unavailable','retired')
  is_default  boolean  -- one default per kind (partial unique index)
  is_system   boolean  -- seeded defaults: cannot be archived or change kind
  position, archived_at, created_at, updated_at

assets
  id               uuid PK
  tag              text   -- 'LAP-0001': trimmed, uppercased, ^[A-Z0-9][A-Z0-9._-]{0,63}$,
                          -- unique on lower(tag) across all assets incl. retired (BR-01, BR-06),
                          -- immutable after create (printed on labels/QR)
  name             text   -- 1..200
  description      text DEFAULT ''
  asset_type_id    uuid -> asset_types
  status_id        uuid -> asset_statuses
  location_id      uuid NULL  -- directory later, no FK
  holder_member_id uuid NULL  -- assigned person, directory later, no FK
  purchase_date    date NULL
  retired_at       timestamptz NULL
  retired_reason   text NULL
  version, created_at, updated_at
  UNIQUE (id, asset_type_id)  -- target of the values FK
  INDEX (asset_type_id), (status_id), (location_id) WHERE retired_at IS NULL
```

Statuses:

- Archived statuses and archived types cannot be chosen for a new asset or a changed
  value; assets already using them keep them and remain editable.
- Making a status the default of its kind moves the flag from the previous default (one
  transaction).
- Behaviour depends on `kind`, never on the name (borrowing will require `available`;
  retiring uses the default `retired` status).
- Managers can add statuses ("Lost", "Being repaired") with one of the four kinds.
- Seed: Available (`available`), In use (`in_use`), Under repair (`unavailable`),
  Retired (`retired`), each the system default of its kind.

Assets:

- Tag required and user-entered; immutable after create.
- Retire (US-04): sets `retired_at`, optional reason and the default `retired` status in
  one step. Retired assets are hidden from the list by default, cannot be edited, keep
  their tag forever, and can be restored (to the default `available` status).
- A `retired`-kind status cannot be chosen by a normal edit; only retire/restore set it.
- Changing an asset's type is allowed: old values are dropped and the new type's required
  attributes must be supplied, in one transaction (the form warns first).
- Updates are a full `PUT` that must carry the `version` read (409 when stale), as in
  identity; see section 4.
- No hard deletes. No `created_by`/`updated_by` columns: the actor lives in events
  (`asset_created`, `asset_updated` with field and attribute changes, `asset_retired`,
  `asset_restored`); the activity module turns them into history (US-16).

Search and filter (US-05): text search over tag and name (case-insensitive); filters by
type, status (or status kind), location, holder, include-retired; sort by tag, name,
purchase date, updated date; shared paging parameters (`page` <= 100000, `page_size` <= 200).

## 3. Later features (sketch only, not created in Sprint 2)

| Feature | What it adds | Prepared in Sprint 2 by |
|---|---|---|
| Borrowing/return (US-10..12) | `asset_loans(id, asset_id, borrower_member_id, borrowed_at, due_at, returned_at, note)`, unique partial index: one open loan per asset (BR-02). Borrow sets default `in_use` status + `holder_member_id`; return sets default `available` and clears the holder, same transaction. Loan rows are never rewritten: they are the history. | status `kind`, `holder_member_id` |
| Replacement (US-13) | `asset_replacements(old_asset_id, new_asset_id, replaced_at, reason)`; retires the old asset with the reason in the same transaction. | `retired_at`, `retired_reason` |
| Activity history (US-16) | Nothing in inventory; the activity module consumes inventory events. | events carry actor and changes |
| Excel import/export (US-06..09) | `imports` staging table, `export_profiles` + columns. Custom attributes become columns keyed by attribute `key`, headers "Label (unit)". | stable never-reused keys, units |
| Directory | No cross-module FKs; validate `location_id`/`holder_member_id` via `directory/contract`. | nullable ID columns |
| Parent/child | `assets.parent_id` nullable self-reference, no cycles (if still wanted). | nothing needed |
| QR labels | `GET /a/{tag}` redirect to the asset page. | immutable tags |

## 4. API and permissions

### Permissions

Seeded in the inventory migration together with the role grants. Administrator receives
every inventory permission (nobody can grant a permission they do not hold).

| Code | Allows | Granted to |
|---|---|---|
| `inventory.asset.read` | View assets, asset types and statuses | Administrator, Authorized Manager, Inventory Officer, Employee |
| `inventory.asset.manage` | Create, update, retire, restore assets | Administrator, Inventory Officer |
| `inventory.type.manage` | Asset types, attributes, options | Administrator, Authorized Manager |
| `inventory.status.manage` | Statuses | Administrator, Authorized Manager |

### Endpoints (`/api/v1`)

| Endpoint | Needs |
|---|---|
| `GET /asset-types` (`?include_archived`) | asset.read |
| `GET /asset-types/{typeID}` (with attributes and their options) | asset.read |
| `POST /asset-types` (may include initial attributes and options, US17-AC4) | type.manage |
| `PATCH /asset-types/{typeID}` (name, description, `version`) | type.manage |
| `POST /asset-types/{typeID}/archive`, `POST /asset-types/{typeID}/restore` | type.manage |
| `POST /asset-types/{typeID}/attributes` | type.manage |
| `PATCH /asset-types/{typeID}/attributes/{attributeID}` (label, unit, data_type, is_required, position) | type.manage |
| `DELETE /asset-types/{typeID}/attributes/{attributeID}` (soft remove) | type.manage |
| `POST /asset-types/{typeID}/attributes/{attributeID}/options` | type.manage |
| `PATCH /asset-types/{typeID}/attributes/{attributeID}/options/{optionID}` (label, position) | type.manage |
| `DELETE /asset-types/{typeID}/attributes/{attributeID}/options/{optionID}` (soft remove) | type.manage |
| `GET /asset-statuses` (`?include_archived`) | asset.read |
| `POST /asset-statuses`, `PATCH /asset-statuses/{statusID}` (name, position, is_default), `POST /asset-statuses/{statusID}/archive` | status.manage |
| `GET /assets` (`q`, `type_id`, `status_id`, `status_kind`, `location_id`, `holder_member_id`, `include_retired`, `sort`, `page`, `page_size`) | asset.read |
| `GET /assets/{assetID}` | asset.read |
| `POST /assets` | asset.manage |
| `PUT /assets/{assetID}` (whole asset plus `version`) | asset.manage |
| `POST /assets/{assetID}/retire` (`reason`, `version`), `POST /assets/{assetID}/restore` (`version`) | asset.manage |

- `sort`: `tag`, `name`, `purchase_date`, `updated_at`, prefix `-` for descending; default
  `tag`.
- `PATCH` endpoints change only the fields sent. An attribute's `unit` is cleared by
  sending `"unit": ""` (stored as NULL); there is no null-versus-absent distinction to
  rely on.
- `POST /assets` without `status_id` uses the default `available` status.
- Asset updates are a full `PUT`: the edit form sends everything, so there is no
  absent-versus-null ambiguity for `location_id`, `holder_member_id`, `purchase_date`
  (identity needed a `clear_member_id` flag for that with PATCH).

### Custom values in JSON

Request (`POST`, `PUT`): an object keyed by attribute key; an optional attribute left out
has no value; on `PUT` the object replaces all values.

```json
"attributes": {
  "ram_gb": 16,
  "os": "0193a6c2-…",            // select: option id
  "warranty_end": "2027-06-30",  // date: YYYY-MM-DD
  "has_dock": true,
  "notes_hw": "Spare battery in drawer"
}
```

Response (`GET /assets/{assetID}`): a list in display order with labels and units:
`[{key, label, data_type, unit, value, option_label, option_removed}]`. Values of removed
attributes are not returned (they stay in the database for history and export).
`GET /assets` items carry the common fields plus type and status names, without custom
values.

### Errors

| Status | Type | When |
|---|---|---|
| 422 | `/errors/invalid-attribute-values` | one field entry per bad key: `attributes.<key>` "is required", "must be a number" (etc.), "is not an option of this attribute", "unknown attribute" |
| 422 | (own types) | archived type or status for a new asset or changed value; a `retired`-kind status chosen by a normal edit; invalid tag/code/key format |
| 409 | taken | tag, type code, type name, attribute key, attribute label, option label, status name |
| 409 | `/errors/asset-changed`, `/errors/asset-type-changed` | stale `version` |
| 409 | `/errors/asset-retired` | editing a retired asset |
| 409 | `/errors/attribute-in-use` | changing an attribute's data type or unit while values exist |
| 409 | `/errors/system-type`, `/errors/system-status` | archiving a system type or status, or changing a system status's kind |
| 409 | `/errors/status-is-default` | archiving the default status of a kind (make another one default first) |
| 404 | not found | asset, type, attribute, option, status |

### Events (outbox, same transaction, actor recorded)

- `inventory.asset_created`, `inventory.asset_updated` (field changes, attribute changes,
  type change), `inventory.asset_retired`, `inventory.asset_restored`
- `inventory.asset_type_created`, `inventory.asset_type_updated` (including attribute and
  option changes), `inventory.asset_type_archived`, `inventory.asset_type_restored`
- `inventory.status_created`, `inventory.status_updated`, `inventory.status_archived`

Payloads carry ids, names and before/after values, never more than the API shows. The
activity module turns asset events into history (US-16).

## Testing

- Repository tests on Postgres: every database guarantee in section 1 (wrong-typed value,
  value for another type's attribute, option of another attribute, data type change with
  values, tag uniqueness across retired assets), retire/restore, type change dropping
  values, search filters and sorting.
- Service tests with fakes: required attributes, removed attributes/options, archived
  type/status rules, retired-kind status rule, permission checks.
- HTTP tests: each endpoint's status codes and problem types, attribute JSON round trip.
- End to end: create a type with number (unit), select and required text attributes,
  create an asset, read it back, change type, retire, list with filters.
