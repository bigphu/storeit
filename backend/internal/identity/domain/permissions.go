package domain

import "github.com/google/uuid"

// Quyền của module identity. Mã quyền nằm trong bảng identity.permissions
// (seed ở migration 00002); module khác thêm mã của mình bằng migration riêng.
const (
	PermAccountRead   = "identity.account.read"
	PermAccountManage = "identity.account.manage"
	PermRoleRead      = "identity.role.read"
	PermRoleManage    = "identity.role.manage"
)

// ID cố định của role hệ thống, seed ở migration 00002
var (
	AdministratorRoleID     = uuid.MustParse("00000000-0000-7000-8000-000000000001")
	AuthorizedManagerRoleID = uuid.MustParse("00000000-0000-7000-8000-000000000002")
	InventoryOfficerRoleID  = uuid.MustParse("00000000-0000-7000-8000-000000000003")
	EmployeeRoleID          = uuid.MustParse("00000000-0000-7000-8000-000000000004")
)
