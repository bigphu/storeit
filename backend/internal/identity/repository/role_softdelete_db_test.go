package repository_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"storeit/internal/identity/domain"
)

// customRole tạo role tự định nghĩa mang một quyền
func customRole(t *testing.T, r repos, perm string) domain.Role {
	t.Helper()
	role, err := r.roles.Create(context.Background(), domain.Role{Name: "Race " + uuid.NewString()[:8], Permissions: []string{perm}})
	if err != nil {
		t.Fatal(err)
	}
	return role
}

// blocked: kết quả chưa về sau một lúc, tức là đang đợi khoá
func blocked(done <-chan error) bool {
	select {
	case <-done:
		return false
	case <-time.After(300 * time.Millisecond):
		return true
	}
}

// Gán role chưa commit rồi xoá role: xoá phải đợi và thấy role đang được giữ (409)
func TestRole_DeleteWaitsForUncommittedAssignment(t *testing.T) {
	r := newRepos(t)
	ctx := context.Background()
	role := customRole(t, r, domain.PermAccountRead)
	acc := newAccount(t, r)

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if _, err := tx.Exec(ctx, `INSERT INTO identity.account_roles (account_id, role_id) VALUES ($1, $2)`, acc.ID, role.ID); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- r.roles.Delete(ctx, role.ID) }()
	if !blocked(done) {
		t.Fatal("delete did not wait for the uncommitted assignment")
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, domain.ErrRoleInUse) {
		t.Errorf("delete = %v, want ErrRoleInUse", err)
	}
}

// Xoá role chưa commit rồi gán role: gán phải đợi và thấy role đã xoá (422)
func TestRole_AssignWaitsForUncommittedDelete(t *testing.T) {
	r := newRepos(t)
	ctx := context.Background()
	role := customRole(t, r, domain.PermAccountRead)
	acc := newAccount(t, r)

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if _, err := tx.Exec(ctx, `UPDATE identity.roles SET deleted_at = now() WHERE id = $1`, role.ID); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := r.accounts.ReplaceRoles(ctx, acc.ID, []uuid.UUID{role.ID})
		done <- err
	}()
	if !blocked(done) {
		t.Fatal("assign did not wait for the uncommitted delete")
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, domain.ErrUnknownRoles) {
		t.Errorf("assign = %v, want ErrUnknownRoles", err)
	}
}

// Role đã xoá còn sót trong account_roles (dữ liệu cũ) không mang quyền và không hiện
func TestRole_DeletedRoleGrantsNothing(t *testing.T) {
	r := newRepos(t)
	ctx := context.Background()
	role := customRole(t, r, domain.PermRoleManage)
	acc := newAccount(t, r, role.ID)
	if _, err := r.pool.Exec(ctx, `UPDATE identity.roles SET deleted_at = now() WHERE id = $1`, role.ID); err != nil {
		t.Fatal(err)
	}
	perms, err := r.accounts.Permissions(ctx, acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(perms, domain.PermRoleManage) {
		t.Errorf("deleted role still grants %s: %v", domain.PermRoleManage, perms)
	}
	// danh sách account đọc role qua RolesOf
	of, err := r.accounts.RolesOf(ctx, []uuid.UUID{acc.ID})
	if err != nil {
		t.Fatal(err)
	}
	for _, ro := range of[acc.ID] {
		if ro.ID == role.ID {
			t.Errorf("account list shows deleted role %s", role.Name)
		}
	}
}
