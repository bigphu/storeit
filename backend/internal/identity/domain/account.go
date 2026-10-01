package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Account là tài khoản đăng nhập. Tạo bởi người quản trị, không tự đăng ký.
type Account struct {
	ID           uuid.UUID
	Email        string // luôn chữ thường (NormalizeEmail)
	Name         string
	PasswordHash string
	MemberID     *uuid.UUID // liên kết tới directory, có thể chưa có
	Active       bool
	Version      int32 // optimistic locking cho cập nhật hồ sơ
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// CanLogin: account bị khoá thì không đăng nhập, không refresh được
func (a Account) CanLogin() error {
	if !a.Active {
		return ErrAccountDisabled
	}
	return nil
}

// NormalizeEmail bỏ khoảng trắng hai đầu, chuyển chữ thường và kiểm tra dạng
// local@domain. Không kiểm tra sâu hơn: email có tồn tại hay không thì chỉ gửi
// thư mới biết, và hệ thống không gửi thư.
func NormalizeEmail(s string) (string, error) {
	e := strings.ToLower(strings.TrimSpace(s))
	local, domain, ok := strings.Cut(e, "@")
	if !ok || local == "" || domain == "" || strings.Contains(domain, "@") ||
		strings.ContainsAny(e, " \t\r\n") || len(e) > 254 {
		return "", ErrInvalidEmail
	}
	return e, nil
}
