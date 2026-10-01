package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"storeit/internal/platform/jwt"
	"storeit/internal/platform/logger"
)

func newTokens(t *testing.T) *jwt.Provider {
	t.Helper()
	p, err := jwt.New(jwt.Config{
		Keys:           "v1:" + base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32))),
		ActiveKID:      "v1",
		Issuer:         "storeit",
		Audience:       "storeit-api",
		AccessTokenTTL: 15 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// serve chạy req qua Middleware; handler phía sau ghi lại actor nó thấy
func serve(t *testing.T, mw func(http.Handler) http.Handler, req *http.Request) (*httptest.ResponseRecorder, *Actor) {
	t.Helper()
	var seen *Actor
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a, ok := FromContext(r.Context()); ok {
			seen = &a
		} else {
			seen = &Actor{} // tới được handler nhưng không có actor
		}
		w.WriteHeader(http.StatusNoContent)
	})
	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)
	return rec, seen
}

func TestMiddleware_ValidTokenPutsActorInContext(t *testing.T) {
	tokens := newTokens(t)
	accountID := uuid.New()
	perms := []string{"inventory.asset.read"}
	tok, err := tokens.Issue(accountID, perms)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok.Value)

	rec, seen := serve(t, Middleware(tokens), req)

	if rec.Code != http.StatusNoContent || seen == nil {
		t.Fatalf("status = %d, handler reached = %v", rec.Code, seen != nil)
	}
	if seen.AccountID != accountID || !slices.Equal(seen.Permissions, perms) {
		t.Errorf("actor = %+v, want %v %v", *seen, accountID, perms)
	}
}

func TestMiddleware_Rejects(t *testing.T) {
	tokens := newTokens(t)
	tests := []struct {
		name, header, wantType, wantChallenge string
	}{
		{"no header", "", "/errors/missing-token", `Bearer`},
		{"other scheme", "Basic dXNlcjpwdw==", "/errors/missing-token", `Bearer`},
		{"empty token", "Bearer ", "/errors/missing-token", `Bearer`},
		{"garbage token", "Bearer not.a.jwt", "/errors/invalid-token", `Bearer error="invalid_token"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}

			rec, seen := serve(t, Middleware(tokens), req)

			if seen != nil {
				t.Fatal("handler reached without a valid token")
			}
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}
			if got := rec.Header().Get("WWW-Authenticate"); got != tt.wantChallenge {
				t.Errorf("WWW-Authenticate = %q, want %q", got, tt.wantChallenge)
			}
			var body struct{ Type string }
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Type != tt.wantType {
				t.Errorf("body = %s, want type %s", rec.Body, tt.wantType)
			}
		})
	}
}

func TestMiddleware_BearerSchemeIsCaseInsensitive(t *testing.T) {
	tokens := newTokens(t)
	tok, err := tokens.Issue(uuid.New(), nil)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "bearer "+tok.Value)

	if rec, _ := serve(t, Middleware(tokens), req); rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
}

// route dựng router chi có các route như router sinh từ OpenAPI; auth chạy
// sau khi chi route xong (Middlewares của oapi-codegen, ở đây là With)
func routeWithAuth(mw func(http.Handler) http.Handler, next http.Handler) *chi.Mux {
	r := chi.NewRouter()
	r.With(mw).Post("/api/v1/auth/login", next.ServeHTTP)
	r.With(mw).Get("/api/v1/auth/login", next.ServeHTTP)
	r.With(mw).Get("/api/v1/files/{id}", next.ServeHTTP)
	return r
}

func TestMiddleware_PublicMatchesRoutePattern(t *testing.T) {
	mw := Middleware(newTokens(t), Public("POST /api/v1/auth/login", "GET /api/v1/files/{id}"))
	tests := []struct {
		name, method, path string
		want               int
	}{
		{"exact path", http.MethodPost, "/api/v1/auth/login", http.StatusNoContent},
		{"path param", http.MethodGet, "/api/v1/files/7f0c", http.StatusNoContent},
		{"same path, other method", http.MethodGet, "/api/v1/auth/login", http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var seen *Actor
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				a, _ := FromContext(r.Context())
				seen = &a
				w.WriteHeader(http.StatusNoContent)
			})
			rec := httptest.NewRecorder()
			routeWithAuth(mw, next).ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))

			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d", rec.Code, tt.want)
			}
			if tt.want == http.StatusNoContent && seen.AccountID != uuid.Nil {
				t.Errorf("public route got actor %+v, want none", *seen)
			}
		})
	}
}

// Gắn nhầm bằng r.Use (chạy trước khi chi route) thì chưa biết route nào,
// nên không có gì là public: thiếu token là 401, không lặng lẽ mở route ra
func TestMiddleware_PublicBeforeRoutingFailsClosed(t *testing.T) {
	r := chi.NewRouter()
	r.Use(Middleware(newTokens(t), Public("POST /api/v1/auth/login")))
	r.Post("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// actor_id phải vào scope log của request, để access log bên ngoài cũng có
func TestMiddleware_AddsActorToLogScope(t *testing.T) {
	tokens := newTokens(t)
	accountID := uuid.New()
	tok, err := tokens.Issue(accountID, nil)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	log := logger.New(&buf, logger.Config{})
	ctx := logger.WithScope(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+tok.Value)

	serve(t, Middleware(tokens), req)
	log.InfoContext(ctx, "http request")

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("not JSON: %q", buf.String())
	}
	if line["actor_id"] != accountID.String() {
		t.Errorf("actor_id = %v, want %v", line["actor_id"], accountID)
	}
}
