package domain

import "storeit/internal/platform/errs"

var (
	// Sai email hay sai mật khẩu đều là lỗi này, để không dò được email nào tồn tại
	ErrBadCredentials = errs.Unauthorized("/errors/bad-credentials", "Bad credentials",
		errs.WithDetail("Wrong email or password."))

	ErrAccountDisabled = errs.Forbidden("/errors/account-disabled", "Account disabled",
		errs.WithDetail("This account has been disabled. Contact an administrator."))

	// Mọi lần refresh thất bại (thiếu, sai, hết hạn, bị thu hồi, dùng lại) trả
	// đúng lỗi này: phân biệt chúng là cho kẻ dò thêm thông tin
	ErrInvalidRefreshToken = errs.Unauthorized("/errors/invalid-refresh-token", "Invalid refresh token",
		errs.WithDetail("Sign in again."))

	ErrInvalidEmail = errs.Unprocessable("/errors/invalid-email", "Invalid email",
		errs.WithFields(errs.FieldError{Field: "email", Detail: "must be a valid email address"}))

	ErrWeakPassword = errs.Unprocessable("/errors/weak-password", "Password does not meet the policy",
		errs.WithFields(errs.FieldError{Field: "password", Detail: "must be between 12 and 72 bytes"}))

	// Đổi mật khẩu của chính mình mà nhập sai mật khẩu hiện tại. Không dùng
	// 401: client đang đăng nhập hợp lệ, 401 sẽ khiến nó tưởng phiên đã hết
	ErrWrongPassword = errs.Unprocessable("/errors/wrong-password", "Current password is incorrect",
		errs.WithFields(errs.FieldError{Field: "current_password", Detail: "is incorrect"}))

	ErrInvalidName = errs.Unprocessable("/errors/invalid-name", "Invalid name",
		errs.WithFields(errs.FieldError{Field: "name", Detail: "must not be blank"}))

	ErrEmailTaken = errs.Conflict("/errors/email-taken", "Email already in use",
		errs.WithDetail("Another account already uses this email address."))

	ErrRoleNameTaken = errs.Conflict("/errors/role-name-taken", "Role name already in use")

	ErrAccountNotFound = errs.NotFound("/errors/account-not-found", "Account not found")

	ErrRoleNotFound = errs.NotFound("/errors/role-not-found", "Role not found")

	// Optimistic locking: version gửi lên đã cũ
	ErrAccountChanged = errs.Conflict("/errors/account-changed", "Account changed",
		errs.WithDetail("The account was changed by someone else. Reload it and try again."))

	ErrSystemRole = errs.Conflict("/errors/system-role", "System role",
		errs.WithDetail("System roles cannot be renamed or deleted."))

	ErrRoleInUse = errs.Conflict("/errors/role-in-use", "Role in use",
		errs.WithDetail("Remove the role from every account before deleting it."))

	// Chặn tự khoá mình hoặc làm mất người quản trị cuối cùng
	ErrLockout = errs.Conflict("/errors/lockout", "Change would lock you out")

	ErrUnknownRoles = errs.Unprocessable("/errors/unknown-roles", "Unknown roles",
		errs.WithFields(errs.FieldError{Field: "role_ids", Detail: "contains a role that does not exist"}))

	ErrUnknownPermissions = errs.Unprocessable("/errors/unknown-permissions", "Unknown permissions",
		errs.WithFields(errs.FieldError{Field: "permissions", Detail: "contains a permission that does not exist"}))

	// Mọi lý do một link đặt mật khẩu không dùng được đều chung một lỗi: không
	// tồn tại, đã dùng, đã bị thay, hết hạn, account bị khoá
	ErrInvalidPasswordToken = errs.Unprocessable("/errors/invalid-password-token", "Invalid or expired link",
		errs.WithDetail("This link is invalid or has expired. Ask for a new one."),
		errs.WithFields(errs.FieldError{Field: "token", Detail: "is invalid or has expired"}))

	ErrNotInvited = errs.Conflict("/errors/not-invited", "Account already active",
		errs.WithDetail("This account has already set a password. Send a password reset link instead."))

	ErrAccountInactive = errs.Conflict("/errors/account-inactive", "Account disabled",
		errs.WithDetail("Enable the account before sending it a link."))

	// Không trao, gỡ hay tác động lên quyền mình không có: chặn người có
	// identity.account.manage tự gán Administrator cho mình
	ErrExceedsOwnPermissions = errs.Forbidden("/errors/exceeds-own-permissions", "Beyond your own permissions",
		errs.WithDetail("You can only grant, remove or act on permissions you hold yourself."))

	ErrMemberConflict = errs.Unprocessable("/errors/member-conflict", "Conflicting member change",
		errs.WithFields(errs.FieldError{Field: "clear_member_id", Detail: "cannot be combined with member_id"}))
)
