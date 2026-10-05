package repository_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"storeit/internal/identity/contract"
	"storeit/internal/identity/domain"
)

func TestAccount_CreateGetAndEvent(t *testing.T) {
	r := newRepos(t)
	admin := uuid.New()
	email := "New-" + uuid.NewString()[:8] + "@StoreIT.test"

	a, err := r.accounts.Create(actorCtx(admin), domain.NewAccount{
		Email: strings.ToLower(email), Name: "Lan", PasswordHash: "h",
		RoleIDs: []uuid.UUID{domain.EmployeeRoleID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !a.Active || a.Version != 1 {
		t.Errorf("new account = %+v, want active version 1", a)
	}

	// Email không phân biệt hoa thường
	got, err := r.accounts.GetByEmail(context.Background(), email)
	if err != nil || got.ID != a.ID {
		t.Errorf("GetByEmail = %v, %v", got.ID, err)
	}

	n, actor := countEvents(t, r, contract.EventAccountCreated, a.ID)
	if n != 1 || actor == nil || *actor != admin {
		t.Errorf("account_created events = %d, actor %v, want 1 by %v", n, actor, admin)
	}

	roles, err := r.accounts.Roles(context.Background(), a.ID)
	if err != nil || len(roles) != 1 || roles[0].ID != domain.EmployeeRoleID {
		t.Errorf("roles = %v, %v", roles, err)
	}
}

func TestAccount_DuplicateEmailAnyCase(t *testing.T) {
	r := newRepos(t)
	a := newAccount(t, r)

	_, err := r.accounts.Create(context.Background(), domain.NewAccount{
		Email: strings.ToUpper(a.Email), Name: "Dup", PasswordHash: "h",
	})
	if !errors.Is(err, domain.ErrEmailTaken) {
		t.Errorf("err = %v, want ErrEmailTaken", err)
	}
}

func TestAccount_UnknownRoleAndNotFound(t *testing.T) {
	r := newRepos(t)
	_, err := r.accounts.Create(context.Background(), domain.NewAccount{
		Email: "x-" + uuid.NewString()[:8] + "@storeit.test", Name: "X", PasswordHash: "h",
		RoleIDs: []uuid.UUID{uuid.New()},
	})
	if !errors.Is(err, domain.ErrUnknownRoles) {
		t.Errorf("create with unknown role: %v", err)
	}
	if _, err := r.accounts.Get(context.Background(), uuid.New()); !errors.Is(err, domain.ErrAccountNotFound) {
		t.Errorf("get missing: %v", err)
	}
}

func TestAccount_UpdateProfileOptimisticLock(t *testing.T) {
	r := newRepos(t)
	a := newAccount(t, r)
	name := "Renamed"

	updated, err := r.accounts.UpdateProfile(context.Background(), a.ID, domain.ProfileChange{Name: &name, Version: a.Version})
	if err != nil || updated.Name != name || updated.Version != a.Version+1 {
		t.Fatalf("update = %+v, %v", updated, err)
	}
	if n, _ := countEvents(t, r, contract.EventAccountUpdated, a.ID); n != 1 {
		t.Errorf("account_updated events = %d", n)
	}

	// Gửi lại version cũ: người khác đã sửa
	_, err = r.accounts.UpdateProfile(context.Background(), a.ID, domain.ProfileChange{Name: &name, Version: a.Version})
	if !errors.Is(err, domain.ErrAccountChanged) {
		t.Errorf("stale version: %v, want ErrAccountChanged", err)
	}
}

func TestAccount_DisableRevokesSessions(t *testing.T) {
	r := newRepos(t)
	a := newAccount(t, r)
	ctx := context.Background()
	now := time.Now()
	for i := range 2 {
		if _, err := r.sessions.Start(ctx, domain.NewSession{
			AccountID: a.ID, TokenHash: hash(uuid.NewString() + string(rune(i))),
			ExpiresAt: now.Add(time.Hour), AbsoluteExpiresAt: now.Add(24 * time.Hour),
		}); err != nil {
			t.Fatal(err)
		}
	}

	disabled, err := r.accounts.SetActive(ctx, a.ID, false)
	if err != nil || disabled.Active {
		t.Fatalf("disable = %+v, %v", disabled, err)
	}

	var live int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM identity.refresh_families
		WHERE account_id = $1 AND revoked_at IS NULL`, a.ID).Scan(&live); err != nil {
		t.Fatal(err)
	}
	if live != 0 {
		t.Errorf("live families after disable = %d, want 0", live)
	}
	if n, _ := countEvents(t, r, contract.EventAccountDisabled, a.ID); n != 1 {
		t.Errorf("account_disabled events = %d", n)
	}

	// Khoá lại lần nữa: không đổi gì, không thêm event
	if _, err := r.accounts.SetActive(ctx, a.ID, false); err != nil {
		t.Fatal(err)
	}
	if n, _ := countEvents(t, r, contract.EventAccountDisabled, a.ID); n != 1 {
		t.Errorf("repeat disable added events: %d", n)
	}
}

func TestAccount_ReplaceRolesAndPermissions(t *testing.T) {
	r := newRepos(t)
	a := newAccount(t, r, domain.EmployeeRoleID)
	ctx := context.Background()

	if _, err := r.accounts.ReplaceRoles(ctx, a.ID, []uuid.UUID{uuid.New()}); !errors.Is(err, domain.ErrUnknownRoles) {
		t.Errorf("unknown role: %v", err)
	}

	if _, err := r.accounts.ReplaceRoles(ctx, a.ID,
		[]uuid.UUID{domain.AuthorizedManagerRoleID, domain.EmployeeRoleID, domain.EmployeeRoleID}); err != nil {
		t.Fatal(err)
	}
	perms, err := r.accounts.Permissions(ctx, a.ID)
	// Chỉ xét quyền của identity: migration của module khác cũng phân quyền cho role hệ thống
	identityPerms := slices.DeleteFunc(slices.Clone(perms), func(p string) bool { return !strings.HasPrefix(p, "identity.") })
	if err != nil || !slices.Equal(identityPerms, []string{domain.PermAccountRead, domain.PermRoleRead}) {
		t.Errorf("permissions = %v, %v", perms, err)
	}
	// Không trùng dù hai role cùng cấp một quyền
	if len(slices.Compact(slices.Clone(perms))) != len(perms) {
		t.Errorf("duplicate permissions: %v", perms)
	}
	if n, _ := countEvents(t, r, contract.EventRolesAssigned, a.ID); n != 1 {
		t.Errorf("roles_assigned events = %d", n)
	}
}

func TestAccount_ListFilters(t *testing.T) {
	r := newRepos(t)
	tag := uuid.NewString()[:8]
	ctx := context.Background()
	for _, name := range []string{"Alpha " + tag, "Beta " + tag} {
		if _, err := r.accounts.Create(ctx, domain.NewAccount{
			Email: strings.ToLower(strings.ReplaceAll(name, " ", "-")) + "@storeit.test", Name: name, PasswordHash: "h",
		}); err != nil {
			t.Fatal(err)
		}
	}

	items, total, err := r.accounts.List(ctx, domain.AccountFilter{Query: tag, Limit: 1})
	if err != nil || total != 2 || len(items) != 1 || items[0].Name != "Alpha "+tag {
		t.Errorf("list = %v total %d, %v", items, total, err)
	}
	inactive := false
	if _, total, _ := r.accounts.List(ctx, domain.AccountFilter{Query: tag, Active: &inactive, Limit: 10}); total != 0 {
		t.Errorf("inactive filter total = %d, want 0", total)
	}
}

// Tự đổi mật khẩu: giữ phiên đang dùng, thu hồi phiên khác với lý do password_change
func TestAccount_SetPasswordRevokesOtherSessions(t *testing.T) {
	r := newRepos(t)
	ctx := context.Background()
	a := newAccount(t, r)
	start := func() uuid.UUID {
		fam, err := r.sessions.Start(ctx, domain.NewSession{
			AccountID: a.ID, TokenHash: hash(uuid.NewString()),
			ExpiresAt: time.Now().Add(time.Hour), AbsoluteExpiresAt: time.Now().Add(2 * time.Hour),
		})
		if err != nil {
			t.Fatal(err)
		}
		return fam
	}
	keep, other := start(), start()
	if err := r.accounts.SetPassword(ctx, a.ID, "$2a$04$changed", &keep); err != nil {
		t.Fatal(err)
	}
	if reason, _ := familyState(t, r, keep); reason != nil {
		t.Errorf("current session revoked: %v", *reason)
	}
	if reason, _ := familyState(t, r, other); reason == nil || *reason != string(domain.RevokePasswordChange) {
		t.Errorf("other session reason = %v, want password_change", reason)
	}
}
