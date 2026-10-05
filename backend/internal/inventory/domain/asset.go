package domain

import (
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Asset là một tài sản: trường chung là cột, thuộc tính riêng của loại nằm ở Values
type Asset struct {
	ID             uuid.UUID
	Tag            string // viết hoa, duy nhất kể cả tài sản đã retire, không đổi
	Name           string
	Description    string
	TypeID         uuid.UUID
	StatusID       uuid.UUID
	LocationID     *uuid.UUID // directory làm sau
	HolderMemberID *uuid.UUID // người đang giữ, directory làm sau
	PurchaseDate   *time.Time // chỉ ngày
	RetiredAt      *time.Time
	RetiredReason  string
	Version        int32
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Values         []Value // chỉ có khi đọc một tài sản
}

func (a Asset) Retired() bool { return a.RetiredAt != nil }

var tagPattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9._-]{0,63}$`)

// NormalizeTag bỏ khoảng trắng, viết hoa và kiểm tra dạng tag. Tag in lên nhãn
// và mã QR nên chỉ gồm chữ không dấu, số và . _ -
func NormalizeTag(s string) (string, error) {
	t := strings.ToUpper(strings.TrimSpace(s))
	if !tagPattern.MatchString(t) {
		return "", ErrInvalidTag
	}
	return t, nil
}
