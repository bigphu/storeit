# Excel Export Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let every role download the asset list as Excel (a re-importable data export, or a customizable report), and save report layouts as personal or shared profiles.

**Architecture:** One `ExportLayout` model (inventory `domain`) drives both export modes. `POST /assets/export` resolves filters exactly like `GET /assets`, streams the matching rows page by page from Postgres (one read-only snapshot per sheet) into an excelize workbook built by a small `spreadsheet` package, and returns the `.xlsx`. Profiles are rows in `inventory.export_profiles` with CRUD endpoints. The frontend adds an Export split button, a report dialog with a live preview, and an Export profiles page.

**Tech Stack:** Go 1.26, chi, oapi-codegen strict server, kin-openapi, pgx v5, sqlc 1.31.1, goose, excelize v2.11.0 (new); Vue 3.5, TypeScript, PrimeVue 4.5, TanStack Vue Query 5, openapi-fetch, Vitest.

**Spec:** `docs/superpowers/specs/2026-10-06-excel-export-design.md`

## Global Constraints

- Branch `feat/excel-export`. Commit after each task; messages end with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Stage explicit paths; never stage `backend/docs/tasks/`.
- Code comments and package docs in Vietnamese; UI text in English. Files use LF. When scripting edits with Python use `open(..., newline='')`.
- Never hand-edit `*.gen.go`, `repository/db/*`, or `frontend/src/lib/api/*.d.ts`; regenerate with `make generate` (in `backend/`) and `npm run gen:api` (in `frontend/`).
- Module routes: `Middlewares` order `{web.ValidateRequests(spec, base), auth.Middleware(tokens)}` stays as it is. The validator does not apply schema defaults: defaults are handled in Go (`ExportLayout.WithDefaults`).
- Transactions, outbox `Append` and events live in repositories; services never see a `pgx.Tx`.
- Permissions: `inventory.asset.export` granted to all four seeded roles; `inventory.export_profile.manage` to Administrator (`…0001`) and Authorized Manager (`…0002`). Exporting needs `inventory.asset.export` **and** `inventory.asset.read`.
- Limits: at most 60 columns per layout; header ≤ 100 chars; sheet name 1–31 chars without `[ ] : * ? / \`; column width 0 or 4–80; profile name 1–100 chars, unique per owner (any case); `ids` 1–200; rows ≤ `INVENTORY_EXPORT_MAX_ROWS` (default 50,000, allowed 1–1,000,000; 0 means default).
- Data export: one sheet per asset type named by type **code**, columns `tag, name, description, status, purchase_date` + the type's active attributes, headers are field keys (`attr:ram_gb`), ISO dates, numbers without units, booleans as TRUE/FALSE, status by name, plain header, frozen first row, no title, no summary, no filter, no stripes.
- Frontend pages use the shared components in `frontend/src/components` (`PageHeader`, `SegmentedFilter`, `IconAction`, …); PrimeVue v4 components; keep Aura colours.
- Backend tests that hit Postgres run with `CI=true go test -p 1 ./internal/inventory/...` (Docker); `make check` must pass in `backend/`, `npm run check` in `frontend/`.

## Review Focus

1. A report profile that names an attribute removed since it was saved: the export still downloads, the column is skipped, and the user is told which (header `X-Export-Skipped-Columns`, toast). Test in Task 6 (`TestExport_SkipsRemovedAttribute`) and Task 7 (HTTP).
2. "Export selected" with rows of several types and per-type sheets: each sheet holds only the selected rows of that type, never the whole type. Test in Task 6 (`TestExport_SelectedIDsPerType`).
3. Two columns ending up with the same header (user renames "Name" to "Tag"): the file must still open in Excel (table headers must be unique). Test in Task 2 (`TestWriter_DuplicateHeaders`).
4. A user without `inventory.asset.read` but with `inventory.asset.export` (custom role): 403, never an empty file. Test in Task 6 (`TestExport_Permissions`).
5. A non-ASCII profile name ("Kiểm kê quý 3") in the download file name: the browser saves a readable name. Test in Task 7 (`TestExportOverHTTP` checks `Content-Disposition` has `filename*=utf-8''`) and Task 9 (`fileNameFrom` decodes it).

---

## File Structure

Backend (`backend/`):

| File | Responsibility |
|---|---|
| `internal/inventory/domain/export.go` (new) | Layout model, enums, defaults, validation, per-sheet columns, skipped keys, `StatusKind.Label` |
| `internal/inventory/domain/export_test.go` (new) | Unit tests for the above |
| `internal/inventory/domain/export_profile.go` (new) | `ExportProfile`, inputs, `ExportRecord`, `ExportProfileRepository` |
| `internal/inventory/domain/permissions.go` | + `PermAssetExport`, `PermExportProfileManage` |
| `internal/inventory/domain/errors.go` | + export errors |
| `internal/inventory/domain/repository.go` | `AssetFilter.IDs`; `AssetRepository.Count`, `Stream` |
| `internal/inventory/spreadsheet/writer.go` (replace placeholder) | excelize wrapper: sheets, title, header, rows, styles, freeze, table |
| `internal/inventory/spreadsheet/format.go` (new) | One asset field → one cell |
| `internal/inventory/spreadsheet/*_test.go` (new) | Write and read back |
| `migrations/00005_export_profiles.sql` (new) | Table, index, permissions, grants |
| `internal/inventory/repository/queries/export_profiles.sql` (new), `queries/assets.sql` | Profile queries; `ids` filter |
| `internal/inventory/repository/export_profile_repository.go` (new) | Profile CRUD, events, `RecordExport` |
| `internal/inventory/repository/asset_repository.go` | `Count`, `Stream`, internal `page` / `count` helpers |
| `internal/inventory/contract/events.go` | Export events and payloads |
| `internal/inventory/service/export.go`, `export_profiles.go` (new) | Use cases |
| `internal/inventory/service/service.go` | Deps: `Profiles`, `Accounts`, `ExportMaxRows` |
| `internal/inventory/config.go` (new), `module.go` | `Config` (`INVENTORY_EXPORT_MAX_ROWS`), Deps wiring |
| `internal/inventory/handler/openapi.yaml`, `exports.go`, `export_profiles.go` (new), `imports.go` | API |
| `cmd/server/config.go`, `cmd/server/main.go` | Config block and module deps |
| `docs/inventory.md` | Endpoints, permissions, formats |

Frontend (`frontend/src/`):

| File | Responsibility |
|---|---|
| `lib/auth/permissions.ts` | + `AssetExport`, `ExportProfileManage` |
| `lib/api/types.ts` | + export schema aliases |
| `lib/download.ts` (+ test) | File name from `Content-Disposition`, save a blob |
| `features/assets/export/layout.ts` (+ test) | Default layout, field options, default headers, preview sheets, skipped keys |
| `features/assets/export/format.ts` (+ test) | Preview cell text |
| `features/assets/export/api.ts` | Profile hooks, `exportAssets` |
| `features/assets/export/useExport.ts` | Run an export with toasts |
| `features/assets/export/usePreviewData.ts` | Types and sample rows for the preview |
| `features/assets/export/components/{ExportButton,ReportDialog,ColumnEditor,SheetPreview}.vue` | UI |
| `features/assets/pages/AssetsPage.vue` | Toolbar button, selection bar actions |
| `features/export-profiles/pages/ExportProfilesPage.vue` | Profiles page |
| `app/routes.ts`, `app/layouts/AppSidebar.vue` | Route and Configuration link |
| `README.md` (frontend) | Export units |

---

### Task 1: Export layout model (domain)

**Files:**
- Create: `backend/internal/inventory/domain/export.go`
- Create: `backend/internal/inventory/domain/export_test.go`
- Modify: `backend/internal/inventory/domain/errors.go` (append to the `var (...)` block)
- Modify: `backend/internal/inventory/domain/permissions.go` (const block)

**Interfaces:**
- Produces:
  - `type ExportLayout struct{ Columns []ExportColumn; Sheets SheetMode; EachTypeAttrs bool; SheetName string; TitleRow, Summary bool; Header HeaderStyle; Freeze, Filter, Stripes bool; DateFormat DateFormat; BoolStyle BoolStyle; StatusAs StatusAs; UnitIn UnitIn; Sort string; KeyHeaders bool /* json:"-" */ }`
  - `type ExportColumn struct{ Field, Header string; Width float64 }`
  - `type SheetColumn struct{ Field, Key, Header string; Width float64 }`
  - `func DefaultReportLayout() ExportLayout`, `func DataLayout() ExportLayout`
  - `func (l ExportLayout) WithDefaults() ExportLayout`, `func (l ExportLayout) Validate() error`
  - `func (l ExportLayout) SheetColumns(types []AssetType) []SheetColumn`, `func (l ExportLayout) SkippedKeys(types []AssetType) []string`
  - `func AttrKey(field string) (string, bool)`, `func (k StatusKind) Label() string`
  - Enums: `SheetSingle/SheetPerType`, `HeaderPlain/HeaderBold/HeaderBoldFill`, `DateDMY/DateISO/DateDMonY`, `BoolYesNo/BoolCheck/BoolTrueFalse`, `StatusAsName/StatusAsKind`, `UnitInHeader/UnitInCell`, `MaxExportColumns = 60`
  - Errors: `ErrInvalidExportLayout`, `ErrExportTooLarge`, `ErrInvalidExportIDs`, `ErrExportProfileNotFound`, `ErrExportProfileNameTaken`, `ErrExportProfileChanged`, `ErrExportProfileForbidden`
  - Permissions: `PermAssetExport = "inventory.asset.export"`, `PermExportProfileManage = "inventory.export_profile.manage"`

- [ ] **Step 1: Write the failing tests**

`backend/internal/inventory/domain/export_test.go`:

```go
package domain

import (
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"

	"storeit/internal/platform/errs"
)

func laptopType() AssetType {
	return AssetType{ID: uuid.New(), Code: "LAPTOP", Name: "Laptop", Attributes: []Attribute{
		{Key: "cpu", Label: "CPU", DataType: TypeText, Position: 1},
		{Key: "ram_gb", Label: "RAM", DataType: TypeNumber, Unit: "GB", Position: 2},
		{Key: "old", Label: "Old", DataType: TypeText, Position: 3, RemovedAt: ptrNow()},
	}}
}

func phoneType() AssetType {
	return AssetType{ID: uuid.New(), Code: "PHONE", Name: "Phone", Attributes: []Attribute{
		{Key: "imei", Label: "IMEI", DataType: TypeText, Position: 1},
	}}
}

func fieldsOf(err error) map[string]bool {
	var e *errs.Error
	out := map[string]bool{}
	if errors.As(err, &e) {
		for _, f := range e.Fields() {
			out[f.Field] = true
		}
	}
	return out
}

func TestExportLayout_Validate(t *testing.T) {
	ok := DefaultReportLayout()
	if err := ok.Validate(); err != nil {
		t.Fatalf("default layout: %v", err)
	}
	bad := ok
	bad.Columns = []ExportColumn{{Field: "tag"}, {Field: "tag"}, {Field: "price"}, {Field: "attr:Bad Key"}, {Field: "name", Header: "a\x00b"}, {Field: "status", Width: 2}}
	bad.SheetName = "a/b"
	bad.Header = "fancy"
	bad.Sort = "price"
	err := bad.Validate()
	if !errors.Is(err, ErrInvalidExportLayout) {
		t.Fatalf("err = %v, want ErrInvalidExportLayout", err)
	}
	got := fieldsOf(err)
	for _, f := range []string{"layout.columns[1].field", "layout.columns[2].field", "layout.columns[3].field",
		"layout.columns[4].header", "layout.columns[5].width", "layout.sheet_name", "layout.header", "layout.sort"} {
		if !got[f] {
			t.Errorf("missing field error %s (got %v)", f, got)
		}
	}
	empty := ok
	empty.Columns = nil
	if !fieldsOf(empty.Validate())["layout.columns"] {
		t.Error("no columns in single mode must be rejected")
	}
	empty.Sheets, empty.EachTypeAttrs = SheetPerType, true
	if err := empty.Validate(); err != nil {
		t.Errorf("no columns with each_type_attrs: %v", err)
	}
	many := ok
	many.Columns = nil
	for i := 0; i <= MaxExportColumns; i++ {
		many.Columns = append(many.Columns, ExportColumn{Field: "attr:k" + string(rune('a'+i%26)) + string(rune('a'+i/26))})
	}
	if !fieldsOf(many.Validate())["layout.columns"] {
		t.Error("more than 60 columns must be rejected")
	}
	attrSort := ok
	attrSort.Sort = "-attributes.ram_gb"
	if err := attrSort.Validate(); err != nil {
		t.Errorf("attribute sort: %v", err)
	}
}

func TestExportLayout_WithDefaults(t *testing.T) {
	l := ExportLayout{Columns: []ExportColumn{{Field: "tag"}}}.WithDefaults()
	if l.Sheets != SheetSingle || l.SheetName != "Assets" || l.Header != HeaderBold || l.DateFormat != DateDMY ||
		l.BoolStyle != BoolYesNo || l.StatusAs != StatusAsName || l.UnitIn != UnitInHeader {
		t.Errorf("defaults = %+v", l)
	}
}

func TestExportLayout_SheetColumns(t *testing.T) {
	lap, ph := laptopType(), phoneType()
	l := DefaultReportLayout()
	l.Columns = []ExportColumn{{Field: "tag", Header: "Asset tag"}, {Field: "attr:ram_gb"}, {Field: "attr:imei"}, {Field: "attr:gone"}}

	single := l.SheetColumns([]AssetType{lap, ph})
	if got := headers(single); !slices.Equal(got, []string{"Asset tag", "RAM (GB)", "IMEI"}) {
		t.Errorf("single headers = %v", got)
	}

	l.Sheets = SheetPerType
	if got := headers(l.SheetColumns([]AssetType{ph})); !slices.Equal(got, []string{"Asset tag", "IMEI"}) {
		t.Errorf("phone sheet = %v", got)
	}
	l.EachTypeAttrs = true
	if got := headers(l.SheetColumns([]AssetType{lap})); !slices.Equal(got, []string{"Asset tag", "RAM (GB)", "CPU"}) {
		t.Errorf("laptop sheet with each type's attributes = %v (removed 'old' must not appear)", got)
	}

	l.UnitIn = UnitInCell
	if got := headers(l.SheetColumns([]AssetType{lap}))[1]; got != "RAM" {
		t.Errorf("unit in cell header = %q", got)
	}
	if got := l.SkippedKeys([]AssetType{lap, ph}); !slices.Equal(got, []string{"gone"}) {
		t.Errorf("skipped = %v", got)
	}
}

func TestDataLayout(t *testing.T) {
	d := DataLayout()
	if err := d.Validate(); err != nil {
		t.Fatalf("data layout invalid: %v", err)
	}
	got := headers(d.SheetColumns([]AssetType{laptopType()}))
	want := []string{"tag", "name", "description", "status", "purchase_date", "attr:cpu", "attr:ram_gb"}
	if !slices.Equal(got, want) {
		t.Errorf("data headers = %v, want %v", got, want)
	}
}

func headers(cols []SheetColumn) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = c.Header
	}
	return out
}
```

Check before writing the test: the data type constants and a "now pointer" helper. Run `grep -n "TypeText\|TypeNumber" backend/internal/inventory/domain/*.go` and `grep -rn "func ptrNow" backend/internal/inventory/domain`. If the constants are named differently, use the real names. If `ptrNow` does not exist, add it at the bottom of the test file:

```go
func ptrNow() *time.Time { t := time.Now(); return &t }
```

(and add `"time"` to the imports). Also check `errs.Error` exposes field errors: `grep -n "func (e \*Error) Fields" backend/internal/platform/errs/*.go`. If the accessor has another name, use it in `fieldsOf`.

- [ ] **Step 2: Run the tests to see them fail**

Run: `cd backend && go test ./internal/inventory/domain/ -run 'ExportLayout|DataLayout'`
Expected: FAIL to compile (`undefined: DefaultReportLayout`, …).

- [ ] **Step 3: Add permissions and errors**

In `domain/permissions.go`, extend the const block:

```go
	PermAssetExport         = "inventory.asset.export"          // tải danh sách tài sản dạng Excel, lưu profile của mình
	PermExportProfileManage = "inventory.export_profile.manage" // sửa, xoá profile người khác chia sẻ
```

In `domain/errors.go`, inside the `var (...)` block (keep groups by status):

```go
	ErrInvalidExportLayout = errs.Unprocessable("/errors/invalid-export-layout", "Invalid export layout")
	ErrExportTooLarge      = errs.Unprocessable("/errors/export-too-large", "Too many rows to export")
	ErrInvalidExportIDs    = errs.Unprocessable("/errors/invalid-export-selection", "Invalid selection",
		errs.WithFields(errs.FieldError{Field: "filters.ids", Detail: "choose between 1 and 200 assets"}))

	ErrExportProfileNameTaken = errs.Conflict("/errors/export-profile-name-taken", "You already have a profile with this name")
	ErrExportProfileChanged   = errs.Conflict("/errors/export-profile-changed", "Profile changed",
		errs.WithDetail("The profile was changed by someone else. Reload it and try again."))
	ErrExportProfileForbidden = errs.Forbidden("/errors/export-profile-forbidden", "Can't change this profile",
		errs.WithDetail("Only its owner, or someone who can manage export profiles, can change a shared profile."))
	ErrExportProfileNotFound = errs.NotFound("/errors/export-profile-not-found", "Export profile not found")
```

- [ ] **Step 4: Write the layout model**

`backend/internal/inventory/domain/export.go`:

```go
package domain

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"storeit/internal/platform/errs"
)

// Bố cục của file export: cột, sheet, định dạng. Export dữ liệu dùng DataLayout (cố
// định, import đọc lại được); báo cáo dùng bố cục gửi kèm hoặc của profile, lưu dạng JSON.

type SheetMode string

const (
	SheetSingle  SheetMode = "single"
	SheetPerType SheetMode = "per_type"
)

type HeaderStyle string

const (
	HeaderPlain    HeaderStyle = "plain"
	HeaderBold     HeaderStyle = "bold"
	HeaderBoldFill HeaderStyle = "bold_fill"
)

// DateFormat là định dạng số của Excel cho ô ngày
type DateFormat string

const (
	DateDMY   DateFormat = "dd/mm/yyyy"
	DateISO   DateFormat = "yyyy-mm-dd"
	DateDMonY DateFormat = "d mmm yyyy"
)

type BoolStyle string

const (
	BoolYesNo     BoolStyle = "yes_no"
	BoolCheck     BoolStyle = "check"
	BoolTrueFalse BoolStyle = "true_false" // chỉ export dữ liệu: ô kiểu boolean của Excel
)

type StatusAs string

const (
	StatusAsName StatusAs = "name"
	StatusAsKind StatusAs = "kind"
)

type UnitIn string

const (
	UnitInHeader UnitIn = "header"
	UnitInCell   UnitIn = "cell"
)

const (
	MaxExportColumns   = 60
	maxExportHeaderLen = 100
	maxSheetNameLen    = 31
	minColumnWidth     = 4
	maxColumnWidth     = 80
	attrFieldPrefix    = "attr:"
)

type ExportColumn struct {
	Field  string  `json:"field"`  // trường chung hoặc "attr:<key>"
	Header string  `json:"header"` // "" là nhãn mặc định
	Width  float64 `json:"width"`  // 0 là tự tính
}

type ExportLayout struct {
	Columns       []ExportColumn `json:"columns"`
	Sheets        SheetMode      `json:"sheets"`
	EachTypeAttrs bool           `json:"each_type_attrs"` // mỗi loại một sheet: thêm mọi thuộc tính của loại đó
	SheetName     string         `json:"sheet_name"`
	TitleRow      bool           `json:"title_row"`
	Summary       bool           `json:"summary"`
	Header        HeaderStyle    `json:"header"`
	Freeze        bool           `json:"freeze"`
	Filter        bool           `json:"filter"`
	Stripes       bool           `json:"stripes"`
	DateFormat    DateFormat     `json:"date_format"`
	BoolStyle     BoolStyle      `json:"bool_style"`
	StatusAs      StatusAs       `json:"status_as"`
	UnitIn        UnitIn         `json:"unit_in"`
	Sort          string         `json:"sort"` // "" là theo sắp của danh sách
	// KeyHeaders: tiêu đề cột là khoá trường ("tag", "attr:ram_gb"); chỉ export dữ liệu
	KeyHeaders bool `json:"-"`
}

// Trường chung xuất được và nhãn mặc định (tiếng Anh như giao diện)
var exportCommonLabels = map[string]string{
	"tag": "Tag", "name": "Name", "description": "Description", "type": "Type",
	"status": "Status", "purchase_date": "Purchase date", "updated_at": "Updated",
}

var exportSorts = []AssetSort{
	SortTag, SortTagDesc, SortName, SortNameDesc, SortPurchaseDate, SortPurchaseDateDesc,
	SortUpdatedAt, SortUpdatedAtDesc, SortAssetType, SortAssetTypeDesc, SortStatus, SortStatusDesc,
}

// AttrKey: "attr:ram_gb" trả "ram_gb", true; trường chung trả "", false
func AttrKey(field string) (string, bool) { return strings.CutPrefix(field, attrFieldPrefix) }

func columns(fields ...string) []ExportColumn {
	out := make([]ExportColumn, len(fields))
	for i, f := range fields {
		out[i] = ExportColumn{Field: f}
	}
	return out
}

// DefaultReportLayout: báo cáo khi không gửi bố cục và không chọn profile
func DefaultReportLayout() ExportLayout {
	return ExportLayout{
		Columns: columns("tag", "name", "type", "status", "purchase_date"),
		Sheets:  SheetSingle, SheetName: "Assets", Header: HeaderBold, Freeze: true, Filter: true,
		DateFormat: DateDMY, BoolStyle: BoolYesNo, StatusAs: StatusAsName, UnitIn: UnitInHeader,
	}
}

// DataLayout: export dữ liệu, cố định để import đọc lại được
func DataLayout() ExportLayout {
	return ExportLayout{
		Columns: columns("tag", "name", "description", "status", "purchase_date"),
		Sheets:  SheetPerType, EachTypeAttrs: true, SheetName: "Assets", Header: HeaderPlain, Freeze: true,
		DateFormat: DateISO, BoolStyle: BoolTrueFalse, StatusAs: StatusAsName, UnitIn: UnitInHeader,
		KeyHeaders: true,
	}
}

// WithDefaults điền trường bỏ trống (validator của API không chèn default)
func (l ExportLayout) WithDefaults() ExportLayout {
	d := DefaultReportLayout()
	if l.Sheets == "" {
		l.Sheets = d.Sheets
	}
	if strings.TrimSpace(l.SheetName) == "" {
		l.SheetName = d.SheetName
	}
	if l.Header == "" {
		l.Header = d.Header
	}
	if l.DateFormat == "" {
		l.DateFormat = d.DateFormat
	}
	if l.BoolStyle == "" {
		l.BoolStyle = d.BoolStyle
	}
	if l.StatusAs == "" {
		l.StatusAs = d.StatusAs
	}
	if l.UnitIn == "" {
		l.UnitIn = d.UnitIn
	}
	return l
}

// Validate trả mọi lỗi cùng lúc, field theo đường dẫn JSON trong body ("layout.columns[2].field")
func (l ExportLayout) Validate() error {
	var fe []errs.FieldError
	add := func(field, detail string) { fe = append(fe, errs.FieldError{Field: field, Detail: detail}) }

	switch {
	case len(l.Columns) > MaxExportColumns:
		add("layout.columns", fmt.Sprintf("at most %d columns", MaxExportColumns))
	case len(l.Columns) == 0 && !(l.Sheets == SheetPerType && l.EachTypeAttrs):
		add("layout.columns", "choose at least one column")
	}
	seen := map[string]bool{}
	for i, c := range l.Columns {
		p := fmt.Sprintf("layout.columns[%d]", i)
		key, isAttr := AttrKey(c.Field)
		switch {
		case seen[c.Field]:
			add(p+".field", "this column is already in the layout")
		case isAttr && !attrKeyPattern.MatchString(key):
			add(p+".field", "unknown attribute key")
		case !isAttr && exportCommonLabels[c.Field] == "":
			add(p+".field", "unknown field")
		}
		seen[c.Field] = true
		if utf8.RuneCountInString(c.Header) > maxExportHeaderLen || strings.ContainsFunc(c.Header, unicode.IsControl) {
			add(p+".header", fmt.Sprintf("up to %d characters, no control characters", maxExportHeaderLen))
		}
		if c.Width != 0 && (c.Width < minColumnWidth || c.Width > maxColumnWidth) {
			add(p+".width", fmt.Sprintf("0 (automatic) or between %d and %d", minColumnWidth, maxColumnWidth))
		}
	}
	if n := utf8.RuneCountInString(l.SheetName); n < 1 || n > maxSheetNameLen || strings.ContainsAny(l.SheetName, `[]:*?/\`) {
		add("layout.sheet_name", "1-31 characters, without [ ] : * ? / \\")
	}
	enum := func(field string, v string, allowed ...string) {
		if !slices.Contains(allowed, v) {
			add("layout."+field, "must be one of "+strings.Join(allowed, ", "))
		}
	}
	enum("sheets", string(l.Sheets), string(SheetSingle), string(SheetPerType))
	enum("header", string(l.Header), string(HeaderPlain), string(HeaderBold), string(HeaderBoldFill))
	enum("date_format", string(l.DateFormat), string(DateDMY), string(DateISO), string(DateDMonY))
	allowedBool := []string{string(BoolYesNo), string(BoolCheck)}
	if l.KeyHeaders {
		allowedBool = append(allowedBool, string(BoolTrueFalse))
	}
	enum("bool_style", string(l.BoolStyle), allowedBool...)
	enum("status_as", string(l.StatusAs), string(StatusAsName), string(StatusAsKind))
	enum("unit_in", string(l.UnitIn), string(UnitInHeader), string(UnitInCell))
	if l.Sort != "" {
		if _, _, ok := AssetSort(l.Sort).Attribute(); !ok && !slices.Contains(exportSorts, AssetSort(l.Sort)) {
			add("layout.sort", "unknown sort")
		}
	}
	if len(fe) > 0 {
		return ErrInvalidExportLayout.With(errs.WithFields(fe...))
	}
	return nil
}

// SheetColumn là một cột đã giải trên một sheet
type SheetColumn struct {
	Field  string
	Key    string // khoá thuộc tính; "" là trường chung
	Header string
	Width  float64
}

// SheetColumns: các cột của một sheet gồm tài sản thuộc types (một loại khi mỗi loại
// một sheet). Cột thuộc tính chỉ có khi một trong types có thuộc tính đó (đang dùng).
func (l ExportLayout) SheetColumns(types []AssetType) []SheetColumn {
	out := []SheetColumn{}
	seen := map[string]bool{}
	for _, c := range l.Columns {
		key, isAttr := AttrKey(c.Field)
		if !isAttr {
			out = append(out, SheetColumn{Field: c.Field, Header: l.header(c, exportCommonLabels[c.Field], ""), Width: c.Width})
			seen[c.Field] = true
			continue
		}
		a, ok := findAttr(types, key)
		if !ok {
			continue
		}
		out = append(out, SheetColumn{Field: c.Field, Key: key, Header: l.header(c, a.Label, a.Unit), Width: c.Width})
		seen[c.Field] = true
	}
	if l.Sheets == SheetPerType && l.EachTypeAttrs {
		for _, t := range types {
			for _, a := range t.ActiveAttributes() {
				f := attrFieldPrefix + a.Key
				if seen[f] {
					continue
				}
				seen[f] = true
				out = append(out, SheetColumn{Field: f, Key: a.Key, Header: l.header(ExportColumn{Field: f}, a.Label, a.Unit)})
			}
		}
	}
	return out
}

func (l ExportLayout) header(c ExportColumn, label, unit string) string {
	switch {
	case l.KeyHeaders:
		return c.Field
	case c.Header != "":
		return c.Header
	case unit != "" && l.UnitIn == UnitInHeader:
		return label + " (" + unit + ")"
	}
	return label
}

// SkippedKeys: khoá thuộc tính bố cục nhắc tới mà không loại nào trong types còn có
func (l ExportLayout) SkippedKeys(types []AssetType) []string {
	var out []string
	for _, c := range l.Columns {
		if key, isAttr := AttrKey(c.Field); isAttr {
			if _, ok := findAttr(types, key); !ok {
				out = append(out, key)
			}
		}
	}
	return out
}

func findAttr(types []AssetType, key string) (Attribute, bool) {
	for _, t := range types {
		for _, a := range t.ActiveAttributes() {
			if a.Key == key {
				return a, true
			}
		}
	}
	return Attribute{}, false
}

// Label: tên kind cho người đọc (báo cáo "status theo kind", sheet tổng hợp)
func (k StatusKind) Label() string {
	switch k {
	case KindAvailable:
		return "Available"
	case KindInUse:
		return "In use"
	case KindUnavailable:
		return "Unavailable"
	case KindRetired:
		return "Retired"
	}
	return string(k)
}
```

`attrKeyPattern` already exists in `asset_type.go`.

- [ ] **Step 5: Run the tests to see them pass**

Run: `cd backend && go test ./internal/inventory/domain/`
Expected: PASS (all domain tests, old and new).

- [ ] **Step 6: Commit**

```bash
git add backend/internal/inventory/domain/export.go backend/internal/inventory/domain/export_test.go backend/internal/inventory/domain/errors.go backend/internal/inventory/domain/permissions.go
git commit -m "feat(inventory): export layout model, validation and per-sheet columns

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Spreadsheet writer and cell formatting

**Files:**
- Modify (replace placeholder): `backend/internal/inventory/spreadsheet/writer.go`
- Create: `backend/internal/inventory/spreadsheet/format.go`
- Create: `backend/internal/inventory/spreadsheet/writer_test.go`, `backend/internal/inventory/spreadsheet/format_test.go`
- Modify: `backend/go.mod`, `backend/go.sum` (excelize)

**Interfaces:**
- Consumes: `domain.ExportLayout`, `domain.SheetColumn`, `domain.AssetListItem`, `domain.AssetType`, enums from Task 1.
- Produces:
  - `func NewWriter() *Writer`; `func (w *Writer) Sheet(o SheetOptions, headers []string) (*Sheet, error)`; `func (s *Sheet) Row(cells []Cell) error`; `func (s *Sheet) Close() error`; `func (w *Writer) WriteTo(dst io.Writer) (int64, error)`; `func (w *Writer) Close() error`
  - `type SheetOptions struct{ Name string; Widths []float64; Header domain.HeaderStyle; Freeze, Filter, Stripes bool; Title []string }`
  - `type Cell struct{ Kind CellKind; Text string; Num float64; Time time.Time; Bool bool; NumFmt string }`, kinds `Text, Number, Date, Bool`; helpers `TextCell(string) Cell`
  - `type Formatter struct{ Layout domain.ExportLayout; Types map[uuid.UUID]domain.AssetType; Loc *time.Location }`; `func (f Formatter) Cell(a domain.AssetListItem, c domain.SheetColumn) Cell`

- [ ] **Step 1: Add excelize**

Run: `cd backend && go get github.com/xuri/excelize/v2@v2.11.0`
Expected: `go.mod` lists `github.com/xuri/excelize/v2 v2.11.0`.

- [ ] **Step 2: Write the failing writer tests**

`backend/internal/inventory/spreadsheet/writer_test.go`:

```go
package spreadsheet

import (
	"bytes"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"storeit/internal/inventory/domain"
)

// build ghi một file rồi mở lại để đọc như Excel
func build(t *testing.T, fn func(w *Writer)) *excelize.File {
	t.Helper()
	w := NewWriter()
	defer w.Close()
	fn(w)
	var buf bytes.Buffer
	if _, err := w.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestWriter_SheetsHeaderRowsAndTypes(t *testing.T) {
	day := time.Date(2025, 3, 14, 0, 0, 0, 0, time.UTC)
	f := build(t, func(w *Writer) {
		s, err := w.Sheet(SheetOptions{Name: "Laptop", Header: domain.HeaderBold, Freeze: true, Filter: true,
			Title: []string{"Monthly laptops", "Generated 06/10/2026"}}, []string{"Tag", "RAM (GB)", "Bought", "Touch"})
		must(t, err)
		must(t, s.Row([]Cell{TextCell("LAP-0042"), {Kind: Number, Num: 32}, {Kind: Date, Time: day, NumFmt: "dd/mm/yyyy"}, {Kind: Bool, Bool: true}}))
		must(t, s.Row([]Cell{TextCell("LAP-0043"), {Kind: Number, Num: 16}, {Kind: Date, Time: day, NumFmt: "dd/mm/yyyy"}, {Kind: Bool}}))
		must(t, s.Close())
		s2, err := w.Sheet(SheetOptions{Name: "Phone"}, []string{"Tag"})
		must(t, err)
		must(t, s2.Close())
	})
	if got := f.GetSheetList(); len(got) != 2 || got[0] != "Laptop" || got[1] != "Phone" {
		t.Fatalf("sheets = %v", got)
	}
	if v, _ := f.GetCellValue("Laptop", "A1"); v != "Monthly laptops" {
		t.Errorf("title = %q", v)
	}
	if v, _ := f.GetCellValue("Laptop", "B3"); v != "RAM (GB)" {
		t.Errorf("header = %q", v)
	}
	if typ, _ := f.GetCellType("Laptop", "B4"); typ != excelize.CellTypeNumber && typ != excelize.CellTypeUnset {
		t.Errorf("number cell type = %v", typ)
	}
	if v, _ := f.GetCellValue("Laptop", "C4"); v != "14/03/2025" {
		t.Errorf("formatted date = %q", v)
	}
	if v, _ := f.GetCellValue("Laptop", "C4", excelize.Options{RawCellValue: true}); v == "" || v == "14/03/2025" {
		t.Errorf("date must be stored as a serial number, raw = %q", v)
	}
	if v, _ := f.GetCellValue("Laptop", "D4"); v != "TRUE" {
		t.Errorf("bool = %q", v)
	}
	panes, err := f.GetPanes("Laptop")
	must(t, err)
	if !panes.Freeze || panes.YSplit != 3 {
		t.Errorf("panes = %+v, want frozen below row 3", panes)
	}
	tables, err := f.GetTables("Laptop")
	must(t, err)
	if len(tables) != 1 || tables[0].Range != "A3:D5" {
		t.Errorf("tables = %+v, want one table A3:D5", tables)
	}
	if tables, _ := f.GetTables("Phone"); len(tables) != 0 {
		t.Errorf("empty sheet must not get a table: %+v", tables)
	}
}

func TestWriter_DuplicateHeaders(t *testing.T) {
	f := build(t, func(w *Writer) {
		s, err := w.Sheet(SheetOptions{Name: "Assets", Filter: true}, []string{"Tag", "Tag", "Tag"})
		must(t, err)
		must(t, s.Row([]Cell{TextCell("a"), TextCell("b"), TextCell("c")}))
		must(t, s.Close())
	})
	rows, _ := f.GetRows("Assets")
	if got := rows[0]; got[0] != "Tag" || got[1] != "Tag (2)" || got[2] != "Tag (3)" {
		t.Errorf("headers = %v, want unique", got)
	}
}

func TestWriter_SheetNames(t *testing.T) {
	f := build(t, func(w *Writer) {
		for _, n := range []string{"A/B: test", "A/B: test", "", "This name is much longer than thirty-one characters"} {
			s, err := w.Sheet(SheetOptions{Name: n}, []string{"x"})
			must(t, err)
			must(t, s.Close())
		}
	})
	want := []string{"A-B- test", "A-B- test (2)", "Sheet", "This name is much longer than t"}
	got := f.GetSheetList()
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("sheet %d = %q, want %q", i, got[i], want[i])
		}
	}
}
```

- [ ] **Step 3: Run them to see them fail**

Run: `cd backend && go test ./internal/inventory/spreadsheet/`
Expected: FAIL to compile (`undefined: NewWriter`).

- [ ] **Step 4: Write the writer**

Replace `backend/internal/inventory/spreadsheet/writer.go` entirely:

```go
// Package spreadsheet ghi file .xlsx cho export (và sau này đọc file import). writer.go
// không biết gì về tài sản: nhận tên cột, kiểu ô và giá trị; format.go đổi tài sản thành ô.
package spreadsheet

import (
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"

	"storeit/internal/inventory/domain"
)

type CellKind int

const (
	Text CellKind = iota
	Number
	Date
	Bool
)

// Cell là một ô: kiểu và giá trị; NumFmt là định dạng số của Excel cho ô ngày
type Cell struct {
	Kind   CellKind
	Text   string
	Num    float64
	Time   time.Time
	Bool   bool
	NumFmt string
}

func TextCell(s string) Cell { return Cell{Kind: Text, Text: s} }

type SheetOptions struct {
	Name    string
	Widths  []float64 // theo cột; 0 hay thiếu là tự tính theo tiêu đề
	Header  domain.HeaderStyle
	Freeze  bool
	Filter  bool // nút lọc: một bảng Excel bao tiêu đề và các dòng
	Stripes bool
	Title   []string // các dòng tiêu đề trên dòng tên cột (0, 1 hoặc 2)
}

// Writer ghi một file nhiều sheet; mỗi sheet ghi từng dòng (StreamWriter của excelize).
// Sheet trước phải Close xong mới mở sheet sau.
type Writer struct {
	f      *excelize.File
	count  int
	names  map[string]bool
	styles map[styleKey]int
	tables int
}

type styleKey struct {
	role    string // "title", "subtitle", "header:<style>", "cell"
	numFmt  string
	striped bool
}

func NewWriter() *Writer {
	return &Writer{f: excelize.NewFile(), names: map[string]bool{}, styles: map[styleKey]int{}}
}

func (w *Writer) Close() error { return w.f.Close() }

// WriteTo ghi cả file vào dst (excelize dựng file zip lúc này)
func (w *Writer) WriteTo(dst io.Writer) (int64, error) {
	if w.count == 0 {
		return 0, fmt.Errorf("spreadsheet: no sheet written")
	}
	return w.f.WriteTo(dst)
}

// Sheet là một sheet đang ghi
type Sheet struct {
	w         *Writer
	sw        *excelize.StreamWriter
	opts      SheetOptions
	cols      int
	headerRow int
	row       int
}

func (w *Writer) Sheet(o SheetOptions, headers []string) (*Sheet, error) {
	name := w.uniqueName(o.Name)
	if w.count == 0 {
		// file mới có sẵn "Sheet1": đổi tên thay vì thêm
		if err := w.f.SetSheetName("Sheet1", name); err != nil {
			return nil, fmt.Errorf("spreadsheet: rename sheet: %w", err)
		}
	} else if _, err := w.f.NewSheet(name); err != nil {
		return nil, fmt.Errorf("spreadsheet: new sheet: %w", err)
	}
	w.count++
	sw, err := w.f.NewStreamWriter(name)
	if err != nil {
		return nil, fmt.Errorf("spreadsheet: stream %s: %w", name, err)
	}
	s := &Sheet{w: w, sw: sw, opts: o, cols: max(len(headers), 1), headerRow: len(o.Title) + 1}
	headers = uniqueHeaders(headers)
	// độ rộng và khung cố định phải đặt trước dòng đầu tiên
	for i, h := range headers {
		width := 0.0
		if i < len(o.Widths) {
			width = o.Widths[i]
		}
		if width == 0 {
			width = float64(min(max(utf8.RuneCountInString(h)+4, 10), 40))
		}
		if err := sw.SetColWidth(i+1, i+1, width); err != nil {
			return nil, err
		}
	}
	if o.Freeze {
		if err := sw.SetPanes(&excelize.Panes{
			Freeze: true, YSplit: s.headerRow, TopLeftCell: fmt.Sprintf("A%d", s.headerRow+1), ActivePane: "bottomLeft",
		}); err != nil {
			return nil, err
		}
	}
	for i, line := range o.Title {
		role := "title"
		if i > 0 {
			role = "subtitle"
		}
		st, err := w.style(styleKey{role: role})
		if err != nil {
			return nil, err
		}
		if err := s.set([]any{excelize.Cell{StyleID: st, Value: line}}); err != nil {
			return nil, err
		}
		last, _ := excelize.CoordinatesToCellName(s.cols, s.row)
		if s.cols > 1 {
			if err := sw.MergeCell(fmt.Sprintf("A%d", s.row), last); err != nil {
				return nil, err
			}
		}
	}
	hs, err := w.style(styleKey{role: "header:" + string(o.Header)})
	if err != nil {
		return nil, err
	}
	cells := make([]any, len(headers))
	for i, h := range headers {
		cells[i] = excelize.Cell{StyleID: hs, Value: h}
	}
	return s, s.set(cells)
}

// Row ghi một dòng dữ liệu
func (s *Sheet) Row(cells []Cell) error {
	striped := s.opts.Stripes && (s.row-s.headerRow)%2 == 1
	out := make([]any, len(cells))
	for i, c := range cells {
		st, err := s.w.style(styleKey{role: "cell", numFmt: c.NumFmt, striped: striped})
		if err != nil {
			return err
		}
		var v any
		switch c.Kind {
		case Number:
			v = c.Num
		case Date:
			v = c.Time
		case Bool:
			v = c.Bool
		default:
			v = c.Text
		}
		out[i] = excelize.Cell{StyleID: st, Value: v}
	}
	return s.set(out)
}

// Close thêm bảng (nút lọc) nếu cần và kết thúc sheet
func (s *Sheet) Close() error {
	if s.opts.Filter && s.row > s.headerRow {
		s.w.tables++
		last, _ := excelize.CoordinatesToCellName(s.cols, s.row)
		noStripes := false
		if err := s.sw.AddTable(&excelize.Table{
			Range: fmt.Sprintf("A%d:%s", s.headerRow, last), Name: fmt.Sprintf("Assets%d", s.w.tables),
			StyleName: "TableStyleLight1", ShowRowStripes: &noStripes,
		}); err != nil {
			return fmt.Errorf("spreadsheet: add table: %w", err)
		}
	}
	return s.sw.Flush()
}

func (s *Sheet) set(values []any) error {
	s.row++
	return s.sw.SetRow(fmt.Sprintf("A%d", s.row), values)
}

func (w *Writer) style(k styleKey) (int, error) {
	if id, ok := w.styles[k]; ok {
		return id, nil
	}
	st := &excelize.Style{}
	switch k.role {
	case "title":
		st.Font = &excelize.Font{Bold: true, Size: 13}
	case "subtitle":
		st.Font = &excelize.Font{Color: "64748B", Size: 10}
	case "header:" + string(domain.HeaderBold):
		st.Font = &excelize.Font{Bold: true}
	case "header:" + string(domain.HeaderBoldFill):
		st.Font = &excelize.Font{Bold: true, Color: "064E3B"}
		st.Fill = excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"D1FAE5"}}
	}
	if k.numFmt != "" {
		f := k.numFmt
		st.CustomNumFmt = &f
	}
	if k.striped {
		st.Fill = excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"F1F5F9"}}
	}
	id, err := w.f.NewStyle(st)
	if err != nil {
		return 0, fmt.Errorf("spreadsheet: style: %w", err)
	}
	w.styles[k] = id
	return id, nil
}

// uniqueName: tên sheet hợp lệ với Excel (≤ 31 ký tự, không [ ] : * ? / \) và không trùng
func (w *Writer) uniqueName(name string) string {
	clean := strings.Map(func(r rune) rune {
		if strings.ContainsRune(`[]:*?/\`, r) {
			return '-'
		}
		return r
	}, strings.TrimSpace(name))
	if clean == "" {
		clean = "Sheet"
	}
	clean = truncate(clean, 31)
	out := clean
	for n := 2; w.names[strings.ToLower(out)]; n++ {
		suffix := fmt.Sprintf(" (%d)", n)
		out = truncate(clean, 31-len(suffix)) + suffix
	}
	w.names[strings.ToLower(out)] = true
	return out
}

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// uniqueHeaders: bảng Excel cần tên cột khác nhau; trùng thì thêm " (2)", " (3)"
func uniqueHeaders(headers []string) []string {
	out := make([]string, len(headers))
	seen := map[string]int{}
	for i, h := range headers {
		key := strings.ToLower(h)
		seen[key]++
		if seen[key] > 1 {
			h = fmt.Sprintf("%s (%d)", h, seen[key])
		}
		out[i] = h
	}
	return out
}
```

- [ ] **Step 5: Run the writer tests**

Run: `cd backend && go test ./internal/inventory/spreadsheet/ -run Writer -v`
Expected: PASS. If `f.WriteTo` is not available on the excelize version, use `w.f.Write(dst)` and return `(0, err)`; if `GetCellValue` for the date does not format the custom number format, compare the raw serial with `excelize.ExcelDateToTime` instead and keep the "stored as serial" assertion.

- [ ] **Step 6: Write the failing formatter tests**

`backend/internal/inventory/spreadsheet/format_test.go`:

```go
package spreadsheet

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"storeit/internal/inventory/domain"
)

func TestFormatter_Cells(t *testing.T) {
	ramID, touchID, osID, winID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	typ := domain.AssetType{ID: uuid.New(), Name: "Laptop", Attributes: []domain.Attribute{
		{ID: ramID, Key: "ram_gb", Label: "RAM", DataType: domain.TypeNumber, Unit: "GB"},
		{ID: touchID, Key: "touch", Label: "Touch", DataType: domain.TypeBoolean},
		{ID: osID, Key: "os", Label: "OS", DataType: domain.TypeSelect, Options: []domain.Option{{ID: winID, Label: "Windows 11"}}},
	}}
	day := time.Date(2025, 3, 14, 0, 0, 0, 0, time.UTC)
	num, yes := "16", true
	a := domain.AssetListItem{
		Asset: domain.Asset{Tag: "LAP-1", Name: "ThinkPad", TypeID: typ.ID, PurchaseDate: &day, Values: []domain.Value{
			{AttributeID: ramID, DataType: domain.TypeNumber, Number: &num},
			{AttributeID: touchID, DataType: domain.TypeBoolean, Bool: &yes},
			{AttributeID: osID, DataType: domain.TypeSelect, OptionID: &winID},
		}},
		TypeName: "Laptop", StatusName: "On loan", StatusKind: domain.KindInUse,
	}
	l := domain.DefaultReportLayout()
	f := Formatter{Layout: l, Types: map[uuid.UUID]domain.AssetType{typ.ID: typ}, Loc: time.UTC}
	col := func(field string) domain.SheetColumn {
		k, _ := domain.AttrKey(field)
		return domain.SheetColumn{Field: field, Key: k}
	}

	if c := f.Cell(a, col("attr:ram_gb")); c.Kind != Number || c.Num != 16 {
		t.Errorf("number = %+v", c)
	}
	if c := f.Cell(a, col("purchase_date")); c.Kind != Date || !c.Time.Equal(day) || c.NumFmt != "dd/mm/yyyy" {
		t.Errorf("date = %+v", c)
	}
	if c := f.Cell(a, col("attr:touch")); c.Text != "Yes" {
		t.Errorf("bool yes_no = %+v", c)
	}
	if c := f.Cell(a, col("attr:os")); c.Text != "Windows 11" {
		t.Errorf("select = %+v", c)
	}
	if c := f.Cell(a, col("status")); c.Text != "On loan" {
		t.Errorf("status name = %+v", c)
	}
	if c := f.Cell(a, col("attr:missing")); c.Kind != Text || c.Text != "" {
		t.Errorf("missing attribute = %+v", c)
	}

	f.Layout.UnitIn, f.Layout.BoolStyle, f.Layout.StatusAs = domain.UnitInCell, domain.BoolCheck, domain.StatusAsKind
	if c := f.Cell(a, col("attr:ram_gb")); c.Kind != Text || c.Text != "16 GB" {
		t.Errorf("unit in cell = %+v", c)
	}
	if c := f.Cell(a, col("attr:touch")); c.Text != "✓" {
		t.Errorf("bool check = %+v", c)
	}
	if c := f.Cell(a, col("status")); c.Text != "In use" {
		t.Errorf("status kind = %+v", c)
	}

	f.Layout = domain.DataLayout()
	if c := f.Cell(a, col("attr:touch")); c.Kind != Bool || !c.Bool {
		t.Errorf("data bool = %+v", c)
	}
	if c := f.Cell(a, col("purchase_date")); c.NumFmt != "yyyy-mm-dd" {
		t.Errorf("data date format = %+v", c)
	}
}
```

Before writing, check the real names: `grep -n "Data[A-Z][a-z]* *DataType = " backend/internal/inventory/domain/*.go` and `grep -n "TypeID\|PurchaseDate\|Values" backend/internal/inventory/domain/asset.go`. Use the real constant and field names.

- [ ] **Step 7: Run to see it fail**

Run: `cd backend && go test ./internal/inventory/spreadsheet/ -run Formatter`
Expected: FAIL to compile (`undefined: Formatter`).

- [ ] **Step 8: Write the formatter**

`backend/internal/inventory/spreadsheet/format.go`:

```go
package spreadsheet

import (
	"strconv"
	"time"

	"github.com/google/uuid"

	"storeit/internal/inventory/domain"
)

// Formatter đổi giá trị một trường của một tài sản thành ô theo bố cục. Types phải có
// thuộc tính và option của mọi loại có trong file.
type Formatter struct {
	Layout domain.ExportLayout
	Types  map[uuid.UUID]domain.AssetType
	Loc    *time.Location // múi giờ cho updated_at; nil là UTC
}

func (f Formatter) Cell(a domain.AssetListItem, c domain.SheetColumn) Cell {
	switch c.Field {
	case "tag":
		return TextCell(a.Tag)
	case "name":
		return TextCell(a.Name)
	case "description":
		return TextCell(a.Description)
	case "type":
		return TextCell(a.TypeName)
	case "status":
		if f.Layout.StatusAs == domain.StatusAsKind {
			return TextCell(a.StatusKind.Label())
		}
		return TextCell(a.StatusName)
	case "purchase_date":
		if a.PurchaseDate == nil {
			return TextCell("")
		}
		return f.date(*a.PurchaseDate)
	case "updated_at":
		loc := f.Loc
		if loc == nil {
			loc = time.UTC
		}
		t := a.UpdatedAt.In(loc)
		return Cell{Kind: Date, Time: time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, time.UTC),
			NumFmt: string(f.Layout.DateFormat) + " hh:mm"}
	}
	return f.attribute(a, c.Key)
}

func (f Formatter) date(d time.Time) Cell {
	return Cell{Kind: Date, Time: d, NumFmt: string(f.Layout.DateFormat)}
}

func (f Formatter) attribute(a domain.AssetListItem, key string) Cell {
	t, ok := f.Types[a.TypeID]
	if !ok || key == "" {
		return TextCell("")
	}
	var attr *domain.Attribute
	for i := range t.Attributes {
		if t.Attributes[i].Key == key && t.Attributes[i].RemovedAt == nil {
			attr = &t.Attributes[i]
		}
	}
	if attr == nil {
		return TextCell("")
	}
	for _, v := range a.Values {
		if v.AttributeID != attr.ID {
			continue
		}
		switch {
		case v.Text != nil:
			return TextCell(*v.Text)
		case v.Number != nil:
			if f.Layout.UnitIn == domain.UnitInCell && attr.Unit != "" {
				return TextCell(*v.Number + " " + attr.Unit)
			}
			n, err := strconv.ParseFloat(*v.Number, 64)
			if err != nil {
				return TextCell(*v.Number)
			}
			return Cell{Kind: Number, Num: n}
		case v.Date != nil:
			return f.date(*v.Date)
		case v.Bool != nil:
			return f.boolean(*v.Bool)
		case v.OptionID != nil:
			for _, o := range attr.Options {
				if o.ID == *v.OptionID {
					return TextCell(o.Label)
				}
			}
		}
	}
	return TextCell("")
}

func (f Formatter) boolean(b bool) Cell {
	switch f.Layout.BoolStyle {
	case domain.BoolTrueFalse:
		return Cell{Kind: Bool, Bool: b}
	case domain.BoolCheck:
		if b {
			return TextCell("✓")
		}
		return TextCell("–")
	}
	if b {
		return TextCell("Yes")
	}
	return TextCell("No")
}
```

- [ ] **Step 9: Run all spreadsheet tests**

Run: `cd backend && go test ./internal/inventory/spreadsheet/ -v`
Expected: PASS.

- [ ] **Step 10: Commit**

```bash
git add backend/go.mod backend/go.sum backend/internal/inventory/spreadsheet/writer.go backend/internal/inventory/spreadsheet/format.go backend/internal/inventory/spreadsheet/writer_test.go backend/internal/inventory/spreadsheet/format_test.go
git commit -m "feat(inventory): xlsx writer and asset cell formatting for export

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Profiles schema, permissions and repository

**Files:**
- Create: `backend/migrations/00005_export_profiles.sql`
- Create: `backend/internal/inventory/repository/queries/export_profiles.sql`
- Create: `backend/internal/inventory/domain/export_profile.go`
- Create: `backend/internal/inventory/repository/export_profile_repository.go`
- Create: `backend/internal/inventory/repository/export_profile_repository_db_test.go`
- Modify: `backend/internal/inventory/contract/events.go`
- Modify: `backend/internal/inventory/repository/map.go` (`uniqueErrors`)
- Modify: `backend/internal/inventory/repository/helpers_db_test.go` (`repos.profiles`)
- Modify: `backend/internal/inventory/repository/schema_db_test.go` (grants)

**Interfaces:**
- Consumes: `domain.ExportLayout` (Task 1), `ErrExportProfile*` (Task 1).
- Produces:
  - `type ExportProfile struct{ ID, OwnerID uuid.UUID; Name string; Shared bool; Layout ExportLayout; Version int32; CreatedAt, UpdatedAt time.Time }`
  - `type NewExportProfile struct{ OwnerID uuid.UUID; Name string; Shared bool; Layout ExportLayout }`
  - `type ExportProfileChange struct{ Name *string; Shared *bool; Layout *ExportLayout; Version int32 }`
  - `type ExportRecord struct{ Mode string; ProfileID *uuid.UUID; Rows int64; Sheets int; Filters map[string]any }`
  - `type ExportProfileRepository interface{ List(ctx, ownerID uuid.UUID) ([]ExportProfile, error); Get(ctx, id uuid.UUID) (ExportProfile, error); Create(ctx, in NewExportProfile) (ExportProfile, error); Update(ctx, id uuid.UUID, ch ExportProfileChange) (ExportProfile, error); Delete(ctx, id uuid.UUID) error; RecordExport(ctx, r ExportRecord) error }`
  - `repository.NewExportProfileRepository(pool *pgxpool.Pool, outbox *events.Outbox) *ExportProfileRepository`
  - Events: `contract.EventExportProfileCreated/Updated/Deleted`, `contract.EventAssetsExported`, aggregates `AggregateExportProfile = "export_profile"`, `AggregateAssetExport = "asset_export"`

- [ ] **Step 1: Migration**

`backend/migrations/00005_export_profiles.sql`:

```sql
-- Profile export (US-09): bố cục báo cáo Excel đã lưu, của riêng người tạo hoặc chia sẻ
-- cho mọi người được export. owner_id là account của identity, không khoá ngoại sang
-- module khác (như member_id). Kèm hai quyền mới và phân quyền cho role hệ thống.

-- +goose Up
CREATE TABLE inventory.export_profiles (
    id         uuid        PRIMARY KEY,
    owner_id   uuid        NOT NULL,
    name       text        NOT NULL,
    shared     boolean     NOT NULL DEFAULT false,
    layout     jsonb       NOT NULL,
    version    integer     NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT export_profiles_name_check CHECK (char_length(btrim(name)) BETWEEN 1 AND 100)
);
CREATE UNIQUE INDEX export_profiles_owner_name ON inventory.export_profiles (owner_id, lower(name));
CREATE INDEX export_profiles_shared ON inventory.export_profiles (owner_id) WHERE shared;

-- Mọi role export được; sửa profile người khác chia sẻ: Administrator, Authorized Manager
INSERT INTO identity.permissions (code, description) VALUES
    ('inventory.asset.export',          'Export assets to Excel and save export profiles'),
    ('inventory.export_profile.manage', 'Change or delete export profiles other people shared');

INSERT INTO identity.role_permissions (role_id, permission) VALUES
    ('00000000-0000-7000-8000-000000000001', 'inventory.asset.export'),
    ('00000000-0000-7000-8000-000000000001', 'inventory.export_profile.manage'),
    ('00000000-0000-7000-8000-000000000002', 'inventory.asset.export'),
    ('00000000-0000-7000-8000-000000000002', 'inventory.export_profile.manage'),
    ('00000000-0000-7000-8000-000000000003', 'inventory.asset.export'),
    ('00000000-0000-7000-8000-000000000004', 'inventory.asset.export');

-- +goose Down
DELETE FROM identity.role_permissions WHERE permission IN ('inventory.asset.export', 'inventory.export_profile.manage');
DELETE FROM identity.permissions WHERE code IN ('inventory.asset.export', 'inventory.export_profile.manage');
DROP TABLE inventory.export_profiles;
```

- [ ] **Step 2: Queries**

`backend/internal/inventory/repository/queries/export_profiles.sql`:

```sql
-- name: CreateExportProfile :one
INSERT INTO inventory.export_profiles (id, owner_id, name, shared, layout)
VALUES (@id, @owner_id, @name, @shared, @layout)
RETURNING *;

-- name: GetExportProfile :one
SELECT * FROM inventory.export_profiles WHERE id = @id;

-- name: GetExportProfileForUpdate :one
SELECT * FROM inventory.export_profiles WHERE id = @id FOR UPDATE;

-- Của mình và mọi profile được chia sẻ
-- name: ListExportProfiles :many
SELECT * FROM inventory.export_profiles
WHERE owner_id = @owner_id OR shared
ORDER BY lower(name), id;

-- Optimistic locking: 0 hàng là version đã đổi
-- name: UpdateExportProfile :one
UPDATE inventory.export_profiles
SET name = @name, shared = @shared, layout = @layout, version = version + 1, updated_at = now()
WHERE id = @id AND version = @version
RETURNING *;

-- name: DeleteExportProfile :execrows
DELETE FROM inventory.export_profiles WHERE id = @id;
```

Run: `cd backend && make sqlc`
Expected: `repository/db/export_profiles.sql.go` generated; `db.InventoryExportProfile` has `Layout []byte`.

- [ ] **Step 3: Domain types**

`backend/internal/inventory/domain/export_profile.go`:

```go
package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ExportProfile là một bố cục báo cáo đã lưu; Shared thì mọi người export được đều thấy
type ExportProfile struct {
	ID        uuid.UUID
	OwnerID   uuid.UUID
	Name      string
	Shared    bool
	Layout    ExportLayout
	Version   int32
	CreatedAt time.Time
	UpdatedAt time.Time
}

type NewExportProfile struct {
	OwnerID uuid.UUID
	Name    string
	Shared  bool
	Layout  ExportLayout
}

// ExportProfileChange: field nil là giữ nguyên; Version cũ là ErrExportProfileChanged
type ExportProfileChange struct {
	Name    *string
	Shared  *bool
	Layout  *ExportLayout
	Version int32
}

// ExportRecord: một lần export, ghi thành event assets_exported cho lịch sử sau này
type ExportRecord struct {
	Mode      string
	ProfileID *uuid.UUID
	Rows      int64
	Sheets    int
	Filters   map[string]any
}

type ExportProfileRepository interface {
	// List: của owner và mọi profile được chia sẻ, theo tên
	List(ctx context.Context, ownerID uuid.UUID) ([]ExportProfile, error)
	// Get: ErrExportProfileNotFound
	Get(ctx context.Context, id uuid.UUID) (ExportProfile, error)
	// Create: ErrExportProfileNameTaken; event export_profile_created
	Create(ctx context.Context, in NewExportProfile) (ExportProfile, error)
	// Update: ErrExportProfileChanged, ErrExportProfileNameTaken; event export_profile_updated
	Update(ctx context.Context, id uuid.UUID, ch ExportProfileChange) (ExportProfile, error)
	// Delete: ErrExportProfileNotFound; event export_profile_deleted
	Delete(ctx context.Context, id uuid.UUID) error
	// RecordExport ghi event assets_exported trong transaction riêng
	RecordExport(ctx context.Context, r ExportRecord) error
}
```

- [ ] **Step 4: Events**

Append to the const block in `backend/internal/inventory/contract/events.go`:

```go
	EventExportProfileCreated = "inventory.export_profile_created"
	EventExportProfileUpdated = "inventory.export_profile_updated"
	EventExportProfileDeleted = "inventory.export_profile_deleted"
	EventAssetsExported       = "inventory.assets_exported"
```

to the aggregate const block:

```go
	AggregateExportProfile = "export_profile"
	AggregateAssetExport   = "asset_export"
```

and at the end of the file:

```go
type ExportProfileCreated struct {
	ProfileID uuid.UUID `json:"profile_id"`
	Name      string    `json:"name"`
	Shared    bool      `json:"shared"`
}

// ExportProfileUpdated: bố cục đổi thì một FieldChange "layout" không kèm giá trị
type ExportProfileUpdated struct {
	ProfileID uuid.UUID     `json:"profile_id"`
	Changes   []FieldChange `json:"changes"`
}

type ExportProfileDeleted struct {
	ProfileID uuid.UUID `json:"profile_id"`
}

// AssetsExported: ai export gì (người làm là actor của event)
type AssetsExported struct {
	Mode      string         `json:"mode"`
	ProfileID *uuid.UUID     `json:"profile_id,omitempty"`
	Rows      int64          `json:"rows"`
	Sheets    int            `json:"sheets"`
	Filters   map[string]any `json:"filters"`
}
```

- [ ] **Step 5: Write the failing repository tests**

Add `profiles *repository.ExportProfileRepository` to `repos` in `helpers_db_test.go` and set it in `newRepos`: `profiles: repository.NewExportProfileRepository(pool, outbox),`.

`backend/internal/inventory/repository/export_profile_repository_db_test.go`:

```go
package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"storeit/internal/inventory/contract"
	"storeit/internal/inventory/domain"
)

func TestExportProfiles_CRUD(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	owner, other := uuid.New(), uuid.New()
	layout := domain.DefaultReportLayout()
	layout.TitleRow = true

	p, err := r.profiles.Create(ctx, domain.NewExportProfile{OwnerID: owner, Name: "Monthly " + uniq(), Layout: layout})
	if err != nil {
		t.Fatal(err)
	}
	if !p.Layout.TitleRow || p.Version != 1 {
		t.Errorf("created = %+v", p)
	}
	if _, err := r.profiles.Create(ctx, domain.NewExportProfile{OwnerID: owner, Name: p.Name + "", Layout: layout}); !errors.Is(err, domain.ErrExportProfileNameTaken) {
		t.Errorf("same name same owner: %v", err)
	}
	if _, err := r.profiles.Create(ctx, domain.NewExportProfile{OwnerID: other, Name: p.Name, Layout: layout}); err != nil {
		t.Errorf("same name other owner: %v", err)
	}
	shared, err := r.profiles.Create(ctx, domain.NewExportProfile{OwnerID: other, Name: "Shared " + uniq(), Shared: true, Layout: layout})
	if err != nil {
		t.Fatal(err)
	}

	list, err := r.profiles.List(context.Background(), owner)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[uuid.UUID]bool{}
	for _, x := range list {
		ids[x.ID] = true
	}
	if !ids[p.ID] || !ids[shared.ID] {
		t.Errorf("list misses own or shared profile")
	}

	name, sh := "Renamed "+uniq(), true
	up, err := r.profiles.Update(ctx, p.ID, domain.ExportProfileChange{Name: &name, Shared: &sh, Version: p.Version})
	if err != nil || up.Name != name || !up.Shared || up.Version != 2 {
		t.Fatalf("update = %+v, %v", up, err)
	}
	if _, err := r.profiles.Update(ctx, p.ID, domain.ExportProfileChange{Name: &name, Version: 1}); !errors.Is(err, domain.ErrExportProfileChanged) {
		t.Errorf("stale version: %v", err)
	}
	if err := r.profiles.Delete(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.profiles.Get(context.Background(), p.ID); !errors.Is(err, domain.ErrExportProfileNotFound) {
		t.Errorf("get deleted: %v", err)
	}
	for _, typ := range []string{contract.EventExportProfileCreated, contract.EventExportProfileUpdated, contract.EventExportProfileDeleted} {
		if countEvents(t, r, typ, p.ID) != 1 {
			t.Errorf("want one %s event", typ)
		}
	}
	if err := r.profiles.RecordExport(ctx, domain.ExportRecord{Mode: "data", Rows: 3, Sheets: 1, Filters: map[string]any{"q": "x"}}); err != nil {
		t.Errorf("record export: %v", err)
	}
}
```

Check `countEvents`'s signature in `helpers_db_test.go` (it may return `(int, *uuid.UUID)`); adapt the comparison.

In `schema_db_test.go` add a check (inside the seeds test, or as a new test) that Employee (`…0004`) holds `inventory.asset.export` and Administrator holds `inventory.export_profile.manage`:

```go
func TestSchema_ExportGrants(t *testing.T) {
	r := newRepos(t)
	for role, perm := range map[string]string{
		"00000000-0000-7000-8000-000000000004": "inventory.asset.export",
		"00000000-0000-7000-8000-000000000001": "inventory.export_profile.manage",
		"00000000-0000-7000-8000-000000000002": "inventory.export_profile.manage",
	} {
		var n int
		if err := r.pool.QueryRow(context.Background(),
			`SELECT count(*) FROM identity.role_permissions WHERE role_id = $1 AND permission = $2`, role, perm).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Errorf("role %s lacks %s", role, perm)
		}
	}
}
```

- [ ] **Step 6: Run to see them fail**

Run: `cd backend && CI=true go test -p 1 ./internal/inventory/repository/ -run 'ExportProfiles|ExportGrants'`
Expected: FAIL to compile (`undefined: repository.NewExportProfileRepository`).

- [ ] **Step 7: Repository**

Add to `uniqueErrors` in `map.go`: `"export_profiles_owner_name": domain.ErrExportProfileNameTaken,`

`backend/internal/inventory/repository/export_profile_repository.go`:

```go
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"storeit/internal/inventory/contract"
	"storeit/internal/inventory/domain"
	"storeit/internal/inventory/repository/db"
	"storeit/internal/platform/database"
	"storeit/internal/platform/events"
)

// ExportProfileRepository cài đặt domain.ExportProfileRepository
type ExportProfileRepository struct {
	pool   *pgxpool.Pool
	q      *db.Queries
	outbox *events.Outbox
}

var _ domain.ExportProfileRepository = (*ExportProfileRepository)(nil)

func NewExportProfileRepository(pool *pgxpool.Pool, outbox *events.Outbox) *ExportProfileRepository {
	return &ExportProfileRepository{pool: pool, q: db.New(pool), outbox: outbox}
}

func (r *ExportProfileRepository) List(ctx context.Context, ownerID uuid.UUID) ([]domain.ExportProfile, error) {
	rows, err := r.q.ListExportProfiles(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("inventory: list export profiles: %w", err)
	}
	out := make([]domain.ExportProfile, 0, len(rows))
	for _, row := range rows {
		p, err := toExportProfile(row)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (r *ExportProfileRepository) Get(ctx context.Context, id uuid.UUID) (domain.ExportProfile, error) {
	return profileOrNotFound(r.q.GetExportProfile(ctx, id))
}

func (r *ExportProfileRepository) Create(ctx context.Context, in domain.NewExportProfile) (domain.ExportProfile, error) {
	id, err := newID()
	if err != nil {
		return domain.ExportProfile{}, err
	}
	layout, err := json.Marshal(in.Layout)
	if err != nil {
		return domain.ExportProfile{}, fmt.Errorf("inventory: encode layout: %w", err)
	}
	var out domain.ExportProfile
	err = database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		row, err := r.q.WithTx(tx).CreateExportProfile(ctx, db.CreateExportProfileParams{
			ID: id, OwnerID: in.OwnerID, Name: in.Name, Shared: in.Shared, Layout: layout,
		})
		if err != nil {
			return mapWriteErr(err, "create export profile")
		}
		if out, err = toExportProfile(row); err != nil {
			return err
		}
		return appendEvent(ctx, tx, r.outbox, contract.EventExportProfileCreated, contract.AggregateExportProfile, id,
			contract.ExportProfileCreated{ProfileID: id, Name: in.Name, Shared: in.Shared})
	})
	return out, err
}

func (r *ExportProfileRepository) Update(ctx context.Context, id uuid.UUID, ch domain.ExportProfileChange) (domain.ExportProfile, error) {
	var out domain.ExportProfile
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		cur, err := profileOrNotFound(q.GetExportProfileForUpdate(ctx, id))
		if err != nil {
			return err
		}
		if cur.Version != ch.Version {
			return domain.ErrExportProfileChanged
		}
		name, shared, layout := cur.Name, cur.Shared, cur.Layout
		var changes []contract.FieldChange
		if ch.Name != nil && *ch.Name != cur.Name {
			changes = append(changes, change("name", cur.Name, *ch.Name))
			name = *ch.Name
		}
		if ch.Shared != nil && *ch.Shared != cur.Shared {
			changes = append(changes, change("shared", cur.Shared, *ch.Shared))
			shared = *ch.Shared
		}
		if ch.Layout != nil {
			changes = append(changes, contract.FieldChange{Field: "layout"})
			layout = *ch.Layout
		}
		raw, err := json.Marshal(layout)
		if err != nil {
			return fmt.Errorf("inventory: encode layout: %w", err)
		}
		row, err := q.UpdateExportProfile(ctx, db.UpdateExportProfileParams{
			ID: id, Name: name, Shared: shared, Layout: raw, Version: ch.Version,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrExportProfileChanged
		}
		if err != nil {
			return mapWriteErr(err, "update export profile")
		}
		if out, err = toExportProfile(row); err != nil {
			return err
		}
		return appendEvent(ctx, tx, r.outbox, contract.EventExportProfileUpdated, contract.AggregateExportProfile, id,
			contract.ExportProfileUpdated{ProfileID: id, Changes: changes})
	})
	return out, err
}

func (r *ExportProfileRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		n, err := r.q.WithTx(tx).DeleteExportProfile(ctx, id)
		if err != nil {
			return fmt.Errorf("inventory: delete export profile: %w", err)
		}
		if n == 0 {
			return domain.ErrExportProfileNotFound
		}
		return appendEvent(ctx, tx, r.outbox, contract.EventExportProfileDeleted, contract.AggregateExportProfile, id,
			contract.ExportProfileDeleted{ProfileID: id})
	})
}

func (r *ExportProfileRepository) RecordExport(ctx context.Context, rec domain.ExportRecord) error {
	id, err := newID()
	if err != nil {
		return err
	}
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		return appendEvent(ctx, tx, r.outbox, contract.EventAssetsExported, contract.AggregateAssetExport, id,
			contract.AssetsExported{Mode: rec.Mode, ProfileID: rec.ProfileID, Rows: rec.Rows, Sheets: rec.Sheets, Filters: rec.Filters})
	})
}

func profileOrNotFound(row db.InventoryExportProfile, err error) (domain.ExportProfile, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ExportProfile{}, domain.ErrExportProfileNotFound
	}
	if err != nil {
		return domain.ExportProfile{}, fmt.Errorf("inventory: get export profile: %w", err)
	}
	return toExportProfile(row)
}

func toExportProfile(row db.InventoryExportProfile) (domain.ExportProfile, error) {
	var l domain.ExportLayout
	if err := json.Unmarshal(row.Layout, &l); err != nil {
		return domain.ExportProfile{}, fmt.Errorf("inventory: decode layout of profile %s: %w", row.ID, err)
	}
	return domain.ExportProfile{
		ID: row.ID, OwnerID: row.OwnerID, Name: row.Name, Shared: row.Shared, Layout: l,
		Version: row.Version, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}
```

- [ ] **Step 8: Run the tests**

Run: `cd backend && CI=true go test -p 1 ./internal/inventory/repository/ -run 'ExportProfiles|ExportGrants|Schema' -v`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add backend/migrations/00005_export_profiles.sql backend/internal/inventory/repository backend/internal/inventory/domain/export_profile.go backend/internal/inventory/contract/events.go backend/internal/platform/events/db/models.go
git commit -m "feat(inventory): export profiles table, permissions and repository

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

(`make sqlc` regenerates `models.go` in every package that reads the schema; add any changed `*/db/models.go` the status lists.)

---

### Task 4: Count, selection filter and streaming rows

**Files:**
- Modify: `backend/internal/inventory/repository/queries/assets.sql` (`ListAssets`, `CountAssets`)
- Modify: `backend/internal/inventory/domain/repository.go` (`AssetFilter`, `AssetRepository`)
- Modify: `backend/internal/inventory/repository/asset_repository.go`
- Modify: `backend/internal/inventory/service/fakes_test.go` (fake `Count`, `Stream`)
- Test: `backend/internal/inventory/repository/asset_repository_db_test.go`

**Interfaces:**
- Produces:
  - `AssetFilter.IDs []uuid.UUID` (nil = no filter)
  - `AssetRepository.Count(ctx, f AssetFilter) (int64, error)`
  - `AssetRepository.Stream(ctx, f AssetFilter, pageSize int32, fn func([]AssetListItem) error) error` — always loads attribute values; pages share one read-only snapshot.

- [ ] **Step 1: Write the failing test**

Append to `asset_repository_db_test.go`:

```go
func TestAssets_StreamMatchesListAndIDs(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	typ := laptop(t, r)
	var made []domain.Asset
	for range 7 {
		a, err := r.assets.Create(ctx, "ST-"+uniq(), domain.AssetFields{Name: "x", TypeID: typ.ID, StatusID: domain.AvailableStatusID, Values: fullValues(t, typ)})
		if err != nil {
			t.Fatal(err)
		}
		made = append(made, a)
	}
	f := domain.AssetFilter{TypeID: &typ.ID, Sort: domain.SortTag}
	want, total, err := r.assets.List(context.Background(), domain.AssetFilter{TypeID: &typ.ID, Sort: domain.SortTag, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	var got []domain.AssetListItem
	pages := 0
	err = r.assets.Stream(context.Background(), f, 3, func(items []domain.AssetListItem) error {
		pages++
		got = append(got, items...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if int64(len(got)) != total || pages != 3 {
		t.Fatalf("stream = %d rows in %d pages, want %d rows in 3 pages", len(got), pages, total)
	}
	for i := range got {
		if got[i].ID != want[i].ID {
			t.Fatalf("row %d = %s, want %s", i, got[i].Tag, want[i].Tag)
		}
		if len(got[i].Values) == 0 {
			t.Fatalf("row %d has no attribute values", i)
		}
	}
	if n, err := r.assets.Count(context.Background(), f); err != nil || n != total {
		t.Errorf("count = %d, %v, want %d", n, err, total)
	}
	sel := domain.AssetFilter{TypeID: &typ.ID, IDs: []uuid.UUID{made[0].ID, made[3].ID}}
	if n, _ := r.assets.Count(context.Background(), sel); n != 2 {
		t.Errorf("count by ids = %d, want 2", n)
	}
}
```

(`laptop`, `fullValues`, `uniq`, `actorCtx` already exist in the repository test helpers.)

- [ ] **Step 2: Run to see it fail**

Run: `cd backend && go vet ./internal/inventory/repository/`
Expected: FAIL (`r.assets.Stream undefined`, `unknown field IDs`).

- [ ] **Step 3: SQL**

In `queries/assets.sql`, in **both** `ListAssets` and `CountAssets`, add after the line `AND (@include_retired::boolean OR a.retired_at IS NULL)`:

```sql
  AND (sqlc.narg('ids')::uuid[] IS NULL OR a.id = ANY(sqlc.narg('ids')::uuid[]))
```

Run: `cd backend && make sqlc`
Expected: `ListAssetsParams` and `CountAssetsParams` gain `Ids []uuid.UUID`.

- [ ] **Step 4: Domain**

In `domain/repository.go`, add to `AssetFilter` after `IncludeRetired bool`:

```go
	// IDs: chỉ các tài sản này ("Export selected"); nil là không lọc
	IDs []uuid.UUID
```

and to `AssetRepository` after `List`:

```go
	// Count: số dòng List sẽ trả với cùng bộ lọc (bỏ qua Limit, Offset)
	Count(ctx context.Context, f AssetFilter) (int64, error)
	// Stream đọc mọi dòng của bộ lọc theo trang pageSize, kèm giá trị thuộc tính, trong
	// một transaction chỉ đọc REPEATABLE READ (mọi trang cùng một ảnh dữ liệu)
	Stream(ctx context.Context, f AssetFilter, pageSize int32, fn func([]AssetListItem) error) error
```

- [ ] **Step 5: Repository**

In `asset_repository.go`, split `List` into helpers that take the querier. Replace the body of `List` from `var q *string` down to `return out, total, nil` with calls to two new methods, and add `Count` and `Stream`:

```go
func (r *AssetRepository) List(ctx context.Context, f domain.AssetFilter) ([]domain.AssetListItem, int64, error) {
	items, err := r.page(ctx, r.q, f)
	if err != nil {
		return nil, 0, err
	}
	total, err := r.count(ctx, r.q, f)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *AssetRepository) Count(ctx context.Context, f domain.AssetFilter) (int64, error) {
	return r.count(ctx, r.q, f)
}

func (r *AssetRepository) Stream(ctx context.Context, f domain.AssetFilter, pageSize int32, fn func([]domain.AssetListItem) error) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return fmt.Errorf("inventory: begin export read: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := r.q.WithTx(tx)
	f.IncludeValues, f.Limit = true, pageSize
	for f.Offset = 0; ; f.Offset += pageSize {
		items, err := r.page(ctx, q, f)
		if err != nil {
			return err
		}
		if len(items) > 0 {
			if err := fn(items); err != nil {
				return err
			}
		}
		if int32(len(items)) < pageSize {
			break
		}
	}
	return tx.Commit(ctx)
}

// page: một trang của danh sách (truy vấn ListAssets), kèm giá trị khi IncludeValues
func (r *AssetRepository) page(ctx context.Context, q *db.Queries, f domain.AssetFilter) ([]domain.AssetListItem, error) {
	args := listArgs(f)
	rows, err := q.ListAssets(ctx, db.ListAssetsParams{
		Q: args.q, TypeID: f.TypeID, StatusID: f.StatusID, StatusKind: args.kind, LocationID: f.LocationID,
		HolderMemberID: f.HolderMemberID, IncludeRetired: f.IncludeRetired, Ids: f.IDs,
		FAttrs: args.fAttrs, FOps: args.fOps, FVals: args.fVals,
		SortAttr: args.sortAttr, Sort: args.sort, Lim: f.Limit, Off: f.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("inventory: list assets: %w", err)
	}
	out := make([]domain.AssetListItem, len(rows))
	for i, row := range rows {
		out[i] = domain.AssetListItem{
			Asset: toAsset(db.InventoryAsset{
				ID: row.ID, Tag: row.Tag, Name: row.Name, Description: row.Description, AssetTypeID: row.AssetTypeID,
				StatusID: row.StatusID, LocationID: row.LocationID, HolderMemberID: row.HolderMemberID,
				PurchaseDate: row.PurchaseDate, RetiredAt: row.RetiredAt, RetiredReason: row.RetiredReason,
				Version: row.Version, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
			}),
			TypeName: row.TypeName, StatusName: row.StatusName, StatusKind: domain.StatusKind(row.StatusKind),
		}
	}
	if f.IncludeValues && len(out) > 0 {
		if err := r.attachValues(ctx, q, out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r *AssetRepository) count(ctx context.Context, q *db.Queries, f domain.AssetFilter) (int64, error) {
	args := listArgs(f)
	total, err := q.CountAssets(ctx, db.CountAssetsParams{
		Q: args.q, TypeID: f.TypeID, StatusID: f.StatusID, StatusKind: args.kind, LocationID: f.LocationID,
		HolderMemberID: f.HolderMemberID, IncludeRetired: f.IncludeRetired, Ids: f.IDs,
		FAttrs: args.fAttrs, FOps: args.fOps, FVals: args.fVals,
	})
	if err != nil {
		return 0, fmt.Errorf("inventory: count assets: %w", err)
	}
	return total, nil
}

type listArguments struct {
	q, kind              *string
	sort                 string
	sortAttr             *uuid.UUID
	fAttrs               []uuid.UUID
	fOps, fVals          []string
}

// listArgs đổi bộ lọc thành tham số chung của ListAssets và CountAssets
func listArgs(f domain.AssetFilter) listArguments {
	a := listArguments{sort: string(f.Sort)}
	if f.Query != "" {
		a.q = ptr(likeEscaper.Replace(f.Query))
	}
	if f.StatusKind != nil {
		a.kind = ptr(string(*f.StatusKind))
	}
	switch {
	case f.AttrOrder != nil:
		// khoá sắp theo kiểu dữ liệu: "attr_number", "-attr_date"...
		a.sort, a.sortAttr = "attr_"+string(f.AttrOrder.DataType), &f.AttrOrder.AttributeID
		if f.AttrOrder.Desc {
			a.sort = "-" + a.sort
		}
	case a.sort == "":
		a.sort = string(domain.SortTag)
	}
	a.fAttrs, a.fOps, a.fVals = attrFilterArgs(f.AttrFilters)
	return a
}
```

Change `attachValues` to take the querier: signature `func (r *AssetRepository) attachValues(ctx context.Context, q *db.Queries, items []domain.AssetListItem) error`, and inside use `q.ListValuesForAssets(...)` instead of `r.q.ListValuesForAssets(...)`. Make sure `pgx` is imported in this file.

- [ ] **Step 6: Service fakes**

In `service/fakes_test.go`, add to `fakeAssets` (rows of all assets in the map, filtered by type and ids; enough for service tests):

```go
func (f *fakeAssets) matching(filter domain.AssetFilter) []domain.AssetListItem {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.AssetListItem
	for _, a := range f.assets {
		if filter.TypeID != nil && a.TypeID != *filter.TypeID {
			continue
		}
		if filter.IDs != nil && !slices.Contains(filter.IDs, a.ID) {
			continue
		}
		if a.Retired() && !filter.IncludeRetired {
			continue
		}
		out = append(out, domain.AssetListItem{Asset: a, StatusName: "Available", StatusKind: domain.KindAvailable})
	}
	slices.SortFunc(out, func(x, y domain.AssetListItem) int { return strings.Compare(x.Tag, y.Tag) })
	return out
}

func (f *fakeAssets) Count(_ context.Context, filter domain.AssetFilter) (int64, error) {
	return int64(len(f.matching(filter))), nil
}

func (f *fakeAssets) Stream(_ context.Context, filter domain.AssetFilter, size int32, fn func([]domain.AssetListItem) error) error {
	rows := f.matching(filter)
	for i := 0; i < len(rows); i += int(size) {
		if err := fn(rows[i:min(i+int(size), len(rows))]); err != nil {
			return err
		}
	}
	return nil
}
```

(add `"slices"` and `"strings"` imports if missing).

- [ ] **Step 7: Run the tests**

Run: `cd backend && go build ./... && go vet ./internal/inventory/... && CI=true go test -p 1 ./internal/inventory/...`
Expected: PASS (all inventory tests, including the existing list tests that now go through `page` / `count`).

- [ ] **Step 8: Commit**

```bash
git add backend/internal/inventory/repository backend/internal/inventory/domain/repository.go backend/internal/inventory/service/fakes_test.go
git commit -m "feat(inventory): count, selection filter and snapshot streaming of assets

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Export profiles service

**Files:**
- Modify: `backend/internal/inventory/service/service.go` (Deps, fields)
- Create: `backend/internal/inventory/service/export_profiles.go`
- Create: `backend/internal/inventory/service/export_profiles_test.go`
- Modify: `backend/internal/inventory/service/fakes_test.go` (fake profiles and accounts)
- Modify: `backend/internal/inventory/service/service_test.go` (`newEnv`, permission table)

**Interfaces:**
- Consumes: `domain.ExportProfileRepository` (Task 3), `contract.AccountReader` from `storeit/internal/identity/contract`.
- Produces:
  - `service.Deps{…, Profiles domain.ExportProfileRepository, Accounts idcontract.AccountReader, ExportMaxRows int}`
  - `type ExportProfileView struct{ domain.ExportProfile; OwnerName string; CanEdit bool }`
  - `type ExportProfileInput struct{ Name string; Shared bool; Layout domain.ExportLayout }`
  - `ListExportProfiles(ctx) ([]ExportProfileView, error)`, `GetExportProfile(ctx, id) (ExportProfileView, error)`, `CreateExportProfile(ctx, in ExportProfileInput) (ExportProfileView, error)`, `UpdateExportProfile(ctx, id, ch domain.ExportProfileChange) (ExportProfileView, error)`, `DeleteExportProfile(ctx, id) error`
  - unexported `(s *Service) visibleProfile(ctx, actor, id) (domain.ExportProfile, error)` used by Task 6

- [ ] **Step 1: Wire Deps**

In `service/service.go`:

```go
import idcontract "storeit/internal/identity/contract"

type Deps struct {
	Types    domain.TypeRepository
	Statuses domain.StatusRepository
	Assets   domain.AssetRepository
	Profiles domain.ExportProfileRepository
	Accounts idcontract.AccountReader // tên chủ profile, tên người export trong tiêu đề báo cáo
	// ExportMaxRows: số dòng tối đa một lần export; 0 là mặc định 50.000
	ExportMaxRows int
}

type Service struct {
	types         domain.TypeRepository
	statuses      domain.StatusRepository
	assets        domain.AssetRepository
	profiles      domain.ExportProfileRepository
	accounts      idcontract.AccountReader
	exportMaxRows int
}

func New(d Deps) *Service {
	max := d.ExportMaxRows
	if max == 0 {
		max = defaultExportMaxRows
	}
	return &Service{types: d.Types, statuses: d.Statuses, assets: d.Assets, profiles: d.Profiles, accounts: d.Accounts, exportMaxRows: max}
}
```

`defaultExportMaxRows` is declared in Task 6's `export.go`; to compile now, declare it in `export_profiles.go` for this task and move nothing later (Task 6 uses it):

```go
// Số dòng tối đa một lần export khi cấu hình để 0
const defaultExportMaxRows = 50_000
```

- [ ] **Step 2: Fakes and test env**

In `fakes_test.go`:

```go
type fakeProfiles struct {
	mu       sync.Mutex
	profiles map[uuid.UUID]domain.ExportProfile
	records  []domain.ExportRecord
}

func newFakeProfiles() *fakeProfiles { return &fakeProfiles{profiles: map[uuid.UUID]domain.ExportProfile{}} }

func (f *fakeProfiles) List(_ context.Context, owner uuid.UUID) ([]domain.ExportProfile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.ExportProfile
	for _, p := range f.profiles {
		if p.OwnerID == owner || p.Shared {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fakeProfiles) Get(_ context.Context, id uuid.UUID) (domain.ExportProfile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.profiles[id]
	if !ok {
		return p, domain.ErrExportProfileNotFound
	}
	return p, nil
}

func (f *fakeProfiles) Create(_ context.Context, in domain.NewExportProfile) (domain.ExportProfile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range f.profiles {
		if p.OwnerID == in.OwnerID && strings.EqualFold(p.Name, in.Name) {
			return domain.ExportProfile{}, domain.ErrExportProfileNameTaken
		}
	}
	p := domain.ExportProfile{ID: uuid.New(), OwnerID: in.OwnerID, Name: in.Name, Shared: in.Shared, Layout: in.Layout, Version: 1}
	f.profiles[p.ID] = p
	return p, nil
}

func (f *fakeProfiles) Update(_ context.Context, id uuid.UUID, ch domain.ExportProfileChange) (domain.ExportProfile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.profiles[id]
	if !ok {
		return p, domain.ErrExportProfileNotFound
	}
	if p.Version != ch.Version {
		return p, domain.ErrExportProfileChanged
	}
	if ch.Name != nil {
		p.Name = *ch.Name
	}
	if ch.Shared != nil {
		p.Shared = *ch.Shared
	}
	if ch.Layout != nil {
		p.Layout = *ch.Layout
	}
	p.Version++
	f.profiles[id] = p
	return p, nil
}

func (f *fakeProfiles) Delete(_ context.Context, id uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.profiles[id]; !ok {
		return domain.ErrExportProfileNotFound
	}
	delete(f.profiles, id)
	return nil
}

func (f *fakeProfiles) RecordExport(_ context.Context, r domain.ExportRecord) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.records = append(f.records, r)
	return nil
}

// fakeAccounts: mọi account có tên "Account <8 ký tự đầu của id>"
type fakeAccounts struct{}

func (fakeAccounts) GetAccount(_ context.Context, id uuid.UUID) (idcontract.Account, error) {
	return idcontract.Account{ID: id, Name: "Account " + id.String()[:8], Active: true}, nil
}

func (a fakeAccounts) GetAccounts(ctx context.Context, ids []uuid.UUID) ([]idcontract.Account, error) {
	out := make([]idcontract.Account, len(ids))
	for i, id := range ids {
		out[i], _ = a.GetAccount(ctx, id)
	}
	return out, nil
}
```

(import `idcontract "storeit/internal/identity/contract"`).

In `service_test.go`, extend `env` with `profiles *fakeProfiles` and build it in `newEnv`:

```go
func newEnv() *env {
	e := &env{types: newFakeTypes(), statuses: newFakeStatuses(), assets: newFakeAssets(), profiles: newFakeProfiles()}
	e.svc = New(Deps{Types: e.types, Statuses: e.statuses, Assets: e.assets, Profiles: e.profiles, Accounts: fakeAccounts{}})
	return e
}
```

Add `domain.PermAssetExport` and `domain.PermExportProfileManage` to the "every other permission" list in `TestPermissionChecks`, and add entries:

```go
		"ListExportProfiles":  {domain.PermAssetExport, func(c context.Context) error { _, err := e.svc.ListExportProfiles(c); return err }},
		"CreateExportProfile": {domain.PermAssetExport, func(c context.Context) error { _, err := e.svc.CreateExportProfile(c, ExportProfileInput{}); return err }},
		"GetExportProfile":    {domain.PermAssetExport, func(c context.Context) error { _, err := e.svc.GetExportProfile(c, id); return err }},
		"DeleteExportProfile": {domain.PermAssetExport, func(c context.Context) error { return e.svc.DeleteExportProfile(c, id) }},
```

- [ ] **Step 3: Write the failing tests**

`backend/internal/inventory/service/export_profiles_test.go`:

```go
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"storeit/internal/inventory/domain"
	"storeit/internal/platform/auth"
)

func actorWith(id uuid.UUID, perms ...string) context.Context {
	return auth.WithActor(context.Background(), auth.Actor{AccountID: id, Permissions: perms})
}

func TestExportProfiles_OwnershipAndSharing(t *testing.T) {
	e := newEnv()
	ownerID, otherID, managerID := uuid.New(), uuid.New(), uuid.New()
	owner := actorWith(ownerID, domain.PermAssetExport)
	other := actorWith(otherID, domain.PermAssetExport)
	manager := actorWith(managerID, domain.PermAssetExport, domain.PermExportProfileManage)

	p, err := e.svc.CreateExportProfile(owner, ExportProfileInput{Name: "  Monthly  ", Layout: domain.DefaultReportLayout()})
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Monthly" || !p.CanEdit || p.OwnerName == "" {
		t.Errorf("created = %+v", p)
	}
	if _, err := e.svc.GetExportProfile(other, p.ID); !errors.Is(err, domain.ErrExportProfileNotFound) {
		t.Errorf("private profile seen by another user: %v", err)
	}
	if _, err := e.svc.GetExportProfile(manager, p.ID); !errors.Is(err, domain.ErrExportProfileNotFound) {
		t.Errorf("private profile seen by a manager: %v", err)
	}

	shared := true
	p, err = e.svc.UpdateExportProfile(owner, p.ID, domain.ExportProfileChange{Shared: &shared, Version: p.Version})
	if err != nil {
		t.Fatal(err)
	}
	seen, err := e.svc.GetExportProfile(other, p.ID)
	if err != nil || seen.CanEdit {
		t.Errorf("shared profile for another user = %+v, %v (want visible, not editable)", seen, err)
	}
	name := "Hijacked"
	if _, err := e.svc.UpdateExportProfile(other, p.ID, domain.ExportProfileChange{Name: &name, Version: p.Version}); !errors.Is(err, domain.ErrExportProfileForbidden) {
		t.Errorf("non-owner edits shared profile: %v", err)
	}
	if err := e.svc.DeleteExportProfile(other, p.ID); !errors.Is(err, domain.ErrExportProfileForbidden) {
		t.Errorf("non-owner deletes shared profile: %v", err)
	}
	if v, _ := e.svc.GetExportProfile(manager, p.ID); !v.CanEdit {
		t.Error("manager can edit a shared profile")
	}
	if _, err := e.svc.UpdateExportProfile(manager, p.ID, domain.ExportProfileChange{Name: &name, Version: p.Version}); err != nil {
		t.Errorf("manager edits shared profile: %v", err)
	}
	list, _ := e.svc.ListExportProfiles(other)
	if len(list) != 1 {
		t.Errorf("other user lists %d profiles, want the shared one", len(list))
	}
}

func TestExportProfiles_ValidatesInput(t *testing.T) {
	e := newEnv()
	ctx := actorWith(uuid.New(), domain.PermAssetExport)
	bad := domain.DefaultReportLayout()
	bad.Columns = []domain.ExportColumn{{Field: "nope"}}
	if _, err := e.svc.CreateExportProfile(ctx, ExportProfileInput{Name: "x", Layout: bad}); !errors.Is(err, domain.ErrInvalidExportLayout) {
		t.Errorf("bad layout: %v", err)
	}
	if _, err := e.svc.CreateExportProfile(ctx, ExportProfileInput{Name: "  ", Layout: domain.DefaultReportLayout()}); !errors.Is(err, domain.ErrInvalidLabel) {
		t.Errorf("blank name: %v", err)
	}
	p, err := e.svc.CreateExportProfile(ctx, ExportProfileInput{Name: "x", Layout: domain.ExportLayout{Columns: []domain.ExportColumn{{Field: "tag"}}}})
	if err != nil || p.Layout.Header != domain.HeaderBold {
		t.Errorf("defaults not applied: %+v, %v", p.Layout, err)
	}
}
```

- [ ] **Step 4: Run to see it fail**

Run: `cd backend && go test ./internal/inventory/service/ -run ExportProfiles`
Expected: FAIL to compile (`undefined: ExportProfileInput`).

- [ ] **Step 5: Implement**

`backend/internal/inventory/service/export_profiles.go`:

```go
package service

import (
	"context"

	"github.com/google/uuid"

	"storeit/internal/inventory/domain"
	"storeit/internal/platform/auth"
)

// Số dòng tối đa một lần export khi cấu hình để 0
const defaultExportMaxRows = 50_000

// ExportProfileView: profile kèm tên chủ và quyền sửa của người đang xem
type ExportProfileView struct {
	domain.ExportProfile
	OwnerName string
	CanEdit   bool
}

type ExportProfileInput struct {
	Name   string
	Shared bool
	Layout domain.ExportLayout
}

// ListExportProfiles: của mình và mọi profile được chia sẻ
func (s *Service) ListExportProfiles(ctx context.Context) ([]ExportProfileView, error) {
	actor, err := auth.Require(ctx, domain.PermAssetExport)
	if err != nil {
		return nil, err
	}
	ps, err := s.profiles.List(ctx, actor.AccountID)
	if err != nil {
		return nil, err
	}
	return s.profileViews(ctx, actor, ps)
}

func (s *Service) GetExportProfile(ctx context.Context, id uuid.UUID) (ExportProfileView, error) {
	actor, err := auth.Require(ctx, domain.PermAssetExport)
	if err != nil {
		return ExportProfileView{}, err
	}
	p, err := s.visibleProfile(ctx, actor, id)
	if err != nil {
		return ExportProfileView{}, err
	}
	return s.profileView(ctx, actor, p)
}

func (s *Service) CreateExportProfile(ctx context.Context, in ExportProfileInput) (ExportProfileView, error) {
	actor, err := auth.Require(ctx, domain.PermAssetExport)
	if err != nil {
		return ExportProfileView{}, err
	}
	name, err := domain.CleanLabel(in.Name, maxNameLen)
	if err != nil {
		return ExportProfileView{}, err
	}
	layout := in.Layout.WithDefaults()
	if err := layout.Validate(); err != nil {
		return ExportProfileView{}, err
	}
	p, err := s.profiles.Create(ctx, domain.NewExportProfile{OwnerID: actor.AccountID, Name: name, Shared: in.Shared, Layout: layout})
	if err != nil {
		return ExportProfileView{}, err
	}
	return s.profileView(ctx, actor, p)
}

// UpdateExportProfile: chủ sửa được; profile chia sẻ thì người có quyền quản lý cũng sửa được
func (s *Service) UpdateExportProfile(ctx context.Context, id uuid.UUID, ch domain.ExportProfileChange) (ExportProfileView, error) {
	actor, err := auth.Require(ctx, domain.PermAssetExport)
	if err != nil {
		return ExportProfileView{}, err
	}
	cur, err := s.visibleProfile(ctx, actor, id)
	if err != nil {
		return ExportProfileView{}, err
	}
	if !canEditProfile(actor, cur) {
		return ExportProfileView{}, domain.ErrExportProfileForbidden
	}
	if ch.Name != nil {
		n, err := domain.CleanLabel(*ch.Name, maxNameLen)
		if err != nil {
			return ExportProfileView{}, err
		}
		ch.Name = &n
	}
	if ch.Layout != nil {
		l := ch.Layout.WithDefaults()
		if err := l.Validate(); err != nil {
			return ExportProfileView{}, err
		}
		ch.Layout = &l
	}
	p, err := s.profiles.Update(ctx, id, ch)
	if err != nil {
		return ExportProfileView{}, err
	}
	return s.profileView(ctx, actor, p)
}

func (s *Service) DeleteExportProfile(ctx context.Context, id uuid.UUID) error {
	actor, err := auth.Require(ctx, domain.PermAssetExport)
	if err != nil {
		return err
	}
	cur, err := s.visibleProfile(ctx, actor, id)
	if err != nil {
		return err
	}
	if !canEditProfile(actor, cur) {
		return domain.ErrExportProfileForbidden
	}
	return s.profiles.Delete(ctx, id)
}

// visibleProfile: profile của người khác mà không chia sẻ thì như không tồn tại
func (s *Service) visibleProfile(ctx context.Context, actor auth.Actor, id uuid.UUID) (domain.ExportProfile, error) {
	p, err := s.profiles.Get(ctx, id)
	if err != nil {
		return domain.ExportProfile{}, err
	}
	if p.OwnerID != actor.AccountID && !p.Shared {
		return domain.ExportProfile{}, domain.ErrExportProfileNotFound
	}
	return p, nil
}

func canEditProfile(actor auth.Actor, p domain.ExportProfile) bool {
	return p.OwnerID == actor.AccountID || (p.Shared && actor.Can(domain.PermExportProfileManage))
}

func (s *Service) profileView(ctx context.Context, actor auth.Actor, p domain.ExportProfile) (ExportProfileView, error) {
	vs, err := s.profileViews(ctx, actor, []domain.ExportProfile{p})
	if err != nil {
		return ExportProfileView{}, err
	}
	return vs[0], nil
}

func (s *Service) profileViews(ctx context.Context, actor auth.Actor, ps []domain.ExportProfile) ([]ExportProfileView, error) {
	ids := make([]uuid.UUID, 0, len(ps))
	for _, p := range ps {
		ids = append(ids, p.OwnerID)
	}
	accounts, err := s.accounts.GetAccounts(ctx, ids)
	if err != nil {
		return nil, err
	}
	names := make(map[uuid.UUID]string, len(accounts))
	for _, a := range accounts {
		names[a.ID] = a.Name
	}
	out := make([]ExportProfileView, len(ps))
	for i, p := range ps {
		name := names[p.OwnerID]
		if name == "" {
			name = "Unknown account"
		}
		out[i] = ExportProfileView{ExportProfile: p, OwnerName: name, CanEdit: canEditProfile(actor, p)}
	}
	return out, nil
}
```

- [ ] **Step 6: Run the service tests**

Run: `cd backend && go test ./internal/inventory/service/`
Expected: PASS (new and existing). Then `go build ./...` fails in `inventory/module.go` and the HTTP/e2e tests only if they pass positional Deps; they use named fields, so they still compile.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/inventory/service
git commit -m "feat(inventory): export profile use cases with ownership and sharing rules

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: Export service

**Files:**
- Create: `backend/internal/inventory/service/export.go`
- Create: `backend/internal/inventory/service/export_test.go`

**Interfaces:**
- Consumes: Task 1 layout API, Task 2 `spreadsheet.NewWriter/Formatter`, Task 4 `Count/Stream`, Task 5 `visibleProfile`, `s.resolveAttrQuery` (exists in `service/assets.go`).
- Produces:
  - `type ExportMode string` with `ExportData = "data"`, `ExportReport = "report"`
  - `type ExportRequest struct{ Mode ExportMode; Filter domain.AssetFilter; IDs []uuid.UUID; ProfileID *uuid.UUID; Layout *domain.ExportLayout }`
  - `type ExportFile struct{ Name string; Data []byte; Skipped []string; Rows int64 }`
  - `func (s *Service) Export(ctx context.Context, req ExportRequest) (ExportFile, error)`

- [ ] **Step 1: Write the failing tests**

`backend/internal/inventory/service/export_test.go`:

```go
package service

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"storeit/internal/inventory/domain"
)

func open(t *testing.T, f ExportFile) *excelize.File {
	t.Helper()
	x, err := excelize.OpenReader(bytes.NewReader(f.Data))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = x.Close() })
	return x
}

var exporter = as(domain.PermAssetRead, domain.PermAssetExport, domain.PermAssetManage, domain.PermTypeManage)

func TestExport_Permissions(t *testing.T) {
	e := newEnv()
	if _, err := e.svc.Export(as(domain.PermAssetRead), ExportRequest{Mode: ExportData}); status(err) != 403 {
		t.Errorf("without export: %v", err)
	}
	if _, err := e.svc.Export(as(domain.PermAssetExport), ExportRequest{Mode: ExportData}); status(err) != 403 {
		t.Errorf("without read: %v", err)
	}
}

func TestExport_DataPerTypeWithKeyHeaders(t *testing.T) {
	e := newEnv()
	lap := e.laptop(t)
	e.mustAsset(t, lap, "LAP-1")
	e.mustAsset(t, lap, "LAP-2")
	gen := e.generalAsset(t, "GEN-1")
	_ = gen

	f, err := e.svc.Export(exporter, ExportRequest{Mode: ExportData})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(f.Name, "storeit-assets-") || f.Rows != 3 {
		t.Errorf("file = %s, rows %d", f.Name, f.Rows)
	}
	x := open(t, f)
	sheets := x.GetSheetList()
	if len(sheets) != 2 || sheets[0] != "GENERAL" || sheets[1] != lap.Code {
		t.Fatalf("sheets = %v, want one per type code in name order", sheets)
	}
	rows, _ := x.GetRows(lap.Code)
	if rows[0][0] != "tag" || !slices.Contains(rows[0], "attr:serial") || len(rows) != 3 {
		t.Errorf("laptop sheet = %v", rows)
	}
	if len(e.profiles.records) != 1 || e.profiles.records[0].Mode != "data" {
		t.Errorf("export event = %+v", e.profiles.records)
	}
}

func TestExport_SelectedIDsPerType(t *testing.T) {
	e := newEnv()
	lap := e.laptop(t)
	a := e.mustAsset(t, lap, "LAP-1")
	e.mustAsset(t, lap, "LAP-2")
	g := e.generalAsset(t, "GEN-1")
	f, err := e.svc.Export(exporter, ExportRequest{Mode: ExportData, IDs: []uuid.UUID{a.ID, g.ID}})
	if err != nil {
		t.Fatal(err)
	}
	x := open(t, f)
	rows, _ := x.GetRows(lap.Code)
	if len(rows) != 2 || rows[1][0] != "LAP-1" {
		t.Errorf("laptop sheet must hold only the selected laptop: %v", rows)
	}
	if _, err := e.svc.Export(exporter, ExportRequest{Mode: ExportData, IDs: []uuid.UUID{}}); !errors.Is(err, domain.ErrInvalidExportIDs) {
		t.Errorf("empty selection: %v", err)
	}
}

func TestExport_ReportLayoutAndTooLarge(t *testing.T) {
	e := newEnv()
	lap := e.laptop(t)
	e.mustAsset(t, lap, "LAP-1")
	l := domain.DefaultReportLayout()
	l.TitleRow, l.SheetName, l.Summary = true, "Laptops", true
	l.Columns = []domain.ExportColumn{{Field: "tag", Header: "Asset tag"}, {Field: "attr:serial"}}
	f, err := e.svc.Export(exporter, ExportRequest{Mode: ExportReport, Layout: &l})
	if err != nil {
		t.Fatal(err)
	}
	x := open(t, f)
	if s := x.GetSheetList(); len(s) != 2 || s[0] != "Laptops" || s[1] != "Summary" {
		t.Errorf("sheets = %v", s)
	}
	if v, _ := x.GetCellValue("Laptops", "A3"); v != "Asset tag" {
		t.Errorf("renamed header below two title rows = %q", v)
	}

	e.svc.exportMaxRows = 0
	if _, err := e.svc.Export(exporter, ExportRequest{Mode: ExportData}); !errors.Is(err, domain.ErrExportTooLarge) {
		t.Errorf("over the cap: %v", err)
	}
}

func TestExport_SkipsRemovedAttribute(t *testing.T) {
	e := newEnv()
	lap := e.laptop(t)
	e.mustAsset(t, lap, "LAP-1")
	owner := uuid.New()
	ctx := actorWith(owner, domain.PermAssetRead, domain.PermAssetExport)
	l := domain.DefaultReportLayout()
	l.Columns = []domain.ExportColumn{{Field: "tag"}, {Field: "attr:warranty_until"}}
	p, err := e.svc.CreateExportProfile(ctx, ExportProfileInput{Name: "Warranty", Layout: l})
	if err != nil {
		t.Fatal(err)
	}
	f, err := e.svc.Export(ctx, ExportRequest{Mode: ExportReport, ProfileID: &p.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Skipped) != 1 || f.Skipped[0] != "warranty_until" || !strings.HasPrefix(f.Name, "warranty-") {
		t.Errorf("file = %+v", f)
	}
	other := actorWith(uuid.New(), domain.PermAssetRead, domain.PermAssetExport)
	if _, err := e.svc.Export(other, ExportRequest{Mode: ExportReport, ProfileID: &p.ID}); !errors.Is(err, domain.ErrExportProfileNotFound) {
		t.Errorf("someone else's private profile: %v", err)
	}
}
```

Helpers: the env already has `laptop(t)` (a type with attributes `serial` and `os`). Add to `service_test.go`:

```go
// mustAsset tạo một tài sản của loại typ với giá trị bắt buộc
func (e *env) mustAsset(t *testing.T, typ domain.AssetType, tag string) domain.Asset {
	t.Helper()
	v, err := e.svc.CreateAsset(manager, tag, AssetInput{Name: tag, TypeID: typ.ID, Attributes: requiredValues(typ)})
	if err != nil {
		t.Fatal(err)
	}
	return v.Asset
}

func (e *env) generalAsset(t *testing.T, tag string) domain.Asset {
	t.Helper()
	v, err := e.svc.CreateAsset(manager, tag, AssetInput{Name: tag, TypeID: domain.GeneralTypeID})
	if err != nil {
		t.Fatal(err)
	}
	return v.Asset
}
```

Before writing these helpers, read the existing asset-creation tests in `service_test.go` (`grep -n "CreateAsset(" backend/internal/inventory/service/service_test.go`) and reuse the attribute map they pass for the laptop's required `serial` instead of a new `requiredValues` if one exists; check that `newFakeTypes()` seeds the GENERAL type (if not, create a type named "General" with code "GENERAL" through `e.svc.CreateAssetType` in `generalAsset`). Add `"slices"` to the test imports. `AssetView` exposes the asset as `.Asset` — check with `grep -n "type AssetView" -A5 backend/internal/inventory/service/assets.go` and adapt.

- [ ] **Step 2: Run to see them fail**

Run: `cd backend && go test ./internal/inventory/service/ -run Export_`
Expected: FAIL to compile (`undefined: ExportRequest`).

- [ ] **Step 3: Implement**

`backend/internal/inventory/service/export.go`:

```go
package service

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"storeit/internal/inventory/domain"
	"storeit/internal/inventory/spreadsheet"
	"storeit/internal/platform/auth"
	"storeit/internal/platform/errs"
)

// ExportMode: "data" (mỗi loại một sheet, import đọc lại được) hay "report" (theo bố cục)
type ExportMode string

const (
	ExportData   ExportMode = "data"
	ExportReport ExportMode = "report"
)

type ExportRequest struct {
	Mode      ExportMode
	Filter    domain.AssetFilter // như danh sách; Limit/Offset bỏ qua
	IDs       []uuid.UUID        // "Export selected": 1..200; nil là không lọc
	ProfileID *uuid.UUID
	Layout    *domain.ExportLayout // gửi kèm thì thắng profile
}

type ExportFile struct {
	Name    string
	Data    []byte
	Skipped []string // khoá thuộc tính bị bỏ (không loại nào còn có)
	Rows    int64
}

const (
	maxExportIDs   = 200
	exportPageSize = 500
)

// Export dựng file .xlsx theo bộ lọc của danh sách. Vượt số dòng tối đa thì 422 trước
// khi ghi gì; file nằm trong bộ nhớ đến khi trả về.
func (s *Service) Export(ctx context.Context, req ExportRequest) (ExportFile, error) {
	actor, err := auth.Require(ctx, domain.PermAssetExport)
	if err != nil {
		return ExportFile{}, err
	}
	if _, err := auth.Require(ctx, domain.PermAssetRead); err != nil {
		return ExportFile{}, err
	}
	layout, profile, err := s.exportLayout(ctx, actor, req)
	if err != nil {
		return ExportFile{}, err
	}
	f := req.Filter
	f.Limit, f.Offset, f.IncludeValues = 0, 0, false
	if req.IDs != nil {
		if len(req.IDs) == 0 || len(req.IDs) > maxExportIDs {
			return ExportFile{}, domain.ErrInvalidExportIDs
		}
		f.IDs = req.IDs
	}
	if layout.Sort != "" {
		f.Sort = domain.AssetSort(layout.Sort)
	}
	if _, _, byAttr := f.Sort.Attribute(); byAttr || len(f.Attrs) > 0 {
		if err := s.resolveAttrQuery(ctx, &f); err != nil {
			return ExportFile{}, err
		}
	}
	total, err := s.assets.Count(ctx, f)
	if err != nil {
		return ExportFile{}, err
	}
	if total > int64(s.exportMaxRows) {
		return ExportFile{}, domain.ErrExportTooLarge.With(errs.WithDetailf(
			"This export has %d rows; the limit is %d. Narrow the filters and try again.", total, s.exportMaxRows))
	}
	types, err := s.exportTypes(ctx, f)
	if err != nil {
		return ExportFile{}, err
	}

	w := spreadsheet.NewWriter()
	defer func() { _ = w.Close() }()
	byID := make(map[uuid.UUID]domain.AssetType, len(types))
	for _, t := range types {
		byID[t.ID] = t
	}
	fm := spreadsheet.Formatter{Layout: layout, Types: byID, Loc: time.Local}
	title, err := s.exportTitle(ctx, actor, req, layout, profile, f, types)
	if err != nil {
		return ExportFile{}, err
	}

	type group struct {
		name   string
		types  []domain.AssetType
		filter domain.AssetFilter
	}
	var groups []group
	if layout.Sheets == domain.SheetPerType {
		for _, t := range types {
			gf := f
			gf.TypeID = &t.ID
			name := t.Name
			if req.Mode == ExportData {
				name = t.Code
			}
			groups = append(groups, group{name: name, types: []domain.AssetType{t}, filter: gf})
		}
	} else {
		groups = []group{{name: layout.SheetName, types: types, filter: f}}
	}
	if len(groups) == 0 {
		// không có dòng nào: vẫn trả file có một sheet với tên cột
		groups = []group{{name: layout.SheetName, filter: f}}
	}

	summary := map[uuid.UUID]map[domain.StatusKind]int64{}
	var rows int64
	for _, g := range groups {
		cols := layout.SheetColumns(g.types)
		headers, widths := make([]string, len(cols)), make([]float64, len(cols))
		for i, c := range cols {
			headers[i], widths[i] = c.Header, columnWidth(c)
		}
		sh, err := w.Sheet(spreadsheet.SheetOptions{
			Name: g.name, Widths: widths, Header: layout.Header, Freeze: layout.Freeze,
			Filter: layout.Filter, Stripes: layout.Stripes, Title: title,
		}, headers)
		if err != nil {
			return ExportFile{}, err
		}
		err = s.assets.Stream(ctx, g.filter, exportPageSize, func(items []domain.AssetListItem) error {
			for _, a := range items {
				cells := make([]spreadsheet.Cell, len(cols))
				for i, c := range cols {
					cells[i] = fm.Cell(a, c)
				}
				if err := sh.Row(cells); err != nil {
					return err
				}
				if summary[a.TypeID] == nil {
					summary[a.TypeID] = map[domain.StatusKind]int64{}
				}
				summary[a.TypeID][a.StatusKind]++
				rows++
			}
			return nil
		})
		if err != nil {
			return ExportFile{}, err
		}
		if err := sh.Close(); err != nil {
			return ExportFile{}, err
		}
	}
	if layout.Summary {
		if err := writeSummary(w, types, summary); err != nil {
			return ExportFile{}, err
		}
	}
	var buf bytes.Buffer
	if _, err := w.WriteTo(&buf); err != nil {
		return ExportFile{}, err
	}

	rec := domain.ExportRecord{Mode: string(req.Mode), Rows: rows, Sheets: len(groups), Filters: filterRecord(f)}
	if profile != nil {
		rec.ProfileID = &profile.ID
	}
	if err := s.profiles.RecordExport(ctx, rec); err != nil {
		// file đã dựng xong; mất một dòng lịch sử không đáng làm hỏng lần tải
		slog.ErrorContext(ctx, "inventory: record export", "error", err)
	}
	return ExportFile{Name: exportFileName(req.Mode, profile), Data: buf.Bytes(), Skipped: layout.SkippedKeys(types), Rows: rows}, nil
}

// exportLayout: dữ liệu dùng DataLayout; báo cáo dùng bố cục gửi kèm, của profile, hay mặc định
func (s *Service) exportLayout(ctx context.Context, actor auth.Actor, req ExportRequest) (domain.ExportLayout, *domain.ExportProfile, error) {
	if req.Mode == ExportData {
		return domain.DataLayout(), nil, nil
	}
	var profile *domain.ExportProfile
	if req.ProfileID != nil {
		p, err := s.visibleProfile(ctx, actor, *req.ProfileID)
		if err != nil {
			return domain.ExportLayout{}, nil, err
		}
		profile = &p
	}
	l := domain.DefaultReportLayout()
	switch {
	case req.Layout != nil:
		l = *req.Layout
	case profile != nil:
		l = profile.Layout
	}
	l = l.WithDefaults()
	if err := l.Validate(); err != nil {
		return domain.ExportLayout{}, nil, err
	}
	return l, profile, nil
}

// exportTypes: các loại (kèm thuộc tính) có dòng trong export, theo tên
func (s *Service) exportTypes(ctx context.Context, f domain.AssetFilter) ([]domain.AssetType, error) {
	var candidates []domain.AssetType
	if f.TypeID != nil {
		candidates = []domain.AssetType{{ID: *f.TypeID}}
	} else {
		all, err := s.types.List(ctx, true)
		if err != nil {
			return nil, err
		}
		candidates = all
	}
	var out []domain.AssetType
	for _, c := range candidates {
		tf := f
		tf.TypeID = &c.ID
		n, err := s.assets.Count(ctx, tf)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			continue
		}
		t, err := s.types.Get(ctx, c.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	slices.SortFunc(out, func(a, b domain.AssetType) int { return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })
	return out, nil
}

// exportTitle: hai dòng tiêu đề của báo cáo (tên báo cáo; ngày, người export, bộ lọc)
func (s *Service) exportTitle(ctx context.Context, actor auth.Actor, req ExportRequest, l domain.ExportLayout, p *domain.ExportProfile, f domain.AssetFilter, types []domain.AssetType) ([]string, error) {
	if req.Mode != ExportReport || !l.TitleRow {
		return nil, nil
	}
	name := "Asset report"
	if p != nil {
		name = p.Name
	}
	who, err := s.accounts.GetAccount(ctx, actor.AccountID)
	by := who.Name
	if err != nil || by == "" {
		by = "unknown"
	}
	summary, err := s.filterSummary(ctx, f, types)
	if err != nil {
		return nil, err
	}
	date := time.Now().Format("02/01/2006")
	return []string{name, fmt.Sprintf("Generated %s by %s · %s", date, by, summary)}, nil
}

// filterSummary: mô tả bộ lọc cho người đọc ("Laptop · In use · search \"think\"")
func (s *Service) filterSummary(ctx context.Context, f domain.AssetFilter, types []domain.AssetType) (string, error) {
	var parts []string
	if f.TypeID != nil && len(types) == 1 {
		parts = append(parts, types[0].Name)
	}
	if f.StatusID != nil {
		st, err := s.statuses.Get(ctx, *f.StatusID)
		if err != nil {
			return "", err
		}
		parts = append(parts, st.Name)
	}
	if f.StatusKind != nil {
		parts = append(parts, f.StatusKind.Label())
	}
	if f.Query != "" {
		parts = append(parts, fmt.Sprintf("search %q", f.Query))
	}
	for _, af := range f.AttrFilters {
		parts = append(parts, attrFilterText(types, af))
	}
	if f.IDs != nil {
		parts = append(parts, fmt.Sprintf("%d selected", len(f.IDs)))
	}
	if f.IncludeRetired {
		parts = append(parts, "including retired")
	}
	if len(parts) == 0 {
		return "All assets", nil
	}
	return strings.Join(parts, " · "), nil
}

var opSymbols = map[domain.AttrOp]string{
	domain.OpEq: "=", domain.OpGt: ">", domain.OpGte: "≥", domain.OpLt: "<", domain.OpLte: "≤",
	domain.OpContains: "contains", domain.OpIn: "in",
}

func attrFilterText(types []domain.AssetType, af domain.AttrFilter) string {
	for _, t := range types {
		for _, a := range t.Attributes {
			if a.ID == af.AttributeID {
				return fmt.Sprintf("%s %s %s", a.Label, opSymbols[af.Op], af.Value)
			}
		}
	}
	return "attribute filter"
}

func filterRecord(f domain.AssetFilter) map[string]any {
	m := map[string]any{}
	if f.Query != "" {
		m["q"] = f.Query
	}
	if f.TypeID != nil {
		m["type_id"] = f.TypeID.String()
	}
	if f.StatusID != nil {
		m["status_id"] = f.StatusID.String()
	}
	if f.StatusKind != nil {
		m["status_kind"] = string(*f.StatusKind)
	}
	if len(f.Attrs) > 0 {
		m["attr"] = f.Attrs
	}
	if f.IDs != nil {
		m["ids"] = len(f.IDs)
	}
	if f.IncludeRetired {
		m["include_retired"] = true
	}
	return m
}

// columnWidth: độ rộng người dùng chọn, hay mặc định theo trường; 0 để writer tự tính
func columnWidth(c domain.SheetColumn) float64 {
	if c.Width != 0 {
		return c.Width
	}
	switch c.Field {
	case "name":
		return 32
	case "description":
		return 40
	case "tag":
		return 14
	}
	return 0
}

func writeSummary(w *spreadsheet.Writer, types []domain.AssetType, counts map[uuid.UUID]map[domain.StatusKind]int64) error {
	kinds := []domain.StatusKind{domain.KindAvailable, domain.KindInUse, domain.KindUnavailable, domain.KindRetired}
	headers := []string{"Type"}
	for _, k := range kinds {
		headers = append(headers, k.Label())
	}
	headers = append(headers, "Total")
	sh, err := w.Sheet(spreadsheet.SheetOptions{Name: "Summary", Header: domain.HeaderBold, Freeze: true}, headers)
	if err != nil {
		return err
	}
	for _, t := range types {
		row := []spreadsheet.Cell{spreadsheet.TextCell(t.Name)}
		var total int64
		for _, k := range kinds {
			n := counts[t.ID][k]
			total += n
			row = append(row, spreadsheet.Cell{Kind: spreadsheet.Number, Num: float64(n)})
		}
		row = append(row, spreadsheet.Cell{Kind: spreadsheet.Number, Num: float64(total)})
		if err := sh.Row(row); err != nil {
			return err
		}
	}
	return sh.Close()
}

var slugBad = regexp.MustCompile(`[^\p{L}\p{N}]+`)

// exportFileName: storeit-assets-<ngày>.xlsx; báo cáo theo tên profile
func exportFileName(mode ExportMode, p *domain.ExportProfile) string {
	date := time.Now().Format("2006-01-02")
	switch {
	case mode == ExportData:
		return "storeit-assets-" + date + ".xlsx"
	case p != nil:
		slug := strings.Trim(slugBad.ReplaceAllString(strings.ToLower(p.Name), "-"), "-")
		if slug == "" {
			slug = "storeit-report"
		}
		return slug + "-" + date + ".xlsx"
	}
	return "storeit-report-" + date + ".xlsx"
}
```

Check `s.types.List(ctx, includeArchived bool)` and `s.statuses.Get` signatures with `grep -n "List(ctx\|Get(ctx" backend/internal/inventory/domain/repository.go`; adjust the calls if they differ.

- [ ] **Step 4: Run the tests**

Run: `cd backend && go test ./internal/inventory/service/ -v -run 'Export'`
Expected: PASS.
Run: `cd backend && go test ./internal/inventory/...` (no DB) and `go vet ./internal/inventory/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/inventory/service/export.go backend/internal/inventory/service/export_test.go backend/internal/inventory/service/service_test.go
git commit -m "feat(inventory): export service for data and report files

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: API, handlers and wiring

**Files:**
- Modify: `backend/internal/inventory/handler/openapi.yaml`
- Create: `backend/internal/inventory/handler/exports.go`, `backend/internal/inventory/handler/export_profiles.go`
- Modify: `backend/internal/inventory/handler/imports.go` (keep only the import placeholder)
- Create: `backend/internal/inventory/config.go`, `backend/internal/inventory/config_test.go`
- Modify: `backend/internal/inventory/module.go`, `backend/cmd/server/config.go`, `backend/cmd/server/main.go`
- Modify: `backend/internal/inventory/http_db_test.go`, `backend/internal/inventory/e2e_db_test.go`

**Interfaces:**
- Consumes: Tasks 5–6 service API.
- Produces: `POST /api/v1/assets/export`, `GET/POST /api/v1/export-profiles`, `GET/PATCH/DELETE /api/v1/export-profiles/{profileID}`; `inventory.Config{ExportMaxRows int}`; `inventory.Deps{Pool, Outbox, Accounts idcontract.AccountReader, Config Config}`.

- [ ] **Step 1: OpenAPI paths**

Add under `paths:` in `openapi.yaml` (place `/assets/export` before `/assets/{assetID}`):

```yaml
  /assets/export:
    post:
      operationId: exportAssets
      summary: Download assets as .xlsx (inventory.asset.read and inventory.asset.export)
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: '#/components/schemas/ExportRequest' }
      responses:
        "200":
          description: The workbook
          headers:
            Content-Disposition:
              schema: { type: string }
            X-Export-Skipped-Columns:
              description: Attribute keys in the layout that no exported type has, comma-separated
              schema: { type: string }
          content:
            application/vnd.openxmlformats-officedocument.spreadsheetml.sheet:
              schema: { type: string, format: binary }
        default: { $ref: '#/components/responses/Problem' }

  /export-profiles:
    get:
      operationId: listExportProfiles
      summary: Your export profiles and every shared one (inventory.asset.export)
      responses:
        "200":
          description: Profiles by name
          content:
            application/json:
              schema:
                type: object
                required: [items]
                properties:
                  items:
                    type: array
                    items: { $ref: '#/components/schemas/ExportProfile' }
        default: { $ref: '#/components/responses/Problem' }
    post:
      operationId: createExportProfile
      summary: Save a report layout (inventory.asset.export)
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: '#/components/schemas/CreateExportProfileRequest' }
      responses:
        "201":
          description: Created
          content:
            application/json:
              schema: { $ref: '#/components/schemas/ExportProfile' }
        default: { $ref: '#/components/responses/Problem' }

  /export-profiles/{profileID}:
    parameters:
      - name: profileID
        in: path
        required: true
        schema: { $ref: '../../../api/common.yaml#/components/schemas/ID' }
    get:
      operationId: getExportProfile
      summary: One profile (yours or shared)
      responses:
        "200":
          description: Profile
          content:
            application/json:
              schema: { $ref: '#/components/schemas/ExportProfile' }
        default: { $ref: '#/components/responses/Problem' }
    patch:
      operationId: updateExportProfile
      summary: Rename, share or change the layout (owner; shared ones also inventory.export_profile.manage)
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: '#/components/schemas/UpdateExportProfileRequest' }
      responses:
        "200":
          description: Updated
          content:
            application/json:
              schema: { $ref: '#/components/schemas/ExportProfile' }
        default: { $ref: '#/components/responses/Problem' }
    delete:
      operationId: deleteExportProfile
      summary: Delete a profile (owner; shared ones also inventory.export_profile.manage)
      responses:
        "204": { description: Deleted }
        default: { $ref: '#/components/responses/Problem' }
```

and under `components: schemas:`:

```yaml
    ExportColumn:
      type: object
      required: [field]
      properties:
        field: { type: string, maxLength: 40, description: 'tag, name, description, type, status, purchase_date, updated_at or attr:<key>' }
        header: { type: string, maxLength: 100 }
        width: { type: number, minimum: 0, maximum: 80 }

    ExportLayout:
      type: object
      required: [columns, sheets, sheet_name, title_row, summary, header, freeze, filter, stripes, date_format, bool_style, status_as, unit_in]
      properties:
        columns:
          type: array
          maxItems: 60
          items: { $ref: '#/components/schemas/ExportColumn' }
        sheets: { type: string, enum: [single, per_type] }
        each_type_attrs: { type: boolean }
        sheet_name: { type: string, maxLength: 31 }
        title_row: { type: boolean }
        summary: { type: boolean }
        header: { type: string, enum: [plain, bold, bold_fill] }
        freeze: { type: boolean }
        filter: { type: boolean }
        stripes: { type: boolean }
        date_format: { type: string, enum: ['dd/mm/yyyy', 'yyyy-mm-dd', 'd mmm yyyy'] }
        bool_style: { type: string, enum: [yes_no, check] }
        status_as: { type: string, enum: [name, kind] }
        unit_in: { type: string, enum: [header, cell] }
        sort: { type: string, maxLength: 60, description: 'Empty: the request sort' }

    ExportFilters:
      type: object
      description: Same filters and rules as GET /assets, plus ids for "Export selected"
      properties:
        q: { type: string, maxLength: 200 }
        type_id: { $ref: '../../../api/common.yaml#/components/schemas/ID' }
        status_id: { $ref: '../../../api/common.yaml#/components/schemas/ID' }
        status_kind: { $ref: '#/components/schemas/StatusKind' }
        include_retired: { type: boolean }
        attr:
          type: array
          maxItems: 20
          items: { type: string, maxLength: 300 }
        sort: { type: string, maxLength: 60 }
        ids:
          type: array
          maxItems: 200
          items: { $ref: '../../../api/common.yaml#/components/schemas/ID' }

    ExportRequest:
      type: object
      required: [mode]
      properties:
        mode: { type: string, enum: [data, report] }
        filters: { $ref: '#/components/schemas/ExportFilters' }
        profile_id: { $ref: '../../../api/common.yaml#/components/schemas/ID' }
        layout: { $ref: '#/components/schemas/ExportLayout' }

    ExportProfile:
      type: object
      required: [id, name, shared, layout, owner, can_edit, version, created_at, updated_at]
      properties:
        id: { $ref: '../../../api/common.yaml#/components/schemas/ID' }
        name: { type: string }
        shared: { type: boolean }
        layout: { $ref: '#/components/schemas/ExportLayout' }
        owner:
          type: object
          required: [id, name]
          properties:
            id: { $ref: '../../../api/common.yaml#/components/schemas/ID' }
            name: { type: string }
        can_edit: { type: boolean }
        version: { type: integer, format: int32 }
        created_at: { type: string, format: date-time }
        updated_at: { type: string, format: date-time }

    CreateExportProfileRequest:
      type: object
      required: [name, layout]
      properties:
        name: { type: string, minLength: 1, maxLength: 100 }
        shared: { type: boolean }
        layout: { $ref: '#/components/schemas/ExportLayout' }

    UpdateExportProfileRequest:
      type: object
      required: [version]
      properties:
        name: { type: string, minLength: 1, maxLength: 100 }
        shared: { type: boolean }
        layout: { $ref: '#/components/schemas/ExportLayout' }
        version: { type: integer, format: int32 }
```

Run: `cd backend && make generate`
Expected: success; `api.gen.go` has `ExportAssets`, `ListExportProfiles`, … in `StrictServerInterface`. Find the generated 200 response type: `grep -n "type ExportAssets200" backend/internal/inventory/handler/api/api.gen.go`. Expected names: `ExportAssets200ApplicationvndOpenxmlformatsOfficedocumentSpreadsheetmlSheetResponse` with fields `Body io.Reader`, `Headers ExportAssets200ResponseHeaders`, `ContentLength int64`, and `ExportAssets200ResponseHeaders{ContentDisposition string; XExportSkippedColumns string}`. Use the names the generator actually produced in the next step.

- [ ] **Step 2: Handlers**

`backend/internal/inventory/handler/exports.go`:

```go
package handler

import (
	"bytes"
	"context"
	"mime"
	"strings"

	"storeit/internal/inventory/domain"
	"storeit/internal/inventory/handler/api"
	"storeit/internal/inventory/service"
)

func (h *Handler) ExportAssets(ctx context.Context, req api.ExportAssetsRequestObject) (api.ExportAssetsResponseObject, error) {
	b := req.Body
	in := service.ExportRequest{Mode: service.ExportMode(b.Mode), ProfileID: b.ProfileId}
	if b.Filters != nil {
		p := b.Filters
		in.Filter = domain.AssetFilter{
			Query: deref(p.Q), TypeID: p.TypeId, StatusID: p.StatusId, IncludeRetired: deref(p.IncludeRetired),
			Attrs: deref(p.Attr), Sort: domain.AssetSort(deref(p.Sort)),
		}
		if p.StatusKind != nil {
			k := domain.StatusKind(*p.StatusKind)
			in.Filter.StatusKind = &k
		}
		if p.Ids != nil {
			in.IDs = append([]uuid.UUID{}, (*p.Ids)...)
		}
	}
	if b.Layout != nil {
		l := toDomainLayout(*b.Layout)
		in.Layout = &l
	}
	file, err := h.svc.Export(ctx, in)
	if err != nil {
		return nil, err
	}
	return api.ExportAssets200ApplicationvndOpenxmlformatsOfficedocumentSpreadsheetmlSheetResponse{
		Body:          bytes.NewReader(file.Data),
		ContentLength: int64(len(file.Data)),
		Headers: api.ExportAssets200ResponseHeaders{
			ContentDisposition:    mime.FormatMediaType("attachment", map[string]string{"filename": file.Name}),
			XExportSkippedColumns: strings.Join(file.Skipped, ","),
		},
	}, nil
}

func toDomainLayout(l api.ExportLayout) domain.ExportLayout {
	out := domain.ExportLayout{
		Sheets: domain.SheetMode(l.Sheets), EachTypeAttrs: deref(l.EachTypeAttrs), SheetName: l.SheetName,
		TitleRow: l.TitleRow, Summary: l.Summary, Header: domain.HeaderStyle(l.Header), Freeze: l.Freeze,
		Filter: l.Filter, Stripes: l.Stripes, DateFormat: domain.DateFormat(l.DateFormat),
		BoolStyle: domain.BoolStyle(l.BoolStyle), StatusAs: domain.StatusAs(l.StatusAs), UnitIn: domain.UnitIn(l.UnitIn),
		Sort: deref(l.Sort),
	}
	for _, c := range l.Columns {
		out.Columns = append(out.Columns, domain.ExportColumn{Field: c.Field, Header: deref(c.Header), Width: float64(deref(c.Width))})
	}
	return out
}

func toAPILayout(l domain.ExportLayout) api.ExportLayout {
	cols := make([]api.ExportColumn, len(l.Columns))
	for i, c := range l.Columns {
		header, width := c.Header, float32(c.Width)
		cols[i] = api.ExportColumn{Field: c.Field, Header: &header, Width: &width}
	}
	each, sort := l.EachTypeAttrs, l.Sort
	return api.ExportLayout{
		Columns: cols, Sheets: api.ExportLayoutSheets(l.Sheets), EachTypeAttrs: &each, SheetName: l.SheetName,
		TitleRow: l.TitleRow, Summary: l.Summary, Header: api.ExportLayoutHeader(l.Header), Freeze: l.Freeze,
		Filter: l.Filter, Stripes: l.Stripes, DateFormat: api.ExportLayoutDateFormat(l.DateFormat),
		BoolStyle: api.ExportLayoutBoolStyle(l.BoolStyle), StatusAs: api.ExportLayoutStatusAs(l.StatusAs),
		UnitIn: api.ExportLayoutUnitIn(l.UnitIn), Sort: &sort,
	}
}
```

Import `github.com/google/uuid` (or `openapi_types "github.com/oapi-codegen/runtime/types"` if `Ids` is `[]openapi_types.UUID`; it is an alias of `uuid.UUID`, so `append([]uuid.UUID{}, …)` works). The generated `Width` is `*float32` for `type: number` without format; if it is `*float64`, drop the conversions.

`backend/internal/inventory/handler/export_profiles.go`:

```go
package handler

import (
	"context"

	"storeit/internal/inventory/domain"
	"storeit/internal/inventory/handler/api"
	"storeit/internal/inventory/service"
)

func (h *Handler) ListExportProfiles(ctx context.Context, _ api.ListExportProfilesRequestObject) (api.ListExportProfilesResponseObject, error) {
	ps, err := h.svc.ListExportProfiles(ctx)
	if err != nil {
		return nil, err
	}
	out := api.ListExportProfiles200JSONResponse{Items: make([]api.ExportProfile, len(ps))}
	for i, p := range ps {
		out.Items[i] = toAPIProfile(p)
	}
	return out, nil
}

func (h *Handler) CreateExportProfile(ctx context.Context, req api.CreateExportProfileRequestObject) (api.CreateExportProfileResponseObject, error) {
	p, err := h.svc.CreateExportProfile(ctx, service.ExportProfileInput{
		Name: req.Body.Name, Shared: deref(req.Body.Shared), Layout: toDomainLayout(req.Body.Layout),
	})
	if err != nil {
		return nil, err
	}
	return api.CreateExportProfile201JSONResponse(toAPIProfile(p)), nil
}

func (h *Handler) GetExportProfile(ctx context.Context, req api.GetExportProfileRequestObject) (api.GetExportProfileResponseObject, error) {
	p, err := h.svc.GetExportProfile(ctx, req.ProfileID)
	if err != nil {
		return nil, err
	}
	return api.GetExportProfile200JSONResponse(toAPIProfile(p)), nil
}

func (h *Handler) UpdateExportProfile(ctx context.Context, req api.UpdateExportProfileRequestObject) (api.UpdateExportProfileResponseObject, error) {
	ch := domain.ExportProfileChange{Name: req.Body.Name, Shared: req.Body.Shared, Version: req.Body.Version}
	if req.Body.Layout != nil {
		l := toDomainLayout(*req.Body.Layout)
		ch.Layout = &l
	}
	p, err := h.svc.UpdateExportProfile(ctx, req.ProfileID, ch)
	if err != nil {
		return nil, err
	}
	return api.UpdateExportProfile200JSONResponse(toAPIProfile(p)), nil
}

func (h *Handler) DeleteExportProfile(ctx context.Context, req api.DeleteExportProfileRequestObject) (api.DeleteExportProfileResponseObject, error) {
	if err := h.svc.DeleteExportProfile(ctx, req.ProfileID); err != nil {
		return nil, err
	}
	return api.DeleteExportProfile204Response{}, nil
}

func toAPIProfile(p service.ExportProfileView) api.ExportProfile {
	out := api.ExportProfile{
		Id: p.ID, Name: p.Name, Shared: p.Shared, Layout: toAPILayout(p.Layout), CanEdit: p.CanEdit,
		Version: p.Version, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
	out.Owner.Id, out.Owner.Name = p.OwnerID, p.OwnerName
	return out
}
```

Replace the content of `handler/imports.go` with just the package line and `// TODO(M5): import và file mẫu (spec riêng)`.

- [ ] **Step 3: Config and module wiring**

`backend/internal/inventory/config.go`:

```go
package inventory

import "fmt"

// Config của inventory; binary liệt kê khối này trong cmd/<bin>/config.go
type Config struct {
	// Số dòng tối đa một lần export Excel; 0 là mặc định của service (50.000)
	ExportMaxRows int `env:"INVENTORY_EXPORT_MAX_ROWS" envDefault:"50000"`
}

func (c Config) Validate() error {
	if c.ExportMaxRows < 0 || c.ExportMaxRows > 1_000_000 {
		return fmt.Errorf("inventory: INVENTORY_EXPORT_MAX_ROWS must be between 1 and 1000000")
	}
	return nil
}
```

`backend/internal/inventory/config_test.go`:

```go
package inventory

import "testing"

func TestConfig_Validate(t *testing.T) {
	for _, n := range []int{0, 1, 50000, 1_000_000} {
		if err := (Config{ExportMaxRows: n}).Validate(); err != nil {
			t.Errorf("%d: %v", n, err)
		}
	}
	for _, n := range []int{-1, 1_000_001} {
		if err := (Config{ExportMaxRows: n}).Validate(); err == nil {
			t.Errorf("%d accepted", n)
		}
	}
}
```

In `module.go`: add `Accounts idcontract.AccountReader` and `Config Config` to `Deps`; require `Accounts` (`if d.Pool == nil || d.Outbox == nil || d.Accounts == nil { return nil, fmt.Errorf("inventory: Pool, Outbox and Accounts are required") }`); pass to `service.New`:

```go
		Profiles:      repository.NewExportProfileRepository(d.Pool, d.Outbox),
		Accounts:      d.Accounts,
		ExportMaxRows: d.Config.ExportMaxRows,
```

Update the doc comment example to `inventory.New(inventory.Deps{Pool: pool, Outbox: outbox, Accounts: identityMod.AccountReader(), Config: cfg.Inventory})`.

In `cmd/server/config.go` add `Inventory inventory.Config // INVENTORY_*` (import `storeit/internal/inventory`). In `cmd/server/main.go`:

```go
	inventoryMod, err := inventory.New(inventory.Deps{
		Pool: pool, Outbox: outbox, Accounts: identityMod.AccountReader(), Config: cfg.Inventory,
	})
```

In `e2e_db_test.go`, pass `Accounts: stubAccounts{}` (define in that file a type implementing `GetAccount`/`GetAccounts` returning a fixed name) to both `inventory.New` calls; the "missing deps" test still expects an error.

- [ ] **Step 4: HTTP tests**

In `http_db_test.go`, build the service with profiles, accounts and a configurable cap: change `newApp` to `newAppWith(t, maxRows int)` and keep `newApp(t)` as `newAppWith(t, 0)`:

```go
	svc := service.New(service.Deps{
		Types:    repository.NewTypeRepository(pool, outbox),
		Statuses: repository.NewStatusRepository(pool, outbox),
		Assets:   repository.NewAssetRepository(pool, outbox),
		Profiles: repository.NewExportProfileRepository(pool, outbox),
		Accounts: stubAccounts{},
		ExportMaxRows: maxRows,
	})
```

(`stubAccounts` defined once in a shared `_test.go` of package `inventory_test`, e.g. at the bottom of `http_db_test.go`; reuse it in `e2e_db_test.go`.)

Add the test:

```go
func TestExportOverHTTP(t *testing.T) {
	a := newApp(t)
	tok := a.token(allPerms...)
	exp := a.token(domain.PermAssetRead, domain.PermAssetExport)
	code := "EX" + strings.ToUpper(uuid.NewString()[:6])
	typ := decode[typeDetail](t, a.do("POST", "/api/v1/asset-types", tok, map[string]any{
		"code": code, "name": "Export " + code,
		"attributes": []map[string]any{{"key": "ram_gb", "label": "RAM", "data_type": "number", "unit": "GB", "position": 1}},
	}), 201)
	var ids []string
	for i := range 3 {
		x := decode[assetDetail](t, a.do("POST", "/api/v1/assets", tok, map[string]any{
			"tag": fmt.Sprintf("%s-%d", code, i), "name": "x", "asset_type_id": typ.ID, "attributes": map[string]any{"ram_gb": 16},
		}), 201)
		ids = append(ids, x.ID)
	}

	rec := a.do("POST", "/api/v1/assets/export", exp, map[string]any{"mode": "data", "filters": map[string]any{"type_id": typ.ID}})
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Type"), "spreadsheetml") {
		t.Fatalf("data export: %d %s", rec.Code, rec.Header())
	}
	x, err := excelize.OpenReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	rows, _ := x.GetRows(code)
	if len(rows) != 4 || rows[0][0] != "tag" || rows[0][len(rows[0])-1] != "attr:ram_gb" {
		t.Errorf("data sheet = %v", rows)
	}

	// selection + report with a non-ASCII profile name and a removed attribute
	layout := map[string]any{
		"columns": []map[string]any{{"field": "tag"}, {"field": "attr:gone"}}, "sheets": "single", "sheet_name": "Báo cáo",
		"title_row": true, "summary": false, "header": "bold", "freeze": true, "filter": true, "stripes": false,
		"date_format": "dd/mm/yyyy", "bool_style": "yes_no", "status_as": "name", "unit_in": "header",
	}
	p := decode[struct{ ID string }](t, a.do("POST", "/api/v1/export-profiles", exp, map[string]any{"name": "Kiểm kê quý 3", "layout": layout}), 201)
	rec = a.do("POST", "/api/v1/assets/export", exp, map[string]any{
		"mode": "report", "profile_id": p.ID, "filters": map[string]any{"ids": ids[:2]},
	})
	if rec.Code != 200 {
		t.Fatalf("report export: %d %s", rec.Code, rec.Body)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "filename*=utf-8''") {
		t.Errorf("content-disposition = %q", cd)
	}
	if got := rec.Header().Get("X-Export-Skipped-Columns"); got != "gone" {
		t.Errorf("skipped = %q", got)
	}

	if rec := a.do("POST", "/api/v1/assets/export", a.token(domain.PermAssetRead), map[string]any{"mode": "data"}); rec.Code != 403 {
		t.Errorf("without export permission: %d", rec.Code)
	}
	other := a.token(domain.PermAssetRead, domain.PermAssetExport)
	if rec := a.do("GET", "/api/v1/export-profiles/"+p.ID, other, nil); rec.Code != 404 {
		t.Errorf("someone else's private profile: %d", rec.Code)
	}
	if rec := a.do("PATCH", "/api/v1/export-profiles/"+p.ID, exp, map[string]any{"shared": true, "version": 1}); rec.Code != 200 {
		t.Errorf("share: %d %s", rec.Code, rec.Body)
	}
	if rec := a.do("DELETE", "/api/v1/export-profiles/"+p.ID, other, nil); rec.Code != 403 {
		t.Errorf("non-manager deletes shared profile: %d", rec.Code)
	}

	small := newAppWith(t, 1)
	stok := small.token(allPerms...)
	if rec := small.do("POST", "/api/v1/assets/export", stok, map[string]any{"mode": "data"}); rec.Code != 422 {
		t.Errorf("over the cap: %d", rec.Code)
	} else if typ, _ := problem(t, rec); typ != "/errors/export-too-large" {
		t.Errorf("problem type = %s", typ)
	}
}
```

Add `domain.PermAssetExport` and `domain.PermExportProfileManage` to `allPerms` if that list is explicit; import `github.com/xuri/excelize/v2` and `fmt`. Note: the `token` helper issues a token for a **new random account** each call, so `exp` and `other` are different owners.

- [ ] **Step 5: Run everything**

Run: `cd backend && make check` then `CI=true go test -p 1 ./internal/inventory/...`
Expected: both PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/inventory backend/cmd/server
git commit -m "feat(inventory): export and export profile endpoints

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8: Backend documentation

**Files:**
- Modify: `backend/docs/inventory.md`
- Modify: `backend/docs/platform.md` (config table)

- [ ] **Step 1: Document**

In `inventory.md`, add a section "Export (US-07 to US-09)" with: the endpoints table (from the spec, section 2), the permissions and their grants, the data export format (Global Constraints line), report layout fields and validation limits, skipped columns header, the 50,000 cap and `INVENTORY_EXPORT_MAX_ROWS`, profile visibility and edit rules, and the events (`export_profile_created/updated/deleted`, `assets_exported`). Update the events list sentence and the permissions table rows to include the two new codes.

In `platform.md`'s configuration table, add the row:

```markdown
| `inventory.Config` | `INVENTORY_EXPORT_MAX_ROWS` (50000; 1 to 1000000). Server only |
```

- [ ] **Step 2: Commit**

```bash
git add backend/docs/inventory.md backend/docs/platform.md
git commit -m "docs(inventory): export, report layouts and export profiles

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 9: Frontend export logic (types, layout, format, download, API)

**Files:**
- Modify: `frontend/src/lib/api/inventory.d.ts` (generated), `frontend/src/lib/api/types.ts`, `frontend/src/lib/auth/permissions.ts`
- Create: `frontend/src/lib/download.ts`, `frontend/src/lib/download.test.ts`
- Create: `frontend/src/features/assets/export/layout.ts`, `layout.test.ts`, `format.ts`, `format.test.ts`, `api.ts`, `useExport.ts`

**Interfaces:**
- Produces:
  - Types: `ExportLayout`, `ExportColumn`, `ExportFilters`, `ExportRequest`, `ExportProfile` (aliases of generated schemas)
  - `Perm.AssetExport`, `Perm.ExportProfileManage`
  - `fileNameFrom(header: string | null, fallback: string): string`; `saveBlob(blob: Blob, name: string): void`
  - `layout.ts`: `COMMON_FIELDS`, `defaultReportLayout(): ExportLayout`, `fieldOptions(types: TypeInfo[]): FieldOption[]`, `defaultHeader(field: string, layout: ExportLayout, options: FieldOption[]): string`, `previewSheets(layout, rows, types): PreviewSheet[]`, `skippedKeys(layout, types): string[]`, `normalizeLayout(layout): string`, types `TypeInfo`, `FieldOption`, `PreviewSheet`, `EditorColumn`
  - `format.ts`: `formatCell(row: AssetListItem, field: string, layout: ExportLayout, types: TypeInfo[]): { text: string; align?: 'right' | 'center' }`
  - `api.ts`: `exportProfileKeys`, `useExportProfiles()`, `useCreateExportProfile()`, `useUpdateExportProfile()`, `useDeleteExportProfile()`, `exportAssets(body: ExportRequest): Promise<{ name: string; skipped: string[] }>`
  - `useExport.ts`: `useExport()` → `{ run(body: ExportRequest, fallbackName: string): Promise<void>, running: Ref<boolean> }`

- [ ] **Step 1: Generate types and aliases**

Run: `cd frontend && npm run gen:api`
Expected: `src/lib/api/inventory.d.ts` contains `ExportLayout`, `ExportProfile`, `/assets/export`.

Add to `src/lib/api/types.ts`:

```ts
export type ExportLayout = V['ExportLayout']
export type ExportColumn = V['ExportColumn']
export type ExportFilters = V['ExportFilters']
export type ExportRequest = V['ExportRequest']
export type ExportProfile = V['ExportProfile']
```

(and `export type AttributeValue = V['AttributeValue']` if not present). Add to `Perm` in `src/lib/auth/permissions.ts`:

```ts
  AssetExport: 'inventory.asset.export',
  ExportProfileManage: 'inventory.export_profile.manage',
```

- [ ] **Step 2: Write the failing tests**

`src/lib/download.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import { fileNameFrom } from './download'

describe('fileNameFrom', () => {
  it('prefers the UTF-8 name', () => {
    expect(fileNameFrom(`attachment; filename*=utf-8''Ki%E1%BB%83m-k%C3%AA-2026-10-06.xlsx`, 'x.xlsx')).toBe('Kiểm-kê-2026-10-06.xlsx')
  })
  it('reads a quoted or plain name', () => {
    expect(fileNameFrom('attachment; filename="storeit-assets-2026-10-06.xlsx"', 'x.xlsx')).toBe('storeit-assets-2026-10-06.xlsx')
    expect(fileNameFrom('attachment; filename=a.xlsx', 'x.xlsx')).toBe('a.xlsx')
  })
  it('falls back without a header', () => {
    expect(fileNameFrom(null, 'x.xlsx')).toBe('x.xlsx')
  })
})
```

`src/features/assets/export/layout.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import type { AssetListItem } from '@/lib/api/types'
import { defaultHeader, defaultReportLayout, fieldOptions, previewSheets, skippedKeys, type TypeInfo } from './layout'

const laptop: TypeInfo = { id: 'L', name: 'Laptop', code: 'LAPTOP', attributes: [{ key: 'ram_gb', label: 'RAM', data_type: 'number', unit: 'GB' }, { key: 'cpu', label: 'CPU', data_type: 'text' }] }
const phone: TypeInfo = { id: 'P', name: 'Phone', code: 'PHONE', attributes: [{ key: 'imei', label: 'IMEI', data_type: 'text' }] }
const row = (tag: string, typeId: string) => ({ id: tag, tag, name: tag, asset_type_id: typeId, asset_type_name: typeId, status_id: 's', status_name: 'Available', status_kind: 'available', version: 1, updated_at: '2026-10-06T00:00:00Z' }) as AssetListItem

describe('layout', () => {
  it('lists common fields then attributes once', () => {
    const opts = fieldOptions([laptop, phone])
    expect(opts.map((o) => o.field)).toEqual(['tag', 'name', 'description', 'type', 'status', 'purchase_date', 'updated_at', 'attr:ram_gb', 'attr:cpu', 'attr:imei'])
  })

  it('puts the unit in the default header when asked', () => {
    const l = defaultReportLayout()
    const opts = fieldOptions([laptop])
    expect(defaultHeader('attr:ram_gb', l, opts)).toBe('RAM (GB)')
    expect(defaultHeader('attr:ram_gb', { ...l, unit_in: 'cell' }, opts)).toBe('RAM')
  })

  it('builds per-type sheets with only that type’s attribute columns', () => {
    const l = { ...defaultReportLayout(), sheets: 'per_type' as const, columns: [{ field: 'tag' }, { field: 'attr:imei' }] }
    const sheets = previewSheets(l, [row('L1', 'L'), row('P1', 'P')], [laptop, phone])
    expect(sheets.map((s) => s.name)).toEqual(['Laptop', 'Phone'])
    expect(sheets[0].columns.map((c) => c.field)).toEqual(['tag'])
    expect(sheets[1].columns.map((c) => c.field)).toEqual(['tag', 'attr:imei'])
    const each = previewSheets({ ...l, each_type_attrs: true }, [row('L1', 'L')], [laptop])
    expect(each[0].columns.map((c) => c.field)).toEqual(['tag', 'attr:ram_gb', 'attr:cpu'])
  })

  it('reports attribute keys no type has', () => {
    const l = { ...defaultReportLayout(), columns: [{ field: 'tag' }, { field: 'attr:gone' }] }
    expect(skippedKeys(l, [laptop])).toEqual(['gone'])
  })
})
```

`src/features/assets/export/format.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import type { AssetListItem } from '@/lib/api/types'
import { formatCell } from './format'
import { defaultReportLayout, type TypeInfo } from './layout'

const laptop: TypeInfo = { id: 'L', name: 'Laptop', code: 'LAPTOP', attributes: [{ key: 'ram_gb', label: 'RAM', data_type: 'number', unit: 'GB' }] }
const row = {
  id: '1', tag: 'LAP-1', name: 'ThinkPad', asset_type_id: 'L', asset_type_name: 'Laptop', status_id: 's', status_name: 'On loan',
  status_kind: 'in_use', purchase_date: '2025-03-14', version: 1, updated_at: '2026-10-06T00:00:00Z',
  attributes: [
    { key: 'ram_gb', label: 'RAM', data_type: 'number', unit: 'GB', value: 16 },
    { key: 'touch', label: 'Touch', data_type: 'boolean', value: true },
  ],
} as AssetListItem

describe('formatCell', () => {
  it('formats dates, numbers, booleans and status like the server', () => {
    const l = defaultReportLayout()
    expect(formatCell(row, 'purchase_date', l, [laptop]).text).toBe('14/03/2025')
    expect(formatCell(row, 'purchase_date', { ...l, date_format: 'd mmm yyyy' }, [laptop]).text).toBe('14 Mar 2025')
    expect(formatCell(row, 'attr:ram_gb', l, [laptop])).toEqual({ text: '16', align: 'right' })
    expect(formatCell(row, 'attr:ram_gb', { ...l, unit_in: 'cell' }, [laptop]).text).toBe('16 GB')
    expect(formatCell(row, 'attr:touch', l, [laptop]).text).toBe('Yes')
    expect(formatCell(row, 'attr:touch', { ...l, bool_style: 'check' }, [laptop]).text).toBe('✓')
    expect(formatCell(row, 'status', l, [laptop]).text).toBe('On loan')
    expect(formatCell(row, 'status', { ...l, status_as: 'kind' }, [laptop]).text).toBe('In use')
    expect(formatCell(row, 'attr:missing', l, [laptop]).text).toBe('')
  })
})
```

- [ ] **Step 3: Run to see them fail**

Run: `cd frontend && npx vitest run src/lib/download.test.ts src/features/assets/export`
Expected: FAIL (modules not found).

- [ ] **Step 4: Implement `download.ts`**

```ts
// Tải file nhận từ API (blob): tên file lấy từ Content-Disposition, lưu qua một liên kết tạm

// fileNameFrom: ưu tiên filename*=utf-8''… (tên có dấu), rồi filename="…"
export function fileNameFrom(header: string | null, fallback: string): string {
  if (!header) return fallback
  const star = /filename\*\s*=\s*utf-8''([^;]+)/i.exec(header)
  if (star) {
    try {
      return decodeURIComponent(star[1].trim())
    } catch {
      // tên mã hoá hỏng: dùng tên thường bên dưới
    }
  }
  const plain = /filename\s*=\s*"?([^";]+)"?/i.exec(header)
  return plain ? plain[1].trim() : fallback
}

export function saveBlob(blob: Blob, name: string): void {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = name
  document.body.appendChild(a)
  a.click()
  a.remove()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}
```

- [ ] **Step 5: Implement `layout.ts`**

```ts
// Bố cục báo cáo ở giao diện: mặc định, danh sách trường, tiêu đề mặc định, dựng sheet
// cho bản xem trước. Khớp backend (inventory/domain/export.go).
import type { AssetListItem, DataType, ExportLayout } from '@/lib/api/types'

export interface TypeAttr {
  key: string
  label: string
  data_type: DataType
  unit?: string
}
export interface TypeInfo {
  id: string
  name: string
  code: string
  attributes: TypeAttr[] // đang dùng, theo thứ tự hiển thị
}
export interface FieldOption {
  field: string
  label: string
  unit?: string
  types: string[] // tên loại có thuộc tính này; rỗng là trường chung
}
export interface EditorColumn {
  field: string
  header: string
  include: boolean
}
export interface PreviewColumn {
  field: string
  header: string
}
export interface PreviewSheet {
  name: string
  columns: PreviewColumn[]
  rows: AssetListItem[]
}

export const COMMON_FIELDS: { field: string; label: string }[] = [
  { field: 'tag', label: 'Tag' },
  { field: 'name', label: 'Name' },
  { field: 'description', label: 'Description' },
  { field: 'type', label: 'Type' },
  { field: 'status', label: 'Status' },
  { field: 'purchase_date', label: 'Purchase date' },
  { field: 'updated_at', label: 'Updated' },
]

export const attrKey = (field: string) => (field.startsWith('attr:') ? field.slice(5) : null)

export function defaultReportLayout(): ExportLayout {
  return {
    columns: ['tag', 'name', 'type', 'status', 'purchase_date'].map((field) => ({ field })),
    sheets: 'single',
    each_type_attrs: false,
    sheet_name: 'Assets',
    title_row: false,
    summary: false,
    header: 'bold',
    freeze: true,
    filter: true,
    stripes: false,
    date_format: 'dd/mm/yyyy',
    bool_style: 'yes_no',
    status_as: 'name',
    unit_in: 'header',
    sort: '',
  }
}

export function fieldOptions(types: TypeInfo[]): FieldOption[] {
  const out: FieldOption[] = COMMON_FIELDS.map((c) => ({ ...c, types: [] }))
  const byKey = new Map<string, FieldOption>()
  for (const t of types) {
    for (const a of t.attributes) {
      const f = byKey.get(a.key)
      if (f) f.types.push(t.name)
      else {
        const opt = { field: `attr:${a.key}`, label: a.label, unit: a.unit, types: [t.name] }
        byKey.set(a.key, opt)
        out.push(opt)
      }
    }
  }
  return out
}

export function defaultHeader(field: string, layout: ExportLayout, options: FieldOption[]): string {
  const o = options.find((x) => x.field === field)
  const label = o?.label ?? attrKey(field) ?? field
  return o?.unit && layout.unit_in === 'header' ? `${label} (${o.unit})` : label
}

// editorColumns: cột của bố cục (đã chọn, đúng thứ tự) rồi các trường còn lại (chưa chọn)
export function editorColumns(layout: ExportLayout, options: FieldOption[]): EditorColumn[] {
  const chosen = layout.columns.map((c) => ({ field: c.field, header: c.header ?? '', include: true }))
  const rest = options.filter((o) => !chosen.some((c) => c.field === o.field)).map((o) => ({ field: o.field, header: '', include: false }))
  return [...chosen, ...rest]
}

// withColumns: bố cục với các cột đang chọn của trình sửa cột
export function withColumns(layout: ExportLayout, cols: EditorColumn[]): ExportLayout {
  return { ...layout, columns: cols.filter((c) => c.include).map((c) => ({ field: c.field, header: c.header.trim() || undefined })) }
}

function sheetColumns(layout: ExportLayout, types: TypeInfo[], options: FieldOption[]): PreviewColumn[] {
  const has = (key: string) => types.some((t) => t.attributes.some((a) => a.key === key))
  const out: PreviewColumn[] = []
  for (const c of layout.columns) {
    const key = attrKey(c.field)
    if (key && !has(key)) continue
    out.push({ field: c.field, header: c.header?.trim() || defaultHeader(c.field, layout, options) })
  }
  if (layout.sheets === 'per_type' && layout.each_type_attrs) {
    for (const t of types) {
      for (const a of t.attributes) {
        const f = `attr:${a.key}`
        if (!out.some((c) => c.field === f)) out.push({ field: f, header: defaultHeader(f, layout, options) })
      }
    }
  }
  return out
}

export function previewSheets(layout: ExportLayout, rows: AssetListItem[], types: TypeInfo[]): PreviewSheet[] {
  const options = fieldOptions(types)
  if (layout.sheets === 'single') {
    return [{ name: layout.sheet_name || 'Assets', columns: sheetColumns(layout, types, options), rows }]
  }
  return [...types]
    .sort((a, b) => a.name.localeCompare(b.name))
    .map((t) => ({ name: t.name, columns: sheetColumns(layout, [t], options), rows: rows.filter((r) => r.asset_type_id === t.id) }))
    .filter((s) => s.rows.length)
}

export function skippedKeys(layout: ExportLayout, types: TypeInfo[]): string[] {
  return layout.columns
    .map((c) => attrKey(c.field))
    .filter((k): k is string => !!k && !types.some((t) => t.attributes.some((a) => a.key === k)))
}

// normalizeLayout: chuỗi so sánh để biết bố cục đã đổi so với profile chưa
export function normalizeLayout(l: ExportLayout): string {
  return JSON.stringify({ ...l, columns: l.columns.map((c) => ({ field: c.field, header: c.header?.trim() || '' })), sort: l.sort ?? '', each_type_attrs: !!l.each_type_attrs })
}
```

Check the `DataType` alias exists in `types.ts` (`grep -n "DataType" frontend/src/lib/api/types.ts`); add `export type DataType = V['DataType']` if missing.

- [ ] **Step 6: Implement `format.ts`**

```ts
// Ô của bản xem trước, cùng luật định dạng với server (spreadsheet/format.go)
import type { AssetListItem, ExportLayout } from '@/lib/api/types'
import { attrKey, type TypeInfo } from './layout'

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']
const KIND: Record<string, string> = { available: 'Available', in_use: 'In use', unavailable: 'Unavailable', retired: 'Retired' }

export function formatDate(iso: string | undefined | null, f: ExportLayout['date_format']): string {
  if (!iso) return ''
  const [y, m, d] = iso.slice(0, 10).split('-')
  if (f === 'yyyy-mm-dd') return `${y}-${m}-${d}`
  if (f === 'd mmm yyyy') return `${Number(d)} ${MONTHS[Number(m) - 1]} ${y}`
  return `${d}/${m}/${y}`
}

export function formatCell(row: AssetListItem, field: string, l: ExportLayout, _types: TypeInfo[]): { text: string; align?: 'right' | 'center' } {
  switch (field) {
    case 'tag':
      return { text: row.tag }
    case 'name':
      return { text: row.name }
    case 'description':
      return { text: row.description ?? '' }
    case 'type':
      return { text: row.asset_type_name }
    case 'status':
      return { text: l.status_as === 'kind' ? KIND[row.status_kind] : row.status_name }
    case 'purchase_date':
      return { text: formatDate(row.purchase_date, l.date_format), align: 'right' }
    case 'updated_at':
      return { text: `${formatDate(row.updated_at, l.date_format)} ${new Date(row.updated_at).toTimeString().slice(0, 5)}`, align: 'right' }
  }
  const key = attrKey(field)
  const v = row.attributes?.find((a) => a.key === key)
  if (!v || v.value === null || v.value === undefined) return { text: '' }
  switch (v.data_type) {
    case 'number':
      return { text: l.unit_in === 'cell' && v.unit ? `${v.value} ${v.unit}` : String(v.value), align: 'right' }
    case 'date':
      return { text: formatDate(String(v.value), l.date_format), align: 'right' }
    case 'boolean':
      return { text: l.bool_style === 'check' ? (v.value ? '✓' : '–') : v.value ? 'Yes' : 'No', align: 'center' }
    case 'select':
      return { text: v.option_label ?? '' }
  }
  return { text: String(v.value) }
}
```

If `AssetListItem` has no `description`, use `''` for that case (check the generated type).

- [ ] **Step 7: Implement `api.ts` and `useExport.ts`**

`features/assets/export/api.ts`:

```ts
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { inventoryApi } from '@/lib/api/client'
import type { ExportLayout, ExportRequest } from '@/lib/api/types'
import { unwrap } from '@/lib/errors'
import { fileNameFrom, saveBlob } from '@/lib/download'

export const exportProfileKeys = { all: ['export-profiles'] as const }

export function useExportProfiles(enabled = true) {
  return useQuery({
    queryKey: exportProfileKeys.all,
    queryFn: async () => (await unwrap(inventoryApi.GET('/export-profiles'))).items,
    enabled,
  })
}

function useProfileMutation<V, R>(fn: (v: V) => Promise<R>, toast = true) {
  const qc = useQueryClient()
  return useMutation({ mutationFn: fn, meta: { toast }, onSuccess: () => qc.invalidateQueries({ queryKey: exportProfileKeys.all }) })
}

export function useCreateExportProfile() {
  return useProfileMutation(
    (body: { name: string; shared: boolean; layout: ExportLayout }) => unwrap(inventoryApi.POST('/export-profiles', { body })),
    false,
  )
}

export function useUpdateExportProfile() {
  return useProfileMutation(({ id, ...body }: { id: string; version: number; name?: string; shared?: boolean; layout?: ExportLayout }) =>
    unwrap(inventoryApi.PATCH('/export-profiles/{profileID}', { params: { path: { profileID: id } }, body })),
  )
}

export function useDeleteExportProfile() {
  return useProfileMutation((id: string) => unwrap(inventoryApi.DELETE('/export-profiles/{profileID}', { params: { path: { profileID: id } } })))
}

// exportAssets tải file và lưu; trả tên file và các cột bị bỏ
export async function exportAssets(body: ExportRequest, fallbackName: string): Promise<{ name: string; skipped: string[] }> {
  const res = await inventoryApi.POST('/assets/export', { body, parseAs: 'blob' })
  const blob = (await unwrap(Promise.resolve(res))) as Blob
  const name = fileNameFrom(res.response.headers.get('Content-Disposition'), fallbackName)
  saveBlob(blob, name)
  const skipped = (res.response.headers.get('X-Export-Skipped-Columns') ?? '').split(',').filter(Boolean)
  return { name, skipped }
}
```

Check how `unwrap` is typed (`grep -n "export async function unwrap" -A10 frontend/src/lib/errors.ts`) and adjust the call if it does not accept the openapi-fetch result directly.

`features/assets/export/useExport.ts`:

```ts
// Chạy một lần export kèm thông báo: tên file, cột bị bỏ, hay lỗi (vd quá số dòng)
import { ref } from 'vue'
import type { ExportRequest } from '@/lib/api/types'
import { describeError } from '@/lib/errors'
import { notify } from '@/lib/notify'
import { exportAssets } from './api'

export function useExport() {
  const running = ref(false)
  async function run(body: ExportRequest, fallbackName: string) {
    running.value = true
    try {
      const { name, skipped } = await exportAssets(body, fallbackName)
      notify.success(`Downloaded ${name}.`)
      if (skipped.length) notify.info(`Skipped columns: ${skipped.join(', ')} (attribute removed).`)
    } catch (err) {
      notify.error(describeError(err))
    } finally {
      running.value = false
    }
  }
  return { run, running }
}
```

Check `notify` has `success`, `info`, `error` (`grep -n "success\|info\|error" frontend/src/lib/notify.ts`).

- [ ] **Step 8: Run tests and checks**

Run: `cd frontend && npx vitest run src/lib/download.test.ts src/features/assets/export && npx vue-tsc --noEmit -p tsconfig.app.json`
Expected: PASS, no type errors.

- [ ] **Step 9: Commit**

```bash
git add frontend/src/lib/api/inventory.d.ts frontend/src/lib/api/types.ts frontend/src/lib/auth/permissions.ts frontend/src/lib/download.ts frontend/src/lib/download.test.ts frontend/src/features/assets/export
git commit -m "feat(frontend): export layout, preview formatting, download and profile API

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 10: Report dialog components

**Files:**
- Create: `frontend/src/features/assets/export/usePreviewData.ts`
- Create: `frontend/src/features/assets/export/components/SheetPreview.vue`
- Create: `frontend/src/features/assets/export/components/ColumnEditor.vue`
- Create: `frontend/src/features/assets/export/components/ReportDialog.vue`

**Interfaces:**
- Consumes: Task 9 modules; `useAssetType` from `features/asset-types/api`; `inventoryApi`.
- Produces:
  - `export interface ExportScope { filters: ExportFilters; label: string; count: number; typeIds: string[]; rows: AssetListItem[]; selection: boolean }`
  - `usePreviewData(scope: () => ExportScope)` → `{ types: ComputedRef<TypeInfo[]>, rows: ComputedRef<AssetListItem[]> }`
  - `<ReportDialog v-model:visible :scope :profile-id? />`

- [ ] **Step 1: Preview data**

`usePreviewData.ts`:

```ts
// Dữ liệu cho bản xem trước: định nghĩa thuộc tính của các loại trong phạm vi và tối đa
// 20 dòng mỗi loại (danh sách chỉ có giá trị thuộc tính khi lọc theo một loại)
import { useQueries } from '@tanstack/vue-query'
import { computed } from 'vue'
import { inventoryApi } from '@/lib/api/client'
import type { AssetListItem, ExportFilters } from '@/lib/api/types'
import { unwrap } from '@/lib/errors'
import type { TypeInfo } from './layout'

export interface ExportScope {
  filters: ExportFilters
  label: string
  count: number
  typeIds: string[]
  rows: AssetListItem[] // dòng đang thấy (dùng khi xem trước phần đã chọn)
  selection: boolean
}

const MAX_PREVIEW_TYPES = 8

export function usePreviewData(scope: () => ExportScope, enabled: () => boolean) {
  const ids = computed(() => scope().typeIds.slice(0, MAX_PREVIEW_TYPES))
  const typeQueries = useQueries({
    queries: computed(() =>
      ids.value.map((id) => ({
        queryKey: ['asset-types', id],
        queryFn: () => unwrap(inventoryApi.GET('/asset-types/{typeID}', { params: { path: { typeID: id } } })),
        enabled: enabled(),
      })),
    ),
  })
  const rowQueries = useQueries({
    queries: computed(() =>
      ids.value.map((id) => {
        const f = scope().filters
        return {
          queryKey: ['export-preview', id, f],
          queryFn: async () =>
            (
              await unwrap(
                inventoryApi.GET('/assets', {
                  params: { query: { q: f.q, type_id: id, status_id: f.status_id, status_kind: f.status_kind, include_retired: f.include_retired, attr: f.type_id ? f.attr : undefined, sort: f.sort, page: 1, page_size: 20 } },
                }),
              )
            ).items,
          enabled: enabled() && !scope().selection,
        }
      }),
    ),
  })
  const types = computed<TypeInfo[]>(() =>
    typeQueries.value
      .map((q) => q.data)
      .filter((t): t is NonNullable<typeof t> => !!t)
      .map((t) => ({
        id: t.id,
        name: t.name,
        code: t.code,
        attributes: t.attributes.filter((a) => !a.removed).map((a) => ({ key: a.key, label: a.label, data_type: a.data_type, unit: a.unit ?? undefined })),
      })),
  )
  const rows = computed<AssetListItem[]>(() =>
    scope().selection ? scope().rows : rowQueries.value.flatMap((q) => q.data ?? []),
  )
  return { types, rows }
}
```

Check the attribute field that marks removal on the type detail (`grep -n "removed" frontend/src/lib/api/inventory.d.ts | head`) and the exact `GET /assets` query parameter names; adapt.

- [ ] **Step 2: SheetPreview.vue**

```vue
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { ExportLayout } from '@/lib/api/types'
import { formatCell } from '../format'
import type { PreviewSheet, TypeInfo } from '../layout'

// Bản xem trước như Excel: chữ cột, số dòng, tab sheet; 20 dòng đầu mỗi sheet
const props = defineProps<{ sheets: PreviewSheet[]; layout: ExportLayout; types: TypeInfo[]; title: string[] }>()
const active = ref(0)
const tabs = computed(() => [...props.sheets.map((s) => s.name), ...(props.layout.summary ? ['Summary'] : [])])
watch(tabs, (t) => {
  if (active.value >= t.length) active.value = 0
})
const isSummary = computed(() => props.layout.summary && active.value === props.sheets.length)
const sheet = computed(() => props.sheets[active.value])
const letter = (i: number) => {
  let s = ''
  for (let n = i + 1; n > 0; n = Math.floor((n - 1) / 26)) s = String.fromCharCode(65 + ((n - 1) % 26)) + s
  return s
}
const KINDS = ['available', 'in_use', 'unavailable', 'retired'] as const
const KIND_LABEL = { available: 'Available', in_use: 'In use', unavailable: 'Unavailable', retired: 'Retired' }
const summary = computed(() =>
  props.sheets.flatMap((s) => s.rows).reduce<Record<string, Record<string, number>>>((acc, r) => {
    acc[r.asset_type_name] ??= {}
    acc[r.asset_type_name][r.status_kind] = (acc[r.asset_type_name][r.status_kind] ?? 0) + 1
    return acc
  }, {}),
)
</script>

<template>
  <div class="sheet">
    <div class="sheet-scroll">
      <table v-if="isSummary" class="xl h-bold freeze">
        <thead>
          <tr><th class="corner" /><th v-for="i in 6" :key="i" class="ch">{{ letter(i - 1) }}</th></tr>
        </thead>
        <tbody>
          <tr class="hdr"><td class="rh">1</td><td>Type</td><td v-for="k in KINDS" :key="k">{{ KIND_LABEL[k] }}</td><td>Total</td></tr>
          <tr v-for="(byKind, name, n) in summary" :key="name">
            <td class="rh">{{ n + 2 }}</td><td>{{ name }}</td>
            <td v-for="k in KINDS" :key="k" class="n">{{ byKind[k] ?? 0 }}</td>
            <td class="n">{{ Object.values(byKind).reduce((a, b) => a + b, 0) }}</td>
          </tr>
        </tbody>
      </table>
      <table v-else-if="sheet" class="xl" :class="[`h-${layout.header}`, { freeze: layout.freeze, stripes: layout.stripes }]">
        <thead>
          <tr><th class="corner" /><th v-for="(_, i) in sheet.columns" :key="i" class="ch">{{ letter(i) }}</th></tr>
        </thead>
        <tbody>
          <tr v-for="(line, i) in title" :key="`t${i}`">
            <td class="rh">{{ i + 1 }}</td>
            <td :class="i === 0 ? 'title' : 'titlesub'" :colspan="sheet.columns.length">{{ line }}</td>
          </tr>
          <tr class="hdr">
            <td class="rh">{{ title.length + 1 }}</td>
            <td v-for="c in sheet.columns" :key="c.field">{{ c.header }}<span v-if="layout.filter" class="filter-arrow">▼</span></td>
          </tr>
          <tr v-for="(r, i) in sheet.rows.slice(0, 20)" :key="r.id" class="data">
            <td class="rh">{{ title.length + i + 2 }}</td>
            <td v-for="c in sheet.columns" :key="c.field" :class="formatCell(r, c.field, layout, types).align">
              {{ formatCell(r, c.field, layout, types).text }}
            </td>
          </tr>
        </tbody>
      </table>
      <p v-else class="empty">No rows to preview.</p>
    </div>
    <div class="sheet-tabs" role="tablist">
      <button v-for="(t, i) in tabs" :key="t" type="button" role="tab" class="sheet-tab" :class="{ active: i === active }" :aria-selected="i === active" @click="active = i">
        {{ t }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.sheet { border: 1px solid var(--app-line); border-radius: 8px; background: var(--p-content-background); overflow: hidden; font: 13px/1.3 Calibri, Carlito, 'Segoe UI', sans-serif; }
.sheet-scroll { overflow: auto; max-height: 26rem; }
table.xl { border-collapse: collapse; }
table.xl th, table.xl td { border: 1px solid var(--app-line); padding: 0.18rem 0.45rem; white-space: nowrap; height: 1.55rem; }
.rh, .ch { background: var(--app-soft); color: var(--p-text-muted-color); font: 11px var(--app-body); text-align: center; position: sticky; z-index: 1; }
.ch { top: 0; }
.rh { left: 0; min-width: 2.2rem; }
.corner { position: sticky; top: 0; left: 0; z-index: 2; background: var(--app-soft); }
.title { font-weight: 700; font-size: 14px; }
.titlesub { color: var(--p-text-muted-color); font-size: 12px; }
.h-bold .hdr td, .h-bold_fill .hdr td { font-weight: 700; }
.h-bold_fill .hdr td { background: var(--p-highlight-background); color: var(--p-highlight-color); }
.freeze .hdr td { border-bottom: 2px solid var(--p-text-muted-color); }
.stripes tr.data:nth-of-type(even) td { background: var(--app-ground); }
td.right { text-align: right; font-variant-numeric: tabular-nums; }
td.center { text-align: center; }
.filter-arrow { display: inline-grid; place-items: center; width: 0.95rem; height: 0.95rem; margin-left: 0.35rem; border: 1px solid var(--app-line); border-radius: 2px; font-size: 8px; color: var(--p-text-muted-color); }
.sheet-tabs { display: flex; gap: 2px; padding: 0 0.4rem; border-top: 1px solid var(--app-line); background: var(--app-soft); overflow-x: auto; }
.sheet-tab { padding: 0.3rem 0.8rem; border: 0; border-bottom: 2px solid transparent; background: transparent; color: var(--p-text-muted-color); font: 12px var(--app-body); cursor: pointer; }
.sheet-tab.active { background: var(--p-content-background); color: var(--p-text-color); font-weight: 600; border-bottom-color: var(--app-accent); }
.empty { padding: 2rem; text-align: center; color: var(--p-text-muted-color); }
</style>
```

- [ ] **Step 3: ColumnEditor.vue**

```vue
<script setup lang="ts">
import Checkbox from 'primevue/checkbox'
import Column from 'primevue/column'
import DataTable, { type DataTableRowReorderEvent } from 'primevue/datatable'
import InputText from 'primevue/inputtext'
import type { ExportLayout } from '@/lib/api/types'
import { defaultHeader, type EditorColumn, type FieldOption } from '../layout'

// Chọn, sắp (kéo hay Alt+↑/↓) và đổi tên cột của báo cáo
const props = defineProps<{ options: FieldOption[]; layout: ExportLayout }>()
const columns = defineModel<EditorColumn[]>({ required: true })

const option = (f: string) => props.options.find((o) => o.field === f)
function onReorder(e: DataTableRowReorderEvent) {
  columns.value = e.value as EditorColumn[]
}
function move(i: number, d: number) {
  const j = i + d
  if (j < 0 || j >= columns.value.length) return
  const next = [...columns.value]
  ;[next[i], next[j]] = [next[j], next[i]]
  columns.value = next
}
function set(i: number, patch: Partial<EditorColumn>) {
  columns.value = columns.value.map((c, n) => (n === i ? { ...c, ...patch } : c))
}
</script>

<template>
  <DataTable :value="columns" data-key="field" :show-headers="false" size="small" class="col-editor" table-style="width: 100%; table-layout: fixed" @row-reorder="onReorder">
    <Column row-reorder header-style="width: 2rem" body-style="width: 2rem" />
    <Column header-style="width: 2rem" body-style="width: 2rem">
      <template #body="{ data: c, index: i }: { data: EditorColumn; index: number }">
        <Checkbox :model-value="c.include" binary :aria-label="`Include ${option(c.field)?.label ?? c.field}`" @update:model-value="(v: boolean) => set(i, { include: v })" />
      </template>
    </Column>
    <Column>
      <template #body="{ data: c, index: i }: { data: EditorColumn; index: number }">
        <div class="label" :class="{ off: !c.include }" tabindex="0" @keydown.alt.up.prevent="move(i, -1)" @keydown.alt.down.prevent="move(i, 1)">
          <span>{{ option(c.field)?.label ?? c.field }}</span>
          <small>{{ c.field }}<template v-if="option(c.field)?.types.length"> · {{ option(c.field)!.types.join(', ') }}</template></small>
        </div>
      </template>
    </Column>
    <Column header-style="width: 9rem" body-style="width: 9rem">
      <template #body="{ data: c, index: i }: { data: EditorColumn; index: number }">
        <InputText :model-value="c.header" size="small" fluid :placeholder="defaultHeader(c.field, layout, options)" :disabled="!c.include" :aria-label="`Header for ${option(c.field)?.label ?? c.field}`" maxlength="100" @update:model-value="(v) => set(i, { header: v ?? '' })" />
      </template>
    </Column>
  </DataTable>
</template>

<style scoped>
.col-editor { border: 0; }
.col-editor :deep(td) { border-bottom: 0; }
.label { display: flex; flex-direction: column; min-width: 0; }
.label span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.label small { font: 0.7rem var(--app-mono); color: var(--p-text-muted-color); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.label.off { opacity: 0.5; }
</style>
```

- [ ] **Step 4: ReportDialog.vue**

```vue
<script setup lang="ts">
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import Select from 'primevue/select'
import Tag from 'primevue/tag'
import { computed, ref, watch } from 'vue'
import SegmentedFilter from '@/components/SegmentedFilter.vue'
import type { ExportLayout, ExportProfile } from '@/lib/api/types'
import { useSession } from '@/lib/auth/session'
import { useFormErrors } from '@/lib/forms'
import { notify } from '@/lib/notify'
import { useCreateExportProfile, useExportProfiles, useUpdateExportProfile } from '../api'
import { defaultReportLayout, editorColumns, fieldOptions, normalizeLayout, previewSheets, skippedKeys, withColumns, type EditorColumn } from '../layout'
import { useExport } from '../useExport'
import { usePreviewData, type ExportScope } from '../usePreviewData'
import ColumnEditor from './ColumnEditor.vue'
import SheetPreview from './SheetPreview.vue'

// Hộp thoại "Export report": chọn profile, chỉnh cột và định dạng, xem trước, tải về
const props = defineProps<{ scope: ExportScope; profileId?: string }>()
const visible = defineModel<boolean>('visible', { required: true })

const session = useSession()
const { data: profiles } = useExportProfiles(true)
const { types, rows } = usePreviewData(() => props.scope, () => visible.value)
const options = computed(() => fieldOptions(types.value))

const profileId = ref<string | null>(null)
const profile = computed<ExportProfile | undefined>(() => profiles.value?.find((p) => p.id === profileId.value))
const layout = ref<ExportLayout>(defaultReportLayout())
const columns = ref<EditorColumn[]>([])
const saved = ref<string | null>(null)

function load(p: ExportProfile | undefined) {
  layout.value = p ? structuredClone(p.layout) : { ...defaultReportLayout(), sheet_name: props.scope.label.slice(0, 31).replace(/[[\]:*?/\\]/g, '-') || 'Assets' }
  saved.value = p ? normalizeLayout(p.layout) : null
}
watch(visible, (open) => {
  if (!open) return
  profileId.value = props.profileId ?? null
  load(profile.value)
})
watch(profileId, () => load(profile.value))
// cột của trình sửa = cột của bố cục + các trường còn lại, khi biết đủ loại
watch([() => layout.value.columns, options], () => {
  if (!columns.value.length || saved.value !== null || columns.value.length !== options.value.length) columns.value = editorColumns(layout.value, options.value)
}, { immediate: true })
const current = computed(() => withColumns(layout.value, columns.value))
const dirty = computed(() => saved.value !== null && normalizeLayout(current.value) !== saved.value)
const sheets = computed(() => previewSheets(current.value, rows.value, types.value))
const skipped = computed(() => skippedKeys(current.value, types.value))
const title = computed(() =>
  current.value.title_row ? [profile.value?.name ?? 'Asset report', `Generated ${new Date().toLocaleDateString('en-GB')} by ${session.me?.account.name ?? ''} · ${props.scope.label}`] : [],
)

const sheetsOptions = [
  { label: 'One sheet', value: 'single' as const },
  { label: 'Sheet per type', value: 'per_type' as const },
]
const sheetMode = computed({ get: () => layout.value.sheets, set: (v) => (layout.value = { ...layout.value, sheets: v }) })

// Lưu: sửa profile của mình (hay có quyền quản lý); "Lưu thành…" tạo bản mới
const update = useUpdateExportProfile()
const create = useCreateExportProfile()
const errors = useFormErrors()
const saveAs = ref<{ name: string; shared: boolean } | null>(null)
async function save() {
  const p = profile.value
  if (!p) return
  const next = await update.mutateAsync({ id: p.id, version: p.version, layout: current.value })
  saved.value = normalizeLayout(next.layout)
  notify.success(`Saved ${p.name}.`)
}
async function submitSaveAs() {
  if (!saveAs.value) return
  errors.clear()
  try {
    const p = await create.mutateAsync({ name: saveAs.value.name, shared: saveAs.value.shared, layout: current.value })
    saveAs.value = null
    profileId.value = p.id
    notify.success(`Saved ${p.name}.`)
  } catch (err) {
    errors.set(err)
  }
}

const { run, running } = useExport()
async function download() {
  await run({ mode: 'report', filters: props.scope.filters, layout: current.value, profile_id: profileId.value ?? undefined }, 'storeit-report.xlsx')
  visible.value = false
}

const headerOptions = [{ label: 'Plain', value: 'plain' }, { label: 'Bold', value: 'bold' }, { label: 'Bold with fill', value: 'bold_fill' }]
const dateOptions = [{ label: '06/10/2026', value: 'dd/mm/yyyy' }, { label: '2026-10-06', value: 'yyyy-mm-dd' }, { label: '6 Oct 2026', value: 'd mmm yyyy' }]
const unitOptions = [{ label: 'In the header', value: 'header' }, { label: 'In each cell', value: 'cell' }]
const boolOptions = [{ label: 'Yes / No', value: 'yes_no' }, { label: '✓ / –', value: 'check' }]
const statusOptions = [{ label: 'Status name', value: 'name' }, { label: 'Kind', value: 'kind' }]
const sortOptions = [{ label: "List's sort", value: '' }, { label: 'Tag', value: 'tag' }, { label: 'Name', value: 'name' }, { label: 'Purchase date', value: 'purchase_date' }, { label: 'Type', value: 'asset_type' }, { label: 'Status', value: 'status' }]
</script>

<template>
  <Dialog v-model:visible="visible" modal header="Export report" :style="{ width: 'min(72rem, 96vw)' }" :content-style="{ padding: 0 }">
    <div class="head">
      <label for="report-profile" class="muted">Profile</label>
      <Select v-model="profileId" input-id="report-profile" :options="profiles ?? []" option-label="name" option-value="id" placeholder="No profile" show-clear class="profile-select" />
      <Tag v-if="profile && !profile.can_edit" :value="`Shared by ${profile.owner.name}`" icon="pi pi-lock" severity="secondary" />
      <Tag v-else-if="profile" :value="profile.shared ? 'Shared' : 'Only you'" severity="secondary" />
      <span v-if="dirty" class="unsaved">Unsaved changes</span>
      <Button label="Save" size="small" severity="secondary" outlined :disabled="!profile?.can_edit || !dirty" :loading="update.isPending.value" @click="save" />
      <Button label="Save as…" size="small" severity="secondary" outlined @click="saveAs = { name: profile ? `${profile.name} (copy)` : 'New report', shared: false }" />
    </div>
    <form v-if="saveAs" class="sub" @submit.prevent="submitSaveAs">
      <label for="save-as-name">Name</label>
      <InputText id="save-as-name" v-model="saveAs.name" required maxlength="100" autofocus />
      <Checkbox v-model="saveAs.shared" input-id="save-as-shared" binary />
      <label for="save-as-shared">Share with everyone who can export</label>
      <Button type="submit" label="Save profile" size="small" :loading="create.isPending.value" />
      <Button label="Cancel" size="small" text severity="secondary" @click="saveAs = null" />
      <small v-if="errors.general.value || errors.fields.value.name" class="field-error">{{ errors.fields.value.name ?? errors.general.value }}</small>
    </form>
    <div v-else class="sub"><i class="pi pi-table" /> Exporting <b>{{ scope.count }}</b> assets · {{ scope.label }}</div>

    <div class="body">
      <div class="config">
        <section>
          <h3>Columns</h3>
          <ColumnEditor v-model="columns" :options="options" :layout="layout" />
          <div v-if="layout.sheets === 'per_type'" class="check">
            <Checkbox v-model="layout.each_type_attrs" input-id="each-type" binary />
            <label for="each-type">Add each type's own attributes</label>
          </div>
        </section>
        <section>
          <h3>Layout</h3>
          <SegmentedFilter v-model="sheetMode" :options="sheetsOptions" label="Sheets" />
          <div v-if="layout.sheets === 'single'" class="field">
            <label for="sheet-name">Sheet name</label>
            <InputText id="sheet-name" v-model="layout.sheet_name" maxlength="31" />
          </div>
          <div class="checks">
            <span class="check"><Checkbox v-model="layout.title_row" input-id="title-row" binary /><label for="title-row">Title row</label></span>
            <span class="check"><Checkbox v-model="layout.summary" input-id="summary" binary /><label for="summary">Summary sheet</label></span>
          </div>
        </section>
        <section>
          <h3>Formatting</h3>
          <div class="grid">
            <div class="field"><label for="f-header">Header style</label><Select v-model="layout.header" input-id="f-header" :options="headerOptions" option-label="label" option-value="value" /></div>
            <div class="field"><label for="f-date">Dates</label><Select v-model="layout.date_format" input-id="f-date" :options="dateOptions" option-label="label" option-value="value" /></div>
            <div class="field"><label for="f-unit">Units</label><Select v-model="layout.unit_in" input-id="f-unit" :options="unitOptions" option-label="label" option-value="value" /></div>
            <div class="field"><label for="f-bool">Yes/no fields</label><Select v-model="layout.bool_style" input-id="f-bool" :options="boolOptions" option-label="label" option-value="value" /></div>
            <div class="field"><label for="f-status">Status shows</label><Select v-model="layout.status_as" input-id="f-status" :options="statusOptions" option-label="label" option-value="value" /></div>
            <div class="field"><label for="f-sort">Sort</label><Select v-model="layout.sort" input-id="f-sort" :options="sortOptions" option-label="label" option-value="value" /></div>
          </div>
          <div class="checks">
            <span class="check"><Checkbox v-model="layout.freeze" input-id="f-freeze" binary /><label for="f-freeze">Freeze header</label></span>
            <span class="check"><Checkbox v-model="layout.filter" input-id="f-filter" binary /><label for="f-filter">Filter buttons</label></span>
            <span class="check"><Checkbox v-model="layout.stripes" input-id="f-stripes" binary /><label for="f-stripes">Striped rows</label></span>
          </div>
        </section>
      </div>
      <div class="preview">
        <h3>Preview <small class="muted">first 20 rows of each sheet</small></h3>
        <Message v-if="skipped.length" severity="warn" :closable="false">
          This layout names attributes that no longer exist: <b>{{ skipped.join(', ') }}</b>. Those columns are skipped.
        </Message>
        <SheetPreview :sheets="sheets" :layout="current" :types="types" :title="title" />
      </div>
    </div>
    <template #footer>
      <span class="muted foot-note">Built on the server with the same filters as the list. Up to 50,000 rows.</span>
      <Button label="Cancel" text severity="secondary" @click="visible = false" />
      <Button label="Download .xlsx" icon="pi pi-download" :loading="running" :disabled="!current.columns.length && !(current.sheets === 'per_type' && current.each_type_attrs)" @click="download" />
    </template>
  </Dialog>
</template>

<style scoped>
.head, .sub { display: flex; flex-wrap: wrap; align-items: center; gap: 0.6rem; padding: 0.7rem 1.1rem; border-bottom: 1px solid var(--app-line); }
.sub { background: var(--app-ground); font-size: 0.88rem; }
.profile-select { min-width: 13rem; }
.unsaved { color: var(--p-orange-500); font-weight: 600; font-size: 0.85rem; }
.body { display: grid; grid-template-columns: minmax(0, 23rem) minmax(0, 1fr); max-height: 70vh; }
.config { overflow-y: auto; padding: 0.9rem 1.1rem; border-right: 1px solid var(--app-line); display: flex; flex-direction: column; gap: 1.1rem; }
.config h3, .preview h3 { margin-bottom: 0.5rem; }
.preview { overflow: auto; padding: 0.9rem 1.1rem; background: var(--app-ground); display: flex; flex-direction: column; gap: 0.6rem; min-width: 0; }
.grid { display: grid; grid-template-columns: 1fr 1fr; gap: 0.5rem 0.75rem; }
.checks { display: flex; flex-wrap: wrap; gap: 0.4rem 1rem; margin-top: 0.6rem; }
.check { display: inline-flex; align-items: center; gap: 0.45rem; }
.muted { color: var(--p-text-muted-color); }
.foot-note { margin-right: auto; font-size: 0.85rem; }
@media (max-width: 900px) {
  .body { grid-template-columns: minmax(0, 1fr); max-height: none; }
  .config { border-right: 0; border-bottom: 1px solid var(--app-line); }
}
</style>
```

- [ ] **Step 5: Type-check**

Run: `cd frontend && npx vue-tsc --noEmit -p tsconfig.app.json && npx vitest run`
Expected: no errors; all tests pass. Fix any generated-type name mismatches reported (for example `ExportLayout['sheets']` literal types, `profile.owner`).

- [ ] **Step 6: Commit**

```bash
git add frontend/src/features/assets/export
git commit -m "feat(frontend): report dialog with column editor and sheet preview

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 11: Export button on asset lists

**Files:**
- Create: `frontend/src/features/assets/export/components/ExportButton.vue`
- Modify: `frontend/src/features/assets/pages/AssetsPage.vue`

**Interfaces:**
- Consumes: `ExportScope`, `ReportDialog`, `useExport`, `useExportProfiles`, `toApiParams` (`features/assets/listQuery.ts`), `useAssetTypes` (counts).
- Produces: `<ExportButton :scope @report />` (split button: data export, report, recent profiles, manage).

- [ ] **Step 1: ExportButton.vue**

```vue
<script setup lang="ts">
import SplitButton from 'primevue/splitbutton'
import type { MenuItem } from 'primevue/menuitem'
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useExportProfiles } from '../api'
import { useExport } from '../useExport'
import type { ExportScope } from '../usePreviewData'

// Nút Export: bấm chính tải export dữ liệu theo bộ lọc hiện tại; mũi tên mở báo cáo,
// profile gần đây (tải ngay) và trang quản lý profile
const props = defineProps<{ scope: ExportScope }>()
const emit = defineEmits<{ report: [profileId?: string] }>()
const router = useRouter()
const { data: profiles } = useExportProfiles(true)
const { run, running } = useExport()

function dataExport() {
  run({ mode: 'data', filters: props.scope.filters }, 'storeit-assets.xlsx')
}
const items = computed<MenuItem[]>(() => [
  { label: 'Export report…', icon: 'pi pi-file-edit', command: () => emit('report') },
  { label: 'Data export (.xlsx)', icon: 'pi pi-table', command: dataExport },
  ...(profiles.value?.length ? [{ separator: true }] : []),
  ...(profiles.value ?? []).slice(0, 3).map((p) => ({
    label: p.name,
    icon: 'pi pi-bolt',
    command: () => run({ mode: 'report', filters: props.scope.filters, profile_id: p.id }, `${p.name}.xlsx`),
  })),
  { separator: true },
  { label: 'Manage profiles…', icon: 'pi pi-cog', command: () => router.push('/export-profiles') },
])
</script>

<template>
  <SplitButton label="Export" icon="pi pi-download" :model="items" :loading="running" @click="dataExport" />
</template>
```

- [ ] **Step 2: Wire into AssetsPage.vue**

In the script:

```ts
import ExportButton from '../export/components/ExportButton.vue'
import ReportDialog from '../export/components/ReportDialog.vue'
import type { ExportScope } from '../export/usePreviewData'

const canExport = computed(() => session.can(Perm.AssetExport))
const reportOpen = ref(false)
const reportProfile = ref<string | undefined>()
const reportSelection = ref(false)

// Phạm vi export: bộ lọc của danh sách (không phân trang), hay các dòng đang chọn
function scopeFor(selection: boolean): ExportScope {
  const { page: _p, page_size: _s, ...filters } = toApiParams(state.value, pageSize.value)
  const label = chips.value.length ? [selectedType.value?.name, ...chips.value.map((c) => c.label)].filter(Boolean).join(' · ') : selectedType.value?.name ?? 'All assets'
  if (selection) {
    const ids = selected.value.map((a) => a.id)
    return { filters: { ...filters, ids }, label: `${ids.length} selected assets`, count: ids.length, typeIds: [...new Set(selected.value.map((a) => a.asset_type_id))], rows: selected.value, selection: true }
  }
  const typeIds = state.value.typeId ? [state.value.typeId] : (typeCounts.value ?? []).filter((t) => (t.asset_count ?? 0) > 0).map((t) => t.id)
  return { filters, label, count: data.value?.total ?? 0, typeIds, rows: data.value?.items ?? [], selection: false }
}
const exportScope = computed(() => scopeFor(reportSelection.value))
function openReport(selection: boolean, profileId?: string) {
  reportSelection.value = selection
  reportProfile.value = profileId
  reportOpen.value = true
}
const { run: runExport } = useExport()
```

(import `useExport` from `../export/useExport`; `chips`, `selectedType`, `typeCounts`, `data`, `selected`, `pageSize` already exist in the page — check their names with `grep -n "const chips\|const { data: selectedType\|typeCounts\|const selected" frontend/src/features/assets/pages/AssetsPage.vue`).

In the template toolbar, after the existing controls (right side):

```vue
      <ExportButton v-if="canExport" class="export-btn" :scope="scopeFor(false)" @report="(id) => openReport(false, id)" />
```

In the selection bar, before "Clear selection":

```vue
      <Button v-if="canExport" label="Export selected" icon="pi pi-download" size="small" outlined @click="runExport({ mode: 'data', filters: scopeFor(true).filters }, 'storeit-assets.xlsx')" />
      <Button v-if="canExport" label="Report from selected…" icon="pi pi-file-edit" size="small" outlined @click="openReport(true)" />
```

After the `BulkActionDialog`:

```vue
    <ReportDialog v-if="canExport" v-model:visible="reportOpen" :scope="exportScope" :profile-id="reportProfile" />
```

Style: `.export-btn { margin-left: auto; }` (scoped).

- [ ] **Step 3: Check**

Run: `cd frontend && npm run check`
Expected: type-check, tests and build pass.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/features/assets/export/components/ExportButton.vue frontend/src/features/assets/pages/AssetsPage.vue
git commit -m "feat(frontend): export button and selection export on asset lists

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 12: Export profiles page, route and sidebar

**Files:**
- Create: `frontend/src/features/export-profiles/pages/ExportProfilesPage.vue`
- Modify: `frontend/src/app/routes.ts`, `frontend/src/app/layouts/AppSidebar.vue`, `frontend/README.md`

**Interfaces:**
- Consumes: profile hooks (Task 9), `ReportDialog` (Task 10), `useExport`, shared components.

- [ ] **Step 1: Page**

```vue
<script setup lang="ts">
import Button from 'primevue/button'
import Column from 'primevue/column'
import DataTable from 'primevue/datatable'
import Tag from 'primevue/tag'
import { useConfirm } from 'primevue/useconfirm'
import { computed, ref } from 'vue'
import IconAction from '@/components/IconAction.vue'
import PageHeader from '@/components/PageHeader.vue'
import SegmentedFilter, { type SegmentOption } from '@/components/SegmentedFilter.vue'
import { useAssetTypes } from '@/features/asset-types/api'
import { useDeleteExportProfile, useExportProfiles, useUpdateExportProfile } from '@/features/assets/export/api'
import ReportDialog from '@/features/assets/export/components/ReportDialog.vue'
import { useExport } from '@/features/assets/export/useExport'
import type { ExportScope } from '@/features/assets/export/usePreviewData'
import type { ExportProfile } from '@/lib/api/types'
import { useSession } from '@/lib/auth/session'
import { formatDay } from '@/lib/dates'
import { notify } from '@/lib/notify'

// Profile export: của mình và được chia sẻ; mở để sửa, chạy trên mọi tài sản, chia sẻ, xoá
const session = useSession()
const confirm = useConfirm()
const { data: profiles, isFetching } = useExportProfiles(true)
const { data: types } = useAssetTypes(false, true)

type Show = 'all' | 'mine' | 'shared'
const show = ref<Show>('all')
const mine = (p: ExportProfile) => p.owner.id === session.me?.account.id
const showOptions = computed<SegmentOption<Show>[]>(() => [
  { label: 'All', value: 'all', count: profiles.value?.length ?? 0 },
  { label: 'Mine', value: 'mine', count: profiles.value?.filter(mine).length ?? 0 },
  { label: 'Shared', value: 'shared', count: profiles.value?.filter((p) => p.shared).length ?? 0 },
])
const visible = computed(() => (profiles.value ?? []).filter((p) => show.value === 'all' || (show.value === 'mine' ? mine(p) : p.shared)))

// phạm vi khi mở từ trang này: mọi tài sản
const scope = computed<ExportScope>(() => ({
  filters: {},
  label: 'All assets',
  count: (types.value ?? []).reduce((n, t) => n + (t.asset_count ?? 0), 0),
  typeIds: (types.value ?? []).filter((t) => (t.asset_count ?? 0) > 0).map((t) => t.id),
  rows: [],
  selection: false,
}))
const dialogOpen = ref(false)
const dialogProfile = ref<string | undefined>()
function open(p?: ExportProfile) {
  dialogProfile.value = p?.id
  dialogOpen.value = true
}

const { run } = useExport()
const update = useUpdateExportProfile()
const remove = useDeleteExportProfile()
function toggleShare(p: ExportProfile) {
  update.mutateAsync({ id: p.id, version: p.version, shared: !p.shared }).then(
    () => notify.success(p.shared ? `${p.name} is private again.` : `${p.name} is shared with everyone who can export.`),
    () => {},
  )
}
function askDelete(p: ExportProfile) {
  confirm.require({
    header: 'Delete profile',
    message: `Delete ${p.name}?${p.shared ? ' People who use this shared profile lose it too.' : ''}`,
    acceptLabel: 'Delete',
    rejectLabel: 'Cancel',
    acceptProps: { severity: 'danger' },
    accept: () => remove.mutateAsync(p.id).then(() => notify.success(`Deleted ${p.name}.`), () => {}),
  })
}
const why = (p: ExportProfile) => `Only ${p.owner.name} or a profile manager can change this`
function summary(p: ExportProfile) {
  const n = p.layout.columns.length
  return [`${n} column${n === 1 ? '' : 's'}`, p.layout.sheets === 'per_type' ? 'sheet per type' : 'one sheet', p.layout.title_row && 'title row', p.layout.summary && 'summary']
    .filter(Boolean)
    .join(' · ')
}
</script>

<template>
  <section>
    <PageHeader title="Export profiles" subtitle="Saved report layouts. Shared ones can be used by everyone who can export.">
      <Button label="New profile" icon="pi pi-plus" @click="open()" />
    </PageHeader>
    <div class="toolbar">
      <SegmentedFilter v-model="show" :options="showOptions" label="Show" />
    </div>
    <DataTable :value="visible" :loading="isFetching" data-key="id" row-hover>
      <Column header="Name">
        <template #body="{ data: p }: { data: ExportProfile }">
          <Button :label="p.name" link class="name-link" @click="open(p)" />
        </template>
      </Column>
      <Column header="Owner">
        <template #body="{ data: p }: { data: ExportProfile }">{{ mine(p) ? 'You' : p.owner.name }}</template>
      </Column>
      <Column header="Visibility">
        <template #body="{ data: p }: { data: ExportProfile }">
          <Tag :value="p.shared ? 'Shared' : 'Only you'" :severity="p.shared ? 'info' : 'secondary'" />
        </template>
      </Column>
      <Column header="Layout">
        <template #body="{ data: p }: { data: ExportProfile }"><span class="muted">{{ summary(p) }}</span></template>
      </Column>
      <Column header="Updated">
        <template #body="{ data: p }: { data: ExportProfile }">{{ formatDay(p.updated_at) }}</template>
      </Column>
      <Column header="" header-style="width: 10rem">
        <template #body="{ data: p }: { data: ExportProfile }">
          <div class="row-actions">
            <IconAction icon="pi pi-download" label="Export all assets with this profile" @click="run({ mode: 'report', filters: {}, profile_id: p.id }, `${p.name}.xlsx`)" />
            <IconAction icon="pi pi-pencil" :label="p.can_edit ? 'Edit' : 'Open (save as a copy)'" @click="open(p)" />
            <IconAction :icon="p.shared ? 'pi pi-lock' : 'pi pi-share-alt'" :label="p.shared ? 'Stop sharing' : 'Share'" :disabled="!p.can_edit" :reason="why(p)" @click="toggleShare(p)" />
            <IconAction icon="pi pi-trash" label="Delete" danger :disabled="!p.can_edit" :reason="why(p)" @click="askDelete(p)" />
          </div>
        </template>
      </Column>
      <template #empty>No profiles yet. Save one from Export report… on any asset list.</template>
    </DataTable>
    <ReportDialog v-model:visible="dialogOpen" :scope="scope" :profile-id="dialogProfile" />
  </section>
</template>

<style scoped>
.name-link { padding: 0; font-weight: 600; }
.muted { color: var(--p-text-muted-color); }
</style>
```

- [ ] **Step 2: Route and sidebar**

In `routes.ts`, next to the statuses route:

```ts
      {
        path: 'export-profiles',
        name: 'export-profiles',
        component: () => import('@/features/export-profiles/pages/ExportProfilesPage.vue'),
        meta: { title: 'Export profiles', icon: 'pi pi-file-export', perm: Perm.AssetExport },
      },
```

In `AppSidebar.vue`, in the Configuration group items (inside the `session.can(Perm.AssetRead)` block), after Statuses:

```ts
        ...(session.can(Perm.AssetExport)
          ? [{ label: 'Export profiles', icon: 'pi pi-file-export', route: '/export-profiles', active: name.value === 'export-profiles' }]
          : []),
```

In `frontend/README.md`, add a short "Excel export" section: the units in `features/assets/export/` (layout, format, api, useExport, usePreviewData, components) and `lib/download.ts`, and that the preview formatting must match `backend/internal/inventory/spreadsheet/format.go`.

- [ ] **Step 3: Check**

Run: `cd frontend && npm run check`
Expected: pass. Also check the dev server compiles the new files (stack running): `curl -s -o /dev/null -w '%{http_code}' http://localhost:3000/src/features/export-profiles/pages/ExportProfilesPage.vue` → `200`.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/features/export-profiles frontend/src/app/routes.ts frontend/src/app/layouts/AppSidebar.vue frontend/README.md
git commit -m "feat(frontend): export profiles page, route and sidebar link

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 13: Full verification

- [ ] **Step 1: Backend**

Run: `cd backend && make check && CI=true go test -p 1 ./...`
Expected: all pass.

- [ ] **Step 2: Frontend**

Run: `cd frontend && npm run check`
Expected: pass.

- [ ] **Step 3: Dev database**

The dev database needs migration `00005`. Ask the user before running `docker compose run --rm migrate` (the user's standing rule: no container actions without asking).

- [ ] **Step 4: Manual check in the browser (signed in)**

On an asset list: Export downloads a file with one sheet per type code; Export report… shows the preview, Save as… creates a profile, the profile appears in the Export profiles page; a shared profile of another user offers only Save as…; Export selected exports only the ticked rows.
