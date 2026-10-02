package domain

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"storeit/internal/platform/errs"
)

func TestNormalizeTag(t *testing.T) {
	for in, want := range map[string]string{" lap-1 ": "LAP-1", "a.b_c-9": "A.B_C-9", "X": "X"} {
		if got, err := NormalizeTag(in); err != nil || got != want {
			t.Errorf("NormalizeTag(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "  ", "-x", ".x", "A B", "lap/1", strings.Repeat("A", 65), "ÄBC"} {
		if _, err := NormalizeTag(bad); !errors.Is(err, ErrInvalidTag) {
			t.Errorf("NormalizeTag(%q) err = %v, want ErrInvalidTag", bad, err)
		}
	}
}

func TestNormalizeTypeCode(t *testing.T) {
	if got, err := NormalizeTypeCode(" laptop_15 "); err != nil || got != "LAPTOP_15" {
		t.Errorf("got %q, %v", got, err)
	}
	for _, bad := range []string{"", "A.B", strings.Repeat("A", 33), "A B"} {
		if _, err := NormalizeTypeCode(bad); !errors.Is(err, ErrInvalidTypeCode) {
			t.Errorf("NormalizeTypeCode(%q) err = %v", bad, err)
		}
	}
}

func TestValidateAttributeKey(t *testing.T) {
	for _, ok := range []string{"ram_gb", "os", "a" + strings.Repeat("b", 31)} {
		if err := ValidateAttributeKey(ok); err != nil {
			t.Errorf("ValidateAttributeKey(%q) = %v", ok, err)
		}
	}
	for _, bad := range []string{"", "Ram", "1x", "_x", "a-b", "a" + strings.Repeat("b", 32)} {
		if err := ValidateAttributeKey(bad); !errors.Is(err, ErrInvalidAttributeKey) {
			t.Errorf("ValidateAttributeKey(%q) = %v", bad, err)
		}
	}
}

func TestCleanLabel(t *testing.T) {
	if got, err := CleanLabel("  RAM (GB) ", 100); err != nil || got != "RAM (GB)" {
		t.Errorf("got %q, %v", got, err)
	}
	if got, err := CleanLabel("Màn hình", 100); err != nil || got != "Màn hình" {
		t.Errorf("Vietnamese label: %q, %v", got, err)
	}
	for _, bad := range []string{"", "   ", "a\nb", "a\tb", strings.Repeat("x", 101)} {
		if _, err := CleanLabel(bad, 100); !errors.Is(err, ErrInvalidLabel) {
			t.Errorf("CleanLabel(%q) = %v", bad, err)
		}
	}
}

func TestCheckUnit(t *testing.T) {
	if got, err := CheckUnit(TypeNumber, " GB "); err != nil || got != "GB" {
		t.Errorf("number unit: %q, %v", got, err)
	}
	if got, err := CheckUnit(TypeText, ""); err != nil || got != "" {
		t.Errorf("no unit on text: %q, %v", got, err)
	}
	for _, tc := range []struct {
		dt   DataType
		unit string
	}{{TypeText, "GB"}, {TypeSelect, "x"}, {TypeNumber, strings.Repeat("u", 17)}, {TypeNumber, "a\nb"}} {
		if _, err := CheckUnit(tc.dt, tc.unit); !errors.Is(err, ErrInvalidUnit) {
			t.Errorf("CheckUnit(%s, %q) = %v", tc.dt, tc.unit, err)
		}
	}
}

func TestDataTypeAndKind(t *testing.T) {
	for _, dt := range []DataType{TypeText, TypeNumber, TypeDate, TypeBoolean, TypeSelect} {
		if !dt.Valid() {
			t.Errorf("%s not valid", dt)
		}
	}
	if DataType("json").Valid() || StatusKind("lost").Valid() || !KindRetired.Valid() {
		t.Error("Valid() wrong")
	}
}

// laptopType: một loại có đủ năm kiểu, một option đã gỡ và một thuộc tính đã gỡ
func laptopType() (AssetType, uuid.UUID, uuid.UUID) {
	removed := time.Now()
	windows, legacy := uuid.New(), uuid.New()
	osID := uuid.New()
	return AssetType{
		ID: uuid.New(), Code: "LAPTOP", Name: "Laptop",
		Attributes: []Attribute{
			{ID: uuid.New(), Key: "serial", Label: "Serial", DataType: TypeText, Required: true},
			{ID: uuid.New(), Key: "ram_gb", Label: "RAM", DataType: TypeNumber, Unit: "GB"},
			{ID: uuid.New(), Key: "warranty_end", Label: "Warranty end", DataType: TypeDate},
			{ID: uuid.New(), Key: "has_dock", Label: "Dock", DataType: TypeBoolean},
			{ID: osID, Key: "os", Label: "OS", DataType: TypeSelect, Options: []Option{
				{ID: windows, AttributeID: osID, Label: "Windows"},
				{ID: legacy, AttributeID: osID, Label: "Legacy", RemovedAt: &removed},
			}},
			{ID: uuid.New(), Key: "old", Label: "Old", DataType: TypeText, RemovedAt: &removed},
		},
	}, windows, legacy
}

// fieldsOf trả "field: detail" của lỗi ErrInvalidAttributeValues
func fieldsOf(t *testing.T, err error) []string {
	t.Helper()
	if !errors.Is(err, ErrInvalidAttributeValues) {
		t.Fatalf("err = %v, want ErrInvalidAttributeValues", err)
	}
	var e *errs.Error
	errors.As(err, &e)
	var out []string
	for _, f := range e.Fields() {
		out = append(out, f.Field+": "+f.Detail)
	}
	return out
}

func TestValidateValues(t *testing.T) {
	typ, windows, legacy := laptopType()

	vals, err := ValidateValues(typ, map[string]any{
		"serial": " SN-1 ", "ram_gb": 15.6, "warranty_end": "2027-06-30", "has_dock": false, "os": windows.String(),
	})
	if err != nil {
		t.Fatalf("valid input: %v", err)
	}
	got := map[string]Value{}
	for _, v := range vals {
		for _, a := range typ.Attributes {
			if a.ID == v.AttributeID {
				got[a.Key] = v
			}
		}
	}
	if len(vals) != 5 || *got["serial"].Text != "SN-1" || *got["ram_gb"].Number != "15.6" ||
		got["warranty_end"].Date.Format(time.DateOnly) != "2027-06-30" || *got["has_dock"].Bool ||
		*got["os"].OptionID != windows || got["ram_gb"].DataType != TypeNumber {
		t.Errorf("values = %+v", got)
	}
	v, err := ValidateValues(typ, map[string]any{"serial": "x", "ram_gb": 16.0})
	if err != nil || len(v) != 2 || v[1].Number == nil || *v[1].Number != "16" {
		t.Errorf("integer number not canonical (values in attribute order): %+v, %v", v, err)
	}

	for name, tc := range map[string]struct {
		in   map[string]any
		want []string
	}{
		"missing required":  {map[string]any{}, []string{"attributes.serial: is required"}},
		"null required":     {map[string]any{"serial": nil}, []string{"attributes.serial: is required"}},
		"number as string":  {map[string]any{"serial": "x", "ram_gb": "16"}, []string{"attributes.ram_gb: must be a number with at most 15 digits and 6 decimals"}},
		"too many digits":   {map[string]any{"serial": "x", "ram_gb": 1234567890123456.5}, []string{"attributes.ram_gb: must be a number with at most 15 digits and 6 decimals"}},
		"too many decimals": {map[string]any{"serial": "x", "ram_gb": 0.1234567}, []string{"attributes.ram_gb: must be a number with at most 15 digits and 6 decimals"}},
		"bad date":          {map[string]any{"serial": "x", "warranty_end": "30/06/2027"}, []string{"attributes.warranty_end: must be a date YYYY-MM-DD"}},
		"bad bool":          {map[string]any{"serial": "x", "has_dock": "yes"}, []string{"attributes.has_dock: must be true or false"}},
		"removed option":    {map[string]any{"serial": "x", "os": legacy.String()}, []string{"attributes.os: is not an option of this attribute"}},
		"not a uuid":        {map[string]any{"serial": "x", "os": "Windows"}, []string{"attributes.os: is not an option of this attribute"}},
		"blank text":        {map[string]any{"serial": "   "}, []string{"attributes.serial: must be text up to 1000 characters"}},
		"long text":         {map[string]any{"serial": strings.Repeat("x", 1001)}, []string{"attributes.serial: must be text up to 1000 characters"}},
		"control char text": {map[string]any{"serial": "a\x00b"}, []string{"attributes.serial: must be text up to 1000 characters"}},
		"removed attribute": {map[string]any{"serial": "x", "old": "y"}, []string{"attributes.old: unknown attribute"}},
		"unknown attribute": {map[string]any{"serial": "x", "nope": 1.0}, []string{"attributes.nope: unknown attribute"}},
		"several sorted": {map[string]any{"ram_gb": "x", "has_dock": 1.0}, []string{
			"attributes.has_dock: must be true or false",
			"attributes.ram_gb: must be a number with at most 15 digits and 6 decimals",
			"attributes.serial: is required",
		}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ValidateValues(typ, tc.in)
			got := fieldsOf(t, err)
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Errorf("fields = %q, want %q", got, tc.want)
			}
		})
	}

	// Văn bản nhiều dòng là hợp lệ
	if _, err := ValidateValues(typ, map[string]any{"serial": "line 1\nline 2\ttab"}); err != nil {
		t.Errorf("multi-line text: %v", err)
	}
}

func TestActiveAttributesAndRetired(t *testing.T) {
	typ, _, _ := laptopType()
	if n := len(typ.ActiveAttributes()); n != 5 {
		t.Errorf("active attributes = %d, want 5", n)
	}
	now := time.Now()
	if (Asset{}).Retired() || !(Asset{RetiredAt: &now}).Retired() {
		t.Error("Retired() wrong")
	}
}
