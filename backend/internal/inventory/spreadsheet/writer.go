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

// uniqueHeaders: bảng Excel cần tên cột khác nhau (không phân biệt hoa thường); trùng thì
// thêm " (2)", " (3)" cho đến khi chưa dùng, kể cả khi tiêu đề gốc đã có dạng "Tag (2)"
func uniqueHeaders(headers []string) []string {
	out := make([]string, len(headers))
	used := map[string]bool{}
	for _, h := range headers {
		used[strings.ToLower(h)] = false // đánh dấu tên gốc để không bị cấp lại
	}
	for i, h := range headers {
		name := h
		if used[strings.ToLower(name)] {
			for n := 2; ; n++ {
				name = fmt.Sprintf("%s (%d)", h, n)
				if _, taken := used[strings.ToLower(name)]; !taken {
					break
				}
			}
		}
		used[strings.ToLower(name)] = true
		out[i] = name
	}
	return out
}
