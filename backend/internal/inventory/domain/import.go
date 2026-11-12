package domain

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Giới hạn một lần import (US-06): file và số dòng
const (
	MaxImportBytes = 5 << 20
	MaxImportRows  = 5000
)

// ImportAction là kết quả của một dòng: tạo mới, cập nhật, bỏ qua (không có gì đổi) hay lỗi
type ImportAction string

const (
	ImportCreate ImportAction = "create"
	ImportUpdate ImportAction = "update"
	ImportSkip   ImportAction = "skip"
	ImportFail   ImportAction = "fail"
)

// Cột có sẵn của sheet import; cột thuộc tính là "attr:<key>". Giống bố cục export dữ liệu
// (DataLayout), nên export rồi import lại được.
const (
	ColTag          = "tag"
	ColName         = "name"
	ColDescription  = "description"
	ColStatus       = "status"
	ColPurchaseDate = "purchase_date"
	ColAttrPrefix   = "attr:"
)

// ImportColumn là một cột của sheet. Key rỗng là ô tiêu đề trống (cột bị bỏ qua); Attr khác
// nil khi là cột thuộc tính của loại.
type ImportColumn struct {
	Key  string
	Attr *Attribute
}

// ResolveImportColumns đổi hàng tiêu đề của sheet loại t thành các cột. Tiêu đề so không phân
// biệt hoa thường và khoảng trắng hai đầu. Lỗi cấp file (trả chuỗi cho người dùng đọc): thiếu
// cột tag hay name, cột trùng, cột không biết (kể cả thuộc tính của loại khác hay đã gỡ).
// Thiếu cột thuộc tính thì không phải lỗi: file chỉ cập nhật vài cột là hợp lệ.
func ResolveImportColumns(t AssetType, header []string) ([]ImportColumn, []string) {
	attrs := map[string]*Attribute{}
	active := t.ActiveAttributes()
	for i := range active {
		attrs[active[i].Key] = &active[i]
	}
	cols := make([]ImportColumn, len(header))
	seen := map[string]bool{}
	var problems []string
	for i, h := range header {
		key := strings.ToLower(strings.TrimSpace(h))
		if key == "" {
			continue
		}
		switch key {
		case ColTag, ColName, ColDescription, ColStatus, ColPurchaseDate:
			cols[i] = ImportColumn{Key: key}
		default:
			a, ok := attrs[strings.TrimPrefix(key, ColAttrPrefix)]
			if !strings.HasPrefix(key, ColAttrPrefix) || !ok {
				problems = append(problems, fmt.Sprintf("unknown column %q", strings.TrimSpace(h)))
				continue
			}
			cols[i] = ImportColumn{Key: key, Attr: a}
		}
		if seen[key] {
			problems = append(problems, fmt.Sprintf("column %q appears twice", key))
		}
		seen[key] = true
	}
	for _, need := range []string{ColTag, ColName} {
		if !seen[need] {
			problems = append(problems, fmt.Sprintf("column %q is missing", need))
		}
	}
	return cols, problems
}

// ImportValues là một dòng đã đọc: giá trị theo kiểu, ô trống là chưa đặt (nil / vắng key).
// Attrs có dạng ValidateValues nhận (string, float64, bool; select là id option dạng chuỗi).
// Present: cột nào có trong sheet, để cập nhật chỉ đụng các cột đó. Errs: lỗi từng ô.
type ImportValues struct {
	Tag          string // nguyên văn, chưa chuẩn hoá
	Name         *string
	Description  *string
	Status       *string // tên status như trong file
	PurchaseDate *time.Time
	Attrs        map[string]any
	Present      map[string]bool
	Errs         []string
}

// ParseImportRow đọc các ô của một dòng theo cột. Ô sai kiểu thêm một lỗi và không vào Attrs.
func ParseImportRow(cols []ImportColumn, cells []string) ImportValues {
	v := ImportValues{Attrs: map[string]any{}, Present: map[string]bool{}}
	for i, c := range cols {
		if c.Key == "" {
			continue
		}
		v.Present[c.Key] = true
		raw := ""
		if i < len(cells) {
			raw = cells[i]
		}
		blank := strings.TrimSpace(raw) == ""
		switch c.Key {
		case ColTag:
			v.Tag = raw
		case ColName:
			if !blank {
				s := strings.TrimSpace(raw)
				v.Name = &s
			}
		case ColDescription:
			if !blank {
				s := strings.TrimSpace(raw)
				v.Description = &s
			}
		case ColStatus:
			if !blank {
				s := strings.TrimSpace(raw)
				v.Status = &s
			}
		case ColPurchaseDate:
			if blank {
				continue
			}
			d, ok := ParseImportDate(raw)
			if !ok {
				v.Errs = append(v.Errs, fmt.Sprintf("%s: “%s” is not a date", ColPurchaseDate, strings.TrimSpace(raw)))
				continue
			}
			v.PurchaseDate = &d
		default:
			if blank || c.Attr == nil {
				continue
			}
			val, msg := parseImportAttr(*c.Attr, strings.TrimSpace(raw))
			if msg != "" {
				v.Errs = append(v.Errs, c.Attr.Label+": "+msg)
				continue
			}
			v.Attrs[c.Attr.Key] = val
		}
	}
	return v
}

// parseImportAttr đổi ô (đã cắt khoảng trắng, không trống) theo kiểu thuộc tính; msg khác rỗng là lỗi
func parseImportAttr(a Attribute, s string) (any, string) {
	switch a.DataType {
	case TypeText:
		return s, ""
	case TypeNumber:
		f, ok := ParseImportNumber(s)
		if !ok {
			return nil, fmt.Sprintf("“%s” is not a number", s)
		}
		return f, ""
	case TypeDate:
		d, ok := ParseImportDate(s)
		if !ok {
			return nil, fmt.Sprintf("“%s” is not a date", s)
		}
		return d.Format(time.DateOnly), ""
	case TypeBoolean:
		b, ok := ParseImportBool(s)
		if !ok {
			return nil, fmt.Sprintf("“%s” is not yes or no", s)
		}
		return b, ""
	case TypeSelect:
		var labels []string
		for _, o := range a.Options {
			if o.RemovedAt != nil {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(o.Label), s) {
				return o.ID.String(), ""
			}
			labels = append(labels, o.Label)
		}
		return nil, fmt.Sprintf("“%s” is not one of %s", s, strings.Join(labels, ", "))
	}
	return nil, "unsupported attribute type"
}

// ParseImportNumber đọc số thập phân dạng máy ("16", "15.6", "-3"): chỉ chữ số và một dấu chấm.
// Không nhận đơn vị ("16GB"), dấu phẩy hay số mũ; số ô Excel đã ở dạng này.
func ParseImportNumber(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	body := strings.TrimPrefix(s, "-")
	if body == "" || strings.Count(body, ".") > 1 || strings.HasPrefix(body, ".") || strings.HasSuffix(body, ".") {
		return 0, false
	}
	for _, r := range body {
		if (r < '0' || r > '9') && r != '.' {
			return 0, false
		}
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	return f, true
}

// importDateLayouts: các dạng ngày người dùng gõ; ngày/tháng/năm (dd/mm/yyyy) theo thói quen Việt
// Nam và ba dạng của export (yyyy-mm-dd, dd/mm/yyyy, d mmm yyyy)
var importDateLayouts = []string{time.DateOnly, "2/1/2006", "02/01/2006", "2 Jan 2006", time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05"}

// excelEpoch: gốc của số ngày Excel (tính cả lỗi năm nhuận 1900 của Excel cho số từ 61 trở đi)
var excelEpoch = time.Date(1899, time.December, 30, 0, 0, 0, 0, time.UTC)

// ParseImportDate đọc một ngày: số ngày của Excel (ô ngày giữ số gốc) hay chữ theo importDateLayouts.
// Kết quả là ngày (UTC, 00:00); giờ bị bỏ.
func ParseImportDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	if n, ok := ParseImportNumber(s); ok {
		// số ngày hợp lý: từ 1900-03-01 (61) đến năm 9999
		if n < 61 || n >= 2958466 {
			return time.Time{}, false
		}
		return excelEpoch.AddDate(0, 0, int(n)), true
	}
	for _, layout := range importDateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), true
		}
	}
	return time.Time{}, false
}

// ParseImportBool đọc ô có/không: TRUE/FALSE của Excel, yes/no, y/n, 1/0 và ✓ / – của kiểu "check"
func ParseImportBool(s string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "yes", "y", "1", "✓":
		return true, true
	case "false", "no", "n", "0", "–", "-":
		return false, true
	}
	return false, false
}

// ImportTypeCode: tên sheet khớp mã loại (không phân biệt hoa thường). Excel cắt tên sheet ở 31
// ký tự, nên sheet đúng 31 ký tự cũng khớp mã dài hơn có tiền tố đó. Trả loại khớp, hay không có.
func ImportTypeCode(sheet string, types []AssetType) (AssetType, bool) {
	name := strings.ToUpper(strings.TrimSpace(sheet))
	for _, t := range types {
		if strings.ToUpper(t.Code) == name {
			return t, true
		}
	}
	if len([]rune(name)) == 31 {
		var found *AssetType
		for i := range types {
			if strings.HasPrefix(strings.ToUpper(types[i].Code), name) {
				if found != nil {
					return AssetType{}, false
				}
				found = &types[i]
			}
		}
		if found != nil {
			return *found, true
		}
	}
	return AssetType{}, false
}

// ImportRecord là một lần import đã xác nhận (lịch sử, và chặn nhập lại cùng file)
type ImportRecord struct {
	ID         uuid.UUID
	AccountID  uuid.UUID
	FileName   string
	SHA256     string
	HasCreates bool
	Rows       int
	Counts     ImportCounts
	CreatedAt  time.Time
	FinishedAt *time.Time
}

// ImportCounts: số dòng theo kết quả
type ImportCounts struct {
	Create, Update, Skip, Fail int
}

// ImportRepository ghi lịch sử import. Claim đặt chỗ cho file trước khi xử lý dòng nào (trả id
// của lần import): file có dòng tạo mới mà đã được xác nhận trước đó thì ErrImportAlreadyDone,
// kèm ngày giờ và người làm lần trước.
type ImportRepository interface {
	Claim(ctx context.Context, rec ImportRecord) (uuid.UUID, error)
	Finish(ctx context.Context, id uuid.UUID, c ImportCounts) error
}
