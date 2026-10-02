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
	PasswordHash string     // rỗng: được mời, chưa đặt mật khẩu
	MemberID     *uuid.UUID // liên kết tới directory, có thể chưa có
	Active       bool
	Version      int32 // optimistic locking cho cập nhật hồ sơ
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// AccountStatus là trạng thái hiển thị, suy ra từ Active và mật khẩu
type AccountStatus string

const (
	StatusInvited  AccountStatus = "invited"
	StatusActive   AccountStatus = "active"
	StatusDisabled AccountStatus = "disabled"
)

func (a Account) Status() AccountStatus {
	switch {
	case !a.Active:
		return StatusDisabled
	case !a.HasPassword():
		return StatusInvited
	}
	return StatusActive
}

// HasPassword: false là account được mời nhưng chưa nhận lời
func (a Account) HasPassword() bool { return a.PasswordHash != "" }

// CanLogin: account bị khoá thì không đăng nhập, không refresh được. Account
// chưa nhận lời mời trả ErrBadCredentials, không lộ ra là email có tồn tại.
func (a Account) CanLogin() error {
	if !a.Active {
		return ErrAccountDisabled
	}
	if !a.HasPassword() {
		return ErrBadCredentials
	}
	return nil
}

// NormalizeEmail bỏ khoảng trắng hai đầu, chuyển chữ thường và kiểm tra dạng
// local@domain. Không kiểm tra sâu hơn: email có tồn tại hay không thì chỉ gửi
// thư mới biết (lời mời đi tới đó).
func NormalizeEmail(s string) (string, error) {
	e := strings.ToLower(strings.TrimSpace(s))
	local, domain, ok := strings.Cut(e, "@")
	if !ok || local == "" || domain == "" || strings.Contains(domain, "@") ||
		strings.ContainsAny(e, " \t\r\n") || len(e) > 254 {
		return "", ErrInvalidEmail
	}
	return e, nil
}
