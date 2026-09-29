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

func TestMiddleware_PublicOperations(t *testing.T) {
	mw := Middleware(newTokens(t), Public("POST /api/v1/auth/login"))

	// Public: qua được mà không cần token, và không có actor
	rec, seen := serve(t, mw, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil))
	if rec.Code != http.StatusNoContent || seen == nil || seen.AccountID != uuid.Nil {
		t.Errorf("public: status = %d, actor = %v", rec.Code, seen)
	}

	// Cùng path nhưng khác method thì không public
	if rec, _ := serve(t, mw, httptest.NewRequest(http.MethodGet, "/api/v1/auth/login", nil)); rec.Code != http.StatusUnauthorized {
		t.Errorf("GET login: status = %d, want 401", rec.Code)
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
