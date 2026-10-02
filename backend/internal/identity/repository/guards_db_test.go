package repository_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"storeit/internal/identity/domain"
	"storeit/internal/identity/repository/db"
)

// onlyAdmins xoá mọi account (DB test dùng chung) rồi tạo n Administrator đang
// hoạt động, để đếm "admin còn lại" có nghĩa
func onlyAdmins(t *testing.T, r repos, n int) []domain.Account {
	t.Helper()
	if _, err := r.pool.Exec(context.Background(), `TRUNCATE identity.accounts CASCADE`); err != nil {
		t.Fatal(err)
	}
	out := make([]domain.Account, n)
	for i := range out {
		out[i] = newAccount(t, r, domain.AdministratorRoleID)
	}
	return out
}

func activeAdmins(t *testing.T, r repos) int {
	t.Helper()
	var n int
	if err := r.pool.QueryRow(context.Background(), `
		SELECT count(*) FROM identity.accounts a
		JOIN identity.account_roles ar ON ar.account_id = a.id
		WHERE ar.role_id = $1 AND a.active`, domain.AdministratorRoleID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Hai admin khoá lẫn nhau cùng lúc: đúng một bên thành công, luôn còn một admin
func TestGuard_LastAdminConcurrentDisable(t *testing.T) {
	r := newRepos(t)
	for round := range 10 {
		admins := onlyAdmins(t, r, 2)
		var wg sync.WaitGroup
		errs := make([]error, 2)
		for i, a := range admins {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, errs[i] = r.accounts.SetActive(context.Background(), a.ID, false)
			}()
		}
		wg.Wait()
		lockouts := 0
		for _, err := range errs {
			switch {
			case errors.Is(err, domain.ErrLockout):
				lockouts++
			case err != nil:
				t.Fatalf("round %d: %v", round, err)
			}
		}
		if lockouts != 1 || activeAdmins(t, r) != 1 {
			t.Fatalf("round %d: %d lockouts, %d active admins, want 1 and 1", round, lockouts, activeAdmins(t, r))
		}
	}
}

// Hai admin gỡ role Administrator của nhau cùng lúc
func TestGuard_LastAdminConcurrentRoleRemoval(t *testing.T) {
	r := newRepos(t)
	for round := range 10 {
		admins := onlyAdmins(t, r, 2)
		var wg sync.WaitGroup
		errs := make([]error, 2)
		for i, a := range admins {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, errs[i] = r.accounts.ReplaceRoles(context.Background(), a.ID, []uuid.UUID{domain.EmployeeRoleID})
			}()
		}
		wg.Wait()
		ok := 0
		for _, err := range errs {
			if err == nil {
				ok++
			} else if !errors.Is(err, domain.ErrLockout) {
				t.Fatalf("round %d: %v", round, err)
			}
		}
		if ok != 1 || activeAdmins(t, r) != 1 {
			t.Fatalf("round %d: %d succeeded, %d active admins, want 1 and 1", round, ok, activeAdmins(t, r))
		}
	}
}

func TestGuard_LastAdminSequential(t *testing.T) {
	r := newRepos(t)
	ctx := context.Background()
	last := onlyAdmins(t, r, 1)[0]
	if _, err := r.accounts.SetActive(ctx, last.ID, false); !errors.Is(err, domain.ErrLockout) {
		t.Errorf("disable the last admin: %v, want ErrLockout", err)
	}
	if _, err := r.accounts.ReplaceRoles(ctx, last.ID, []uuid.UUID{domain.EmployeeRoleID}); !errors.Is(err, domain.ErrLockout) {
		t.Errorf("demote the last admin: %v, want ErrLockout", err)
	}
	// Admin vẫn giữ Administrator thì đổi role khác thoải mái
	if _, err := r.accounts.ReplaceRoles(ctx, last.ID, []uuid.UUID{domain.AdministratorRoleID, domain.EmployeeRoleID}); err != nil {
		t.Errorf("add a role to the last admin: %v", err)
	}
	// Account thường khoá được dù không còn admin nào khác (luật chỉ chặn việc làm mất admin)
	emp := newAccount(t, r, domain.EmployeeRoleID)
	if _, err := r.accounts.SetActive(ctx, emp.ID, false); err != nil {
		t.Errorf("disable an employee: %v", err)
	}
}

// % và _ trong ô tìm kiếm là chữ, không phải ký tự đại diện của ILIKE
func TestAccount_SearchEscapesWildcards(t *testing.T) {
	r := newRepos(t)
	ctx := context.Background()
	tag := uuid.NewString()[:8]
	create := func(name string) {
		if _, err := r.accounts.Create(ctx, domain.NewAccount{
			Email: "s-" + uuid.NewString()[:8] + "@storeit.test", Name: name, PasswordHash: "h",
		}); err != nil {
			t.Fatal(err)
		}
	}
	create("Sale 50% " + tag)
	create("Sale 500 " + tag)
	create("u_" + tag)
	create("uX" + tag)
	create(`back\slash ` + tag)
	for q, want := range map[string]int{"50% " + tag: 1, "u_" + tag: 1, `back\slash ` + tag: 1, tag: 5} {
		got, total, err := r.accounts.List(ctx, domain.AccountFilter{Query: q, Limit: 50})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != want || total != int64(want) {
			t.Errorf("search %q: %d items (total %d), want %d", q, len(got), total, want)
		}
	}
}

// Khoá hàng account để đổi trạng thái không được chặn người đó đăng nhập (FK
// từ refresh_families lấy KEY SHARE; FOR UPDATE xung đột với nó, FOR NO KEY
// UPDATE thì không)
func TestAccount_RowLockDoesNotBlockLogin(t *testing.T) {
	r := newRepos(t)
	ctx := context.Background()
	a := newAccount(t, r)
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := db.New(tx).GetAccountForUpdate(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	sctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if _, err := r.sessions.Start(sctx, domain.NewSession{
		AccountID: a.ID, TokenHash: hash(uuid.NewString()),
		ExpiresAt: time.Now().Add(time.Hour), AbsoluteExpiresAt: time.Now().Add(2 * time.Hour),
	}); err != nil {
		t.Errorf("login while the account row is locked: %v", err)
	}
}
