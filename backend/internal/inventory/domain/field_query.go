package domain

import (
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"storeit/internal/platform/errs"
)

// Lọc theo trường có sẵn của mọi tài sản (không cần chọn loại): ngày mua, ngày tạo,
// ngày sửa, mô tả. Điều kiện "<field>:<op>:<value>" giống attr. created_at/updated_at so
// theo ngày của múi giờ người dùng nên được đổi thành mốc thời gian (gte/lt) ở đây,
// SQL chỉ so timestamp.

type BuiltinField string

const (
	FieldPurchaseDate BuiltinField = "purchase_date"
	FieldCreatedAt    BuiltinField = "created_at"
	FieldUpdatedAt    BuiltinField = "updated_at"
	FieldDescription  BuiltinField = "description"
)

// maxFieldText: độ dài tối đa của chuỗi tìm trong mô tả
const maxFieldText = 200

var fieldOps = map[BuiltinField][]AttrOp{
	FieldPurchaseDate: {OpEq, OpGt, OpGte, OpLt, OpLte},
	FieldCreatedAt:    {OpEq, OpGt, OpGte, OpLt, OpLte},
	FieldUpdatedAt:    {OpEq, OpGt, OpGte, OpLt, OpLte},
	FieldDescription:  {OpContains},
}

// FieldFilter là điều kiện đã kiểm. purchase_date: Value là YYYY-MM-DD, Op như người dùng
// chọn. created_at/updated_at: Op chỉ là gte hoặc lt, Value là mốc RFC 3339 (UTC).
// description: Op contains, Value là chuỗi đã bỏ khoảng trắng đầu cuối (chưa escape LIKE).
type FieldFilter struct {
	Field BuiltinField
	Op    AttrOp
	Value string
}

// ResolveFieldQuery kiểm các điều kiện; mọi lỗi gom vào một ErrInvalidFieldQuery, field
// "field[i]" theo vị trí điều kiện
func ResolveFieldQuery(conds []string, loc *time.Location) ([]FieldFilter, error) {
	var fields []errs.FieldError
	var out []FieldFilter
	for i, c := range conds {
		ff, msg := resolveFieldCond(c, loc)
		if msg != "" {
			fields = append(fields, errs.FieldError{Field: fmt.Sprintf("field[%d]", i), Detail: msg})
			continue
		}
		out = append(out, ff...)
	}
	if len(fields) > 0 {
		return nil, ErrInvalidFieldQuery.With(errs.WithFields(fields...))
	}
	return out, nil
}

func resolveFieldCond(cond string, loc *time.Location) ([]FieldFilter, string) {
	parts := strings.SplitN(cond, ":", 3) // giá trị có thể chứa ':'
	if len(parts) != 3 {
		return nil, "must be <field>:<op>:<value>"
	}
	field := BuiltinField(parts[0])
	ops, ok := fieldOps[field]
	if !ok {
		return nil, "unknown field: use purchase_date, created_at, updated_at or description"
	}
	op := AttrOp(parts[1])
	if !slices.Contains(ops, op) {
		return nil, fmt.Sprintf("operator must be one of %v for %s", ops, field)
	}
	raw := strings.TrimSpace(parts[2])
	if field == FieldDescription {
		if raw == "" || utf8.RuneCountInString(raw) > maxFieldText || strings.ContainsFunc(raw, badTextRune) {
			return nil, fmt.Sprintf("must be text up to %d characters", maxFieldText)
		}
		return []FieldFilter{{Field: field, Op: op, Value: raw}}, ""
	}
	day, err := time.ParseInLocation(time.DateOnly, raw, loc)
	if err != nil {
		return nil, msgDate
	}
	if field == FieldPurchaseDate {
		return []FieldFilter{{Field: field, Op: op, Value: raw}}, ""
	}
	// ngày theo múi giờ người dùng → [đầu ngày, đầu ngày sau); AddDate giữ đúng khi đổi giờ mùa
	start, next := day, day.AddDate(0, 0, 1)
	at := func(o AttrOp, t time.Time) FieldFilter {
		return FieldFilter{Field: field, Op: o, Value: t.UTC().Format(time.RFC3339)}
	}
	switch op {
	case OpEq:
		return []FieldFilter{at(OpGte, start), at(OpLt, next)}, ""
	case OpGt:
		return []FieldFilter{at(OpGte, next)}, ""
	case OpGte:
		return []FieldFilter{at(OpGte, start)}, ""
	case OpLt:
		return []FieldFilter{at(OpLt, start)}, ""
	default: // OpLte
		return []FieldFilter{at(OpLt, next)}, ""
	}
}
