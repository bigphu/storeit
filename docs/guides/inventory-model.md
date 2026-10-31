---
title: Inventory model
description: Asset types with typed attributes, statuses, assets and their tags, replacement, activity history and search.
---

The inventory module tracks assets. Each asset belongs to an **asset type**, which defines the extra fields
(attributes) the asset carries, and has a **status**.

## Concepts

| Concept | Notes |
|---|---|
| Asset type | `code` (`A-Z`, digits, `_`, `-`, up to 32, immutable), unique name, **tag prefix** (`A-Z` and digits, up to 10, unique, editable). A system type `GENERAL` (prefix `GEN`) is seeded and cannot be archived. "Can be borrowed" turns on [borrowing](/guides/borrowing/) for its assets |
| Attribute | Belongs to a type. `key` is immutable and never reused within the type. Data types: text, number, date, boolean, select. Number attributes may have a unit; select attributes have options |
| Status | Has a **kind**: available, in use, unavailable or retired. One default per kind. Seeded: Available, In use, Under repair, Retired |
| Asset | `tag` (generated from the type's prefix, unique across all assets including retired, immutable), name, description, purchase date, type, status, `location_id`, `holder_member_id`, custom attribute values, `version` |

Attribute values are stored as typed rows, and database constraints guarantee a value matches its attribute's
data type and belongs to the asset's current type.

## Validation of values

Every key is checked against the type's active attributes: unknown keys, missing required values and wrong types
are rejected. Numbers allow at most 15 digits and 6 decimals and are rejected rather than rounded. Dates are
`YYYY-MM-DD`. Text is 1 to 1000 characters. A select value is the id of an active option. All problems come back
together as 422 `/errors/invalid-attribute-values`, with one field per attribute.

## Tags

New assets get their tag automatically: the type's prefix and the next number, at least four digits
(`LAP-0091`). The next number is one more than the highest existing tag of that exact form, typed ones included.
Two people creating assets of the same type at once still get different numbers.

- A new type's prefix defaults to the first three letters or digits of its code (`LAPTOP` → `LAP`), with a
  number added when another type already uses it (`MON`, `MON2`). Type it yourself when creating the type, or
  change it later in the type's settings; a change only affects assets created afterwards.
- The API still accepts a tag on create, for imports that bring their own.
- Migration `00010` gave existing types the prefix their tags already used (`LAP`, `DSK`, `CHR`…).

## Working with assets

- Creating needs an active type and status; an omitted status means the default available status.
- `PUT` replaces the asset. Omitted attributes and optional fields are cleared, and an omitted `status_id` keeps the
  current status. A stale `version` is a 409. Changing the type drops the old values in the same transaction.
- **Retire** moves the asset to the default retired status. Retired assets are hidden from lists unless
  `include_retired` is set and cannot be edited (409) until restored to the default available status.
- **Bulk actions** (`bulk-retire`, `bulk-status`) take 1 to 200 `{id, version}` items. Each asset succeeds or
  fails independently; the response is 200 with `succeeded` ids and `failed` entries.

## Replacement

When one asset replaces another (a broken laptop swapped for a new one), record it from the old asset's page:
pick the replacement, the date and a reason. The old asset is retired with the reason "Replaced by TAG: …", and
both pages link to each other. **Undo replacement** (or Restore on the old asset) removes the link and brings the
old asset back. The date can't be before either asset was purchased.

## Activity history

Each asset's **Activity** tab lists everything that happened to it, newest first: created, field changes with
before and after values (status and type names, attribute labels, units, option labels), retired and restored,
replacements, and borrowing (reserved, lent, returned). Each entry names who did it, or System. The history
is read from the event log, which keeps every event for good.

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

Parent and child assets, QR codes that open an asset (`/a/{tag}`), Excel import, and checking `location_id` and
`holder_member_id` against a directory module.
