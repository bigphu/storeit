package domain

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"storeit/internal/platform/errs"
)

// Lọc và sắp danh sách theo giá trị thuộc tính tuỳ chỉnh. Chỉ dùng được khi
// danh sách lọc theo một loại: key thuộc tính chỉ có nghĩa trong loại đó.

// AttrOp là toán tử so sánh của một điều kiện "<key>:<op>:<value>"
type AttrOp string

const (
	OpEq       AttrOp = "eq"
	OpGt       AttrOp = "gt"
	OpGte      AttrOp = "gte"
	OpLt       AttrOp = "lt"
	OpLte      AttrOp = "lte"
	OpContains AttrOp = "contains" // chữ: chứa chuỗi con, không phân biệt hoa thường
	OpIn       AttrOp = "in"       // select: một trong các id option, ngăn bằng dấu phẩy
)

// attrOps: toán tử hợp lệ theo kiểu dữ liệu. eq của chữ không phân biệt hoa thường.
var attrOps = map[DataType][]AttrOp{
	TypeText:    {OpEq, OpContains},
	TypeNumber:  {OpEq, OpGt, OpGte, OpLt, OpLte},
	TypeDate:    {OpEq, OpGt, OpGte, OpLt, OpLte},
	TypeBoolean: {OpEq},
	TypeSelect:  {OpEq, OpIn},
}

// maxInOptions giới hạn số id trong một điều kiện in
const maxInOptions = 50

// AttrFilter là điều kiện đã kiểm theo thuộc tính. Value ở dạng chuẩn của kiểu:
// số chuẩn ("15.6"), ngày YYYY-MM-DD, "true"/"false", id option (in: nối bằng
// dấu phẩy), chữ nguyên văn (đã bỏ khoảng trắng đầu cuối).
type AttrFilter struct {
	AttributeID uuid.UUID
	DataType    DataType
	Op          AttrOp
	Value       string
}

// AttrOrder: sắp theo giá trị của một thuộc tính; tài sản không có giá trị luôn
// ở cuối, cả khi giảm dần. Select sắp theo thứ tự option.
type AttrOrder struct {
	AttributeID uuid.UUID
	DataType    DataType
	Desc        bool
}

const attrSortPrefix = "attributes."

// Attribute: "attributes.<key>" hay "-attributes.<key>" trả key và chiều giảm
func (s AssetSort) Attribute() (key string, desc bool, ok bool) {
	str := string(s)
	desc = strings.HasPrefix(str, "-")
	key, ok = strings.CutPrefix(strings.TrimPrefix(str, "-"), attrSortPrefix)
	if !ok {
		return "", false, false
	}
	return key, desc, true
}

// ResolveAttrQuery kiểm các điều kiện "<key>:<op>:<value>" và sort theo thuộc
// tính đang hoạt động của t. Mọi lỗi gom vào một ErrInvalidAttributeQuery, field
// "attr[i]" theo vị trí điều kiện và "sort". Sort không theo thuộc tính thì order nil.
func ResolveAttrQuery(t AssetType, conds []string, sort AssetSort) ([]AttrFilter, *AttrOrder, error) {
	byKey := make(map[string]Attribute)
	for _, a := range t.ActiveAttributes() {
		byKey[a.Key] = a
	}
	var fields []errs.FieldError
	var filters []AttrFilter
	for i, c := range conds {
		f, msg := resolveCond(byKey, c)
		if msg != "" {
			fields = append(fields, errs.FieldError{Field: fmt.Sprintf("attr[%d]", i), Detail: msg})
			continue
		}
		filters = append(filters, f)
	}

	var order *AttrOrder
	if key, desc, ok := sort.Attribute(); ok {
		if a, found := byKey[key]; found {
			order = &AttrOrder{AttributeID: a.ID, DataType: a.DataType, Desc: desc}
		} else {
			fields = append(fields, errs.FieldError{Field: "sort", Detail: msgUnknown})
		}
	}
	if len(fields) > 0 {
		return nil, nil, ErrInvalidAttributeQuery.With(errs.WithFields(fields...))
	}
	return filters, order, nil
}

// resolveCond kiểm một điều kiện; trả thông báo lỗi khác rỗng nếu không hợp lệ
func resolveCond(byKey map[string]Attribute, cond string) (AttrFilter, string) {
	parts := strings.SplitN(cond, ":", 3) // giá trị có thể chứa ':'
	if len(parts) != 3 {
		return AttrFilter{}, "must be <key>:<op>:<value>"
	}
	a, ok := byKey[parts[0]]
	if !ok {
		return AttrFilter{}, msgUnknown
	}
	op := AttrOp(parts[1])
	if !slices.Contains(attrOps[a.DataType], op) {
		return AttrFilter{}, fmt.Sprintf("operator must be one of %v for a %s attribute", attrOps[a.DataType], a.DataType)
	}
	f := AttrFilter{AttributeID: a.ID, DataType: a.DataType, Op: op}
	raw := strings.TrimSpace(parts[2])
	switch a.DataType {
	case TypeText:
		if raw == "" || utf8.RuneCountInString(raw) > maxTextLen || strings.ContainsFunc(raw, badTextRune) {
			return f, msgText
		}
		f.Value = raw
	case TypeNumber:
		n, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return f, msgNumber
		}
		if f.Value, ok = canonicalNumber(n); !ok {
			return f, msgNumber
		}
	case TypeDate:
		if _, err := time.Parse(time.DateOnly, raw); err != nil {
			return f, msgDate
		}
		f.Value = raw
	case TypeBoolean:
		if raw != "true" && raw != "false" {
			return f, msgBool
		}
		f.Value = raw
	case TypeSelect:
		ids := []string{raw}
		if op == OpIn {
			ids = strings.Split(raw, ",")
		}
		if len(ids) > maxInOptions {
			return f, fmt.Sprintf("at most %d options", maxInOptions)
		}
		for i, s := range ids {
			id, err := uuid.Parse(strings.TrimSpace(s))
			if err != nil || !hasOption(a, id) {
				return f, msgOption
			}
			ids[i] = id.String()
		}
		f.Value = strings.Join(ids, ",")
	}
	return f, ""
}

// hasOption: kể cả option đã gỡ, vì tài sản cũ vẫn giữ giá trị đó
func hasOption(a Attribute, id uuid.UUID) bool {
	return slices.ContainsFunc(a.Options, func(o Option) bool { return o.ID == id })
}
