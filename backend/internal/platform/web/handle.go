package web

import (
	"net/http"

	"storeit/internal/platform/errs"
)

type HandlerFunc func(http.ResponseWriter, *http.Request) error

func Handle(h HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			WriteProblem(w, r, err)
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

// MethodNotAllowedHandler trả 405 dạng problem+json
func MethodNotAllowedHandler() http.HandlerFunc {
	return Handle(func(w http.ResponseWriter, r *http.Request) error {
		return ErrMethodNotAllowed.With(
			errs.WithDetailf("%s is not allowed on %s", r.Method, r.URL.Path))
	})
}
