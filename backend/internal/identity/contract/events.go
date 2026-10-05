package contract

import "github.com/google/uuid"

// Event công khai của identity. Module khác (activity) nghe theo các tên này
// và đọc payload bằng events.Decode[T]. Payload không bao giờ chứa password
// hash hay token.
const (
	EventAccountCreated         = "identity.account_created"
	EventAccountUpdated         = "identity.account_updated"
	EventAccountDisabled        = "identity.account_disabled"
	EventAccountEnabled         = "identity.account_enabled"
	EventInvitationResent       = "identity.invitation_resent"
	EventInvitationAccepted     = "identity.invitation_accepted"
	EventPasswordResetSent      = "identity.password_reset_sent"
	EventRolesAssigned          = "identity.roles_assigned"
	EventRoleCreated            = "identity.role_created"
	EventRoleUpdated            = "identity.role_updated"
	EventRolePermissionsUpdated = "identity.role_permissions_updated"
	EventRoleDeleted            = "identity.role_deleted"
	EventAccountSignedOut       = "identity.account_signed_out"
)

// Loại aggregate trong platform.events
const (
	AggregateAccount = "account"
	AggregateRole    = "role"
)

// FieldChange là giá trị trước và sau của một field, để activity hiện được
type FieldChange struct {
	Field string `json:"field"`
	From  any    `json:"from"`
	To    any    `json:"to"`
}

type AccountCreated struct {
	AccountID uuid.UUID   `json:"account_id"`
	Email     string      `json:"email"`
	Name      string      `json:"name"`
	MemberID  *uuid.UUID  `json:"member_id,omitempty"`
	RoleIDs   []uuid.UUID `json:"role_ids"`
}

type AccountUpdated struct {
	AccountID uuid.UUID     `json:"account_id"`
	Changes   []FieldChange `json:"changes"`
}

type AccountDisabled struct {
	AccountID uuid.UUID `json:"account_id"`
}

type AccountEnabled struct {
	AccountID uuid.UUID `json:"account_id"`
}

// InvitationResent: quản trị gửi lại lời mời (kể cả khi bấm "gửi link đặt lại"
// cho account chưa nhận lời)
type InvitationResent struct {
	AccountID uuid.UUID `json:"account_id"`
}

// InvitationAccepted: người được mời đặt mật khẩu lần đầu; actor là chính account
type InvitationAccepted struct {
	AccountID uuid.UUID `json:"account_id"`
}

// PasswordResetSent: quản trị gửi link đặt lại mật khẩu. Người dùng tự yêu cầu
// hay tự đổi mật khẩu thì không có event.
type PasswordResetSent struct {
	AccountID uuid.UUID `json:"account_id"`
}

// AccountSignedOut: quản trị đăng xuất account khỏi mọi thiết bị
type AccountSignedOut struct {
	AccountID uuid.UUID `json:"account_id"`
	Sessions  int64     `json:"sessions"`
}

type RolesAssigned struct {
	AccountID uuid.UUID   `json:"account_id"`
	From      []uuid.UUID `json:"from"`
	To        []uuid.UUID `json:"to"`
}

type RoleCreated struct {
	RoleID      uuid.UUID `json:"role_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Permissions []string  `json:"permissions"`
}

type RoleUpdated struct {
	RoleID  uuid.UUID     `json:"role_id"`
	Changes []FieldChange `json:"changes"`
}

type RolePermissionsUpdated struct {
	RoleID uuid.UUID `json:"role_id"`
	From   []string  `json:"from"`
	To     []string  `json:"to"`
}

type RoleDeleted struct {
	RoleID uuid.UUID `json:"role_id"`
	Name   string    `json:"name"`
}
