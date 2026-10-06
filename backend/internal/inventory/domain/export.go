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
