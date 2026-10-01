package domain

import (
	"time"

	"github.com/google/uuid"
)

// RefreshState là trạng thái của một refresh token, family của nó và account,
// đọc dưới khoá FOR UPDATE. Decide quyết định dựa trên đây, không cần biết nó
// tới từ câu SQL nào (port từ mimir-2.0).
type RefreshState struct {
	TokenID           uuid.UUID
	FamilyID          uuid.UUID
	AccountID         uuid.UUID
	UsedAt            *time.Time
	ExpiresAt         time.Time // hạn trượt của chính token
	AbsoluteExpiresAt time.Time // hạn tuyệt đối của family (một lần đăng nhập)
	RevokedAt         *time.Time
	AccountActive     bool
}

type RefreshDecision int

const (
	// Từ chối, không ghi gì: family đã thu hồi, token hết hạn trượt, không tìm thấy
	RefreshReject RefreshDecision = iota
	// Quá hạn tuyệt đối: thu hồi family với lý do expired
	RefreshExpired
	// Token đã dùng, ngoài cửa sổ ân hạn: coi như bị đánh cắp, thu hồi cả family
	RefreshReuse
	// Token đã dùng trong cửa sổ ân hạn: client thật gửi lại (mạng chập chờn,
	// hai tab). Xoay ngọn của family, không thu hồi
	RefreshGrace
	// Đường bình thường: xoay chính token được gửi lên
	RefreshRotate
	// Account đã bị khoá: thu hồi family với lý do admin
	RefreshAccountDisabled
)

func (d RefreshDecision) String() string {
	switch d {
	case RefreshReject:
		return "reject"
	case RefreshExpired:
		return "expired"
	case RefreshReuse:
		return "reuse"
	case RefreshGrace:
		return "grace"
	case RefreshRotate:
		return "rotate"
	case RefreshAccountDisabled:
		return "account_disabled"
	}
	return "unknown"
}

// Decide phân loại một lần refresh. Thứ tự quan trọng: family đã thu hồi thắng
// mọi thứ (không thu hồi lại), rồi tới account bị khoá, hạn tuyệt đối, dùng lại.
func (s RefreshState) Decide(now time.Time, grace time.Duration) RefreshDecision {
	switch {
	case s.RevokedAt != nil:
		return RefreshReject
	case !s.AccountActive:
		return RefreshAccountDisabled
	case !s.AbsoluteExpiresAt.After(now):
		return RefreshExpired
	case s.UsedAt != nil:
		if now.Sub(*s.UsedAt) <= grace {
			return RefreshGrace
		}
		return RefreshReuse
	case !s.ExpiresAt.After(now):
		return RefreshReject
	}
	return RefreshRotate
}

// RevokeReason khớp CHECK refresh_families_reason_check
type RevokeReason string

const (
	RevokeLogout        RevokeReason = "logout"
	RevokeReuseDetected RevokeReason = "reuse_detected"
	RevokeAdmin         RevokeReason = "admin"
	RevokeExpired       RevokeReason = "expired"
)
