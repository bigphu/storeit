# Inventory module: reference

`backend/internal/inventory`: asset types with typed custom attributes, statuses, and
assets (create, view, full update, retire, restore, search), Excel export and export
profiles. Design:
`docs/superpowers/specs/2026-10-02-inventory-schema-design.md`; export:
`docs/superpowers/specs/2026-10-06-excel-export-design.md`. Platform APIs:
`docs/platform.md`.

Not built yet (sketched in the spec): borrowing and returns, replacement, parent/child
assets, QR redirect, Excel import, validation of `location_id` and
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
- Order: `PUT …/attributes/order` and `PUT …/options/order` take every active attribute
  (option) id once in the new order and set positions 1..n in one transaction with one
  `asset_type_updated` event (only changed positions); a different set is 422
  `/errors/invalid-order`. `PUT /asset-statuses/order` does the same for every active
  status (the list order, which lanes per kind follow), with one `status_updated` event per
  status whose position changed.
- Attributes: removing one hides it and keeps its values; changing data type or unit
  while values exist is 409 `/errors/attribute-in-use`; leaving `select` drops its options;
  `"unit": ""` removes the unit.
- `POST /asset-types/{typeID}/attributes/{attributeID}/restore` and
  `…/options/{optionID}/restore` undo a removal and return the attribute (with options) or
  option; already active returns it unchanged. Keys stay unique including removed
  attributes, so only the label can clash: 409 `/errors/attribute-label-taken` or
  `/errors/option-label-taken`. An option's attribute must be active. Recorded on
  `asset_type_updated` as `removed → active`.
- Statuses: a status that is the default of its kind cannot be archived (409
  `/errors/status-is-default`); make another one default first. System statuses and the
  `GENERAL` type cannot be archived. `POST …/restore` offers an archived status again
  (no-op when it is active); names stay unique across archived ones, so it cannot clash.
- Search: `q` matches tag or name as a literal substring, case-insensitive; filters by
  type, status, status kind, location, holder; sort by `tag` (default), `name`,
  `purchase_date`, `updated_at`, `asset_type` (type name), `status` (status position then
  name, the order of `GET /asset-statuses`), `-` = descending. Location and holder are not
  sortable until the directory module gives them names.
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
| `GET /asset-types` (`with_counts=true` adds `asset_count` and `kind_counts` for assets not retired, `attribute_count` and `attribute_labels` for active attributes), `GET /asset-types/{typeID}`, `GET /asset-statuses` (`with_counts=true` adds `asset_count`, retired assets included), `GET /assets`, `GET /assets/{assetID}` | `inventory.asset.read` |
| `POST /assets`, `PUT /assets/{assetID}`, `POST …/retire`, `POST …/restore`, `POST /assets/bulk-retire`, `POST /assets/bulk-status` | `inventory.asset.manage` |
| `POST /asset-types`, `PATCH /asset-types/{typeID}`, `POST …/archive`, `POST …/restore`, attributes (`POST`, `PATCH`, `DELETE`, `POST …/restore`, `PUT …/attributes/order`), options (`POST`, `PATCH`, `DELETE`, `POST …/restore`, `PUT …/options/order`) | `inventory.type.manage` |
| `POST /asset-statuses`, `PATCH /asset-statuses/{statusID}`, `POST …/archive`, `POST …/restore`, `PUT /asset-statuses/order` | `inventory.status.manage` |
| `POST /assets/export` | `inventory.asset.read` + `inventory.asset.export` |
| `GET`/`POST /export-profiles`, `GET /export-profiles/{profileID}` | `inventory.asset.export` |
| `PATCH`/`DELETE /export-profiles/{profileID}`, `POST …/restore` | `inventory.asset.export`, and the profile's owner; a shared profile of someone else also needs `inventory.export_profile.manage` |

Grants (migrations `00003`, `00005`): Administrator the four above + `inventory.asset.export` +
`inventory.export_profile.manage`; Authorized Manager read + type.manage + status.manage +
asset.export + export_profile.manage; Inventory Officer read + asset.manage + asset.export;
Employee read + asset.export.

Custom values in requests: `"attributes": {"ram_gb": 16, "os": "<option id>",
"warranty_end": "2027-06-30", "has_dock": true}`. In `GET /assets/{id}`: every active
attribute in display order with `value` (null when empty), `unit`, and for select
`option_label`/`option_removed`. `GET /assets` items carry type and status names; with
`type_id` each item also has `attributes` in the same shape (values for the whole page
loaded in one extra query), so a client can render a per-type table. Without `type_id`
the field is absent.

## Bulk actions

`POST /assets/bulk-retire` (`items`, optional `reason`) and `POST /assets/bulk-status`
(`items`, `status_id`) take 1–200 `{id, version}` items (duplicates processed once). Each
asset goes through the single-asset rules and its own transaction, with one event per
asset, and succeeds or fails on its own: the response is always 200 with `succeeded` ids
and `failed` entries carrying the problem a single call would return (`asset-changed`,
`asset-retired`, `asset-not-found`). Request-level errors are 422 (`invalid-bulk`,
`retired-status`, `status-archived`). An unexpected error (database) stops the batch with
a 500; assets already done stay done. Setting the status an asset already has writes
nothing. Code: `service/bulk.go`.

## Events (`contract/events.go`)

`inventory.asset_created`, `asset_updated` (field and `attributes.<key>` changes),
`asset_retired`, `asset_restored`; `inventory.asset_type_created`, `asset_type_updated`
(including attribute and option changes), `asset_type_archived`, `asset_type_restored`;
`inventory.status_created`, `status_updated`, `status_archived`, `status_restored`;
`inventory.export_profile_created`, `export_profile_updated`, `export_profile_deleted`,
`export_profile_restored`,
`assets_exported` (see Export).

## Export (US-07 to US-09)

Server-side, synchronous: the file is built in memory (excelize) and returned in the
response; nothing is stored. Code: `service/export.go`, `service/export_profiles.go`,
`domain/export.go`, `spreadsheet/`.

| Endpoint | Purpose |
|---|---|
| `POST /assets/export` | Build and download an `.xlsx` |
| `GET /export-profiles` | Your profiles and every shared one, by name (no filters) |
| `POST /export-profiles` | Create `{name, shared, layout}` |
| `GET /export-profiles/{profileID}` | Read one (own or shared, else 404) |
| `PATCH /export-profiles/{profileID}` | Change `name`, `shared`, `layout` with `version` (409 `/errors/export-profile-changed` when stale) |
| `DELETE /export-profiles/{profileID}` | Delete (soft) |
| `POST /export-profiles/{profileID}/restore` | Undo a delete; returns the profile |

Permissions are in the table above. `inventory.asset.export` goes to all four roles,
`inventory.export_profile.manage` to Administrator and Authorized Manager.

**`POST /assets/export`:** body `{filters, mode, profile_id?, layout?, tz?}`. `filters` takes the
same fields and rules as `GET /assets` (attribute filters and attribute sorts need
`type_id`) plus `ids` (1 to 200, else 422 `/errors/invalid-export-selection`) for "Export
selected". The filters become the same `domain.AssetFilter` the list uses; paging is ignored.
`mode=data` ignores `profile_id` and `layout`. `mode=report` uses `layout` if given, else
the profile, else `DefaultReportLayout()` (tag, name, type, status, purchase date; bold
header, freeze, filter). Omitted layout fields are filled by `WithDefaults()` (the validator
applies no defaults), then validated. The response is 200 with the spreadsheetml content type,
`Content-Disposition: attachment` (`storeit-assets-YYYY-MM-DD.xlsx` for data,
`<profile-name-slug>-YYYY-MM-DD.xlsx` or `storeit-report-YYYY-MM-DD.xlsx` for reports) and,
when anything was skipped, `X-Export-Skipped-Columns: key1,key2`.

`tz` is the user's IANA time zone (`Asia/Ho_Chi_Minh`, max 64 characters; the web client sends
`Intl.DateTimeFormat().resolvedOptions().timeZone` on every export). Empty means UTC; the
server runs with `TZ=UTC` and never uses `time.Local` (`cmd/server` imports `time/tzdata`, so
names resolve in minimal images). The zone applies to `updated_at` cells, the "Generated"
date of the title row (formatted with the layout's `date_format`) and the date in the file
name. An unknown name is 422 `/errors/invalid-time-zone` with a field error on `tz`.

**Data export** (`domain.DataLayout()`, fixed so the future import can read it back): one
sheet per asset type present in the rows, ordered by type name and named by the type
**code**. Columns: `tag`, `name`, `description`, `status`, `purchase_date`, then every active
attribute of the type in display order. Headers are the field keys (`tag`, `attr:ram_gb`).
Status by name; dates are Excel dates formatted `yyyy-mm-dd`; numbers are numbers without
units; booleans are Excel `TRUE`/`FALSE`; select attributes show the option label. Plain
header, frozen first row, no title row, summary, filter buttons or stripes. No rows still gives
a file with one sheet of headers.

**Report layout** (`domain.ExportLayout`, JSON; stored as is in profiles):

| Field | Values |
|---|---|
| `columns` | `[{field, header, width}]`, order = sheet order. `field` is `tag`, `name`, `description`, `type`, `status`, `purchase_date`, `updated_at` or `attr:<key>`. `header` "" = default label; `width` 0 = automatic |
| `sheets` | `single` or `per_type` (sheet per type, named by type name; `each_type_attrs` appends every active attribute of the sheet's type after the chosen columns) |
| `sheet_name` | single mode; default "Assets" |
| `title_row` | Two rows above the header: profile name (or "Asset report"), and "Generated <dd/mm/yyyy> by <name> · <filter summary>" |
| `summary` | Extra "Summary" sheet: assets per type and status kind |
| `header` | `plain`, `bold`, `bold_fill` |
| `freeze`, `filter`, `stripes` | Freeze header, filter buttons and stripes |
| `date_format` | `dd/mm/yyyy`, `yyyy-mm-dd`, `d mmm yyyy` |
| `bool_style` | `yes_no`, `check` |
| `status_as` | `name`, `kind` |
| `unit_in` | `header` (`RAM (GB)`) or `cell` |
| `sort` | "" = the request's sort, else an `AssetSort` value (`attributes.<key>` allowed) |

Validation (`layout.Validate()`, 422 `/errors/invalid-export-layout` with all field errors
at once, paths like `layout.columns[3].field`): 1 to 60 columns (0 allowed when `per_type` with
`each_type_attrs`); no field twice; `attr:` keys match `^[a-z][a-z0-9_]{0,31}$`; `header` up to 100
characters, no control characters; `width` 0 or 4 to 80; `sheet_name` 1 to 31 UTF-16 units
(Excel's count, so an emoji is 2) without `[ ] : * ? / \` and not starting or ending with
`'`; the problem's `detail` names the first bad field (`layout.sheet_name: ...`); enums must hold one of their values; `sort` must be valid.

**Attributes and skipped columns:** an `attr:<key>` column is resolved per sheet. In
`per_type` mode it appears only on sheets whose type has an active attribute with that key;
in `single` mode the cell is blank for assets whose type lacks it. A key no exported type
has (for example a removed attribute in an old profile) is skipped, not an error. A layout
`sort` by `attributes.<key>` is applied per sheet, only when the sheet holds exactly one type
that still has the attribute. Otherwise (single sheet spanning several types, or the attribute
is gone) that sheet keeps the request or default sort. In both cases the key is listed in
`X-Export-Skipped-Columns`. A layout `sort` that is not by attribute overrides the
request's sort.

**Cap:** more rows than `INVENTORY_EXPORT_MAX_ROWS` (default 50000; the config accepts
0, meaning 50000, otherwise 1 to 1,000,000) is 422 `/errors/export-too-large` before anything is
written. Assets are streamed from the database in pages of 500 inside one `REPEATABLE READ`
read-only transaction (`AssetRepository.Stream`).

**Profiles** (`inventory.export_profiles`, migration `00005`): `owner_id` (identity account, no
foreign key), `name` 1 to 100 characters and unique per owner ignoring case (409
`/errors/export-profile-name-taken`), `shared`, `layout` jsonb, `version`. Visibility: a profile
is visible to its owner and, when shared, to everyone who can export; someone else's private
profile is 404 `/errors/export-profile-not-found`. Edit and delete: the owner, or for a shared
profile also anyone with `inventory.export_profile.manage`; others get 403
`/errors/export-profile-forbidden`. Responses carry `owner {id, name}` and `can_edit`
for the caller. Deleting is a soft delete (`deleted_at`): a deleted profile
is 404 everywhere, for everyone (including exports by `profile_id`), and names are unique only
among profiles that aren't deleted. Restore follows the delete rules (someone else's private
profile is 404, a shared one without `inventory.export_profile.manage` is 403, checked on the
row locked inside the restore transaction) and is 409
`/errors/export-profile-name-taken` if the owner reused the name. Layouts are validated on create and update; a stored layout that no longer
validates is returned as is and rejected with 422 only when used for an export.

**Events** (outbox, same transaction as the write): `export_profile_created` (`profile_id`,
`name`, `shared`), `export_profile_updated` (field changes; a layout change is one `layout`
change without values), `export_profile_deleted`, `export_profile_restored`. `inventory.assets_exported` (aggregate
`asset_export`, new id) is recorded after the file is fully built, in its own transaction, with
`{mode, profile_id?, rows, sheets, filters}`; a failure to record is logged, not returned.
