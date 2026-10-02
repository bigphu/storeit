package domain

import (
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"

	"storeit/internal/platform/errs"
)

// Value là giá trị của một thuộc tính trên một tài sản; đúng một con trỏ khác
// nil, khớp DataType (DB cũng kiểm tra bằng CHECK)
type Value struct {
	AttributeID uuid.UUID
	DataType    DataType
	Text        *string
	Number      *string // số thập phân dạng chuẩn ("16", "15.6"), theo đơn vị của thuộc tính
	Date        *time.Time
	Bool        *bool
	OptionID    *uuid.UUID
}

// Giới hạn của số: đủ cho thông số tài sản, và float64 của JSON giữ chính xác
const (
	maxNumberDigits   = 15
	maxNumberDecimals = 6
	maxTextLen        = 1000
)

const (
	msgRequired = "is required"
	msgUnknown  = "unknown attribute"
	msgText     = "must be text up to 1000 characters"
	msgNumber   = "must be a number with at most 15 digits and 6 decimals"
	msgDate     = "must be a date YYYY-MM-DD"
	msgBool     = "must be true or false"
	msgOption   = "is not an option of this attribute"
)

// ValidateValues kiểm tra input (key thuộc tính -> giá trị JSON đã decode:
// string, float64, bool, nil) theo thuộc tính đang hoạt động của loại t. Trả
// giá trị theo thứ tự thuộc tính; mọi lỗi gom vào một ErrInvalidAttributeValues,
// mỗi key một FieldError "attributes.<key>", sắp theo key.
//
// nil coi như bỏ trống. Thuộc tính tuỳ chọn bỏ trống không có giá trị nào.
func ValidateValues(t AssetType, in map[string]any) ([]Value, error) {
	active := t.ActiveAttributes()
	byKey := make(map[string]Attribute, len(active))
	for _, a := range active {
		byKey[a.Key] = a
	}
	var fields []errs.FieldError
	fail := func(key, msg string) {
		fields = append(fields, errs.FieldError{Field: "attributes." + key, Detail: msg})
	}
	for key := range in {
		if _, ok := byKey[key]; !ok {
			fail(key, msgUnknown)
		}
	}

	var out []Value
	for _, a := range active {
		raw, present := in[a.Key]
		if !present || raw == nil {
			if a.Required {
				fail(a.Key, msgRequired)
			}
			continue
		}
		v, msg := convert(a, raw)
		if msg != "" {
			fail(a.Key, msg)
			continue
		}
		out = append(out, v)
	}
	if len(fields) > 0 {
		slices.SortFunc(fields, func(x, y errs.FieldError) int { return strings.Compare(x.Field, y.Field) })
		return nil, ErrInvalidAttributeValues.With(errs.WithFields(fields...))
	}
	return out, nil
}

// convert đổi một giá trị JSON sang Value theo kiểu của thuộc tính; trả thông
// báo lỗi khác rỗng nếu không hợp lệ
func convert(a Attribute, raw any) (Value, string) {
	v := Value{AttributeID: a.ID, DataType: a.DataType}
	switch a.DataType {
	case TypeText:
		s, ok := raw.(string)
		s = strings.TrimSpace(s)
		if !ok || s == "" || utf8.RuneCountInString(s) > maxTextLen || strings.ContainsFunc(s, badTextRune) {
			return v, msgText
		}
		v.Text = &s
	case TypeNumber:
		f, ok := raw.(float64)
		if !ok {
			return v, msgNumber
		}
		n, ok := canonicalNumber(f)
		if !ok {
			return v, msgNumber
		}
		v.Number = &n
	case TypeDate:
		s, ok := raw.(string)
		d, err := time.Parse(time.DateOnly, s)
		if !ok || err != nil {
			return v, msgDate
		}
		v.Date = &d
	case TypeBoolean:
		b, ok := raw.(bool)
		if !ok {
			return v, msgBool
		}
		v.Bool = &b
	case TypeSelect:
		s, _ := raw.(string)
		id, err := uuid.Parse(s)
		if err != nil || !hasActiveOption(a, id) {
			return v, msgOption
		}
		v.OptionID = &id
	}
	return v, ""
}

// canonicalNumber viết số dạng thập phân ngắn nhất ("16", "15.6") và kiểm tra
// giới hạn chữ số: quá giới hạn thì từ chối, không làm tròn
func canonicalNumber(f float64) (string, bool) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return "", false
	}
	s := strconv.FormatFloat(f, 'f', -1, 64)
	digits := strings.TrimPrefix(s, "-")
	intPart, frac, _ := strings.Cut(digits, ".")
	intPart = strings.TrimLeft(intPart, "0")
	if len(frac) > maxNumberDecimals || len(intPart)+len(frac) > maxNumberDigits {
		return "", false
	}
	return s, true
}

// badTextRune: ký tự điều khiển, trừ xuống dòng và tab (ghi chú nhiều dòng là hợp lệ)
func badTextRune(r rune) bool {
	return unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t'
}

func hasActiveOption(a Attribute, id uuid.UUID) bool {
	for _, o := range a.Options {
		if o.ID == id && o.RemovedAt == nil {
			return true
		}
	}
	return false
}
