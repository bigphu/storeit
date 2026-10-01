package middleware

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"storeit/internal/platform/logger"
)

// Server có ReadTimeout ngắn; route upload nới ra bằng ReadTimeout để client
// gửi body chậm vẫn gửi xong. Chạy qua RequestLogger để chắc
// http.ResponseController đi xuyên được writer đã bọc.
func TestReadTimeout_ExtendsServerDeadline(t *testing.T) {
	const serverTimeout, pause = 200 * time.Millisecond, 500 * time.Millisecond

	for _, tt := range []struct {
		name    string
		extend  bool
		timeout time.Duration
		wantErr bool
	}{
		{"server timeout cuts slow body", false, 0, true},
		{"route extends deadline", true, 5 * time.Second, false},
		// 0 (vd đọc từ config) nghĩa là không giới hạn, không phải hết hạn ngay
		{"zero means no deadline", true, 0, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			readErr := make(chan error, 1)
			r := chi.NewRouter()
			r.Use(chimw.RequestID, RequestLogger(logger.New(io.Discard, logger.Config{Level: slog.LevelError})))
			upload := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, err := io.ReadAll(r.Body)
				readErr <- err
			})
			if tt.extend {
				r.With(ReadTimeout(tt.timeout)).Post("/upload", upload)
			} else {
				r.Post("/upload", upload)
			}

			srv := httptest.NewUnstartedServer(r)
			srv.Config.ReadTimeout = serverTimeout
			srv.Start()
			defer srv.Close()

			// Body gửi làm hai lần, nghỉ lâu hơn ReadTimeout của server
			pr, pw := io.Pipe()
			go func() {
				_, _ = pw.Write([]byte("part1-"))
				time.Sleep(pause)
				_, _ = pw.Write([]byte("part2"))
				_ = pw.Close()
			}()
			resp, err := http.Post(srv.URL+"/upload", "application/octet-stream", pr)
			if err == nil {
				_ = resp.Body.Close()
			}

			select {
			case err := <-readErr:
				if (err != nil) != tt.wantErr {
					t.Errorf("body read err = %v, want error: %v", err, tt.wantErr)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("handler never finished reading")
			}
		})
	}
}

// ResponseController không hỗ trợ (vd httptest.ResponseRecorder) thì chỉ log,
// request vẫn chạy tiếp
func TestReadTimeout_UnsupportedWriterStillServes(t *testing.T) {
	var buf bytes.Buffer
	ctx := logger.NewContext(t.Context(), logger.New(&buf, logger.Config{}))
	reached := false
	h := ReadTimeout(time.Second)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
	}))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/upload", nil).WithContext(ctx))

	if !reached {
		t.Error("handler not reached")
	}
	if !strings.Contains(buf.String(), "read deadline") {
		t.Errorf("want a warning about the read deadline, got:\n%s", buf.String())
	}
}
