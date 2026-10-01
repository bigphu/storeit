package service

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"storeit/internal/identity/domain"
	"storeit/internal/platform/auth"
	"storeit/internal/platform/errs"
	"storeit/internal/platform/jobs"
)

const goodPassword = "correct-horse-battery"

type env struct {
	svc      *Service
	accounts *fakeAccounts
	roles    *fakeRoles
	sessions *fakeSessions
	tokens   *fakeTokens
	hasher   *countingHasher
	pwTokens *fakePasswordTokens
	mail     *fakeMail
	now      time.Time
}

func newEnv(t *testing.T) *env {
	t.Helper()
	roles := newFakeRoles()
	e := &env{
		accounts: newFakeAccounts(roles),
		roles:    roles,
		sessions: &fakeSessions{},
		tokens:   &fakeTokens{},
		hasher:   &countingHasher{Hasher: NewBcrypt(4)},
		now:      time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC),
	}
	now := func() time.Time { return e.now }
	e.pwTokens = newFakePasswordTokens(e.accounts, now)
	e.mail = &fakeMail{}
	e.svc = New(Deps{
		Accounts: e.accounts, Roles: e.roles, Sessions: e.sessions,
		Hasher: e.hasher, Tokens: e.tokens, PasswordTokens: e.pwTokens,
		Mail: e.mail, AppURL: "http://app.test/",
		Settings: Settings{
			SlidingTTL: 14 * 24 * time.Hour, AbsoluteTTL: 30 * 24 * time.Hour, Grace: 30 * time.Second, Retention: 30 * 24 * time.Hour,
			InviteTTL: 72 * time.Hour, ResetTTL: time.Hour,
		},
		Now: now,
	})
	return e
}

// seed tạo account trực tiếp trong fake với mật khẩu goodPassword
func (e *env) seed(t *testing.T, email string, active bool, roleIDs ...uuid.UUID) domain.Account {
	t.Helper()
	h, err := e.hasher.Hash(goodPassword)
	if err != nil {
		t.Fatal(err)
	}
	a, err := e.accounts.Create(context.Background(), domain.NewAccount{Email: email, Name: email, PasswordHash: h, RoleIDs: roleIDs})
	if err != nil {
		t.Fatal(err)
	}
	if !active {
		a, _ = e.accounts.SetActive(context.Background(), a.ID, false)
	}
	return a
}

func as(id uuid.UUID, perms ...string) context.Context {
	return auth.WithActor(context.Background(), auth.Actor{AccountID: id, Permissions: perms})
}

func status(err error) int {
	var e *errs.Error
	if errors.As(err, &e) {
		return e.Status()
	}
	return 0
}

func TestLogin(t *testing.T) {
	e := newEnv(t)
	a := e.seed(t, "lan@storeit.test", true, domain.AdministratorRoleID)
	e.seed(t, "off@storeit.test", false)
	if _, err := e.accounts.Create(context.Background(), domain.NewAccount{Email: "new@storeit.test", Name: "New"}); err != nil {
		t.Fatal(err)
	}
	dev := Device{UserAgent: "test", IP: "203.0.113.5"}

	sess, err := e.svc.Login(context.Background(), "  LAN@StoreIT.test ", goodPassword, dev)
	if err != nil {
		t.Fatal(err)
	}
	if sess.AccessToken == "" || sess.RefreshToken == "" || sess.Account.ID != a.ID {
		t.Errorf("session = %+v", sess)
	}
	if !sess.RefreshExpiresAt.Equal(e.now.Add(14 * 24 * time.Hour)) {
		t.Errorf("refresh expiry = %v", sess.RefreshExpiresAt)
	}
	started := e.sessions.started[0]
	if started.AccountID != a.ID || !started.AbsoluteExpiresAt.Equal(e.now.Add(30*24*time.Hour)) || started.IP != dev.IP {
		t.Errorf("session started = %+v", started)
	}
	// Token chỉ lưu hash, không phải token thô
	if string(started.TokenHash) == sess.RefreshToken || len(started.TokenHash) != 32 {
		t.Errorf("stored token hash looks wrong: %d bytes", len(started.TokenHash))
	}
	if got := e.tokens.perms[0]; !slices.Contains(got, domain.PermRoleManage) {
		t.Errorf("access token permissions = %v", got)
	}

	for name, tc := range map[string]struct {
		email, pw string
		want      error
	}{
		"wrong password": {"lan@storeit.test", "wrong-password-xx", domain.ErrBadCredentials},
		"unknown email":  {"ghost@storeit.test", goodPassword, domain.ErrBadCredentials},
		"malformed":      {"not-an-email", goodPassword, domain.ErrBadCredentials},
		"disabled":       {"off@storeit.test", goodPassword, domain.ErrAccountDisabled},
		// Được mời, chưa đặt mật khẩu: giống email lạ
		"invited": {"new@storeit.test", goodPassword, domain.ErrBadCredentials},
	} {
		before := e.hasher.compares
		_, err := e.svc.Login(context.Background(), tc.email, tc.pw, dev)
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
		}
		// Email lạ vẫn so bcrypt một lần, để thời gian phản hồi không lộ email nào tồn tại
		if e.hasher.compares != before+1 {
			t.Errorf("%s: bcrypt compares = %d, want 1", name, e.hasher.compares-before)
		}
	}
}

func TestRefresh(t *testing.T) {
	e := newEnv(t)
	a := e.seed(t, "lan@storeit.test", true, domain.EmployeeRoleID)

	for _, d := range []domain.RefreshDecision{domain.RefreshRotate, domain.RefreshGrace} {
		e.sessions.refresh = domain.RefreshResult{Decision: d, AccountID: a.ID, ExpiresAt: e.now.Add(time.Hour)}
		sess, err := e.svc.Refresh(context.Background(), "raw-token")
		if err != nil || sess.RefreshToken == "" || sess.RefreshToken == "raw-token" || sess.Account.ID != a.ID {
			t.Errorf("%v: session = %+v, %v", d, sess, err)
		}
	}
	in := e.sessions.inputs[0]
	if string(in.TokenHash) == "raw-token" || len(in.NextHash) != 32 || in.Grace != 30*time.Second {
		t.Errorf("refresh input = %+v", in)
	}

	// Quyền được nạp lại mỗi lần refresh: đổi role thì token mới có quyền mới
	if _, err := e.accounts.ReplaceRoles(context.Background(), a.ID, []uuid.UUID{domain.AdministratorRoleID}); err != nil {
		t.Fatal(err)
	}
	e.sessions.refresh = domain.RefreshResult{Decision: domain.RefreshRotate, AccountID: a.ID}
	if _, err := e.svc.Refresh(context.Background(), "raw-token"); err != nil {
		t.Fatal(err)
	}
	if last := e.tokens.perms[len(e.tokens.perms)-1]; !slices.Contains(last, domain.PermRoleManage) {
		t.Errorf("permissions after role change = %v", last)
	}

	for _, d := range []domain.RefreshDecision{domain.RefreshReject, domain.RefreshExpired, domain.RefreshReuse, domain.RefreshAccountDisabled} {
		e.sessions.refresh = domain.RefreshResult{Decision: d, AccountID: a.ID}
		if _, err := e.svc.Refresh(context.Background(), "raw-token"); !errors.Is(err, domain.ErrInvalidRefreshToken) {
			t.Errorf("%v: err = %v, want ErrInvalidRefreshToken", d, err)
		}
	}
	if _, err := e.svc.Refresh(context.Background(), ""); !errors.Is(err, domain.ErrInvalidRefreshToken) {
		t.Errorf("missing cookie: %v", err)
	}
}

func TestLogoutIdempotent(t *testing.T) {
	e := newEnv(t)
	if err := e.svc.Logout(context.Background(), ""); err != nil {
		t.Errorf("no cookie: %v", err)
	}
	if err := e.svc.Logout(context.Background(), "unknown"); err != nil {
		t.Errorf("unknown token: %v", err)
	}
	if len(e.sessions.revoked) != 1 {
		t.Errorf("revoke calls = %d, want 1 (none for empty cookie)", len(e.sessions.revoked))
	}
}

func TestPermissionChecks(t *testing.T) {
	e := newEnv(t)
	target := e.seed(t, "t@storeit.test", true)
	id := target.ID
	calls := map[string]func(ctx context.Context) error{
		"ListAccounts": func(ctx context.Context) error {
			_, _, err := e.svc.ListAccounts(ctx, domain.AccountFilter{})
			return err
		},
		"GetAccount": func(ctx context.Context) error { _, err := e.svc.GetAccount(ctx, id); return err },
		"CreateAccount": func(ctx context.Context) error {
			_, err := e.svc.CreateAccount(ctx, CreateAccountInput{Email: "n@storeit.test", Name: "N"})
			return err
		},
		"UpdateAccount": func(ctx context.Context) error {
			_, err := e.svc.UpdateAccount(ctx, id, domain.ProfileChange{})
			return err
		},
		"DisableAccount":    func(ctx context.Context) error { _, err := e.svc.DisableAccount(ctx, id); return err },
		"EnableAccount":     func(ctx context.Context) error { _, err := e.svc.EnableAccount(ctx, id); return err },
		"ResendInvitation":  func(ctx context.Context) error { return e.svc.ResendInvitation(ctx, id) },
		"SendPasswordReset": func(ctx context.Context) error { return e.svc.SendPasswordReset(ctx, id) },
		"AssignRoles":       func(ctx context.Context) error { _, err := e.svc.AssignRoles(ctx, id, nil); return err },
		"ListRoles":         func(ctx context.Context) error { _, err := e.svc.ListRoles(ctx); return err },
		"GetRole":           func(ctx context.Context) error { _, err := e.svc.GetRole(ctx, domain.EmployeeRoleID); return err },
		"CreateRole":        func(ctx context.Context) error { _, err := e.svc.CreateRole(ctx, "X", "", nil); return err },
		"UpdateRole": func(ctx context.Context) error {
			_, err := e.svc.UpdateRole(ctx, domain.EmployeeRoleID, nil, nil)
			return err
		},
		"UpdateRolePermissions": func(ctx context.Context) error {
			_, err := e.svc.UpdateRolePermissions(ctx, domain.EmployeeRoleID, nil)
			return err
		},
		"DeleteRole":      func(ctx context.Context) error { return e.svc.DeleteRole(ctx, domain.EmployeeRoleID) },
		"ListPermissions": func(ctx context.Context) error { _, err := e.svc.ListPermissions(ctx); return err },
	}
	for name, call := range calls {
		if err := call(context.Background()); status(err) != 401 {
			t.Errorf("%s without actor: %v, want 401", name, err)
		}
		if err := call(as(uuid.New())); status(err) != 403 {
			t.Errorf("%s without permission: %v, want 403", name, err)
		}
	}
}

func TestLockoutGuards(t *testing.T) {
	e := newEnv(t)
	admin := e.seed(t, "admin@storeit.test", true, domain.AdministratorRoleID)
	ctx := as(admin.ID, domain.PermAccountManage, domain.PermRoleManage)

	if _, err := e.svc.DisableAccount(ctx, admin.ID); !errors.Is(err, domain.ErrLockout) {
		t.Errorf("disable self: %v", err)
	}
	if _, err := e.svc.AssignRoles(ctx, admin.ID, []uuid.UUID{domain.EmployeeRoleID}); !errors.Is(err, domain.ErrLockout) {
		t.Errorf("drop own Administrator: %v", err)
	}
	if _, err := e.svc.UpdateRolePermissions(ctx, domain.AdministratorRoleID, []string{domain.PermRoleRead}); !errors.Is(err, domain.ErrLockout) {
		t.Errorf("strip role.manage from Administrator: %v", err)
	}
	if _, err := e.svc.UpdateRole(ctx, domain.AdministratorRoleID, ptr("Boss"), nil); !errors.Is(err, domain.ErrSystemRole) {
		t.Errorf("rename system role: %v", err)
	}
	if err := e.svc.DeleteRole(ctx, domain.EmployeeRoleID); !errors.Is(err, domain.ErrSystemRole) {
		t.Errorf("delete system role: %v", err)
	}
	// Gán thêm role cho chính mình mà vẫn giữ Administrator thì được
	if _, err := e.svc.AssignRoles(ctx, admin.ID, []uuid.UUID{domain.AdministratorRoleID, domain.EmployeeRoleID}); err != nil {
		t.Errorf("keep Administrator: %v", err)
	}
}

func TestAccountManagement(t *testing.T) {
	e := newEnv(t)
	admin := e.seed(t, "admin@storeit.test", true, domain.AdministratorRoleID)
	ctx := as(admin.ID, domain.PermAccountManage, domain.PermAccountRead)

	v, err := e.svc.CreateAccount(ctx, CreateAccountInput{
		Email: " Minh@StoreIT.test", Name: "  Minh  ", RoleIDs: []uuid.UUID{domain.EmployeeRoleID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if v.Email != "minh@storeit.test" || v.Name != "Minh" || len(v.Roles) != 1 {
		t.Errorf("created = %+v", v)
	}
	if v.Status() != domain.StatusInvited || v.PasswordHash != "" {
		t.Errorf("created account status = %s, want invited without password", v.Status())
	}

	for name, in := range map[string]CreateAccountInput{
		"bad email":  {Email: "nope", Name: "A"},
		"blank name": {Email: "b@storeit.test", Name: "   "},
	} {
		if _, err := e.svc.CreateAccount(ctx, in); status(err) != 422 {
			t.Errorf("%s: %v, want 422", name, err)
		}
	}

}

func TestChangePassword(t *testing.T) {
	e := newEnv(t)
	a := e.seed(t, "lan@storeit.test", true)
	family := uuid.New()
	e.sessions.familyOf = map[string]uuid.UUID{string(hashSecret("current-cookie")): family}
	ctx := as(a.ID)

	if err := e.svc.ChangePassword(ctx, "wrong-password-xx", "brand-new-password", "current-cookie"); !errors.Is(err, domain.ErrWrongPassword) {
		t.Errorf("wrong current: %v", err)
	}
	if err := e.svc.ChangePassword(ctx, goodPassword, "short", "current-cookie"); !errors.Is(err, domain.ErrWeakPassword) {
		t.Errorf("weak new: %v", err)
	}
	if err := e.svc.ChangePassword(ctx, goodPassword, "brand-new-password", "current-cookie"); err != nil {
		t.Fatal(err)
	}
	// Giữ phiên đang dùng, thu hồi phiên khác, không event (tự đổi)
	last := e.accounts.passwords[len(e.accounts.passwords)-1]
	if last.keep == nil || *last.keep != family {
		t.Errorf("change call = %+v", last)
	}
	if _, err := e.svc.Login(context.Background(), "lan@storeit.test", "brand-new-password", Device{}); err != nil {
		t.Errorf("login with new password: %v", err)
	}
}

func TestMe(t *testing.T) {
	e := newEnv(t)
	a := e.seed(t, "lan@storeit.test", true, domain.AdministratorRoleID)

	me, err := e.svc.Me(as(a.ID))
	if err != nil || me.Account.ID != a.ID || len(me.Roles) != 1 || !slices.Contains(me.Permissions, domain.PermRoleManage) {
		t.Errorf("me = %+v, %v", me, err)
	}
	if _, err := e.svc.Me(context.Background()); status(err) != 401 {
		t.Errorf("me without actor: %v", err)
	}
}

func TestBootstrap(t *testing.T) {
	e := newEnv(t)
	created, err := e.svc.Bootstrap(context.Background(), "Root@StoreIT.test", goodPassword)
	if err != nil || !created {
		t.Fatalf("first bootstrap = %v, %v", created, err)
	}
	a, err := e.accounts.GetByEmail(context.Background(), "root@storeit.test")
	if err != nil {
		t.Fatal(err)
	}
	if perms, _ := e.accounts.Permissions(context.Background(), a.ID); !slices.Contains(perms, domain.PermRoleManage) {
		t.Errorf("bootstrap admin permissions = %v", perms)
	}
	// Đã có account: không tạo thêm
	if created, err := e.svc.Bootstrap(context.Background(), "other@storeit.test", goodPassword); err != nil || created {
		t.Errorf("second bootstrap = %v, %v", created, err)
	}
}

func TestLoadActorAndReader(t *testing.T) {
	e := newEnv(t)
	a := e.seed(t, "lan@storeit.test", true, domain.AdministratorRoleID)
	off := e.seed(t, "off@storeit.test", false, domain.AdministratorRoleID)

	actor, err := e.svc.LoadActor(context.Background(), a.ID)
	if err != nil || actor.AccountID != a.ID || !actor.Can(domain.PermRoleManage) {
		t.Errorf("actor = %+v, %v", actor, err)
	}
	// Account bị khoá: job chạy dưới tên nó không còn quyền gì
	if actor, err := e.svc.LoadActor(context.Background(), off.ID); err != nil || len(actor.Permissions) != 0 {
		t.Errorf("disabled actor = %+v, %v", actor, err)
	}
	if _, err := e.svc.LoadActor(context.Background(), uuid.New()); !errors.Is(err, jobs.ErrActorNotFound) {
		t.Errorf("missing actor: %v, want jobs.ErrActorNotFound", err)
	}

	reader := e.svc.AccountReader()
	if acc, err := reader.GetAccount(context.Background(), a.ID); err != nil || acc.Email != a.Email {
		t.Errorf("reader = %+v, %v", acc, err)
	}
	if _, err := reader.GetAccount(context.Background(), uuid.New()); status(err) != 404 {
		t.Errorf("reader missing: %v", err)
	}
	if many, err := reader.GetAccounts(context.Background(), []uuid.UUID{a.ID, uuid.New()}); err != nil || len(many) != 1 {
		t.Errorf("reader many = %v, %v", many, err)
	}
}

func ptr[T any](v T) *T { return &v }

// Dọn rác: mốc cắt là now - retention, tính một lần ở service
func TestPruneSessions(t *testing.T) {
	e := newEnv(t)
	res, err := e.svc.PruneSessions(context.Background())
	if err != nil || res.Families != 2 || res.Tokens != 5 || res.PasswordTokens != 3 {
		t.Errorf("prune = %+v, %v", res, err)
	}
	// Link hết hạn là rác ngay, không cần chờ Retention
	if !e.pwTokens.pruned.Equal(e.now) {
		t.Errorf("password token cutoff = %v, want now %v", e.pwTokens.pruned, e.now)
	}
	if want := e.now.Add(-30 * 24 * time.Hour); !e.sessions.pruneCutoff.Equal(want) {
		t.Errorf("cutoff = %v, want %v", e.sessions.pruneCutoff, want)
	}
}

// Hai replica khởi động cùng lúc trên DB trống: cả hai thấy 0 account, replica
// thứ hai tạo trùng email. Không được coi là lỗi khởi động.
func TestBootstrap_ConcurrentStart(t *testing.T) {
	e := newEnv(t)
	if _, err := e.svc.Bootstrap(context.Background(), "root@storeit.test", goodPassword); err != nil {
		t.Fatal(err)
	}
	e.accounts.staleCount = true

	created, err := e.svc.Bootstrap(context.Background(), "root@storeit.test", goodPassword)
	if err != nil || created {
		t.Errorf("second replica bootstrap = %v, %v, want no-op", created, err)
	}
}
