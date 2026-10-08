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
	Status *AccountStatus
	RoleID *uuid.UUID
	Limit  int32
	Offset int32
}

type NewAccount struct {
	Email        string // đã NormalizeEmail
	Name         string
	PasswordHash string // rỗng: account được mời, đặt mật khẩu qua Invite
	MemberID     *uuid.UUID
	RoleIDs      []uuid.UUID
	// Invite phát cùng transaction tạo account (kèm job gửi thư)
	Invite *IssuedToken
}

// ProfileChange: field nil là giữ nguyên. Version là version client đang có.
type ProfileChange struct {
	Name        *string
	MemberID    *uuid.UUID
	ClearMember bool
	Version     int32
}

type AccountRepository interface {
	// Create: ErrEmailTaken, ErrUnknownRoles; event account_created; có
	// Invite thì ghi token và xếp job gửi thư trong cùng transaction
	Create(ctx context.Context, in NewAccount) (Account, error)
	// Get, GetByEmail: ErrAccountNotFound
	Get(ctx context.Context, id uuid.UUID) (Account, error)
	GetByEmail(ctx context.Context, email string) (Account, error)
	GetMany(ctx context.Context, ids []uuid.UUID) ([]Account, error)
	List(ctx context.Context, f AccountFilter) ([]Account, int64, error)
	// UpdateProfile: ErrAccountChanged khi version cũ; event account_updated
	UpdateProfile(ctx context.Context, id uuid.UUID, ch ProfileChange) (Account, error)
	// SetActive: khoá thì thu hồi mọi phiên (admin) và xoá link đặt mật khẩu;
	// event account_disabled/enabled
	SetActive(ctx context.Context, id uuid.UUID, active bool) (Account, error)
	// SetPassword (người dùng tự đổi) thu hồi mọi phiên của account trừ
	// keepFamily; không event
	SetPassword(ctx context.Context, id uuid.UUID, hash string, keepFamily *uuid.UUID) error
	// ReplaceRoles: ErrUnknownRoles; event roles_assigned
	ReplaceRoles(ctx context.Context, id uuid.UUID, roleIDs []uuid.UUID) (Account, error)
	Roles(ctx context.Context, id uuid.UUID) ([]Role, error)
	Permissions(ctx context.Context, id uuid.UUID) ([]string, error)
	Count(ctx context.Context) (int64, error)
	// StatusCounts: số account theo trạng thái với Query và RoleID của f (bỏ qua
	// Active, Status, phân trang); trạng thái không có account thì không có trong map
	StatusCounts(ctx context.Context, f AccountFilter) (map[AccountStatus]int64, error)
	// RolesOf: role của từng account (danh sách account, tránh N+1)
	RolesOf(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]Role, error)
	// InviteExpiries: hạn link mời đang chờ của các account có link mời
	InviteExpiries(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]time.Time, error)
	// LiveSessions: số phiên đăng nhập còn dùng được
	LiveSessions(ctx context.Context, id uuid.UUID) (int64, error)
	// SignOutEverywhere thu hồi mọi phiên (admin); có phiên bị thu hồi thì event
	// account_signed_out. Trả số phiên đã thu hồi.
	SignOutEverywhere(ctx context.Context, id uuid.UUID) (int64, error)
}

// TokenRepository lưu link đặt mật khẩu (invite, reset)
type TokenRepository interface {
	// Issue ghi đè token cùng loại của account, xếp job gửi thư và ghi event
	// ev, trong một transaction. Khoá account trước (cùng thứ tự với SetActive,
	// tránh deadlock); account bị khoá: ErrAccountInactive. minAge > 0: token
	// cùng loại mới phát chưa được minAge thì không phát gì và trả false; kiểm
	// tra và ghi là một câu SQL nên hai request song song không cùng lọt.
	Issue(ctx context.Context, accountID uuid.UUID, t IssuedToken, ev TokenEvent, minAge time.Duration) (issued bool, err error)
	// Use dùng token có hash này để đặt mật khẩu, trong một transaction: khoá
	// account, xoá token, kiểm tra Usable, đặt mật khẩu. Reset thì thu hồi mọi phiên;
	// invite thì ghi event invitation_accepted. Mọi thất bại:
	// ErrInvalidPasswordToken.
	Use(ctx context.Context, hash []byte, now time.Time, passwordHash string) (UsedToken, error)
	// Get: ErrInvalidPasswordToken khi không còn (đã dùng hoặc bị thay)
	Get(ctx context.Context, id uuid.UUID) (PasswordToken, error)
	// Prune xoá token hết hạn trước cutoff
	Prune(ctx context.Context, cutoff time.Time) (int64, error)
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
	// Delete: xoá mềm (giữ hàng và quyền); event role_deleted
	Delete(ctx context.Context, id uuid.UUID) error
	// Restore: khôi phục role đã xoá (chưa xoá thì trả nguyên); ErrRoleNotFound,
	// ErrRoleNameTaken; event role_restored
	Restore(ctx context.Context, id uuid.UUID) (Role, error)
	CountAssignments(ctx context.Context, id uuid.UUID) (int64, error)
	// MemberCounts: số account chưa bị khoá giữ mỗi role; role không ai giữ thì không có
	MemberCounts(ctx context.Context) (map[uuid.UUID]int64, error)
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
