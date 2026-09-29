// Package auth giữ người đang gọi (Actor) trong ctx, kiểm tra quyền và đọc bearer token.
//
// Router gắn Middleware một lần, liệt kê các operation không cần token:
//
//	r.Route("/api/v1", func(r chi.Router) {
//		r.Use(auth.Middleware(tokens, auth.Public("POST /api/v1/auth/login")))
//		identityMod.Handler.Mount(r)
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
