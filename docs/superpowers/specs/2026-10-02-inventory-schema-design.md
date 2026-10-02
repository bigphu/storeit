# Inventory module: schema design

Date: 2026-10-02. Status: **in design**. Sections 1–3 approved by the user; section 4 (API
and permissions) not yet presented or approved. Do not implement until the whole spec is
approved and a plan exists.

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
- `PATCH` requires the `version` read (409 when stale), as in identity.
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

Not yet designed with the user. To cover: permission codes and role grants (seeded in the
inventory migration, including grants to Administrator, since nobody can grant a
permission they do not hold), endpoints for asset types/attributes/options/statuses/assets,
the JSON shape of custom values, error types, and events.
