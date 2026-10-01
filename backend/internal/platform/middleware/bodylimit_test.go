package middleware

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBodyLimit(t *testing.T) {
	tests := []struct {
		name    string
		chain   []func(http.Handler) http.Handler
		size    int
		wantErr bool
	}{
		{"under limit", []func(http.Handler) http.Handler{BodyLimit(10)}, 10, false},
		{"over limit", []func(http.Handler) http.Handler{BodyLimit(10)}, 11, true},
		// Route upload nới giới hạn của server lên: phải đọc được, không bị
		// MaxBytesReader của server chặn ở 10
		{"route raises limit", []func(http.Handler) http.Handler{BodyLimit(10), BodyLimit(100)}, 50, false},
		{"route lowers limit", []func(http.Handler) http.Handler{BodyLimit(100), BodyLimit(10)}, 50, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var readErr error
			var h http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, readErr = io.ReadAll(r.Body)
			})
			for i := len(tt.chain) - 1; i >= 0; i-- {
				h = tt.chain[i](h)
			}

			h.ServeHTTP(httptest.NewRecorder(),
				httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", tt.size))))

			var maxErr *http.MaxBytesError
			if got := errors.As(readErr, &maxErr); got != tt.wantErr {
				t.Errorf("read err = %v, want MaxBytesError: %v", readErr, tt.wantErr)
			}
		})
	}
}
