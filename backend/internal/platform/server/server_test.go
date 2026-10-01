package server

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/go-chi/chi/v5"

	"storeit/internal/platform/logger"
)

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

// start chạy Serve trên cổng ngẫu nhiên, trả base URL và kênh nhận lỗi của Serve
func start(t *testing.T, s *Server) (string, context.CancelFunc, <-chan error) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, ln) }()
	return "http://" + ln.Addr().String(), cancel, done
}

func wait(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return")
		return nil
	}
}

func TestServe_ServesAndStopsCleanly(t *testing.T) {
	s := New(Config{}, discard)
	s.Router().Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("pong"))
	})
	base, cancel, done := start(t, s)

	resp, err := http.Get(base + "/ping")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != 200 || string(body) != "pong" {
		t.Errorf("got %d %q", resp.StatusCode, body)
	}

	resp, err = http.Get(base + "/nope")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); resp.StatusCode != 404 || ct != "application/problem+json" {
		t.Errorf("404 = %d %q", resp.StatusCode, ct)
	}

	cancel()
	if err := wait(t, done); err != nil {
		t.Errorf("Serve = %v, want nil", err)
	}
}

func TestServe_ShutdownTimeoutCutsRequests(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	t.Cleanup(func() { close(release) })

	s := New(Config{ShutdownTimeout: 50 * time.Millisecond}, discard)
	s.Router().Get("/slow", func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
	})
	base, cancel, done := start(t, s)

	clientErr := make(chan error, 1)
	go func() {
		resp, err := http.Get(base + "/slow")
		if err == nil {
			_ = resp.Body.Close()
		}
		clientErr <- err
	}()
	<-started
	cancel()

	if err := wait(t, done); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Serve = %v, want DeadlineExceeded", err)
	}
	// Close đã cắt kết nối nên client nhận lỗi, không treo
	select {
	case err := <-clientErr:
		if err == nil {
			t.Error("slow request got a response, want connection closed")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("client still hanging after shutdown")
	}
}

func TestRun_PortInUse(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()

	err = New(Config{Addr: ln.Addr().String()}, discard).Run(context.Background())
	if err == nil {
		t.Fatal("want listen error")
	}
}

func TestConfig_Defaults(t *testing.T) {
	got := Config{ReadTimeout: time.Minute, IdleTimeout: -1}.withDefaults()
	want := Config{
		Addr:              ":8080",
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       time.Minute, // đã đặt thì giữ
		IdleTimeout:       60 * time.Second,
		ShutdownTimeout:   15 * time.Second,
		MaxBodyBytes:      1 << 20,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestConfig_FromEnv(t *testing.T) {
	var cfg struct {
		HTTP Config // tên biến do package đặt, binary không thêm prefix
	}
	// Map rỗng chứ không nil: nil thì env đọc biến môi trường thật
	err := env.ParseWithOptions(&cfg, env.Options{Environment: map[string]string{
		"HTTP_ADDR": ":9000", "HTTP_IDLE_TIMEOUT": "2m",
	}})
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		Addr:              ":9000",
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       2 * time.Minute,
		ShutdownTimeout:   15 * time.Second,
		MaxBodyBytes:      1 << 20,
	}
	if !reflect.DeepEqual(cfg.HTTP, want) {
		t.Errorf("got %+v, want %+v", cfg.HTTP, want)
	}
}

func TestConfig_ValidateRejectsNegative(t *testing.T) {
	for name, cfg := range map[string]Config{
		"read timeout":   {ReadTimeout: -time.Second},
		"idle timeout":   {IdleTimeout: -1},
		"max body bytes": {MaxBodyBytes: -1},
	} {
		if err := cfg.Validate(); err == nil {
			t.Errorf("%s: want error for negative value", name)
		}
	}
	if err := (Config{}).Validate(); err != nil {
		t.Errorf("zero Config gets defaults, want valid: %v", err)
	}
}

// Response API (JSON, problem+json, kể cả 404 của router) có nosniff, để trình
// duyệt không đoán lại kiểu nội dung
func TestServer_NoSniffHeader(t *testing.T) {
	s := New(Config{}, discard)
	s.Router().Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("pong"))
	})
	for _, path := range []string{"/ping", "/nope"} {
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("%s: X-Content-Type-Options = %q, want nosniff", path, got)
		}
	}
}

func TestConfig_TrustedProxiesFromEnv(t *testing.T) {
	var cfg struct{ HTTP Config }
	err := env.ParseWithOptions(&cfg, env.Options{Environment: map[string]string{
		"HTTP_TRUSTED_PROXIES": "10.0.0.0/8, 172.16.0.0/12",
	}})
	if err != nil {
		t.Fatal(err)
	}
	want := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("172.16.0.0/12")}
	if !slices.Equal(cfg.HTTP.TrustedProxies, want) {
		t.Errorf("TrustedProxies = %v, want %v", cfg.HTTP.TrustedProxies, want)
	}
}

func TestConfig_TrustedProxiesRejectsGarbage(t *testing.T) {
	var cfg struct{ HTTP Config }
	err := env.ParseWithOptions(&cfg, env.Options{Environment: map[string]string{
		"HTTP_TRUSTED_PROXIES": "not-a-cidr",
	}})
	if err == nil {
		t.Error("want parse error for an invalid CIDR")
	}
}

// Access log của server có client_ip; mặc định không tin proxy nào nên
// X-Forwarded-For bị bỏ qua
func TestServer_LogsClientIP(t *testing.T) {
	var buf bytes.Buffer
	s := New(Config{}, logger.New(&buf, logger.Config{}))
	// chi chỉ dựng chuỗi middleware khi có route đầu tiên
	s.Router().Get("/ping", func(http.ResponseWriter, *http.Request) {})
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.RemoteAddr = "203.0.113.5:1234"
	req.Header.Set("X-Forwarded-For", "1.1.1.1")

	s.Handler().ServeHTTP(httptest.NewRecorder(), req)

	if !strings.Contains(buf.String(), `"client_ip":"203.0.113.5"`) {
		t.Errorf("access log missing client_ip:\n%s", buf.String())
	}
}

// 405 phải có header Allow (RFC 9110) liệt kê method route nhận
func TestServer_MethodNotAllowedSetsAllow(t *testing.T) {
	s := New(Config{}, discard)
	noop := func(http.ResponseWriter, *http.Request) {}
	s.Router().Get("/things/{id}", noop)
	s.Router().Put("/things/{id}", noop)
	s.Router().Route("/api", func(r chi.Router) {
		r.Post("/items", noop)
	})

	for path, want := range map[string]string{
		"/things/7":  "GET, PUT",
		"/api/items": "POST",
	} {
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, path, nil))
		if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != want {
			t.Errorf("%s: got %d Allow=%q, want 405 Allow=%q", path, rec.Code, rec.Header().Get("Allow"), want)
		}
	}
}
