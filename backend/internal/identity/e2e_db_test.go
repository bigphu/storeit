package identity_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"storeit/internal/identity"
	"storeit/internal/identity/domain"
	"storeit/internal/platform/database/dbtest"
	"storeit/internal/platform/events"
	"storeit/internal/platform/jobs"
	"storeit/internal/platform/jwt"
	"storeit/internal/platform/server"
)

// Luồng đầy đủ như cmd/server: bootstrap admin → đăng nhập → /me → tạo account
// Employee → Employee bị 403 → được gán role mới → refresh thì có quyền mới →
// đăng xuất → cookie cũ không refresh được nữa.
func TestEndToEnd(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	// Bootstrap chỉ chạy khi chưa có account nào; DB test dùng chung trong package
	if _, err := pool.Exec(ctx, `TRUNCATE identity.accounts CASCADE`); err != nil {
		t.Fatal(err)
	}

	client, err := jobs.NewInsertClient(pool, nil)
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := jwt.New(jwt.Config{
		Keys:      "v1:" + base64.StdEncoding.EncodeToString([]byte(strings.Repeat("e", 32))),
		ActiveKID: "v1", Issuer: "storeit", Audience: "storeit-api", AccessTokenTTL: 15 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	m, err := identity.New(identity.Deps{
		Pool: pool, Tokens: tokens, Outbox: events.NewOutbox(events.NewRegistry(), client),
		Config: identity.Config{AdminEmail: "Root@StoreIT.test", AdminPassword: password},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}
	srv := server.New(server.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := m.Mount(srv.Router()); err != nil {
		t.Fatal(err)
	}
	a := &app{t: t, h: srv.Handler()}

	// Admin đầu tiên đăng nhập, /me có quyền quản trị
	admin, _ := a.login("root@storeit.test")
	rec := a.do(call{method: "GET", path: "/api/v1/me", token: admin})
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), domain.PermRoleManage) {
		t.Fatalf("admin /me: %d %s", rec.Code, rec.Body)
	}

	// Tạo account Employee
	rec = a.do(call{method: "POST", path: "/api/v1/accounts", token: admin, body: map[string]any{
		"email": "lan@storeit.test", "name": "Lan", "password": password,
		"role_ids": []string{domain.EmployeeRoleID.String()},
	}})
	var created struct{ Id string }
	if rec.Code != 201 || json.Unmarshal(rec.Body.Bytes(), &created) != nil {
		t.Fatalf("create employee: %d %s", rec.Code, rec.Body)
	}

	// Employee đăng nhập, không xem được danh sách account
	employee, cookie := a.login("lan@storeit.test")
	if rec := a.do(call{method: "GET", path: "/api/v1/accounts", token: employee}); rec.Code != 403 {
		t.Fatalf("employee lists accounts: %d, want 403", rec.Code)
	}

	// Admin gán thêm Authorized Manager; quyền mới có ở lần refresh kế tiếp
	rec = a.do(call{method: "PUT", path: "/api/v1/accounts/" + created.Id + "/roles", token: admin, body: map[string]any{
		"role_ids": []string{domain.EmployeeRoleID.String(), domain.AuthorizedManagerRoleID.String()},
	}})
	if rec.Code != 200 {
		t.Fatalf("assign roles: %d %s", rec.Code, rec.Body)
	}
	rec = a.do(call{method: "POST", path: "/api/v1/auth/refresh", cookie: cookie})
	if rec.Code != 200 {
		t.Fatalf("refresh: %d %s", rec.Code, rec.Body)
	}
	var s session
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	rotated := refreshCookie(t, rec)
	if rotated.Value == cookie.Value {
		t.Error("refresh did not rotate the cookie")
	}
	if rec := a.do(call{method: "GET", path: "/api/v1/accounts", token: s.AccessToken}); rec.Code != 200 {
		t.Errorf("after refresh with new role: %d, want 200", rec.Code)
	}

	// Đăng xuất: cookie (cả bản cũ lẫn bản mới) không refresh được nữa
	if rec := a.do(call{method: "POST", path: "/api/v1/auth/logout", cookie: rotated}); rec.Code != 204 {
		t.Fatalf("logout: %d", rec.Code)
	}
	for _, c := range []*http.Cookie{rotated, cookie} {
		rec := a.do(call{method: "POST", path: "/api/v1/auth/refresh", cookie: c})
		if rec.Code != 401 {
			t.Errorf("refresh after logout: %d, want 401", rec.Code)
		}
	}
}
