package repository_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"

	"storeit/internal/identity/contract"
	"storeit/internal/identity/domain"
)

func TestRole_CreateUpdatePermissionsDelete(t *testing.T) {
	r := newRepos(t)
	ctx := context.Background()
	name := "Auditor " + uuid.NewString()[:8]

	role, err := r.roles.Create(ctx, domain.Role{
		Name: name, Description: "reads", Permissions: []string{domain.PermRoleRead, domain.PermAccountRead},
	})
	if err != nil {
		t.Fatal(err)
	}
	if role.IsSystem || !slices.Equal(role.Permissions, []string{domain.PermAccountRead, domain.PermRoleRead}) {
		t.Errorf("created role = %+v", role)
	}
	if n, _ := countEvents(t, r, contract.EventRoleCreated, role.ID); n != 1 {
		t.Errorf("role_created events = %d", n)
	}

	if _, err := r.roles.Create(ctx, domain.Role{Name: name}); !errors.Is(err, domain.ErrRoleNameTaken) {
		t.Errorf("duplicate name: %v", err)
	}
	if _, err := r.roles.ReplacePermissions(ctx, role.ID, []string{"no.such.permission"}); !errors.Is(err, domain.ErrUnknownPermissions) {
		t.Errorf("unknown permission: %v", err)
	}

	updated, err := r.roles.ReplacePermissions(ctx, role.ID, []string{domain.PermRoleRead})
	if err != nil || !slices.Equal(updated.Permissions, []string{domain.PermRoleRead}) {
		t.Errorf("replace permissions = %+v, %v", updated, err)
	}
	renamed, err := r.roles.Update(ctx, role.ID, name+" v2", "reads more")
	if err != nil || renamed.Name != name+" v2" {
		t.Errorf("update = %+v, %v", renamed, err)
	}

	if err := r.roles.Delete(ctx, role.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.roles.Get(ctx, role.ID); !errors.Is(err, domain.ErrRoleNotFound) {
		t.Errorf("get deleted: %v", err)
	}
	if n, _ := countEvents(t, r, contract.EventRoleDeleted, role.ID); n != 1 {
		t.Errorf("role_deleted events = %d", n)
	}
}

func TestRole_ListAndCatalogue(t *testing.T) {
	r := newRepos(t)
	ctx := context.Background()

	roles, err := r.roles.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var admin domain.Role
	for _, ro := range roles {
		if ro.ID == domain.AdministratorRoleID {
			admin = ro
		}
	}
	if !admin.IsSystem || !slices.Contains(admin.Permissions, domain.PermRoleManage) {
		t.Errorf("Administrator = %+v", admin)
	}

	perms, err := r.roles.Permissions(ctx)
	if err != nil || len(perms) < 4 || perms[0].Description == "" {
		t.Errorf("catalogue = %v, %v", perms, err)
	}

	a := newAccount(t, r, domain.EmployeeRoleID)
	if n, err := r.roles.CountAssignments(ctx, domain.EmployeeRoleID); err != nil || n < 1 {
		t.Errorf("assignments = %d, %v (account %v)", n, err, a.ID)
	}
}
