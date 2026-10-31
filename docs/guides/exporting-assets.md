---
title: Exporting assets
description: Excel data exports, formatted reports and saved export profiles.
---

Export is synchronous: the server builds the `.xlsx` in memory and returns it in the response. Nothing is stored.

## `POST /assets/export`

The body is `{filters, mode, profile_id?, layout?, tz?}`.

- `filters` accept the same fields as `GET /assets`, plus `ids` (1 to 200) for "export selected". Paging is ignored.
- `tz` is the user's IANA time zone; the web client sends it on every export. It applies to `updated_at` cells,
  the "Generated" date and the file name. Empty means UTC; an unknown name is a 422.

### Data mode

`mode=data` produces a fixed layout, designed so a future import can read it back:

- One sheet per asset type present, ordered by type name and named by the type **code**.
- Columns: `tag`, `name`, `description`, `status`, `purchase_date`, then the type's active attributes in order.
  Headers are field keys such as `attr:ram_gb`.
- Dates are real Excel dates, numbers carry no units, booleans are `TRUE` or `FALSE`, select attributes show the
  option label.
- No rows still gives a file with one sheet of headers.

### Report mode

`mode=report` uses the layout in the request, else the profile's, else the default (tag, name, type, status,
purchase date, bold header, frozen row, filter). Omitted layout fields are filled with defaults and then validated.

File names are `storeit-assets-YYYY-MM-DD.xlsx` for data and `<profile-name>-YYYY-MM-DD.xlsx` or
`storeit-report-YYYY-MM-DD.xlsx` for reports. If columns were skipped, the response carries an
`X-Export-Skipped-Columns` header.

## Export profiles

A profile stores a name and a report layout. It is private to its owner unless `shared` is set.

| Endpoint | Purpose |
|---|---|
| `GET /export-profiles` | Your profiles plus every shared one |
| `POST /export-profiles` | Create `{name, shared, layout}` |
| `PATCH /export-profiles/{id}` | Change name, sharing or layout; needs `version` (409 when stale) |
| `DELETE /export-profiles/{id}` | Soft delete |
| `POST /export-profiles/{id}/restore` | Undo a delete |

Exporting needs `inventory.asset.read` and `inventory.asset.export`. Changing or deleting a profile needs
`inventory.asset.export` and being its owner; changing someone else's shared profile also needs
`inventory.export_profile.manage`.
