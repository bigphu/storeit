package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"storeit/internal/platform/logger"
)

// newRouter dựng router với thứ tự middleware như app thật, log ra buf
func newRouter(buf *bytes.Buffer) *chi.Mux {
	log := logger.New(buf, logger.Config{Level: slog.LevelDebug})
	r := chi.NewRouter()
	r.Use(chimw.RequestID, RequestLogger(log), Recoverer(log))
	r.Get("/assets/{id}", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})
	r.Get("/empty", func(w http.ResponseWriter, r *http.Request) {})
	r.Get("/boom", func(w http.ResponseWriter, r *http.Request) { panic("boom") })
	return r
}

// logLines tách từng dòng JSON trong buf
func logLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var lines []map[string]any
	for _, b := range bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n")) {
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatalf("not JSON: %q", b)
		}
		lines = append(lines, m)
	}
	return lines
}

func TestRequestLogger(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		status int
		level  string
		route  string
		bytes  float64
	}{
		{"ok", "/assets/a-3?token=secret", 200, "INFO", "/assets/{id}", 5},
		{"no write", "/empty", 200, "INFO", "/empty", 0},
		{"not found", "/nope", 404, "WARN", "", -1},
		{"panic", "/boom", 500, "ERROR", "/boom", -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			rec := httptest.NewRecorder()
			newRouter(&buf).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))

			if rec.Code != tt.status {
				t.Fatalf("response status = %d, want %d", rec.Code, tt.status)
			}
			lines := logLines(t, &buf)
			got := lines[len(lines)-1] // dòng access log luôn ghi sau cùng
			if got["msg"] != "http request" {
				t.Fatalf("last line = %v", got)
			}
			if got["level"] != tt.level || got["status"] != float64(tt.status) || got["route"] != tt.route {
				t.Errorf("level=%v status=%v route=%v, want %s %d %q",
					got["level"], got["status"], got["route"], tt.level, tt.status, tt.route)
			}
			if tt.bytes >= 0 && got["bytes"] != tt.bytes {
				t.Errorf("bytes = %v, want %v", got["bytes"], tt.bytes)
			}
			if got["method"] != "GET" {
				t.Errorf("method = %v", got["method"])
			}
			// path không được mang query string
			if p, _ := got["path"].(string); p == "" || bytes.ContainsRune([]byte(p), '?') {
				t.Errorf("path = %v", got["path"])
			}
			// mọi dòng (kể cả log panic của Recoverer) đều có cùng request_id
			id, _ := got["request_id"].(string)
			if id == "" {
				t.Fatal("missing request_id")
			}
			for _, l := range lines {
				if l["request_id"] != id {
					t.Errorf("line %q request_id = %v, want %q", l["msg"], l["request_id"], id)
				}
			}
		})
	}
}

// Actor chỉ biết được bên trong (auth middleware), nhưng dòng access log và log
// panic của Recoverer nằm bên ngoài vẫn phải có actor_id
func TestRequestLogger_ActorSetInside(t *testing.T) {
	for _, tt := range []struct {
		name, path string
	}{
		{"ok", "/me"},
		{"panic", "/me/boom"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			r := newRouter(&buf)
			setActor := func(r *http.Request) {
				logger.AddToScope(r.Context(), slog.String("actor_id", "acc-42"))
			}
			r.Get("/me", func(w http.ResponseWriter, r *http.Request) { setActor(r) })
			r.Get("/me/boom", func(w http.ResponseWriter, r *http.Request) { setActor(r); panic("boom") })

			r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, tt.path, nil))

			for _, l := range logLines(t, &buf) {
				if l["actor_id"] != "acc-42" {
					t.Errorf("line %q actor_id = %v, want acc-42", l["msg"], l["actor_id"])
				}
			}
		})
	}
}
