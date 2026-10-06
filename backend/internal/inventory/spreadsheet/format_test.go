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
