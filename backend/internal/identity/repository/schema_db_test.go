package repository_test

import (
	"context"
	"slices"
	"testing"

	"github.com/google/uuid"

	"storeit/internal/platform/database/dbtest"
)

// Role hệ thống và ma trận quyền identity được seed bởi migration
func TestSchema_SeedRoles(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()

	rows, err := pool.Query(ctx, `SELECT name FROM identity.roles WHERE is_system ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		names = append(names, n)
	}
	want := []string{"Administrator", "Authorized Manager", "Employee", "Inventory Officer"}
	if !slices.Equal(names, want) {
		t.Errorf("system roles = %v, want %v", names, want)
	}

	perms := func(role string) []string {
		t.Helper()
		rows, err := pool.Query(ctx, `
			SELECT rp.permission FROM identity.role_permissions rp
			JOIN identity.roles r ON r.id = rp.role_id
			WHERE r.name = $1 AND rp.permission LIKE 'identity.%' ORDER BY rp.permission`, role)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for rows.Next() {
			var p string
			if err := rows.Scan(&p); err != nil {
				t.Fatal(err)
			}
			out = append(out, p)
		}
		return out
	}
	if got := perms("Administrator"); !slices.Equal(got, []string{
		"identity.account.manage", "identity.account.read", "identity.role.manage", "identity.role.read",
	}) {
		t.Errorf("Administrator permissions = %v", got)
	}
	if got := perms("Authorized Manager"); !slices.Equal(got, []string{"identity.account.read", "identity.role.read"}) {
		t.Errorf("Authorized Manager permissions = %v", got)
	}
	if got := perms("Employee"); len(got) != 0 {
		t.Errorf("Employee permissions = %v, want none", got)
	}
}

// password_tokens: một hàng mỗi (account, purpose), purpose chỉ invite/reset,
// hash duy nhất, xoá theo account. Mật khẩu được phép NULL (chưa nhận lời mời).
func TestSchema_PasswordTokens(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	id := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO identity.accounts (id, email, name, password_hash)
		VALUES ($1, $2, 'Invited', NULL)`, id, "inv-"+id.String()[:8]+"@storeit.test"); err != nil {
		t.Fatalf("account without password: %v", err)
	}
	insert := func(purpose string, h []byte) error {
		_, err := pool.Exec(ctx, `
			INSERT INTO identity.password_tokens (account_id, purpose, id, token_hash, expires_at)
			VALUES ($1, $2, $3, $4, now() + interval '1 hour')`, id, purpose, uuid.New(), h)
		return err
	}
	if err := insert("invite", hash(id.String()+"a")); err != nil {
		t.Fatalf("insert invite: %v", err)
	}
	if err := insert("invite", hash(id.String()+"b")); err == nil {
		t.Error("second invite row for the same account accepted")
	}
	if err := insert("reset", hash(id.String()+"a")); err == nil {
		t.Error("duplicate token_hash accepted")
	}
	if err := insert("other", hash(id.String()+"c")); err == nil {
		t.Error("purpose 'other' accepted")
	}
	if _, err := pool.Exec(ctx, `DELETE FROM identity.accounts WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM identity.password_tokens WHERE account_id = $1`, id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("%d tokens left after deleting the account", n)
	}
}
