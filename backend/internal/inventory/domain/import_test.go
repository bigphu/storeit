package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// laptopType: loại có đủ kiểu thuộc tính để thử cột import
func importLaptopType() AssetType {
	optWin, optMac := uuid.New(), uuid.New()
	removed := time.Now()
	return AssetType{
		ID: uuid.New(), Code: "LAPTOP", Name: "Laptop",
		Attributes: []Attribute{
			{ID: uuid.New(), Key: "cpu", Label: "CPU", DataType: TypeText, Required: true},
			{ID: uuid.New(), Key: "ram_gb", Label: "RAM", DataType: TypeNumber, Unit: "GB"},
			{ID: uuid.New(), Key: "warranty_end", Label: "Warranty end", DataType: TypeDate},
			{ID: uuid.New(), Key: "touch", Label: "Touch screen", DataType: TypeBoolean},
			{ID: uuid.New(), Key: "os", Label: "OS", DataType: TypeSelect, Options: []Option{
				{ID: optWin, Label: "Windows 11"}, {ID: optMac, Label: "macOS"}, {ID: uuid.New(), Label: "Old OS", RemovedAt: &removed},
			}},
			{ID: uuid.New(), Key: "gone", Label: "Gone", DataType: TypeText, RemovedAt: &removed},
		},
	}
}

func TestResolveImportColumns(t *testing.T) {
	typ := importLaptopType()
	cols, problems := ResolveImportColumns(typ, []string{" Tag ", "NAME", "description", "status", "purchase_date", "attr:cpu", "attr:ram_gb", ""})
	if len(problems) != 0 {
		t.Fatalf("problems = %v", problems)
	}
	keys := make([]string, len(cols))
	for i, c := range cols {
		keys[i] = c.Key
	}
	if got := strings.Join(keys, ","); got != "tag,name,description,status,purchase_date,attr:cpu,attr:ram_gb," {
		t.Errorf("keys = %s", got)
	}
	if cols[5].Attr == nil || cols[5].Attr.Key != "cpu" || cols[0].Attr != nil {
		t.Errorf("attribute columns not resolved: %+v %+v", cols[5], cols[0])
	}
}

func TestResolveImportColumnsProblems(t *testing.T) {
	typ := importLaptopType()
	cases := map[string]struct {
		header []string
		want   string
	}{
		"no tag":          {[]string{"name"}, `column "tag" is missing`},
		"no name":         {[]string{"tag", "status"}, `column "name" is missing`},
		"unknown":         {[]string{"tag", "name", "Asset name"}, `unknown column "Asset name"`},
		"twice":           {[]string{"tag", "name", "NAME"}, `column "name" appears twice`},
		"other type attr": {[]string{"tag", "name", "attr:size_in"}, `unknown column "attr:size_in"`},
		"removed attr":    {[]string{"tag", "name", "attr:gone"}, `unknown column "attr:gone"`},
		"empty sheet":     {nil, `column "tag" is missing`},
	}
	for name, c := range cases {
		_, problems := ResolveImportColumns(typ, c.header)
		if !strings.Contains(strings.Join(problems, "|"), c.want) {
			t.Errorf("%s: problems = %v, want one containing %q", name, problems, c.want)
		}
	}
}

func TestParseImportRow(t *testing.T) {
	typ := importLaptopType()
	cols, _ := ResolveImportColumns(typ, []string{"tag", "name", "description", "status", "purchase_date", "attr:cpu", "attr:ram_gb", "attr:warranty_end", "attr:touch", "attr:os"})
	v := ParseImportRow(cols, []string{" lap-0001 ", "MacBook Air", "", "Available", "45000", "Apple M2", "16", "05/03/2027", "TRUE", "macos"})
	if len(v.Errs) != 0 {
		t.Fatalf("errs = %v", v.Errs)
	}
	if v.Tag != " lap-0001 " || v.Name == nil || *v.Name != "MacBook Air" {
		t.Errorf("tag/name = %q %v", v.Tag, v.Name)
	}
	if v.Description != nil {
		t.Errorf("a blank cell must stay unset, got %q", *v.Description)
	}
	if v.Status == nil || *v.Status != "Available" {
		t.Errorf("status = %v", v.Status)
	}
	if v.PurchaseDate == nil || v.PurchaseDate.Format(time.DateOnly) != "2023-03-15" { // serial 45000
		t.Errorf("purchase date = %v", v.PurchaseDate)
	}
	if v.Attrs["cpu"] != "Apple M2" || v.Attrs["ram_gb"] != float64(16) || v.Attrs["warranty_end"] != "2027-03-05" || v.Attrs["touch"] != true {
		t.Errorf("attrs = %#v", v.Attrs)
	}
	var macID string
	for _, o := range typ.Attributes[4].Options {
		if o.Label == "macOS" {
			macID = o.ID.String()
		}
	}
	if v.Attrs["os"] != macID {
		t.Errorf("select should become the option id %s, got %v", macID, v.Attrs["os"])
	}
	if !v.Present["attr:ram_gb"] || !v.Present["name"] {
		t.Errorf("present = %v", v.Present)
	}
}

func TestParseImportRowErrors(t *testing.T) {
	typ := importLaptopType()
	cols, _ := ResolveImportColumns(typ, []string{"tag", "name", "purchase_date", "attr:ram_gb", "attr:warranty_end", "attr:touch", "attr:os"})
	v := ParseImportRow(cols, []string{"", "X", "last week", "sixteen", "31/02/2027", "maybe", "Plan 9"})
	want := []string{
		`purchase_date: “last week” is not a date`,
		`RAM: “sixteen” is not a number`,
		`Warranty end: “31/02/2027” is not a date`,
		`Touch screen: “maybe” is not yes or no`,
		`OS: “Plan 9” is not one of Windows 11, macOS`,
	}
	if strings.Join(v.Errs, "\n") != strings.Join(want, "\n") {
		t.Errorf("errs =\n%s\nwant\n%s", strings.Join(v.Errs, "\n"), strings.Join(want, "\n"))
	}
	if _, has := v.Attrs["ram_gb"]; has {
		t.Error("a bad cell must not reach Attrs")
	}
}

func TestParseImportScalars(t *testing.T) {
	for in, want := range map[string]float64{"16": 16, "15.6": 15.6, " -3 ": -3, "0": 0, "2500000": 2500000} {
		if got, ok := ParseImportNumber(in); !ok || got != want {
			t.Errorf("ParseImportNumber(%q) = %v, %v", in, got, ok)
		}
	}
	for _, in := range []string{"", "16GB", "1,5", "abc", "1.2.3", "--1", "0x10", "NaN", "Inf"} {
		if _, ok := ParseImportNumber(in); ok {
			t.Errorf("ParseImportNumber(%q) should fail", in)
		}
	}
	for in, want := range map[string]string{
		"2026-11-12": "2026-11-12", "12/11/2026": "2026-11-12", "1/2/2026": "2026-02-01", "12 Nov 2026": "2026-11-12",
		"2026-11-12T10:00:00Z": "2026-11-12", "46338": "2026-11-12", "46338.75": "2026-11-12",
	} {
		got, ok := ParseImportDate(in)
		if !ok || got.Format(time.DateOnly) != want {
			t.Errorf("ParseImportDate(%q) = %v, %v; want %s", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "tomorrow", "13/13/2026", "2026-13-01", "0", "-5", "99999999"} {
		if _, ok := ParseImportDate(in); ok {
			t.Errorf("ParseImportDate(%q) should fail", in)
		}
	}
	for in, want := range map[string]bool{"TRUE": true, "true": true, "Yes": true, "y": true, "1": true, "✓": true, "FALSE": false, "no": false, "N": false, "0": false, "–": false} {
		if got, ok := ParseImportBool(in); !ok || got != want {
			t.Errorf("ParseImportBool(%q) = %v, %v", in, got, ok)
		}
	}
	for _, in := range []string{"", "maybe", "2"} {
		if _, ok := ParseImportBool(in); ok {
			t.Errorf("ParseImportBool(%q) should fail", in)
		}
	}
}
