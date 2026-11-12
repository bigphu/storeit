package spreadsheet

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/xuri/excelize/v2"
)

// ErrUnreadable: không mở được như một workbook .xlsx (không phải file Excel, hỏng, quá lớn khi giải nén)
var ErrUnreadable = errors.New("spreadsheet: not a readable .xlsx workbook")

// Giới hạn giải nén (chống file zip phình to): tổng và từng phần XML
const (
	maxUnzipBytes    = 64 << 20
	maxUnzipXMLBytes = 32 << 20
)

// ReadSheet là một sheet đã đọc: tên, hàng tiêu đề (hàng 1) và các dòng dữ liệu
type ReadSheet struct {
	Name   string
	Header []string
	Rows   []ReadRow
}

// ReadRow là một dòng dữ liệu: số dòng trong Excel (hàng 2 là dòng dữ liệu đầu) và các ô dạng chữ
type ReadRow struct {
	Row   int
	Cells []string
}

// ReadWorkbook đọc mọi sheet của file .xlsx. Ô lấy giá trị gốc: ô ngày là số ngày của Excel
// ("46086"), số là số, TRUE/FALSE là 1/0, chữ là chữ; việc đổi theo kiểu cột do domain.ParseImportRow
// làm. Dòng chỉ gồm ô trống bị bỏ.
func ReadWorkbook(r io.Reader) ([]ReadSheet, error) {
	f, err := excelize.OpenReader(r, excelize.Options{UnzipSizeLimit: maxUnzipBytes, UnzipXMLSizeLimit: maxUnzipXMLBytes})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnreadable, err)
	}
	defer func() { _ = f.Close() }()
	var out []ReadSheet
	for _, name := range f.GetSheetList() {
		rows, err := f.GetRows(name, excelize.Options{RawCellValue: true})
		if err != nil {
			return nil, fmt.Errorf("%w: sheet %s: %v", ErrUnreadable, name, err)
		}
		sheet := ReadSheet{Name: name}
		for i, cells := range rows {
			if i == 0 {
				sheet.Header = cells
				continue
			}
			if blankRow(cells) {
				continue
			}
			sheet.Rows = append(sheet.Rows, ReadRow{Row: i + 1, Cells: cells})
		}
		out = append(out, sheet)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: no sheets", ErrUnreadable)
	}
	return out, nil
}

func blankRow(cells []string) bool {
	for _, c := range cells {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}
