package middleware

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// ForOperations chỉ áp mws cho các operation liệt kê, dạng "METHOD <route
// pattern>" như auth.Public, vd "POST /api/v1/imports". Router sinh từ OpenAPI
// chỉ cho một danh sách Middlewares chung cho mọi operation của module, nên
// thông số riêng của một operation (body lớn hơn, đọc lâu hơn) gắn qua đây:
//
//	upload := middleware.ForOperations(web.MustOperations(spec, "/api/v1", "POST /api/v1/imports"),
//		middleware.BodyLimit(50<<20), middleware.ReadTimeout(10*time.Minute))
//
//	// oapi-codegen chạy middleware cuối danh sách trước: auth, rồi upload
//	// (nới giới hạn trước khi validator đọc body), rồi validate
//	Middlewares: []api.MiddlewareFunc{web.ValidateRequests(spec, "/api/v1"), upload, authMW}
//
// mws chạy theo thứ tự liệt kê. So khớp theo route pattern của chi nên phải
// chạy sau khi chi route xong (Middlewares của oapi-codegen, hoặc With).
func ForOperations(ops []string, mws ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	set := make(map[string]bool, len(ops))
	for _, op := range ops {
		set[op] = true
	}
	return func(next http.Handler) http.Handler {
		wrapped := next
		for i := len(mws) - 1; i >= 0; i-- {
			wrapped = mws[i](wrapped)
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var pattern string
			if rc := chi.RouteContext(r.Context()); rc != nil {
				pattern = rc.RoutePattern()
			}
			if set[r.Method+" "+pattern] {
				wrapped.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
