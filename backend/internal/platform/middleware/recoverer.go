package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"storeit/internal/platform/errs"
	"storeit/internal/platform/logger"
	"storeit/internal/platform/web"
)

// Recoverer bắt panic trong handler, log kèm stack rồi trả 500 problem+json.
// Handler đã ghi một phần response trước khi panic thì chỉ log, không ghi
// thêm gì. http.ErrAbortHandler thì để nguyên cho net/http xử lý.
//
// Log bằng logger trong ctx (RequestLogger đặt vào), nên đặt sau RequestLogger.
func Recoverer() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Bọc để RenderProblem biết response đã bắt đầu gửi chưa
			w = web.WrapWriter(w, r)
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				// ErrAbortHandler là cố ý huỷ request, để net/http tự xử lý
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				// request ID, actor ID có sẵn trong ctx (logger.With)
				logger.FromContext(r.Context()).ErrorContext(r.Context(), "panic recovered",
					"method", r.Method, "path", r.URL.Path,
					"panic", rec, "stack", string(debug.Stack()))

				web.RenderProblem(w, r, errs.NewFrom(fmt.Errorf("panic: %v", rec)))
			}()

			next.ServeHTTP(w, r)
		})
	}
}
