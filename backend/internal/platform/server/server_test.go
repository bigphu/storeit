package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/caarlos0/env/v11"
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
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestConfig_FromEnv(t *testing.T) {
	var cfg struct {
		HTTP Config `envPrefix:"HTTP_"`
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
	}
	if cfg.HTTP != want {
		t.Errorf("got %+v, want %+v", cfg.HTTP, want)
	}
}
