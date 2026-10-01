package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"storeit/internal/platform/logger"
)

// RequestLogger gắn request_id vào ctx cho mọi log phía sau, rồi ghi một dòng
// log khi request xong. Đặt sau RequestID và trước Recoverer, để panic vẫn hiện
// thành 500 ở đây.
//
// log được gắn vào ctx (logger.NewContext) để code phía sau, vd
// web.WriteProblem, log bằng logger này thay vì slog.Default().
//
// Request được mở một logger.WithScope: actor_id do auth.Middleware thêm bằng
// logger.AddToScope cũng có trong dòng log này và log panic của Recoverer.
func RequestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ctx := logger.With(r.Context(), slog.String("request_id", chimw.GetReqID(r.Context())))
			ctx = logger.WithScope(ctx)
			ctx = logger.NewContext(ctx, log)
			r = r.WithContext(ctx)
			// Giữ nguyên Flusher/Hijacker của w, chỉ đếm status và bytes
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)

			// Ghi log trong defer để request bị huỷ giữa chừng (panic
			// http.ErrAbortHandler đi xuyên qua) vẫn có dòng log. Không recover:
			// panic đi tiếp nguyên stack.
			completed := false
			defer func() {
				logRequest(log, r, ww, time.Since(start), !completed)
			}()

			next.ServeHTTP(ww, r)
			completed = true
		})
	}
}

// clientIPOrRemote: IP do ClientIP đặt; không có middleware đó thì host của
// RemoteAddr (không đọc X-Forwarded-For)
func clientIPOrRemote(r *http.Request) string {
	if ip := ClientIPFrom(r.Context()); ip != "" {
		return ip
	}
	if a := parseAddr(r.RemoteAddr); a.IsValid() {
		return a.String()
	}
	return ""
}

func logRequest(log *slog.Logger, r *http.Request, ww chimw.WrapResponseWriter, dur time.Duration, aborted bool) {
	status := ww.Status()
	if status == 0 && !aborted { // handler không ghi gì, net/http tự trả 200
		status = http.StatusOK
	}
	level := slog.LevelInfo
	switch {
	case status >= 500:
		level = slog.LevelError
	case status >= 400, aborted:
		level = slog.LevelWarn
	}
	// RoutePattern chỉ có sau khi chi route xong nên phải đọc sau ServeHTTP
	var route string
	if rc := chi.RouteContext(r.Context()); rc != nil {
		route = rc.RoutePattern()
	}
	// Không log query, header hay body: có thể chứa token, dữ liệu cá nhân
	attrs := []slog.Attr{
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.String("client_ip", clientIPOrRemote(r)),
		slog.String("route", route),
		slog.Int("status", status),
		slog.Int("bytes", ww.BytesWritten()),
		slog.Duration("dur", dur),
	}
	if aborted {
		attrs = append(attrs, slog.Bool("aborted", true))
	}
	log.LogAttrs(r.Context(), level, "http request", attrs...)
}
