package auth

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"storeit/internal/platform/errs"
	"storeit/internal/platform/jwt"
	"storeit/internal/platform/logger"
	"storeit/internal/platform/web"
)

var (
	ErrMissingToken = errs.Unauthorized(
		"/errors/missing-token", "Missing bearer token",
		errs.WithDetail("Send the access token as: Authorization: Bearer <token>."))

	ErrInvalidToken = errs.Unauthorized(
		"/errors/invalid-token", "Invalid token",
		errs.WithDetail("The access token is invalid or expired."))
)

type options struct {
	public map[string]bool
}

type Option func(*options)

// Public đánh dấu operation không cần token, dạng "METHOD <route pattern>"
// giống path trong OpenAPI, vd "GET /api/v1/files/{id}". Router sinh từ
// OpenAPI gắn mọi path của module cùng lúc, nên operation public được liệt kê
// ở đây thay vì tách thành group riêng.
//
// So khớp theo route pattern của chi, nên Middleware phải chạy sau khi chi
// route xong: gắn qua Middlewares của oapi-codegen, không dùng r.Use:
//
//	api.HandlerWithOptions(strict, api.ChiServerOptions{
//		BaseRouter: r,
//		BaseURL:    "/api/v1",
//		// cuối danh sách chạy trước: auth trước khi validate
//		Middlewares: []api.MiddlewareFunc{
//			web.ValidateRequests(spec, "/api/v1"),
//			auth.Middleware(tokens, auth.Public("POST /api/v1/auth/login")),
//		},
//		ErrorHandlerFunc: web.RequestError,
//	})
//
// Kiểm danh sách với spec lúc khởi động để bắt lỗi gõ:
// auth.Public(web.MustOperations(spec, "/api/v1", "POST /api/v1/auth/login")...)
//
// Lỡ gắn bằng r.Use thì chưa có route pattern, không operation nào được coi
// là public: thiếu token là 401 chứ không lặng lẽ mở route ra.
func Public(ops ...string) Option {
	return func(o *options) {
		for _, op := range ops {
			o.public[op] = true
		}
	}
}

// Middleware đọc bearer token, verify bằng tokens rồi đặt Actor vào ctx.
// Service kiểm tra quyền bằng Require. actor_id được thêm vào scope log của
// request (logger.AddToScope), nên access log cũng có.
func Middleware(tokens *jwt.Provider, opts ...Option) func(http.Handler) http.Handler {
	o := options{public: map[string]bool{}}
	for _, opt := range opts {
		opt(&o)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if o.public[r.Method+" "+routePattern(r)] {
				next.ServeHTTP(w, r)
				return
			}

			raw, ok := bearerToken(r)
			if !ok {
				w.Header().Set("WWW-Authenticate", `Bearer`)
				web.WriteProblem(w, r, ErrMissingToken)
				return
			}

			claims, err := tokens.Verify(raw)
			if err != nil {
				unauthorized(w, r, err)
				return
			}
			accountID, err := claims.UserID()
			if err != nil {
				unauthorized(w, r, err)
				return
			}

			ctx := WithActor(r.Context(), Actor{AccountID: accountID, Permissions: claims.Permissions})
			logger.AddToScope(ctx, slog.String("actor_id", accountID.String()))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// routePattern là pattern chi đã khớp, vd /api/v1/files/{id}. Chưa route
// xong (middleware gắn bằng r.Use) thì rỗng hoặc dở dang, không khớp Public nào.
func routePattern(r *http.Request) string {
	if rc := chi.RouteContext(r.Context()); rc != nil {
		return rc.RoutePattern()
	}
	return ""
}

func unauthorized(w http.ResponseWriter, r *http.Request, cause error) {
	w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
	web.WriteProblem(w, r, ErrInvalidToken.With(errs.WithCause(cause)))
}

// bearerToken lấy token sau "Bearer " (scheme không phân biệt hoa thường)
func bearerToken(r *http.Request) (string, bool) {
	const prefix = "bearer "
	h := r.Header.Get("Authorization")
	if len(h) < len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return "", false
	}
	raw := strings.TrimSpace(h[len(prefix):])
	return raw, raw != ""
}
