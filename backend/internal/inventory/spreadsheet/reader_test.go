package spreadsheet

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

// workbook dựng một file .xlsx từ các sheet (tên -> các dòng ô), ô là string, float64, bool hay time.Time
func workbook(t *testing.T, sheets map[string][][]any, order ...string) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	for i, name := range order {
		if i == 0 {
			if err := f.SetSheetName("Sheet1", name); err != nil {
				t.Fatal(err)
			}
		} else if _, err := f.NewSheet(name); err != nil {
			t.Fatal(err)
		}
		for r, row := range sheets[name] {
			cell, _ := excelize.CoordinatesToCellName(1, r+1)
			if err := f.SetSheetRow(name, cell, &row); err != nil {
				t.Fatal(err)
			}
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestReadWorkbook(t *testing.T) {
	when := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	data := workbook(t, map[string][][]any{
		"LAPTOP": {
			{"tag", "name", "purchase_date", "attr:ram_gb", "attr:touch"},
			{"LAP-0001", "MacBook Air", when, 16, true},
			{},
			{"", "  ", "", "", ""},
			{"LAP-0002", "Dell Latitude", nil, 8.5, false},
		},
		"MONITOR": {
			{"tag", "name"},
		},
	}, "LAPTOP", "MONITOR")

	sheets, err := ReadWorkbook(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(sheets) != 2 || sheets[0].Name != "LAPTOP" || sheets[1].Name != "MONITOR" {
		t.Fatalf("sheets = %+v", sheets)
	}
	lap := sheets[0]
	if strings.Join(lap.Header, ",") != "tag,name,purchase_date,attr:ram_gb,attr:touch" {
		t.Errorf("header = %v", lap.Header)
	}
	if len(lap.Rows) != 2 {
		t.Fatalf("rows = %+v (blank rows must be dropped)", lap.Rows)
	}
	first := lap.Rows[0]
	if first.Row != 2 || first.Cells[0] != "LAP-0001" {
		t.Errorf("first row = %+v", first)
	}
	// ô ngày giữ số ngày gốc của Excel (46338 = 12/11/2026), số là số, TRUE/FALSE là 1/0
	if first.Cells[2] != "46338" || first.Cells[3] != "16" || first.Cells[4] != "1" {
		t.Errorf("raw cells = %q", first.Cells)
	}
	if second := lap.Rows[1]; second.Row != 5 || second.Cells[3] != "8.5" || second.Cells[4] != "0" {
		t.Errorf("second row = %+v (the row number is Excel's)", second)
	}
	if len(sheets[1].Rows) != 0 {
		t.Errorf("a sheet with only a header has no rows: %+v", sheets[1].Rows)
	}
}

func TestReadWorkbookRejectsNonWorkbooks(t *testing.T) {
	for name, data := range map[string][]byte{
		"csv":   []byte("tag,name\nLAP-1,Laptop\n"),
		"empty": {},
		"zip":   []byte("PK\x03\x04 not really"),
	} {
		_, err := ReadWorkbook(bytes.NewReader(data))
		if !errors.Is(err, ErrUnreadable) {
			t.Errorf("%s: err = %v, want ErrUnreadable", name, err)
		}
	}
}

// Export dữ liệu rồi đọc lại phải ra đúng các giá trị: nền của "export, sửa, import lại"
func TestReadWorkbookReadsWhatExportWrites(t *testing.T) {
	w := NewWriter()
	defer func() { _ = w.Close() }()
	sh, err := w.Sheet(SheetOptions{Name: "LAPTOP"}, []string{"tag", "name", "purchase_date", "attr:ram_gb", "attr:touch", "attr:os"})
	if err != nil {
		t.Fatal(err)
	}
	day := Cell{Kind: Date, Time: time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC), NumFmt: "yyyy-mm-dd"}
	if err := sh.Row([]Cell{TextCell("LAP-0001"), TextCell("MacBook Air"), day, {Kind: Number, Num: 16}, {Kind: Bool, Bool: true}, TextCell("macOS")}); err != nil {
		t.Fatal(err)
	}
	if err := sh.Close(); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := w.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	sheets, err := ReadWorkbook(&buf)
	if err != nil {
		t.Fatal(err)
	}
	row := sheets[0].Rows[0]
	if row.Cells[0] != "LAP-0001" || row.Cells[3] != "16" || row.Cells[5] != "macOS" || row.Cells[4] != "1" {
		t.Errorf("cells = %q", row.Cells)
	}
	// ngày export là số ngày của Excel: 5/3/2026 = 46086
	if row.Cells[2] != "46086" {
		t.Errorf("date cell = %q, want the Excel day number 46086", row.Cells[2])
	}
}

func TestWriteTemplate(t *testing.T) {
	var buf bytes.Buffer
	err := WriteTemplate(&buf, []TemplateSheet{
		{Name: "LAPTOP", Headers: []string{"tag", "name", "status", "attr:os"}, Lists: map[int][]string{2: {"Available", "Under repair"}, 3: {"Windows 11", "macOS"}}},
		{Name: "MONITOR", Headers: []string{"tag", "name"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	data := append([]byte(nil), buf.Bytes()...)
	sheets, err := ReadWorkbook(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if len(sheets) != 2 || strings.Join(sheets[0].Header, ",") != "tag,name,status,attr:os" || len(sheets[0].Rows) != 0 {
		t.Fatalf("template sheets = %+v", sheets)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	dvs, err := f.GetDataValidations("LAPTOP")
	if err != nil || len(dvs) != 2 {
		t.Fatalf("data validations = %d, %v; want a drop-down for status and for OS", len(dvs), err)
	}
}

func TestWriteTemplateLongNamesAndLists(t *testing.T) {
	long := strings.Repeat("A", 40)
	var buf bytes.Buffer
	var big []string
	for i := 0; i < 60; i++ {
		big = append(big, strings.Repeat("x", 10))
	}
	err := WriteTemplate(&buf, []TemplateSheet{
		{Name: long, Headers: []string{"tag", "name", "status"}, Lists: map[int][]string{2: big}},
		{Name: long + "B", Headers: []string{"tag", "name"}},
	})
	if err != nil {
		t.Fatalf("a long type code or list must not break the template: %v", err)
	}
	sheets, err := ReadWorkbook(&buf)
	if err != nil || len(sheets) != 2 {
		t.Fatalf("sheets = %d, %v", len(sheets), err)
	}
	if len([]rune(sheets[0].Name)) != 31 || sheets[0].Name == sheets[1].Name {
		t.Errorf("names = %q, %q: Excel allows 31 characters and every sheet needs its own name", sheets[0].Name, sheets[1].Name)
	}
}
