package middleware

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// Router giống module sinh từ OpenAPI: server giới hạn body 10 byte, và chỉ
// có một middleware chung cho mọi operation (Middlewares của oapi-codegen,
// ở đây là With). ForOperations nới giới hạn cho riêng operation upload.
func TestForOperations(t *testing.T) {
	var readErr error
	h := func(w http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
	}
	mw := ForOperations([]string{"POST /api/v1/imports/{kind}"}, BodyLimit(100))

	r := chi.NewRouter()
	r.Use(BodyLimit(10))
	r.With(mw).Post("/api/v1/imports/{kind}", h)
	r.With(mw).Post("/api/v1/things", h)
	r.With(mw).Get("/api/v1/imports/{kind}", h) // cùng path, khác method

	tests := []struct {
		name, method, path string
		wantLimited        bool
	}{
		{"listed operation gets its own limit", http.MethodPost, "/api/v1/imports/assets", false},
		{"other operation keeps server limit", http.MethodPost, "/api/v1/things", true},
		{"same path other method keeps server limit", http.MethodGet, "/api/v1/imports/assets", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			readErr = nil
			body := strings.NewReader(strings.Repeat("x", 50))
			r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(tt.method, tt.path, body))

			var maxErr *http.MaxBytesError
			if got := errors.As(readErr, &maxErr); got != tt.wantLimited {
				t.Errorf("read err = %v, want limited by server: %v", readErr, tt.wantLimited)
			}
		})
	}
}

// Nhiều middleware: chạy theo đúng thứ tự liệt kê (đầu danh sách ở ngoài)
func TestForOperations_Order(t *testing.T) {
	var order []string
	mark := func(name string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}
	r := chi.NewRouter()
	r.With(ForOperations([]string{"GET /x"}, mark("a"), mark("b"))).Get("/x", func(http.ResponseWriter, *http.Request) {
		order = append(order, "handler")
	})

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))

	if got := strings.Join(order, ","); got != "a,b,handler" {
		t.Errorf("order = %s, want a,b,handler", got)
	}
}
