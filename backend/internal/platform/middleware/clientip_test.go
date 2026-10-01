package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"storeit/internal/platform/logger"
)

func TestClientIP(t *testing.T) {
	proxies := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	tests := []struct {
		name    string
		trusted []netip.Prefix
		remote  string
		xff     []string
		want    string
	}{
		{"no proxy trusted: connection address", nil, "203.0.113.5:1234", nil, "203.0.113.5"},
		// Không tin proxy nào thì X-Forwarded-For do client tự viết, bỏ qua
		{"spoofed header ignored", nil, "203.0.113.5:1234", []string{"1.1.1.1"}, "203.0.113.5"},
		{"untrusted remote ignores header", proxies, "203.0.113.5:1234", []string{"1.1.1.1"}, "203.0.113.5"},
		{"trusted proxy: client from header", proxies, "10.0.0.2:5000", []string{"198.51.100.7"}, "198.51.100.7"},
		// Client tự thêm IP giả ở đầu; lấy IP đầu tiên tính từ phải không thuộc proxy
		{
			"proxy chain skips trusted hops", proxies, "10.0.0.2:5000",
			[]string{"6.6.6.6, 198.51.100.7, 10.0.0.3"}, "198.51.100.7",
		},
		{"header split over lines", proxies, "10.0.0.2:5000", []string{"6.6.6.6", "198.51.100.7"}, "198.51.100.7"},
		{"all hops trusted: leftmost", proxies, "10.0.0.2:5000", []string{"10.0.0.9, 10.0.0.3"}, "10.0.0.9"},
		{"garbage stops the walk", proxies, "10.0.0.2:5000", []string{"198.51.100.7, garbage"}, "10.0.0.2"},
		{"hop with port", proxies, "10.0.0.2:5000", []string{"198.51.100.7:61000"}, "198.51.100.7"},
		{"ipv6 remote", nil, "[2001:db8::1]:443", nil, "2001:db8::1"},
		{"ipv4-mapped ipv6 trusted", proxies, "[::ffff:10.0.0.2]:5000", []string{"198.51.100.7"}, "198.51.100.7"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			h := ClientIP(tt.trusted)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = ClientIPFrom(r.Context())
			}))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remote
			for _, v := range tt.xff {
				req.Header.Add("X-Forwarded-For", v)
			}

			h.ServeHTTP(httptest.NewRecorder(), req)

			if got != tt.want {
				t.Errorf("client IP = %q, want %q", got, tt.want)
			}
		})
	}
}

// Dòng access log có client_ip
func TestRequestLogger_LogsClientIP(t *testing.T) {
	var buf bytes.Buffer
	h := ClientIP(nil)(RequestLogger(logger.New(&buf, logger.Config{}))(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.5:1234"

	h.ServeHTTP(httptest.NewRecorder(), req)

	lines := logLines(t, &buf)
	if len(lines) != 1 || lines[0]["client_ip"] != "203.0.113.5" {
		t.Errorf("log = %v, want client_ip 203.0.113.5", lines)
	}
}
