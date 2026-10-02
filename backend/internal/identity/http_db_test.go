package identity_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"storeit/internal/identity/domain"
	"storeit/internal/identity/handler"
	"storeit/internal/identity/job"
	"storeit/internal/identity/repository"
	"storeit/internal/identity/service"
	"storeit/internal/platform/database/dbtest"
	"storeit/internal/platform/events"
	"storeit/internal/platform/jobs"
	"storeit/internal/platform/jwt"
	"storeit/internal/platform/server"
)

const password = "correct-horse-battery"

type app struct {
	t        *testing.T
	h        http.Handler
	pool     *pgxpool.Pool
	svc      *service.Service
	accounts *repository.AccountRepository
	hasher   service.Hasher
}

func newApp(t *testing.T) *app {
	t.Helper()
	pool := dbtest.Pool(t)
	client, err := jobs.NewInsertClient(pool, nil)
	if err != nil {
		t.Fatal(err)
	}
	outbox := events.NewOutbox(events.NewRegistry(), client)
	tokens, err := jwt.New(jwt.Config{
		Keys:      "v1:" + base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32))),
		ActiveKID: "v1", Issuer: "storeit", Audience: "storeit-api", AccessTokenTTL: 15 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	enq := jobs.NewRiver(client)
	accounts := repository.NewAccountRepository(pool, outbox, enq)
	hasher := service.NewBcrypt(4)
	svc := service.New(service.Deps{
		Accounts: accounts, Roles: repository.NewRoleRepository(pool, outbox),
		Sessions: repository.NewSessionRepository(pool), Hasher: hasher, Tokens: tokens,
		PasswordTokens: repository.NewTokenRepository(pool, outbox, enq), Jobs: enq,
		Settings: service.Settings{
			SlidingTTL: time.Hour, AbsoluteTTL: 24 * time.Hour, Grace: 30 * time.Second,
			InviteTTL: 72 * time.Hour, ResetTTL: time.Hour,
		},
	})
	srv := server.New(server.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := handler.New(svc, handler.CookieSettings{}).Mount(srv.Router(), tokens); err != nil {
		t.Fatal(err)
	}
	return &app{t: t, h: srv.Handler(), pool: pool, svc: svc, accounts: accounts, hasher: hasher}
}

// seed tạo account với role cho trước, trả email (ngẫu nhiên: DB dùng chung)
func (a *app) seed(roleIDs ...uuid.UUID) string {
	a.t.Helper()
	h, err := a.hasher.Hash(password)
	if err != nil {
		a.t.Fatal(err)
	}
	email := "h-" + uuid.NewString()[:8] + "@storeit.test"
	if _, err := a.accounts.Create(context.Background(), domain.NewAccount{
		Email: email, Name: "Handler Test", PasswordHash: h, RoleIDs: roleIDs,
	}); err != nil {
		a.t.Fatal(err)
	}
	return email
}

// runForgot chạy job quên mật khẩu như worker (request chỉ xếp job)
func (a *app) runForgot(email string) {
	a.t.Helper()
	if err := a.svc.ProcessForgotPassword(context.Background(), job.ForgotPasswordArgs{Email: email}); err != nil {
		a.t.Fatal(err)
	}
}

// linkToken đọc token thô của link đang sống (account, purpose) từ job gửi
// thư, như người dùng đọc từ email
func (a *app) linkToken(accountID, purpose string) string {
	a.t.Helper()
	var raw string
	err := a.pool.QueryRow(context.Background(), `
		SELECT j.args->>'token' FROM river_job j
		JOIN identity.password_tokens p ON p.id = (j.args->>'token_id')::uuid
		WHERE p.account_id = $1 AND p.purpose = $2`, accountID, purpose).Scan(&raw)
	if err != nil {
		a.t.Fatalf("no live %s link for %s: %v", purpose, accountID, err)
	}
	return raw
}

type call struct {
	method, path, token string
	body                any
	cookie              *http.Cookie
}

func (a *app) do(c call) *httptest.ResponseRecorder {
	a.t.Helper()
	var body io.Reader
	if c.body != nil {
		b, err := json.Marshal(c.body)
		if err != nil {
			a.t.Fatal(err)
		}
		body = bytes.NewReader(b)
	}
	req := httptest.NewRequest(c.method, c.path, body)
	if c.body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if c.cookie != nil {
		req.AddCookie(c.cookie)
	}
	rec := httptest.NewRecorder()
	a.h.ServeHTTP(rec, req)
	return rec
}

type session struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Account     struct {
		Email string `json:"email"`
	} `json:"account"`
}

// login trả access token và cookie refresh
func (a *app) login(email string) (string, *http.Cookie) {
	a.t.Helper()
	rec := a.do(call{method: "POST", path: "/api/v1/auth/login", body: map[string]string{"email": email, "password": password}})
	if rec.Code != 200 {
		a.t.Fatalf("login %s: %d %s", email, rec.Code, rec.Body)
	}
	var s session
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		a.t.Fatal(err)
	}
	return s.AccessToken, refreshCookie(a.t, rec)
}

func refreshCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == "storeit_refresh" {
			return c
		}
	}
	t.Fatalf("no storeit_refresh cookie in %v", rec.Header().Values("Set-Cookie"))
	return nil
}

func problemType(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var p struct{ Type string }
	_ = json.Unmarshal(rec.Body.Bytes(), &p)
	return p.Type
}

func TestLogin(t *testing.T) {
	a := newApp(t)
	email := a.seed(domain.AdministratorRoleID)

	rec := a.do(call{method: "POST", path: "/api/v1/auth/login", body: map[string]string{"email": strings.ToUpper(email), "password": password}})
	if rec.Code != 200 {
		t.Fatalf("login: %d %s", rec.Code, rec.Body)
	}
	var s session
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil || s.AccessToken == "" || s.TokenType != "Bearer" || s.Account.Email != email {
		t.Errorf("session = %+v, %v", s, err)
	}
	c := refreshCookie(t, rec)
	if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/api/v1/auth" || c.Value == "" {
		t.Errorf("cookie = %+v", c)
	}

	rec = a.do(call{method: "POST", path: "/api/v1/auth/login", body: map[string]string{"email": email, "password": "wrong-password-xx"}})
	if rec.Code != 401 || problemType(t, rec) != "/errors/bad-credentials" {
		t.Errorf("wrong password: %d %s", rec.Code, rec.Body)
	}
}

func TestProtectedAccess(t *testing.T) {
	a := newApp(t)
	admin, _ := a.login(a.seed(domain.AdministratorRoleID))
	employee, _ := a.login(a.seed(domain.EmployeeRoleID))

	if rec := a.do(call{method: "GET", path: "/api/v1/me"}); rec.Code != 401 {
		t.Errorf("me without token: %d", rec.Code)
	}
	if rec := a.do(call{method: "GET", path: "/api/v1/me", token: employee}); rec.Code != 200 {
		t.Errorf("me as employee: %d %s", rec.Code, rec.Body)
	}
	if rec := a.do(call{method: "GET", path: "/api/v1/accounts", token: employee}); rec.Code != 403 {
		t.Errorf("accounts as employee: %d, want 403", rec.Code)
	}

	rec := a.do(call{method: "GET", path: "/api/v1/accounts?page_size=2", token: admin})
	var list struct {
		Items []any `json:"items"`
		Total int64 `json:"total"`
	}
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &list) != nil || len(list.Items) > 2 || list.Total < 2 {
		t.Errorf("accounts as admin: %d %s", rec.Code, rec.Body)
	}
}

func TestRefreshAndLogout(t *testing.T) {
	a := newApp(t)
	_, cookie := a.login(a.seed())

	rec := a.do(call{method: "POST", path: "/api/v1/auth/refresh", cookie: cookie})
	if rec.Code != 200 {
		t.Fatalf("refresh: %d %s", rec.Code, rec.Body)
	}
	next := refreshCookie(t, rec)
	if next.Value == cookie.Value {
		t.Error("refresh did not rotate the cookie")
	}

	rec = a.do(call{method: "POST", path: "/api/v1/auth/refresh"})
	if rec.Code != 401 || problemType(t, rec) != "/errors/invalid-refresh-token" {
		t.Errorf("refresh without cookie: %d %s", rec.Code, rec.Body)
	}
	if c := refreshCookie(t, rec); c.MaxAge >= 0 || c.Value != "" {
		t.Errorf("failed refresh should clear the cookie: %+v", c)
	}

	rec = a.do(call{method: "POST", path: "/api/v1/auth/logout", cookie: next})
	if rec.Code != 204 {
		t.Fatalf("logout: %d %s", rec.Code, rec.Body)
	}
	if c := refreshCookie(t, rec); c.MaxAge >= 0 {
		t.Errorf("logout should clear the cookie: %+v", c)
	}
	if rec := a.do(call{method: "POST", path: "/api/v1/auth/refresh", cookie: next}); rec.Code != 401 {
		t.Errorf("refresh after logout: %d", rec.Code)
	}
}

func TestAccountAndRoleManagement(t *testing.T) {
	a := newApp(t)
	admin, _ := a.login(a.seed(domain.AdministratorRoleID))
	email := "new-" + uuid.NewString()[:8] + "@storeit.test"

	// Tạo không cần mật khẩu: account ở trạng thái invited, lời mời đi kèm
	rec := a.do(call{method: "POST", path: "/api/v1/accounts", token: admin, body: map[string]any{
		"email": email, "name": "New", "role_ids": []string{domain.EmployeeRoleID.String()},
	}})
	var created struct {
		Id     string                  `json:"id"`
		Status string                  `json:"status"`
		Roles  []struct{ Name string } `json:"roles"`
	}
	if rec.Code != 201 || json.Unmarshal(rec.Body.Bytes(), &created) != nil || len(created.Roles) != 1 || created.Roles[0].Name != "Employee" {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	if created.Status != "invited" {
		t.Errorf("status = %q, want invited", created.Status)
	}

	// Trùng email (khác hoa thường): 409
	rec = a.do(call{method: "POST", path: "/api/v1/accounts", token: admin, body: map[string]any{
		"email": strings.ToUpper(email), "name": "Dup",
	}})
	if rec.Code != 409 {
		t.Errorf("duplicate email: %d %s", rec.Code, rec.Body)
	}

	rec = a.do(call{method: "PUT", path: "/api/v1/accounts/" + created.Id + "/roles", token: admin, body: map[string]any{
		"role_ids": []string{domain.AuthorizedManagerRoleID.String()},
	}})
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Authorized Manager") {
		t.Errorf("assign roles: %d %s", rec.Code, rec.Body)
	}

	// Người mới nhận lời mời (đặt mật khẩu), đăng nhập và thấy quyền của role mới
	rec = a.do(call{method: "POST", path: "/api/v1/auth/password/set", body: map[string]any{
		"token": a.linkToken(created.Id, "invite"), "new_password": password,
	}})
	if rec.Code != 204 {
		t.Fatalf("accept invite: %d %s", rec.Code, rec.Body)
	}
	token, _ := a.login(email)
	if rec := a.do(call{method: "GET", path: "/api/v1/accounts", token: token}); rec.Code != 200 {
		t.Errorf("authorized manager can read accounts: %d", rec.Code)
	}
	if rec := a.do(call{method: "POST", path: "/api/v1/roles", token: token, body: map[string]any{"name": "X"}}); rec.Code != 403 {
		t.Errorf("authorized manager cannot create roles: %d", rec.Code)
	}

	if rec := a.do(call{method: "GET", path: "/api/v1/roles", token: admin}); rec.Code != 200 || !strings.Contains(rec.Body.String(), "identity.role.manage") {
		t.Errorf("list roles: %d %s", rec.Code, rec.Body)
	}
	if rec := a.do(call{method: "DELETE", path: "/api/v1/roles/" + domain.EmployeeRoleID.String(), token: admin}); rec.Code != 409 {
		t.Errorf("delete system role: %d", rec.Code)
	}

	rec = a.do(call{method: "POST", path: "/api/v1/accounts/" + created.Id + "/disable", token: admin})
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"active":false`) {
		t.Errorf("disable: %d %s", rec.Code, rec.Body)
	}
	rec = a.do(call{method: "POST", path: "/api/v1/auth/login", body: map[string]string{"email": email, "password": password}})
	if rec.Code != 403 || problemType(t, rec) != "/errors/account-disabled" {
		t.Errorf("login disabled: %d %s", rec.Code, rec.Body)
	}
}

func TestInvitationAndResetRoutes(t *testing.T) {
	a := newApp(t)
	admin, _ := a.login(a.seed(domain.AdministratorRoleID))
	employee, _ := a.login(a.seed(domain.EmployeeRoleID))
	activeEmail := a.seed()
	active, err := a.accounts.GetByEmail(context.Background(), activeEmail)
	if err != nil {
		t.Fatal(err)
	}
	rec := a.do(call{method: "POST", path: "/api/v1/accounts", token: admin, body: map[string]any{
		"email": "inv-" + uuid.NewString()[:8] + "@storeit.test", "name": "Invitee",
	}})
	var inv struct{ Id string }
	if rec.Code != 201 || json.Unmarshal(rec.Body.Bytes(), &inv) != nil {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}

	// Đặt mật khẩu trực tiếp không còn: route cũ đã bỏ
	rec = a.do(call{method: "PUT", path: "/api/v1/accounts/" + inv.Id + "/password", token: admin, body: map[string]any{"password": password}})
	if rec.Code != 404 && rec.Code != 405 {
		t.Errorf("old admin set-password route: %d, want 404/405", rec.Code)
	}

	// Quên mật khẩu: công khai, luôn 202, kể cả email lạ
	for _, email := range []string{"ghost-" + uuid.NewString()[:8] + "@storeit.test", activeEmail} {
		if rec := a.do(call{method: "POST", path: "/api/v1/auth/password/forgot", body: map[string]any{"email": email}}); rec.Code != 202 {
			t.Errorf("forgot %s: %d %s", email, rec.Code, rec.Body)
		}
	}
	a.runForgot(activeEmail)
	reset := a.linkToken(active.ID.String(), "reset")

	rec = a.do(call{method: "POST", path: "/api/v1/auth/password/set", body: map[string]any{"token": "nope", "new_password": password}})
	if rec.Code != 422 || problemType(t, rec) != "/errors/invalid-password-token" || !strings.Contains(rec.Body.String(), `"token"`) {
		t.Errorf("bad token: %d %s", rec.Code, rec.Body)
	}
	rec = a.do(call{method: "POST", path: "/api/v1/auth/password/set", body: map[string]any{"token": reset, "new_password": "brand-new-password"}})
	if rec.Code != 204 {
		t.Errorf("reset: %d %s", rec.Code, rec.Body)
	}
	rec = a.do(call{method: "POST", path: "/api/v1/auth/login", body: map[string]string{"email": activeEmail, "password": "brand-new-password"}})
	if rec.Code != 200 {
		t.Errorf("login with reset password: %d %s", rec.Code, rec.Body)
	}

	// Gửi lại lời mời: chỉ cho account chưa nhận lời
	if rec := a.do(call{method: "POST", path: "/api/v1/accounts/" + inv.Id + "/invitation", token: admin}); rec.Code != 202 {
		t.Errorf("resend: %d %s", rec.Code, rec.Body)
	}
	rec = a.do(call{method: "POST", path: "/api/v1/accounts/" + active.ID.String() + "/invitation", token: admin})
	if rec.Code != 409 || problemType(t, rec) != "/errors/not-invited" {
		t.Errorf("resend to active account: %d %s", rec.Code, rec.Body)
	}

	// Link đặt lại do quản trị gửi: cần identity.account.manage
	if rec := a.do(call{method: "POST", path: "/api/v1/accounts/" + active.ID.String() + "/password-reset", token: employee}); rec.Code != 403 {
		t.Errorf("reset link as employee: %d", rec.Code)
	}
	if rec := a.do(call{method: "POST", path: "/api/v1/accounts/" + active.ID.String() + "/password-reset", token: admin}); rec.Code != 202 {
		t.Errorf("reset link as admin: %d %s", rec.Code, rec.Body)
	}
	rec = a.do(call{method: "GET", path: "/api/v1/accounts/" + active.ID.String(), token: admin})
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"status":"active"`) {
		t.Errorf("account status: %d %s", rec.Code, rec.Body)
	}
}

// Đăng nhập, quên mật khẩu, đặt mật khẩu bị giới hạn theo IP: đoán mật khẩu
// hay spam form công khai gặp 429 kèm Retry-After
func TestAuthRateLimits(t *testing.T) {
	a := newApp(t)
	email := a.seed()
	for i := range 10 {
		rec := a.do(call{method: "POST", path: "/api/v1/auth/login", body: map[string]string{"email": email, "password": "wrong-password-xx"}})
		if rec.Code != 401 {
			t.Fatalf("attempt %d: %d, want 401", i+1, rec.Code)
		}
	}
	rec := a.do(call{method: "POST", path: "/api/v1/auth/login", body: map[string]string{"email": email, "password": password}})
	if rec.Code != 429 || problemType(t, rec) != "/errors/rate-limited" || rec.Header().Get("Retry-After") == "" {
		t.Errorf("11th login: %d %s, want 429 with Retry-After", rec.Code, rec.Body)
	}

	for i := range 5 {
		if rec := a.do(call{method: "POST", path: "/api/v1/auth/password/forgot", body: map[string]any{"email": email}}); rec.Code != 202 {
			t.Fatalf("forgot %d: %d", i+1, rec.Code)
		}
	}
	if rec := a.do(call{method: "POST", path: "/api/v1/auth/password/forgot", body: map[string]any{"email": email}}); rec.Code != 429 {
		t.Errorf("6th forgot: %d, want 429", rec.Code)
	}

	for range 10 {
		a.do(call{method: "POST", path: "/api/v1/auth/password/set", body: map[string]any{"token": "nope", "new_password": password}})
	}
	if rec := a.do(call{method: "POST", path: "/api/v1/auth/password/set", body: map[string]any{"token": "nope", "new_password": password}}); rec.Code != 429 {
		t.Errorf("11th set password: %d, want 429", rec.Code)
	}

	// Refresh không bị giới hạn chung với login
	if rec := a.do(call{method: "POST", path: "/api/v1/auth/refresh"}); rec.Code != 401 {
		t.Errorf("refresh: %d, want 401 (not rate limited)", rec.Code)
	}
}
