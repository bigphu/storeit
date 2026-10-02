# Inventory module: reference

`backend/internal/inventory`: asset types with typed custom attributes, statuses, and
assets (create, view, full update, retire, restore, search). Design:
`docs/superpowers/specs/2026-10-02-inventory-schema-design.md`. Platform APIs:
`docs/platform.md`.

Not built yet (sketched in the spec): borrowing and returns, replacement, parent/child
assets, QR redirect, Excel import/export, validation of `location_id` and
`holder_member_id` against the directory module.

## Layout

| Package | Holds |
|---|---|
| `inventory` (root) | `New(Deps{Pool, Outbox})`, `Module` (`Mount(r, tokens)`, `APIDoc`, `Service`) |
| `domain` | `AssetType`, `Attribute`, `Option`, `Status`, `Asset`, `Value`, `ValidateValues`, tag/code/key/label/unit rules, errors, permission codes, seeded IDs, repository interfaces |
| `service` | Use cases; every call starts with `auth.Require` |
| `repository` | sqlc queries (`repository/queries/*.sql`), transactions, outbox events |
| `handler` | `openapi.yaml` → `handler/api`, conversions, `Mount`, `Spec` |
| `contract` | Event names and payloads |

HTTP and end-to-end tests are at the module root (`*_db_test.go`, `package inventory_test`).

## Schema (`migrations/00003_inventory.sql`, schema `inventory`)

- `asset_types`: `code` (`^[A-Z0-9_-]{1,32}$`, immutable), `name` unique ignoring case,
  `is_system` (seeded `GENERAL`), `archived_at`, `version`.
- `asset_type_attributes`: `key` (`^[a-z][a-z0-9_]{0,31}$`, immutable, never reused in a
  type), `label` (unique among active), `data_type` text/number/date/boolean/select,
  `unit` (number only, ≤ 16), `is_required`, `position`, `removed_at`.
- `asset_attribute_options`: choices of a select attribute, `removed_at`.
- `asset_statuses`: `kind` available/in_use/unavailable/retired, one `is_default` per kind,
  seeded system statuses Available, In use, Under repair, Retired.
- `assets`: `tag` (`^[A-Z0-9][A-Z0-9._-]{0,63}$`, unique across all assets including
  retired, immutable), common fields, `location_id` and `holder_member_id` without
  foreign keys, `retired_at`/`retired_reason`, `version`.
- `asset_attribute_values`: one typed row per asset and attribute. Composite foreign keys
  guarantee the value matches its attribute's data type, belongs to the asset's current
  type, and (select) is one of that attribute's options; a CHECK requires exactly the
  column of the data type. An attribute's data type cannot change while values exist.

Seeded IDs are in `domain/permissions.go` (`GeneralTypeID`, `AvailableStatusID`, …).

## Rules

- Values: `domain.ValidateValues(type, map[string]any)` checks every key against the
  active attributes: unknown key, required missing, wrong type, number with more than 15
  digits or 6 decimals (rejected, never rounded), date `YYYY-MM-DD`, text 1..1000 chars
  (newlines allowed), select = id of an active option. All errors come back together as
  422 `/errors/invalid-attribute-values` with one field per key (`attributes.<key>`).
- Assets: create requires an active type and status (`status_id` omitted = default
  available). `PUT` replaces everything: omitted attributes and optional fields are
  cleared, `status_id` omitted keeps the current status, stale `version` is 409. Changing
  the type drops the old values in the same transaction. A retired-kind status is set
  only by retire. Archived types/statuses stay on assets that already use them.
- Retire: default retired status, hidden from the list unless `include_retired`, not
  editable (409 `/errors/asset-retired`) until restored to the default available status.
- Attributes: removing one hides it and keeps its values; changing data type or unit
  while values exist is 409 `/errors/attribute-in-use`; leaving `select` drops its options;
  `"unit": ""` removes the unit.
- Statuses: a status that is the default of its kind cannot be archived (409
  `/errors/status-is-default`); make another one default first. System statuses and the
  `GENERAL` type cannot be archived.
- Search: `q` matches tag or name as a literal substring, case-insensitive; filters by
  type, status, status kind, location, holder; sort by tag, name, purchase date, updated
  date (`-` = descending).
- Custom attributes in search (need `type_id`, since a key only means something inside one
  type): `attr=<key>:<op>:<value>`, repeatable (AND, max 10); `sort=attributes.<key>` or
  `-attributes.<key>`. Operators: text `eq` (case-insensitive) and `contains` (literal);
  number and date `eq` `gt` `gte` `lt` `lte`; boolean `eq`; select `eq` and `in`
  (comma-separated option ids, removed options allowed). Assets without a value never
  match a filter and sort last in both directions; select sorts by option position; ties
  fall back to tag. `domain.ResolveAttrQuery` checks everything against the type's active
  attributes; errors are 422 `/errors/invalid-attribute-query` with fields `attr[i]`,
  `sort` or `type_id` (missing or unknown type). A malformed `attr`/`sort` is a 400 from
  the request validator. SQL: the filters reach `ListAssets`/`CountAssets` as three
  parallel arrays (attribute, `<data type>_<op>`, value) checked with `NOT EXISTS`.

## API (`/api/v1`) and permissions

| Endpoints | Needs |
|---|---|
| `GET /asset-types`, `GET /asset-types/{typeID}`, `GET /asset-statuses`, `GET /assets`, `GET /assets/{assetID}` | `inventory.asset.read` |
| `POST /assets`, `PUT /assets/{assetID}`, `POST …/retire`, `POST …/restore` | `inventory.asset.manage` |
| `POST /asset-types`, `PATCH /asset-types/{typeID}`, `POST …/archive`, `POST …/restore`, attributes (`POST`, `PATCH`, `DELETE`), options (`POST`, `PATCH`, `DELETE`) | `inventory.type.manage` |
| `POST /asset-statuses`, `PATCH /asset-statuses/{statusID}`, `POST …/archive` | `inventory.status.manage` |

Grants (migration): Administrator all four; Authorized Manager read + type.manage +
status.manage; Inventory Officer read + asset.manage; Employee read.

Custom values in requests: `"attributes": {"ram_gb": 16, "os": "<option id>",
"warranty_end": "2027-06-30", "has_dock": true}`. In `GET /assets/{id}`: every active
attribute in display order with `value` (null when empty), `unit`, and for select
`option_label`/`option_removed`. `GET /assets` items carry type and status names; with
`type_id` each item also has `attributes` in the same shape (values for the whole page
loaded in one extra query), so a client can render a per-type table. Without `type_id`
the field is absent.

## Events (`contract/events.go`)

`inventory.asset_created`, `asset_updated` (field and `attributes.<key>` changes),
`asset_retired`, `asset_restored`; `inventory.asset_type_created`, `asset_type_updated`
(including attribute and option changes), `asset_type_archived`, `asset_type_restored`;
`inventory.status_created`, `status_updated`, `status_archived`.
