package domain

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"storeit/internal/platform/errs"
)

func exportLaptopType() AssetType {
	return AssetType{ID: uuid.New(), Code: "LAPTOP", Name: "Laptop", Attributes: []Attribute{
		{Key: "cpu", Label: "CPU", DataType: TypeText, Position: 1},
		{Key: "ram_gb", Label: "RAM", DataType: TypeNumber, Unit: "GB", Position: 2},
		{Key: "old", Label: "Old", DataType: TypeText, Position: 3, RemovedAt: ptrNow()},
	}}
}

func exportPhoneType() AssetType {
	return AssetType{ID: uuid.New(), Code: "PHONE", Name: "Phone", Attributes: []Attribute{
		{Key: "imei", Label: "IMEI", DataType: TypeText, Position: 1},
	}}
}

func exportFieldsOf(err error) map[string]bool {
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
	got := exportFieldsOf(err)
	for _, f := range []string{"layout.columns[1].field", "layout.columns[2].field", "layout.columns[3].field",
		"layout.columns[4].header", "layout.columns[5].width", "layout.sheet_name", "layout.header", "layout.sort"} {
		if !got[f] {
			t.Errorf("missing field error %s (got %v)", f, got)
		}
	}
	empty := ok
	empty.Columns = nil
	if !exportFieldsOf(empty.Validate())["layout.columns"] {
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
	if !exportFieldsOf(many.Validate())["layout.columns"] {
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
	lap, ph := exportLaptopType(), exportPhoneType()
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
	got := headers(d.SheetColumns([]AssetType{exportLaptopType()}))
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

func ptrNow() *time.Time { t := time.Now(); return &t }
