// Package handler nối API inventory (strict server sinh từ openapi.yaml) với
// service. Không kiểm tra quyền (service làm), không đụng DB.
package handler

//go:generate go tool oapi-codegen -config oapi.yaml openapi.yaml

import (
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi/v5"

	"storeit/internal/inventory/handler/api"
	"storeit/internal/inventory/service"
	"storeit/internal/platform/auth"
	"storeit/internal/platform/jwt"
	"storeit/internal/platform/web"
)

const baseURL = "/api/v1"

// Mặc định của danh sách: validator không chèn default của spec
const defaultPageSize = 50

// Handler cài đặt api.StrictServerInterface
type Handler struct {
	svc *service.Service
}

var _ api.StrictServerInterface = (*Handler)(nil)

func New(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

// Mount gắn mọi route của inventory lên r (router gốc của server). Mọi route
// cần access token; phần tử cuối của Middlewares chạy trước, nên thiếu token
// là 401 trước khi validator đọc body.
func (h *Handler) Mount(r chi.Router, tokens *jwt.Provider) error {
	spec, err := api.GetSwagger()
	if err != nil {
		return fmt.Errorf("inventory: load openapi spec: %w", err)
	}
	strict := api.NewStrictHandlerWithOptions(h, nil, api.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  web.RequestError,
		ResponseErrorHandlerFunc: web.WriteProblem,
	})
	api.HandlerWithOptions(strict, api.ChiServerOptions{
		BaseRouter: r,
		BaseURL:    baseURL,
		Middlewares: []api.MiddlewareFunc{
			web.ValidateRequests(spec, baseURL),
			auth.Middleware(tokens),
		},
		ErrorHandlerFunc: web.RequestError,
	})
	return nil
}

// Spec là spec OpenAPI của inventory (bản nhúng, ref sang api/common.yaml đã
// resolve), cho trang tài liệu API
func Spec() (*openapi3.T, error) {
	return api.GetSwagger()
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}
