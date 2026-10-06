# Excel export, report customization and export profiles: design

Date: 2026-10-06. Status: all four design sections approved in conversation, and the UI
proposal artifact ("StoreIt Excel Export") approved; waiting for the user's review of
this written spec, then an implementation plan. Do not implement before both.

Module: `backend/internal/inventory` (placeholders marked `TODO(M5)` in `spreadsheet/` and
`handler/imports.go`) and `frontend/src/features/assets`. Builds on `backend/docs/platform.md`,
`backend/docs/inventory.md` and `backend/docs/identity.md`.

## Scope

Sprint 3, Epic 2 (Excel data exchange) without import:

- US-07 Export the asset list to Excel (FR-04)
- US-08 Customize the exported Excel file (FR-05)
- US-09 Save and reuse an export profile (FR-05)

US-06 Import (FR-03) gets its own spec. This design keeps the **data export** format
re-importable so the import spec can read it.

## Decisions (with the user)

| Topic | Decision |
|---|---|
| Purpose of the file | Both (option C): a plain **data export** that import can read back, and customizable **report exports** described by profiles. |
| Several asset types in one export | Data export: always one sheet per asset type. Report: the profile chooses one sheet or one sheet per type. |
| Profile visibility | Personal or shared. A shared profile is usable by everyone who can export; only its owner, or someone with `inventory.export_profile.manage`, can change or delete it. |
| Permissions | New `inventory.asset.export` for **all four roles**. New `inventory.export_profile.manage` for Administrator and Authorized Manager. Exporting also needs `inventory.asset.read`. |
| Customization scope | Columns (choose, order, rename, width), value formats (dates, units, yes/no, status as name or kind), sheet layout (single or per type, sheet name, title row, summary sheet), basic formatting (header style preset, freeze, filter buttons, stripes), sort. No formulas, charts, conditional formatting, merged groups or per-cell styles. |
| Generation | Server-side, synchronous, streamed rows with excelize (approach A). A row cap (default 50,000) returns 422 above it. No background job, no stored files. |
| Frontend | Export split button on asset lists, "Export selected" in the selection bar, a report dialog with a live preview, and an Export profiles page under Configuration (as in the approved UI proposal). |

## 1. The export layout

One model describes what a file contains and how it looks. Data exports use a fixed
layout built in code; report exports take a layout inline or from a profile; profiles
store it as JSON.

```go
// inventory/domain/export.go
type ExportLayout struct {
    Columns       []ExportColumn `json:"columns"`         // order = position in the sheet
    Sheets        SheetMode      `json:"sheets"`          // "single" | "per_type"
    EachTypeAttrs bool           `json:"each_type_attrs"` // per_type: add every attribute of the sheet's type after the chosen columns
    SheetName     string         `json:"sheet_name"`      // single mode; per_type uses type names
    TitleRow      bool           `json:"title_row"`
    Summary       bool           `json:"summary"`
    Header        HeaderStyle    `json:"header"`          // "plain" | "bold" | "bold_fill"
    Freeze        bool           `json:"freeze"`
    Filter        bool           `json:"filter"`
    Stripes       bool           `json:"stripes"`
    DateFormat    DateFormat     `json:"date_format"`     // "dd/mm/yyyy" | "yyyy-mm-dd" | "d mmm yyyy"
    BoolStyle     BoolStyle      `json:"bool_style"`      // "yes_no" | "check"
    StatusAs      StatusAs       `json:"status_as"`       // "name" | "kind"
    UnitIn        UnitIn         `json:"unit_in"`         // "header" | "cell"
    Sort          string         `json:"sort"`            // "" = the request's sort; otherwise an AssetSort value
}

type ExportColumn struct {
    Field  string  `json:"field"`  // "tag" | "name" | "description" | "type" | "status" | "purchase_date" | "updated_at" | "attr:<key>"
    Header string  `json:"header"` // "" = default label
    Width  float64 `json:"width"`  // 0 = automatic (from header and first rows)
}
```

**Validation** (`layout.Validate()`, 422 `/errors/invalid-export-layout` with field errors):

- 1 to 60 columns (or 0 columns when `sheets = per_type` and `each_type_attrs`); no field
  twice; `field` is a common field or `attr:` + a key matching `^[a-z][a-z0-9_]{0,31}$`.
- `header` up to 100 characters, no control characters.
- `sheet_name` 1 to 31 characters, none of `[ ] : * ? / \` (Excel's rule); defaults to "Assets".
- Enum fields must hold one of their values; `sort` must be a valid `AssetSort`.
- `width` 0 or between 4 and 80.

**Attribute columns:** an `attr:<key>` column is resolved per sheet. In `per_type` mode it
appears only on sheets whose type has an active attribute with that key. In `single`
mode the cell is blank for assets whose type lacks it. A key that no type in the
export has (for example a removed attribute named in an old profile) is **skipped**,
not an error; skipped keys are reported to the user (section 2).

**Default headers:** the attribute label, or the common field's English label; with
`unit_in = header` a number attribute's unit is appended: `RAM (GB)`.

**Data export layout** (`domain.DataLayout(types)`, not stored, not customizable):

- One sheet per asset type present in the rows, ordered by type name; the sheet is named
  by the type **code** (stable for import; names can change).
- Columns: `tag`, `name`, `description`, `status`, `purchase_date`, then every active
  attribute of that type in display order. Headers are the field keys (`tag`, `status`,
  `attr:ram_gb`).
- Values: status by name; dates as Excel dates formatted `yyyy-mm-dd`; numbers as numbers
  without units; booleans as `TRUE`/`FALSE`; select attributes as the option label.
- Plain header, frozen first row, no title row, no summary, no filter buttons, no stripes.

## 2. API, profiles and permissions

### Endpoints (`/api/v1`, inventory module)

| Endpoint | Purpose | Permission |
|---|---|---|
| `POST /assets/export` | Build and download an `.xlsx` | `inventory.asset.read` + `inventory.asset.export` |
| `GET /export-profiles` | Your profiles and all shared ones (`owner` = `me`, `shared` filters) | `inventory.asset.export` |
| `POST /export-profiles` | Create `{ name, shared, layout }` | `inventory.asset.export` |
| `GET /export-profiles/{profileID}` | Read one (own or shared, else 404) | `inventory.asset.export` |
| `PATCH /export-profiles/{profileID}` | Change `name`, `shared`, `layout` with `version` (409 when stale) | owner; for shared ones also `inventory.export_profile.manage` |
| `DELETE /export-profiles/{profileID}` | Delete | owner; for shared ones also `inventory.export_profile.manage` |

**`POST /assets/export` body:**

```json
{
  "filters": { "q": "think", "type_id": "…", "status_id": "…", "status_kind": "in_use",
               "include_retired": false, "attr": ["ram_gb:gte:16"], "sort": "-attributes.ram_gb",
               "ids": ["…"] },
  "mode": "data",
  "profile_id": "…",
  "layout": { "…": "…" }
}
```

- `filters` takes the same fields and rules as `GET /assets` (attribute filters and
  attribute sorts need `type_id`), plus `ids` (1 to 200, the bulk-action limit) for
  "Export selected". The service converts them into the same `domain.AssetFilter` the list
  uses, so the export never holds a row the list would not show.
- `mode = "data"` ignores `profile_id` and `layout`. `mode = "report"` uses `layout` if given,
  else the profile, else a default report layout (tag, name, type, status, purchase date;
  bold header; freeze; filter).
- The response is `200` with `Content-Type:
  application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`, `Content-Disposition:
  attachment; filename="…"` (`storeit-assets-YYYY-MM-DD.xlsx` for data,
  `<profile-name-slug>-YYYY-MM-DD.xlsx` or `storeit-report-YYYY-MM-DD.xlsx` for reports) and,
  when columns were skipped, `X-Export-Skipped-Columns: warranty_until,extended_warranty`.
  `Access-Control-Expose-Headers` is not needed (same origin through the `/api` proxy).

### Profiles table (migration `00005_export_profiles.sql`)

```sql
inventory.export_profiles
  id          uuid PK
  owner_id    uuid NOT NULL          -- identity account; no FK across modules (as member_id)
  name        text NOT NULL          -- 1..100 chars, unique per owner on lower(name)
  shared      boolean NOT NULL DEFAULT false
  layout      jsonb NOT NULL         -- ExportLayout, validated by the service
  version     integer NOT NULL DEFAULT 1
  created_at, updated_at timestamptz
  UNIQUE (owner_id, lower(name))
```

The same migration inserts the two permissions and grants them: `inventory.asset.export`
to all four seeded roles, `inventory.export_profile.manage` to Administrator and
Authorized Manager. Down removes grants, permissions and the table.

Responses carry `owner: { id, name }` (name from `identity/contract.AccountReader`), `shared`,
`layout`, `version`, `can_edit` (computed for the caller) and timestamps. A stored layout
that no longer validates (an enum value removed in a later release) is returned as is and
rejected only when used, with 422 naming the field.

### Events (outbox, same transaction as the write)

- `inventory.export_profile_created`, `inventory.export_profile_updated` (field changes,
  layout as one change), `inventory.export_profile_deleted`.
- `inventory.assets_exported` after a successful export: `{ mode, profile_id?, rows,
  sheets, filters }` with aggregate type `asset_export` and a new id. Recorded after the
  file is fully written, in its own transaction; a failure to record is logged, not
  returned (the file is already sent).

### Errors

| Case | Response |
|---|---|
| Missing `inventory.asset.export` or `inventory.asset.read` | 403 |
| More rows than `INVENTORY_EXPORT_MAX_ROWS` (default 50,000) | 422 `/errors/export-too-large` with `rows` and `limit` |
| Invalid filters (same as `GET /assets`) | 422 `/errors/invalid-attribute-query`, 400 for bad params |
| Invalid layout | 422 `/errors/invalid-export-layout` with field errors (`layout.columns[3].field`) |
| Unknown or someone else's private profile | 404 `/errors/export-profile-not-found` |
| Profile name taken by the same owner | 409 `/errors/export-profile-name-taken` |
| Stale `version` | 409 `/errors/export-profile-changed` |
| Changing a shared profile you don't own without the manage permission, or someone else's private one | 403 (private ones are 404 to non-owners) |

## 3. Backend components

| Unit | Responsibility | Depends on |
|---|---|---|
| `domain/export.go` | `ExportLayout` and its enums, `Validate`, `DataLayout(types)`, `DefaultReportLayout()`, per-sheet column resolution, skipped keys, default headers | none (pure) |
| `domain/export_profile.go` | `ExportProfile`, `ExportProfileRepository`, errors, `CanEdit(actor)` rule | domain |
| `spreadsheet/writer.go` | Thin excelize wrapper: `NewWriter()`, `Sheet(name, opts)` (widths, freeze, header style), `Title`, `Header`, `Row`, `Finish` (one Excel table per sheet for filter buttons and stripes), `WriteTo(io.Writer)`. Knows nothing about assets. | excelize v2 |
| `spreadsheet/format.go` | One asset value to one cell: real Excel dates with the chosen number format, numbers with or without unit, booleans, status name or kind | domain |
| `service/export.go` | `Export(ctx, req, w)`: permissions, layout resolution, row count against the cap, grouping by type, streaming rows into the writer, the `assets_exported` event, filename and skipped columns | asset and type repositories, profile repository, writer |
| `service/export_profiles.go` | Profile CRUD with the ownership and sharing rules; validates layouts | profile repository, account reader |
| `repository/export_profile_repository.go` | sqlc queries, version check, unique name mapping, outbox events | sqlc, outbox |
| `repository/asset_repository.go` | New `Stream(ctx, filter, fn)`: runs the existing filtered and sorted list query in pages of 500 (LIMIT/OFFSET) inside one read-only `REPEATABLE READ` transaction, so every page sees the same snapshot; attribute values are loaded per page | existing list SQL |
| `handler/exports.go`, `handler/export_profiles.go` | HTTP glue; the export handler writes headers and streams the body | service |
| `config.go` (inventory) | `INVENTORY_EXPORT_MAX_ROWS` with `Validate()` (1 to 1,000,000) | platform config |

**Flow of `POST /assets/export`:**

1. Handler decodes the body and calls `service.Export`.
2. Service checks permissions, converts filters, resolves the layout (inline, profile,
   default, or data), validates it, and loads the asset types involved.
3. `Count(filter)`; over the cap returns 422 before writing.
4. For each sheet (one, or one per type in name order): open it, write the title rows
   (report name; "Generated <date> by <name> · <filter summary>"), the header row, then
   `Stream` the sheet's rows into cells.
5. Summary sheet if asked: assets per type and status kind.
6. `Finish`, write into the response, record `assets_exported`.

excelize builds the zip when the file is written, so a database error during step 4
happens before any byte is sent and becomes a normal problem response. The file is in
memory until then, about 10 to 20 MB at the cap. The filter summary is built on the
server from the filters (type name, status, search, attribute conditions with labels).

`TODO(M5)` placeholders for import (`spreadsheet/reader.go`, `spreadsheet/template.go`,
`job/commit_import.go`, `handler/imports.go`) stay as they are for the import spec.

## 4. Frontend

As in the approved UI proposal artifact.

- **Asset lists** (all assets and type lists): an **Export** split button in the toolbar.
  The main click downloads the data export of the list's current filters; the arrow menu
  has "Export report…", "Data export", up to three recent profiles (run directly) and
  "Manage profiles…".
- **Selection bar:** "Export selected" (data) and "Report from selected…" send the ticked
  ids.
- **Report dialog** (`ReportDialog.vue`, wide PrimeVue Dialog):
  - Header: profile select, owner/sharing tag, "Unsaved changes", **Save** (own profile or
    with the manage permission), **Save as…** (name, share checkbox), close.
  - Scope line: "Exporting N assets · <filter summary>".
  - Left: column editor (DataTable with row reorder, include checkbox, header input with
    the default as placeholder, Alt+↑/↓), "Add each type's attributes" in per-type mode,
    layout and formatting options.
  - Right: preview of the first 20 rows per sheet with sheet tabs (and Summary), built in
    the browser from rows the list already loaded or a first page fetched for the export
    filters, using the same formatting rules as the server.
  - A warning lists skipped attribute columns; footer has Cancel and Download.
- **Export profiles page** (`/export-profiles`, Configuration in the sidebar, needs
  `inventory.asset.export`): All / Mine / Shared with counts; rows open the dialog with
  the profile loaded; `IconAction`s run it on all assets, open it, share or unshare,
  delete (with reasons when disabled).
- **Download:** a helper in `lib/download.ts` fetches with the bearer token as a blob,
  reads the file name from `Content-Disposition` and `X-Export-Skipped-Columns`, saves it
  through a temporary object URL, and shows a toast (with the skipped columns, if any).
  422 too large shows the row count and limit.

New frontend units: `features/assets/export/` (`api.ts` for profiles and export,
`layout.ts` sheet building for the preview, `format.ts` cell formatting, components
`ExportButton.vue`, `ReportDialog.vue`, `ColumnEditor.vue`, `SheetPreview.vue`),
`features/export-profiles/pages/ExportProfilesPage.vue`, `lib/download.ts`, permission
constants in `lib/auth/permissions.ts`. Pages use the shared components
(`PageHeader`, `SegmentedFilter`, `IconAction`, …).

## Testing

**Backend**

- Domain unit tests: layout validation (each rule), `DataLayout`, per-sheet attribute
  resolution, skipped keys, default headers with units.
- Spreadsheet tests write a file and read it back with excelize: sheet names, header
  values, frozen pane, table range (filter buttons), date cells are dates with the chosen
  format, numbers stay numbers, unit placement, booleans, title rows.
- Service tests with fakes: permissions (export, read, manage), cap (422 before writing),
  profile visibility and edit rules (own, shared, manager, private of someone else),
  `ids` limit, inline layout overriding a profile, skipped columns.
- Repository DB tests: profile CRUD, unique name per owner (any case), version conflict,
  events; `Stream` returns exactly the rows `List` returns, in order, across several
  pages, with attribute values.
- HTTP DB tests: data export of mixed types gives one sheet per type code with key headers;
  report with a profile; export selected by ids; 403 without export permission; 422 with
  a low configured cap; 404 for another owner's private profile; 403 when a non-manager
  edits someone else's shared profile; migration grants (`TestSchema_Seeds`).

**Frontend**

- Vitest: `format.ts` (dates, units, booleans, status), `layout.ts` (sheets per type,
  attribute columns, each-type attributes, skipped fields), `lib/download.ts`
  (`Content-Disposition` parsing, including UTF-8 `filename*`).
- `npm run check` (types, tests, build).

**Docs:** `backend/docs/inventory.md` (endpoints, permissions, layout rules, data export
format), `frontend/README.md` (export units), Sprint 3 rows in the task docs when the
sprint is planned.

## Out of scope

Import (US-06, its own spec, reading the data export format), CSV, background or
scheduled exports, emailing files, formulas, charts, conditional formatting, per-cell
styles, role-scoped sharing, export history UI (the event is recorded for the future
activity module).
