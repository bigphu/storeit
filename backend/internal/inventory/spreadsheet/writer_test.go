package spreadsheet

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/xuri/excelize/v2"

	"storeit/internal/inventory/domain"
)

// build ghi một file rồi mở lại để đọc như Excel
func build(t *testing.T, fn func(w *Writer)) *excelize.File {
	t.Helper()
	w := NewWriter()
	defer func() { _ = w.Close() }()
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
	if typ, _ := f.GetCellType("Laptop", "B4"); typ == excelize.CellTypeSharedString || typ == excelize.CellTypeInlineString {
		t.Errorf("number cell type = %v, want numeric", typ)
	}
	if v, _ := f.GetCellValue("Laptop", "B4", excelize.Options{RawCellValue: true}); v != "32" {
		t.Errorf("number raw = %q, want 32", v)
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

func TestWriter_DuplicateHeadersWithExistingSuffix(t *testing.T) {
	for _, in := range [][]string{{"Tag", "Tag", "Tag (2)"}, {"Tag (2)", "Tag", "Tag"}, {"tag", "TAG", "Tag"}} {
		f := build(t, func(w *Writer) {
			s, err := w.Sheet(SheetOptions{Name: "Assets", Filter: true}, in)
			must(t, err)
			must(t, s.Row([]Cell{TextCell("a"), TextCell("b"), TextCell("c")}))
			must(t, s.Close())
		})
		rows, _ := f.GetRows("Assets")
		seen := map[string]bool{}
		for _, h := range rows[0] {
			k := strings.ToLower(h)
			if seen[k] {
				t.Errorf("input %v: duplicate header in %v", in, rows[0])
			}
			seen[k] = true
		}
		if len(rows[0]) != 3 {
			t.Errorf("input %v: headers = %v", in, rows[0])
		}
		if tables, _ := f.GetTables("Assets"); len(tables) != 1 {
			t.Errorf("input %v: tables = %+v", in, tables)
		}
	}
}

func TestWriter_StripesKeepDateFormat(t *testing.T) {
	day := time.Date(2025, 3, 14, 0, 0, 0, 0, time.UTC)
	f := build(t, func(w *Writer) {
		s, err := w.Sheet(SheetOptions{Name: "S", Stripes: true}, []string{"Tag", "Bought"})
		must(t, err)
		for i := 0; i < 4; i++ {
			must(t, s.Row([]Cell{TextCell("x"), {Kind: Date, Time: day, NumFmt: "dd/mm/yyyy"}}))
		}
		must(t, s.Close())
	})
	striped := func(cell string) bool {
		id, err := f.GetCellStyle("S", cell)
		must(t, err)
		st, err := f.GetStyle(id)
		must(t, err)
		return st.Fill.Type == "pattern" && len(st.Fill.Color) > 0
	}
	// dòng 2..5 là dữ liệu; dòng thứ hai và thứ tư của dữ liệu (3 và 5) có nền sọc
	for row, want := range map[int]bool{2: false, 3: true, 4: false, 5: true} {
		for _, col := range []string{"A", "B"} {
			if got := striped(fmt.Sprintf("%s%d", col, row)); got != want {
				t.Errorf("%s%d striped = %v, want %v", col, row, got, want)
			}
		}
		id, _ := f.GetCellStyle("S", fmt.Sprintf("B%d", row))
		st, _ := f.GetStyle(id)
		if st.CustomNumFmt == nil || *st.CustomNumFmt != "dd/mm/yyyy" {
			t.Errorf("B%d lost its date format: %+v", row, st)
		}
	}
}

func TestWriter_TitleMerge(t *testing.T) {
	f := build(t, func(w *Writer) {
		s, err := w.Sheet(SheetOptions{Name: "Wide", Title: []string{"T1", "T2"}}, []string{"a", "b", "c"})
		must(t, err)
		must(t, s.Close())
		s, err = w.Sheet(SheetOptions{Name: "One", Title: []string{"T1", "T2"}}, []string{"a"})
		must(t, err)
		must(t, s.Close())
	})
	merges, err := f.GetMergeCells("Wide")
	must(t, err)
	if len(merges) != 2 || merges[0].GetStartAxis() != "A1" || merges[0].GetEndAxis() != "C1" ||
		merges[1].GetStartAxis() != "A2" || merges[1].GetEndAxis() != "C2" {
		t.Errorf("merges = %v", merges)
	}
	if merges, _ := f.GetMergeCells("One"); len(merges) != 0 {
		t.Errorf("single column must not merge: %v", merges)
	}
}

func TestWriter_SheetNamesExcelRejects(t *testing.T) {
	long := strings.Repeat("😀", 20) // 40 đơn vị UTF-16
	f := build(t, func(w *Writer) {
		for _, n := range []string{"'Quoted'", "'", long, long} {
			s, err := w.Sheet(SheetOptions{Name: n}, []string{"x"})
			must(t, err)
			must(t, s.Close())
		}
	})
	got := f.GetSheetList()
	if got[0] != "Quoted" || got[1] != "Sheet" {
		t.Errorf("sheets = %q", got)
	}
	for i, n := range got {
		if u := len(utf16.Encode([]rune(n))); u > 31 {
			t.Errorf("sheet %d %q has %d UTF-16 units", i, n, u)
		}
	}
	if got[2] == got[3] {
		t.Errorf("duplicate names must stay unique: %q", got)
	}
}

func TestWriter_NoHeadersNoTable(t *testing.T) {
	f := build(t, func(w *Writer) {
		s, err := w.Sheet(SheetOptions{Name: "Empty", Filter: true}, nil)
		must(t, err)
		must(t, s.Row([]Cell{TextCell("x")}))
		must(t, s.Close())
	})
	if tables, _ := f.GetTables("Empty"); len(tables) != 0 {
		t.Errorf("no headers must not get a table: %+v", tables)
	}
}

func TestWriter_TextCellNeverFormula(t *testing.T) {
	f := build(t, func(w *Writer) {
		s, err := w.Sheet(SheetOptions{Name: "S"}, []string{"x"})
		must(t, err)
		must(t, s.Row([]Cell{TextCell("=1+1")}))
		must(t, s.Close())
	})
	if v, _ := f.GetCellValue("S", "A2"); v != "=1+1" {
		t.Errorf("value = %q", v)
	}
	if fm, _ := f.GetCellFormula("S", "A2"); fm != "" {
		t.Errorf("formula = %q, want none", fm)
	}
}
