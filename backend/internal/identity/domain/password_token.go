package domain

import (
	"time"

	"github.com/google/uuid"
)

// TokenPurpose là loại link gửi qua email
type TokenPurpose string

const (
	// Đặt mật khẩu lần đầu cho account được mời
	PurposeInvite TokenPurpose = "invite"
	// Quên mật khẩu, hoặc quản trị gửi link đặt lại
	PurposeReset TokenPurpose = "reset"
)

func (p TokenPurpose) Valid() bool { return p == PurposeInvite || p == PurposeReset }

// IssuedToken là token vừa phát: Raw đi vào thư (qua job), Hash vào DB
type IssuedToken struct {
	ID        uuid.UUID
	Purpose   TokenPurpose
	Raw       string
	Hash      []byte
	ExpiresAt time.Time
}

// PasswordToken là hàng token trong DB (không có token thô)
type PasswordToken struct {
	ID        uuid.UUID
	AccountID uuid.UUID
	Purpose   TokenPurpose
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Usable kiểm tra token còn dùng được cho account không. Mọi lý do đều là
// ErrInvalidPasswordToken: người dùng chỉ cần biết "xin link mới".
func (t PasswordToken) Usable(a Account, now time.Time) error {
	switch {
	case !t.ExpiresAt.After(now):
		return ErrInvalidPasswordToken
	case !a.Active:
		return ErrInvalidPasswordToken
	// Lời mời chỉ để đặt mật khẩu lần đầu
	case t.Purpose == PurposeInvite && a.HasPassword():
		return ErrInvalidPasswordToken
	}
	return nil
}

// UsedToken là kết quả đặt mật khẩu bằng link
type UsedToken struct {
	AccountID uuid.UUID
	Purpose   TokenPurpose
}

// TokenEvent là event ghi kèm khi phát token. Phát do người dùng tự yêu cầu
// (quên mật khẩu) thì không có event.
type TokenEvent int

const (
	TokenEventNone TokenEvent = iota
	// Quản trị gửi lại lời mời
	TokenEventInvitationResent
	// Quản trị gửi link đặt lại mật khẩu
	TokenEventResetSent
)
