// Package handler nối API identity (strict server sinh từ openapi.yaml) với
// service. Không kiểm tra quyền (service làm), không đụng DB.
package handler

//go:generate go tool oapi-codegen -config oapi.yaml openapi.yaml

import (
	"fmt"
	"time"

	"github.com/go-chi/chi/v5"

	"storeit/internal/identity/handler/api"
	"storeit/internal/identity/service"
	"storeit/internal/platform/auth"
	"storeit/internal/platform/jwt"
	"storeit/internal/platform/middleware"
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

// Giới hạn theo IP cho các route công khai dễ bị dò: mỗi IP được burst lượt,
// hồi một lượt sau mỗi every. Refresh và logout không giới hạn: chúng cần
// cookie hợp lệ, và giới hạn refresh làm người dùng thật bị đăng xuất.
var authLimits = []struct {
	op    string
	every time.Duration
	burst int
}{
	// Đoán mật khẩu: 10 lần liền, sau đó 10 lần mỗi phút
	{"POST /api/v1/auth/login", 6 * time.Second, 10},
	// Form công khai gửi thư: 5 lần liền, sau đó mỗi phút một lần
	{"POST /api/v1/auth/password/forgot", time.Minute, 5},
	// Đoán token: token 32 byte không đoán nổi, giới hạn chỉ chặn spam
	{"POST /api/v1/auth/password/set", 6 * time.Second, 10},
}

// Mount gắn mọi route của identity lên r (router gốc của server). Thứ tự
// Middlewares: phần tử cuối chạy trước, nên auth chạy trước (thiếu token là
// 401, không phải 422), rồi giới hạn lượt (rẻ, trước khi đọc body), rồi validator.
func (h *Handler) Mount(r chi.Router, tokens *jwt.Provider) error {
	spec, err := api.GetSwagger()
	if err != nil {
		return fmt.Errorf("identity: load openapi spec: %w", err)
	}
	public := web.MustOperations(spec, baseURL,
		"POST /api/v1/auth/login", "POST /api/v1/auth/refresh", "POST /api/v1/auth/logout",
		"POST /api/v1/auth/password/forgot", "POST /api/v1/auth/password/set")

	mws := []api.MiddlewareFunc{web.ValidateRequests(spec, baseURL)}
	for _, l := range authLimits {
		mws = append(mws, middleware.ForOperations(web.MustOperations(spec, baseURL, l.op),
			middleware.RateLimit(l.every, l.burst)))
	}
	mws = append(mws, auth.Middleware(tokens, auth.Public(public...)))

	strict := api.NewStrictHandlerWithOptions(h, nil, api.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  web.RequestError,
		ResponseErrorHandlerFunc: web.WriteProblem,
	})
	api.HandlerWithOptions(strict, api.ChiServerOptions{
		BaseRouter:       r,
		BaseURL:          baseURL,
		Middlewares:      mws,
		ErrorHandlerFunc: web.RequestError,
	})
	return nil
}
