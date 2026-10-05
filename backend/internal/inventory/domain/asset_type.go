package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

// DataType là kiểu dữ liệu của một thuộc tính riêng
type DataType string

const (
	TypeText    DataType = "text"
	TypeNumber  DataType = "number"
	TypeDate    DataType = "date"
	TypeBoolean DataType = "boolean"
	TypeSelect  DataType = "select" // chọn một trong các option của thuộc tính
)

func (d DataType) Valid() bool {
	switch d {
	case TypeText, TypeNumber, TypeDate, TypeBoolean, TypeSelect:
		return true
	}
	return false
}

// AssetType là loại tài sản (laptop, màn hình...) cùng thuộc tính riêng của nó.
// Trường chung của tài sản nằm ở Asset; loại chỉ thêm thuộc tính.
type AssetType struct {
	ID          uuid.UUID
	Code        string // 'LAPTOP': viết hoa, không đổi
	Name        string
	Description string
	IsSystem    bool // loại GENERAL seed sẵn: không archive được
	ArchivedAt  *time.Time
	Version     int32
	CreatedAt   time.Time
	UpdatedAt   time.Time
	// Chỉ có khi đọc một loại (Get): mọi thuộc tính kể cả đã gỡ, theo position rồi label
	Attributes []Attribute
}

func (t AssetType) Archived() bool { return t.ArchivedAt != nil }

// ActiveAttributes là thuộc tính chưa gỡ, theo thứ tự hiển thị
func (t AssetType) ActiveAttributes() []Attribute {
	out := make([]Attribute, 0, len(t.Attributes))
	for _, a := range t.Attributes {
		if a.RemovedAt == nil {
			out = append(out, a)
		}
	}
	return out
}

// Attribute là một thuộc tính riêng của loại
type Attribute struct {
	ID        uuid.UUID
	TypeID    uuid.UUID
	Key       string // 'ram_gb': không đổi, không dùng lại trong loại
	Label     string
	DataType  DataType
	Unit      string // "" là không có; chỉ cho kiểu number
	Required  bool
	Position  int32
	RemovedAt *time.Time
	Options   []Option // chỉ kiểu select, kể cả option đã gỡ
}

// Option là một lựa chọn của thuộc tính kiểu select
type Option struct {
	ID          uuid.UUID
	AttributeID uuid.UUID
	Label       string
	Position    int32
	RemovedAt   *time.Time
}

var (
	typeCodePattern = regexp.MustCompile(`^[A-Z0-9_-]{1,32}$`)
	attrKeyPattern  = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)
)

// NormalizeTypeCode bỏ khoảng trắng, viết hoa và kiểm tra dạng code
func NormalizeTypeCode(s string) (string, error) {
	c := strings.ToUpper(strings.TrimSpace(s))
	if !typeCodePattern.MatchString(c) {
		return "", ErrInvalidTypeCode
	}
	return c, nil
}

// ValidateAttributeKey: key dùng trong API và cột Excel, nên giữ dạng an toàn
func ValidateAttributeKey(s string) error {
	if !attrKeyPattern.MatchString(s) {
		return ErrInvalidAttributeKey
	}
	return nil
}

// CleanLabel bỏ khoảng trắng hai đầu; rỗng, dài quá max ký tự hay có ký tự
// điều khiển là lỗi
func CleanLabel(s string, maxLen int) (string, error) {
	l := strings.TrimSpace(s)
	if l == "" || utf8.RuneCountInString(l) > maxLen || strings.ContainsFunc(l, unicode.IsControl) {
		return "", ErrInvalidLabel
	}
	return l, nil
}

// CheckUnit: đơn vị chỉ dành cho thuộc tính kiểu number, tối đa 16 ký tự
func CheckUnit(dt DataType, unit string) (string, error) {
	u := strings.TrimSpace(unit)
	if u == "" {
		return "", nil
	}
	if dt != TypeNumber || utf8.RuneCountInString(u) > 16 || strings.ContainsFunc(u, unicode.IsControl) {
		return "", ErrInvalidUnit
	}
	return u, nil
}
