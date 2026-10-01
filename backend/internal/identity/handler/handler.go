// Package handler nối API identity (strict server sinh từ openapi.yaml) với
// service. Không kiểm tra quyền (service làm), không đụng DB.
package handler

//go:generate go tool oapi-codegen -config oapi.yaml openapi.yaml

import (
	"fmt"

	"github.com/go-chi/chi/v5"

	"storeit/internal/identity/handler/api"
	"storeit/internal/identity/service"
	"storeit/internal/platform/auth"
	"storeit/internal/platform/jwt"
	"storeit/internal/platform/web"
)

const baseURL = "/api/v1"

// Handler cài đặt api.StrictServerInterface
type Handler struct {
	svc    *service.Service
	cookie CookieSettings
}

var _ api.StrictServerInterface = (*Handler)(nil)

func New(svc *service.Service, cookie CookieSettings) *Handler {
	return &Handler{svc: svc, cookie: cookie}
}

// Mount gắn mọi route của identity lên r (router gốc của server). Thứ tự
// Middlewares: phần tử cuối chạy trước, nên auth chạy trước validator (thiếu
// token là 401, không phải 422).
func (h *Handler) Mount(r chi.Router, tokens *jwt.Provider) error {
	spec, err := api.GetSwagger()
	if err != nil {
		return fmt.Errorf("identity: load openapi spec: %w", err)
	}
	public := web.MustOperations(spec, baseURL,
		"POST /api/v1/auth/login", "POST /api/v1/auth/refresh", "POST /api/v1/auth/logout")

	strict := api.NewStrictHandlerWithOptions(h, nil, api.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  web.RequestError,
		ResponseErrorHandlerFunc: web.WriteProblem,
	})
	api.HandlerWithOptions(strict, api.ChiServerOptions{
		BaseRouter: r,
		BaseURL:    baseURL,
		Middlewares: []api.MiddlewareFunc{
			web.ValidateRequests(spec, baseURL),
			auth.Middleware(tokens, auth.Public(public...)),
		},
		ErrorHandlerFunc: web.RequestError,
	})
	return nil
}
