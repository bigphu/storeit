package domain

import (
	"slices"

	"github.com/google/uuid"
)

// Role gom một nhóm quyền. Role hệ thống (seed ở migration) không đổi tên,
// không xoá được; quyền của nó thì sửa được.
type Role struct {
	ID          uuid.UUID
	Name        string
	Description string
	IsSystem    bool
	Permissions []string // mã quyền, đã sắp xếp
}

// Permission là một mục trong danh mục quyền
type Permission struct {
	Code        string
	Description string
}

func (r Role) CanRename() error {
	if r.IsSystem {
		return ErrSystemRole
	}
	return nil
}

// CanDelete: role hệ thống không xoá được; role còn gán cho account thì phải
// gỡ trước, để không ai mất quyền mà không biết
func (r Role) CanDelete(assignments int64) error {
	switch {
	case r.IsSystem:
		return ErrSystemRole
	case assignments > 0:
		return ErrRoleInUse
	}
	return nil
}

// CheckRolePermissions chặn bỏ identity.role.manage khỏi Administrator: mất nó
// thì không còn ai sửa lại được quyền của role nào
func CheckRolePermissions(roleID uuid.UUID, perms []string) error {
	if roleID == AdministratorRoleID && !slices.Contains(perms, PermRoleManage) {
		return ErrLockout
	}
	return nil
}
