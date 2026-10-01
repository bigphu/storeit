package identity_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/riverqueue/river"

	"storeit/internal/identity"
	"storeit/internal/identity/domain"
	"storeit/internal/identity/job"
	"storeit/internal/identity/worker"
	"storeit/internal/platform/database/dbtest"
	"storeit/internal/platform/events"
	"storeit/internal/platform/jobs"
	"storeit/internal/platform/jwt"
	"storeit/internal/platform/mail"
	"storeit/internal/platform/server"
)

// inbox là mail.Sender giữ thư lại cho test đọc
type inbox struct {
	mu   sync.Mutex
	msgs []mail.Message
}

func (b *inbox) Send(_ context.Context, m mail.Message) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.msgs = append(b.msgs, m)
	return nil
}

var linkToken = regexp.MustCompile(`#token=([A-Za-z0-9_-]+)`)

// deliver chạy job gửi thư mới nhất cho email như worker làm, rồi trả link
// trong thư (người dùng mở hộp thư)
func deliver(t *testing.T, m *identity.Module, box *inbox, email string) (link, token string) {
	t.Helper()
	var raw []byte
	err := dbtest.Pool(t).QueryRow(context.Background(), `
		SELECT j.args FROM river_job j
		JOIN identity.password_tokens p ON p.id = (j.args->>'token_id')::uuid
		JOIN identity.accounts a ON a.id = p.account_id
		WHERE a.email = $1 ORDER BY j.id DESC LIMIT 1`, email).Scan(&raw)
	if err != nil {
		t.Fatalf("no email job for %s: %v", email, err)
	}
	var args job.SendAccountEmailArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		t.Fatal(err)
	}
	if err := worker.NewSendAccountEmail(m.Service()).Work(context.Background(), &river.Job[job.SendAccountEmailArgs]{Args: args}); err != nil {
		t.Fatalf("send email job: %v", err)
	}
	box.mu.Lock()
	msg := box.msgs[len(box.msgs)-1]
	box.mu.Unlock()
	if msg.To.Email != email {
		t.Fatalf("email went to %s, want %s", msg.To.Email, email)
	}
	match := linkToken.FindStringSubmatch(msg.Text)
	if match == nil {
		t.Fatalf("no link in email:\n%s", msg.Text)
	}
	return strings.Fields(msg.Text[strings.Index(msg.Text, "http"):])[0], match[1]
}

// Luồng đầy đủ như cmd/server + cmd/worker: bootstrap admin → đăng nhập →
// tạo account Employee (được mời) → thư mời → đặt mật khẩu → Employee bị 403 →
// được gán role mới → refresh thì có quyền mới → quên mật khẩu → đặt lại → cookie
// cũ chết → đăng xuất.
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
	box := &inbox{}
	m, err := identity.New(identity.Deps{
		Pool: pool, Tokens: tokens, Outbox: events.NewOutbox(events.NewRegistry(), client), Jobs: jobs.NewRiver(client),
		Mail: box,
		Config: identity.Config{
			AdminEmail: "Root@StoreIT.test", AdminPassword: password, AppURL: "https://storeit.example",
		},
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
	a := &app{t: t, h: srv.Handler(), pool: pool}

	// Admin đầu tiên đăng nhập, /me có quyền quản trị
	admin, _ := a.login("root@storeit.test")
	rec := a.do(call{method: "GET", path: "/api/v1/me", token: admin})
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), domain.PermRoleManage) {
		t.Fatalf("admin /me: %d %s", rec.Code, rec.Body)
	}

	// Tạo account Employee: chưa có mật khẩu, chưa đăng nhập được
	rec = a.do(call{method: "POST", path: "/api/v1/accounts", token: admin, body: map[string]any{
		"email": "lan@storeit.test", "name": "Lan", "role_ids": []string{domain.EmployeeRoleID.String()},
	}})
	var created struct{ Id string }
	if rec.Code != 201 || json.Unmarshal(rec.Body.Bytes(), &created) != nil {
		t.Fatalf("create employee: %d %s", rec.Code, rec.Body)
	}

	// Worker gửi thư mời; Lan mở link và đặt mật khẩu
	link, invite := deliver(t, m, box, "lan@storeit.test")
	if !strings.HasPrefix(link, "https://storeit.example/accept-invite#token=") {
		t.Errorf("invite link = %s", link)
	}
	rec = a.do(call{method: "POST", path: "/api/v1/auth/password/set", body: map[string]any{"token": invite, "new_password": password}})
	if rec.Code != 204 {
		t.Fatalf("accept invite: %d %s", rec.Code, rec.Body)
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

	// Quên mật khẩu: link đặt lại qua thư, mật khẩu mới, mọi phiên cũ chết
	if rec := a.do(call{method: "POST", path: "/api/v1/auth/password/forgot", body: map[string]any{"email": "Lan@StoreIT.test"}}); rec.Code != 202 {
		t.Fatalf("forgot: %d %s", rec.Code, rec.Body)
	}
	link, reset := deliver(t, m, box, "lan@storeit.test")
	if !strings.HasPrefix(link, "https://storeit.example/reset-password#token=") {
		t.Errorf("reset link = %s", link)
	}
	rec = a.do(call{method: "POST", path: "/api/v1/auth/password/set", body: map[string]any{"token": reset, "new_password": "a-brand-new-password"}})
	if rec.Code != 204 {
		t.Fatalf("reset password: %d %s", rec.Code, rec.Body)
	}
	if rec := a.do(call{method: "POST", path: "/api/v1/auth/refresh", cookie: rotated}); rec.Code != 401 {
		t.Errorf("refresh after reset: %d, want 401", rec.Code)
	}
	rec = a.do(call{method: "POST", path: "/api/v1/auth/login", body: map[string]string{"email": "lan@storeit.test", "password": password}})
	if rec.Code != 401 {
		t.Errorf("login with the old password: %d, want 401", rec.Code)
	}

	// Đăng nhập bằng mật khẩu mới rồi đăng xuất: cookie không refresh được nữa
	rec = a.do(call{method: "POST", path: "/api/v1/auth/login", body: map[string]string{"email": "lan@storeit.test", "password": "a-brand-new-password"}})
	if rec.Code != 200 {
		t.Fatalf("login with the new password: %d %s", rec.Code, rec.Body)
	}
	fresh := refreshCookie(t, rec)
	if rec := a.do(call{method: "POST", path: "/api/v1/auth/logout", cookie: fresh}); rec.Code != 204 {
		t.Fatalf("logout: %d", rec.Code)
	}
	for _, c := range []*http.Cookie{fresh, cookie} {
		if rec := a.do(call{method: "POST", path: "/api/v1/auth/refresh", cookie: c}); rec.Code != 401 {
			t.Errorf("refresh after logout: %d, want 401", rec.Code)
		}
	}
}
