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
// log khi request xong. Đặt sau chimw.RequestID và trước Recoverer, để panic
// vẫn hiện thành 500 ở đây.
//
// Request được mở một logger.WithScope: actor_id do auth.Middleware thêm bằng
// logger.AddToScope cũng có trong dòng log này và log panic của Recoverer.
func RequestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ctx := logger.With(r.Context(), slog.String("request_id", chimw.GetReqID(r.Context())))
			ctx = logger.WithScope(ctx)
			r = r.WithContext(ctx)
			// Giữ nguyên Flusher/Hijacker của w, chỉ đếm status và bytes
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			status := ww.Status()
			if status == 0 { // handler không ghi gì, net/http tự trả 200
				status = http.StatusOK
			}
			level := slog.LevelInfo
			switch {
			case status >= 500:
				level = slog.LevelError
			case status >= 400:
				level = slog.LevelWarn
			}
			// RoutePattern chỉ có sau khi chi route xong nên phải đọc sau ServeHTTP
			var route string
			if rc := chi.RouteContext(ctx); rc != nil {
				route = rc.RoutePattern()
			}
			// Không log query, header hay body: có thể chứa token, dữ liệu cá nhân
			log.LogAttrs(ctx, level, "http request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("route", route),
				slog.Int("status", status),
				slog.Int("bytes", ww.BytesWritten()),
				slog.Duration("dur", time.Since(start)),
			)
		})
	}
}
