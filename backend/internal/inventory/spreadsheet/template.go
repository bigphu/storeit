package spreadsheet

import (
	"fmt"
	"io"
	"strings"

	"github.com/xuri/excelize/v2"
)

// TemplateSheet là một sheet của file mẫu import: tên (mã loại), các tên cột và danh sách chọn
// theo vị trí cột (từ 0), vd status và thuộc tính select
type TemplateSheet struct {
	Name    string
	Headers []string
	Lists   map[int][]string
}

// templateRows: số dòng dữ liệu có danh sách chọn
const templateRows = 1000

// maxListChars: Excel giới hạn công thức danh sách chọn gõ thẳng ở 255 ký tự; danh sách dài hơn
// thì ô nhập tự do (dòng vẫn được kiểm khi import)
const maxListChars = 255

// WriteTemplate ghi file mẫu import: mỗi sheet một loại, chỉ có hàng tiêu đề (đóng băng, đậm),
// ô status và thuộc tính select có danh sách chọn. Không có dòng ví dụ để khỏi bị import nhầm.
func WriteTemplate(dst io.Writer, sheets []TemplateSheet) error {
	if len(sheets) == 0 {
		return fmt.Errorf("spreadsheet: template needs at least one sheet")
	}
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	names := (&Writer{names: map[string]bool{}}).uniqueName // cùng quy tắc tên sheet với export
	bold, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}, Fill: excelize.Fill{Type: "pattern", Color: []string{"#E2E8F0"}, Pattern: 1}})
	if err != nil {
		return fmt.Errorf("spreadsheet: template style: %w", err)
	}
	for i, s := range sheets {
		name := names(s.Name)
		if i == 0 {
			err = f.SetSheetName("Sheet1", name)
		} else {
			_, err = f.NewSheet(name)
		}
		if err != nil {
			return fmt.Errorf("spreadsheet: template sheet %s: %w", name, err)
		}
		row := make([]any, len(s.Headers))
		for c, h := range s.Headers {
			row[c] = h
		}
		if err := f.SetSheetRow(name, "A1", &row); err != nil {
			return fmt.Errorf("spreadsheet: template header %s: %w", name, err)
		}
		last, _ := excelize.CoordinatesToCellName(max(len(s.Headers), 1), 1)
		if err := f.SetCellStyle(name, "A1", last, bold); err != nil {
			return fmt.Errorf("spreadsheet: template header style: %w", err)
		}
		if err := f.SetPanes(name, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
			return fmt.Errorf("spreadsheet: template freeze: %w", err)
		}
		for c, h := range s.Headers {
			col, _ := excelize.ColumnNumberToName(c + 1)
			if err := f.SetColWidth(name, col, col, max(float64(len(h))+4, 14)); err != nil {
				return fmt.Errorf("spreadsheet: template width: %w", err)
			}
		}
		for c, list := range s.Lists {
			if len(list) == 0 || len(strings.Join(list, ",")) > maxListChars {
				continue
			}
			col, _ := excelize.ColumnNumberToName(c + 1)
			dv := excelize.NewDataValidation(true)
			dv.Sqref = fmt.Sprintf("%s2:%s%d", col, col, templateRows+1)
			dv.ShowErrorMessage = false // chỉ gợi ý: giá trị ngoài danh sách vẫn được báo rõ lúc kiểm dòng
			if err := dv.SetDropList(list); err != nil {
				return fmt.Errorf("spreadsheet: template list %s: %w", name, err)
			}
			if err := f.AddDataValidation(name, dv); err != nil {
				return fmt.Errorf("spreadsheet: template validation %s: %w", name, err)
			}
		}
	}
	_, err = f.WriteTo(dst)
	return err
}
