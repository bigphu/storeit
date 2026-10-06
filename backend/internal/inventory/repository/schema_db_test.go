package repository_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"storeit/internal/platform/database/dbtest"
)

// pgCode trả mã lỗi Postgres của err, rỗng nếu không phải lỗi Postgres
func pgCode(err error) string {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}

// schemaFixture tạo một loại có đủ năm kiểu thuộc tính và một tài sản thuộc loại đó
type schemaFixture struct {
	typeID, otherTypeID                      uuid.UUID
	text, number, date, boolean, sel, otherA uuid.UUID
	option, otherOption                      uuid.UUID
	assetID                                  uuid.UUID
}

func newSchemaFixture(t *testing.T, pool *pgxpool.Pool) schemaFixture {
	t.Helper()
	ctx := context.Background()
	f := schemaFixture{
		typeID: uuid.New(), otherTypeID: uuid.New(),
		text: uuid.New(), number: uuid.New(), date: uuid.New(), boolean: uuid.New(), sel: uuid.New(), otherA: uuid.New(),
		option: uuid.New(), otherOption: uuid.New(), assetID: uuid.New(),
	}
	suffix := strings.ToUpper(f.typeID.String()[:8])
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	exec(`INSERT INTO inventory.asset_types (id, code, name) VALUES ($1, $2, $3), ($4, $5, $6)`,
		f.typeID, "T"+suffix, "Type "+suffix, f.otherTypeID, "O"+suffix, "Other "+suffix)
	attr := `INSERT INTO inventory.asset_type_attributes (id, asset_type_id, key, label, data_type) VALUES ($1, $2, $3, $3, $4)`
	exec(attr, f.text, f.typeID, "serial", "text")
	exec(attr, f.number, f.typeID, "ram_gb", "number")
	exec(attr, f.date, f.typeID, "warranty_end", "date")
	exec(attr, f.boolean, f.typeID, "has_dock", "boolean")
	exec(attr, f.sel, f.typeID, "os", "select")
	exec(attr, f.otherA, f.otherTypeID, "os", "select")
	opt := `INSERT INTO inventory.asset_attribute_options (id, attribute_id, label) VALUES ($1, $2, $3)`
	exec(opt, f.option, f.sel, "Windows")
	exec(opt, f.otherOption, f.otherA, "Windows")
	exec(`INSERT INTO inventory.assets (id, tag, name, asset_type_id, status_id) VALUES ($1, $2, 'A', $3, $4)`,
		f.assetID, "S-"+suffix, f.typeID, uuid.MustParse("00000000-0000-7000-8000-000000000201"))
	return f
}

func TestSchema_Seeds(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()

	var system bool
	var archived *string
	if err := pool.QueryRow(ctx, `SELECT is_system, archived_at::text FROM inventory.asset_types WHERE code = 'GENERAL'`).
		Scan(&system, &archived); err != nil || !system || archived != nil {
		t.Errorf("GENERAL type: system=%v archived=%v err=%v", system, archived, err)
	}
	rows, err := pool.Query(ctx, `SELECT kind FROM inventory.asset_statuses WHERE is_default ORDER BY kind`)
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for rows.Next() {
		var k string
		_ = rows.Scan(&k)
		kinds = append(kinds, k)
	}
	if want := []string{"available", "in_use", "retired", "unavailable"}; !slices.Equal(kinds, want) {
		t.Errorf("default statuses = %v, want %v", kinds, want)
	}

	grants := func(role string) []string {
		t.Helper()
		rows, err := pool.Query(ctx, `
			SELECT rp.permission FROM identity.role_permissions rp
			JOIN identity.roles r ON r.id = rp.role_id
			WHERE r.name = $1 AND rp.permission LIKE 'inventory.%' ORDER BY 1`, role)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for rows.Next() {
			var p string
			_ = rows.Scan(&p)
			out = append(out, p)
		}
		return out
	}
	for role, want := range map[string][]string{
		"Administrator":      {"inventory.asset.export", "inventory.asset.manage", "inventory.asset.read", "inventory.export_profile.manage", "inventory.status.manage", "inventory.type.manage"},
		"Authorized Manager": {"inventory.asset.export", "inventory.asset.read", "inventory.export_profile.manage", "inventory.status.manage", "inventory.type.manage"},
		"Inventory Officer":  {"inventory.asset.export", "inventory.asset.manage", "inventory.asset.read"},
		"Employee":           {"inventory.asset.export", "inventory.asset.read"},
	} {
		if got := grants(role); !slices.Equal(got, want) {
			t.Errorf("%s inventory permissions = %v, want %v", role, got, want)
		}
	}
}

// Mọi bảo đảm ở mức DB của spec mục 1–2
func TestSchema_Guarantees(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	f := newSchemaFixture(t, pool)
	value := `INSERT INTO inventory.asset_attribute_values
		(asset_id, attribute_id, asset_type_id, data_type, value_text, value_number, value_option_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	cases := []struct {
		name string
		sql  string
		args []any
		want string // mã lỗi Postgres
	}{
		{"wrong column for type", value, []any{f.assetID, f.number, f.typeID, "number", "16", nil, nil}, "23514"},
		{"two columns filled", value, []any{f.assetID, f.text, f.typeID, "text", "x", 1, nil}, "23514"},
		{"attribute of another type", value, []any{f.assetID, f.otherA, f.typeID, "select", nil, nil, f.otherOption}, "23503"},
		{"option of another attribute", value, []any{f.assetID, f.sel, f.typeID, "select", nil, nil, f.otherOption}, "23503"},
		{"blank text", value, []any{f.assetID, f.text, f.typeID, "text", "  ", nil, nil}, "23514"},
		{"unit on text attribute",
			`INSERT INTO inventory.asset_type_attributes (id, asset_type_id, key, label, data_type, unit) VALUES ($1, $2, 'u_text', 'U', 'text', 'GB')`,
			[]any{uuid.New(), f.typeID}, "23514"},
		{"option for non-select attribute",
			`INSERT INTO inventory.asset_attribute_options (id, attribute_id, label) VALUES ($1, $2, 'X')`,
			[]any{uuid.New(), f.text}, "23503"},
		{"duplicate tag",
			`INSERT INTO inventory.assets (id, tag, name, asset_type_id, status_id) SELECT $1, tag, 'B', asset_type_id, status_id FROM inventory.assets WHERE id = $2`,
			[]any{uuid.New(), f.assetID}, "23505"},
		{"lowercase tag",
			`INSERT INTO inventory.assets (id, tag, name, asset_type_id, status_id) VALUES ($1, 'lap-1', 'B', $2, '00000000-0000-7000-8000-000000000201')`,
			[]any{uuid.New(), f.typeID}, "23514"},
		{"second default of a kind",
			`INSERT INTO inventory.asset_statuses (id, name, kind, is_default) VALUES ($1, $2, 'available', true)`,
			[]any{uuid.New(), "Ready " + f.typeID.String()[:8]}, "23505"},
		{"archived default",
			`UPDATE inventory.asset_statuses SET archived_at = now() WHERE id = '00000000-0000-7000-8000-000000000201'`,
			nil, "23514"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := pool.Exec(ctx, tc.sql, tc.args...)
			if got := pgCode(err); got != tc.want {
				t.Errorf("err = %v (code %q), want code %s", err, got, tc.want)
			}
		})
	}

	// Đổi kiểu dữ liệu của thuộc tính khi đã có giá trị bị khoá ngoại chặn
	if _, err := pool.Exec(ctx, value, f.assetID, f.number, f.typeID, "number", nil, 16, nil); err != nil {
		t.Fatalf("valid number value: %v", err)
	}
	_, err := pool.Exec(ctx, `UPDATE inventory.asset_type_attributes SET data_type = 'text' WHERE id = $1`, f.number)
	if pgCode(err) != "23503" {
		t.Errorf("change data type with a value: %v, want FK violation", err)
	}
	// Đổi loại của tài sản khi còn giá trị cũng bị chặn
	_, err = pool.Exec(ctx, `UPDATE inventory.assets SET asset_type_id = $2 WHERE id = $1`, f.assetID, f.otherTypeID)
	if pgCode(err) != "23503" {
		t.Errorf("change asset type with values: %v, want FK violation", err)
	}
}

func TestSchema_ExportGrants(t *testing.T) {
	r := newRepos(t)
	for _, g := range []struct{ role, perm string }{
		{"00000000-0000-7000-8000-000000000004", "inventory.asset.export"},
		{"00000000-0000-7000-8000-000000000001", "inventory.export_profile.manage"},
		{"00000000-0000-7000-8000-000000000002", "inventory.export_profile.manage"},
	} {
		var n int
		if err := r.pool.QueryRow(context.Background(),
			`SELECT count(*) FROM identity.role_permissions WHERE role_id = $1 AND permission = $2`, g.role, g.perm).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Errorf("role %s lacks %s", g.role, g.perm)
		}
	}
}
