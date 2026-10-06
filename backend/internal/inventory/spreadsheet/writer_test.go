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
