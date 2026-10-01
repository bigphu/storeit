package web

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"storeit/internal/platform/errs"
)

type HandlerFunc func(http.ResponseWriter, *http.Request) error

// Handle đổi HandlerFunc trả lỗi thành http.HandlerFunc: lỗi được trả về dạng
// problem+json, hoặc chỉ log nếu handler đã bắt đầu ghi response.
func Handle(h HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ww := WrapWriter(w, r)
		if err := h(ww, r); err != nil {
			WriteProblem(ww, r, err)
		}
	}
}

// NotFoundHandler trả 404 dạng problem+json, thay cho trang text mặc định của
// chi - client không phải đoán format tuỳ theo lỗi rơi vào đâu
func NotFoundHandler() http.HandlerFunc {
	return Handle(func(w http.ResponseWriter, r *http.Request) error {
		return ErrRouteNotFound.With(
			errs.WithDetailf("No route for %s %s", r.Method, r.URL.Path))
	})
}

// MethodNotAllowedHandler trả 405 dạng problem+json, kèm header Allow (RFC 9110
// bắt buộc) liệt kê method route nhận. chi chỉ tự đặt Allow cho handler mặc
// định của nó, handler riêng phải tự hỏi router.
func MethodNotAllowedHandler() http.HandlerFunc {
	return Handle(func(w http.ResponseWriter, r *http.Request) error {
		if allowed := allowedMethods(r); len(allowed) > 0 {
			w.Header().Set("Allow", strings.Join(allowed, ", "))
		}
		return ErrMethodNotAllowed.With(
			errs.WithDetailf("%s is not allowed on %s", r.Method, r.URL.Path))
	})
}

// allowedMethods là các method khớp với path của request. rc.Routes là router
// gốc (chi giữ nguyên khi vào router con), Match tự đi xuống router con, nên
// dùng path đầy đủ; RawPath nếu có, giống cách chi route.
func allowedMethods(r *http.Request) []string {
	rc := chi.RouteContext(r.Context())
	if rc == nil || rc.Routes == nil {
		return nil
	}
	path := r.URL.RawPath
	if path == "" {
		path = r.URL.Path
	}
	var allowed []string
	for _, m := range []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodOptions} {
		if rc.Routes.Match(chi.NewRouteContext(), m, path) {
			allowed = append(allowed, m)
		}
	}
	return allowed
}
