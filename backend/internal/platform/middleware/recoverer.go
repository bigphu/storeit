package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"storeit/internal/platform/errs"
	"storeit/internal/platform/web"
)

// Recoverer bắt panic trong handler, log kèm stack rồi trả 500 problem+json.
// http.ErrAbortHandler thì để nguyên cho net/http xử lý.
func Recoverer(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
				log.ErrorContext(r.Context(), "panic recovered",
					"method", r.Method, "path", r.URL.Path,
					"panic", rec, "stack", string(debug.Stack()))

				web.RenderProblem(w, r, errs.NewFrom(fmt.Errorf("panic: %v", rec)))
			}()

			next.ServeHTTP(w, r)
		})
	}
}
