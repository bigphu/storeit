---
title: Inventory model
description: Asset types with typed attributes, statuses, assets, and how search works.
---

The inventory module tracks assets. Each asset belongs to an **asset type**, which defines the extra fields
(attributes) the asset carries, and has a **status**.

## Concepts

| Concept | Notes |
|---|---|
| Asset type | `code` (`A-Z`, digits, `_`, `-`, up to 32, immutable), unique name. A system type `GENERAL` is seeded and cannot be archived |
| Attribute | Belongs to a type. `key` is immutable and never reused within the type. Data types: text, number, date, boolean, select. Number attributes may have a unit; select attributes have options |
| Status | Has a **kind**: available, in use, unavailable or retired. One default per kind. Seeded: Available, In use, Under repair, Retired |
| Asset | `tag` (unique across all assets including retired, immutable), name, description, purchase date, type, status, `location_id`, `holder_member_id`, custom attribute values, `version` |

Attribute values are stored as typed rows, and database constraints guarantee a value matches its attribute's
data type and belongs to the asset's current type.

## Validation of values

Every key is checked against the type's active attributes: unknown keys, missing required values and wrong types
are rejected. Numbers allow at most 15 digits and 6 decimals and are rejected rather than rounded. Dates are
`YYYY-MM-DD`. Text is 1 to 1000 characters. A select value is the id of an active option. All problems come back
together as 422 `/errors/invalid-attribute-values`, with one field per attribute.

## Working with assets

- Creating needs an active type and status; an omitted status means the default available status.
- `PUT` replaces the asset. Omitted attributes and optional fields are cleared, and an omitted `status_id` keeps the
  current status. A stale `version` is a 409. Changing the type drops the old values in the same transaction.
- **Retire** moves the asset to the default retired status. Retired assets are hidden from lists unless
  `include_retired` is set and cannot be edited (409) until restored to the default available status.
- **Bulk actions** (`bulk-retire`, `bulk-status`) take 1 to 200 `{id, version}` items. Each asset succeeds or
  fails independently; the response is 200 with `succeeded` ids and `failed` entries.

## Changing the schema safely

- Removing an attribute hides it and keeps its values; it can be restored.
- Changing an attribute's data type or unit while values exist is a 409.
- A status that is the default of its kind cannot be archived; make another one the default first.
- Archived types and statuses stay on assets that already use them.
- Reordering takes every active id once in the new order; a different set is rejected.

## Search

`GET /assets` supports:

- `q`: literal, case-insensitive substring of tag or name.
- Filters by type, status, status kind, location and holder.
- Sort by `tag` (default), `name`, `purchase_date`, `updated_at`, `asset_type` or `status`; prefix `-` for descending.
- With `type_id`: `attr=<key>:<op>:<value>` (repeatable, up to 10, combined with AND) and
  `sort=attributes.<key>`. Operators: text `eq` and `contains`; number and date `eq`, `gt`, `gte`, `lt`, `lte`;
  boolean `eq`; select `eq` and `in`.
- `field=<key>:<op>:<value>` for fields every asset has (`purchase_date`, `created_at`, `updated_at`, `description`).
  Day comparisons use the `tz` time zone (default UTC).

Assets with no value never match an attribute filter and sort last.

## Permissions

| Needs | Endpoints |
|---|---|
| `inventory.asset.read` | list and read types, statuses and assets |
| `inventory.asset.manage` | create, update, retire, restore, bulk actions |
| `inventory.type.manage` | create, change, archive, restore types, attributes and options |
| `inventory.status.manage` | create, change, archive, restore, reorder statuses |

## Not built yet

Borrowing and returns, replacement, parent and child assets, QR redirect, Excel import, and checking `location_id`
and `holder_member_id` against a directory module.
