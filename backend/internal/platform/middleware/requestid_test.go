package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	chimw "github.com/go-chi/chi/v5/middleware"
)

func TestRequestID(t *testing.T) {
	tests := []struct {
		name, header string
		keep         bool // header được giữ làm request ID
	}{
		{"no header generates one", "", false},
		{"valid header from proxy kept", "edge-7f3a.b_9", true},
		// Header do client gửi không được tin: quá dài hay có ký tự lạ (xuống
		// dòng, khoảng trắng...) thì sinh ID mới
		{"too long replaced", strings.Repeat("a", 65), false},
		{"bad characters replaced", "abc def\nfake=1", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = chimw.GetReqID(r.Context())
			}))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.header != "" {
				req.Header.Set("X-Request-Id", tt.header)
			}
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if got == "" {
				t.Fatal("no request ID in ctx")
			}
			if tt.keep != (got == tt.header) {
				t.Errorf("request ID = %q, header %q, want kept: %v", got, tt.header, tt.keep)
			}
			// Trả lại cho client để đối chiếu log
			if h := rec.Header().Get("X-Request-Id"); h != got {
				t.Errorf("response X-Request-Id = %q, want %q", h, got)
			}
		})
	}
}
