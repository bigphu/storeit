package service

import (
	"context"

	"github.com/google/uuid"

	"storeit/internal/identity/domain"
	"storeit/internal/platform/auth"
)

func (s *Service) ListRoles(ctx context.Context) ([]domain.Role, error) {
	if _, err := auth.Require(ctx, domain.PermRoleRead); err != nil {
		return nil, err
	}
	return s.roles.List(ctx)
}

// RoleMemberCounts: số account chưa bị khoá giữ mỗi role
func (s *Service) RoleMemberCounts(ctx context.Context) (map[uuid.UUID]int64, error) {
	if _, err := auth.Require(ctx, domain.PermRoleRead); err != nil {
		return nil, err
	}
	return s.roles.MemberCounts(ctx)
}

func (s *Service) GetRole(ctx context.Context, id uuid.UUID) (domain.Role, error) {
	if _, err := auth.Require(ctx, domain.PermRoleRead); err != nil {
		return domain.Role{}, err
	}
	return s.roles.Get(ctx, id)
}

// RestoreRole: hoàn tác xoá role
func (s *Service) RestoreRole(ctx context.Context, id uuid.UUID) (domain.Role, error) {
	if _, err := auth.Require(ctx, domain.PermRoleManage); err != nil {
		return domain.Role{}, err
	}
	return s.roles.Restore(ctx, id)
}

// CreateRole: role mới chỉ được mang quyền người tạo có
func (s *Service) CreateRole(ctx context.Context, name, description string, perms []string) (domain.Role, error) {
	actor, err := auth.Require(ctx, domain.PermRoleManage)
	if err != nil {
		return domain.Role{}, err
	}
	if err := requireHolds(actor, perms); err != nil {
		return domain.Role{}, err
	}
	n, err := cleanName(name)
	if err != nil {
		return domain.Role{}, err
	}
	return s.roles.Create(ctx, domain.Role{Name: n, Description: description, Permissions: perms})
}

// UpdateRole đổi tên và mô tả; field nil là giữ nguyên. Role hệ thống không đổi tên được.
func (s *Service) UpdateRole(ctx context.Context, id uuid.UUID, name, description *string) (domain.Role, error) {
	if _, err := auth.Require(ctx, domain.PermRoleManage); err != nil {
		return domain.Role{}, err
	}
	cur, err := s.roles.Get(ctx, id)
	if err != nil {
		return domain.Role{}, err
	}
	newName, newDesc := cur.Name, cur.Description
	if name != nil {
		n, err := cleanName(*name)
		if err != nil {
			return domain.Role{}, err
		}
		if n != cur.Name {
			if err := cur.CanRename(); err != nil {
				return domain.Role{}, err
			}
			newName = n
		}
	}
	if description != nil {
		newDesc = *description
	}
	return s.roles.Update(ctx, id, newName, newDesc)
}

// UpdateRolePermissions thay toàn bộ quyền của role. Administrator phải giữ
// identity.role.manage; quyền được thêm hay bị gỡ phải là quyền người sửa có.
// Có hiệu lực với người dùng ở lần refresh kế tiếp.
func (s *Service) UpdateRolePermissions(ctx context.Context, id uuid.UUID, perms []string) (domain.Role, error) {
	actor, err := auth.Require(ctx, domain.PermRoleManage)
	if err != nil {
		return domain.Role{}, err
	}
	cur, err := s.roles.Get(ctx, id)
	if err != nil {
		return domain.Role{}, err
	}
	if err := domain.CheckRolePermissions(id, perms); err != nil {
		return domain.Role{}, err
	}
	if err := requireHolds(actor, symmetricDiff(cur.Permissions, perms)); err != nil {
		return domain.Role{}, err
	}
	return s.roles.ReplacePermissions(ctx, id, perms)
}

func (s *Service) DeleteRole(ctx context.Context, id uuid.UUID) error {
	if _, err := auth.Require(ctx, domain.PermRoleManage); err != nil {
		return err
	}
	cur, err := s.roles.Get(ctx, id)
	if err != nil {
		return err
	}
	n, err := s.roles.CountAssignments(ctx, id)
	if err != nil {
		return err
	}
	if err := cur.CanDelete(n); err != nil {
		return err
	}
	return s.roles.Delete(ctx, id)
}

func (s *Service) ListPermissions(ctx context.Context) ([]domain.Permission, error) {
	if _, err := auth.Require(ctx, domain.PermRoleRead); err != nil {
		return nil, err
	}
	return s.roles.Permissions(ctx)
}
