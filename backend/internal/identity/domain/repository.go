package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Interface repository do service dùng, repository/ cài đặt. Mọi method ghi
// tự mở transaction và ghi event vào outbox trong cùng transaction đó; service
// không bao giờ thấy transaction.

type AccountFilter struct {
	Query  string // tìm theo tên hoặc email, rỗng là không lọc
	Active *bool
	Limit  int32
	Offset int32
}

type NewAccount struct {
	Email        string // đã NormalizeEmail
	Name         string
	PasswordHash string
	MemberID     *uuid.UUID
	RoleIDs      []uuid.UUID
}

// ProfileChange: field nil là giữ nguyên. Version là version client đang có.
type ProfileChange struct {
	Name        *string
	MemberID    *uuid.UUID
	ClearMember bool
	Version     int32
}

type AccountRepository interface {
	// Create: ErrEmailTaken, ErrUnknownRoles; event account_created
	Create(ctx context.Context, in NewAccount) (Account, error)
	// Get, GetByEmail: ErrAccountNotFound
	Get(ctx context.Context, id uuid.UUID) (Account, error)
	GetByEmail(ctx context.Context, email string) (Account, error)
	GetMany(ctx context.Context, ids []uuid.UUID) ([]Account, error)
	List(ctx context.Context, f AccountFilter) ([]Account, int64, error)
	// UpdateProfile: ErrAccountChanged khi version cũ; event account_updated
	UpdateProfile(ctx context.Context, id uuid.UUID, ch ProfileChange) (Account, error)
	// SetActive: khoá thì thu hồi mọi phiên (admin); event account_disabled/enabled
	SetActive(ctx context.Context, id uuid.UUID, active bool) (Account, error)
	// SetPassword thu hồi mọi phiên của account trừ keepFamily; reset=true là
	// quản trị đặt lại (event password_reset), false là tự đổi (không event)
	SetPassword(ctx context.Context, id uuid.UUID, hash string, keepFamily *uuid.UUID, reset bool) error
	// ReplaceRoles: ErrUnknownRoles; event roles_assigned
	ReplaceRoles(ctx context.Context, id uuid.UUID, roleIDs []uuid.UUID) (Account, error)
	Roles(ctx context.Context, id uuid.UUID) ([]Role, error)
	Permissions(ctx context.Context, id uuid.UUID) ([]string, error)
	Count(ctx context.Context) (int64, error)
}

type RoleRepository interface {
	List(ctx context.Context) ([]Role, error)
	// Get: ErrRoleNotFound
	Get(ctx context.Context, id uuid.UUID) (Role, error)
	// Create: ErrRoleNameTaken, ErrUnknownPermissions; event role_created
	Create(ctx context.Context, r Role) (Role, error)
	// Update: ErrRoleNameTaken; event role_updated
	Update(ctx context.Context, id uuid.UUID, name, description string) (Role, error)
	// ReplacePermissions: ErrUnknownPermissions; event role_permissions_updated
	ReplacePermissions(ctx context.Context, id uuid.UUID, perms []string) (Role, error)
	// Delete: event role_deleted
	Delete(ctx context.Context, id uuid.UUID) error
	CountAssignments(ctx context.Context, id uuid.UUID) (int64, error)
	Permissions(ctx context.Context) ([]Permission, error)
}

type NewSession struct {
	AccountID         uuid.UUID
	TokenHash         []byte
	UserAgent         string
	IP                string
	ExpiresAt         time.Time
	AbsoluteExpiresAt time.Time
}

type RefreshInput struct {
	TokenHash []byte
	NextHash  []byte // hash của token mới, chỉ dùng khi xoay
	Now       time.Time
	Grace     time.Duration
	Sliding   time.Duration
}

type RefreshResult struct {
	Decision  RefreshDecision
	AccountID uuid.UUID
	FamilyID  uuid.UUID
	ExpiresAt time.Time // hạn của token mới khi xoay
}

type SessionRepository interface {
	// Start mở một family cùng token gốc
	Start(ctx context.Context, s NewSession) (familyID uuid.UUID, err error)
	// Refresh: một transaction, khoá token và family, Decide rồi áp dụng.
	// Không tìm thấy token là RefreshReject.
	Refresh(ctx context.Context, in RefreshInput) (RefreshResult, error)
	// Revoke thu hồi family của token. Token lạ: trả nil, không lỗi.
	Revoke(ctx context.Context, tokenHash []byte, reason RevokeReason) (familyID *uuid.UUID, err error)
	FamilyOf(ctx context.Context, tokenHash []byte) (*uuid.UUID, error)
	// Prune xoá family đã chết và token đã dùng quá mốc cutoff
	Prune(ctx context.Context, cutoff time.Time) (families, tokens int64, err error)
}
