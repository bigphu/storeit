// Package auth giữ người đang gọi (Actor) trong ctx, kiểm tra quyền và đọc bearer token.
//
// main dựng Middleware một lần, liệt kê các operation không cần token theo
// route pattern, rồi đưa cho Mount của từng module. Mount gắn nó vào
// Middlewares của oapi-codegen (chạy sau khi chi route xong), không dùng r.Use:
//
//	public := web.MustOperations(spec, "/api/v1", "POST /api/v1/auth/login") // gõ sai thì panic lúc khởi động
//	authMW := auth.Middleware(tokens, auth.Public(public...))
//	identityMod.Handler.Mount(r, authMW)
//
//	// trong handler.Mount; middleware cuối danh sách chạy trước, nên auth
//	// đứng sau web.ValidateRequests (thiếu token là 401, không phải 422)
//	api.HandlerWithOptions(strict, api.ChiServerOptions{
//		BaseRouter:       r,
//		BaseURL:          "/api/v1",
//		Middlewares:      []api.MiddlewareFunc{web.ValidateRequests(spec, "/api/v1"), authMW},
//		ErrorHandlerFunc: web.RequestError,
//	})
//
// Kiểm tra quyền nằm trong service, để đúng dù handler, worker hay subscriber
// gọi. Hằng quyền do module sở hữu khai báo trong domain của nó:
//
//	actor, err := auth.Require(ctx, domain.PermAssetCreate)
//
// Scheduled job chạy bằng SystemActor (mọi quyền, không có account):
//
//	ctx = auth.WithActor(ctx, auth.SystemActor)
package auth
