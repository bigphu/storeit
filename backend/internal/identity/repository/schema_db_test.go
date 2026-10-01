package repository_test

import (
	"context"
	"slices"
	"testing"

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
			WHERE r.name = $1 ORDER BY rp.permission`, role)
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
