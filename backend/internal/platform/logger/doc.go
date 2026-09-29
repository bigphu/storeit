// Package logger dựng *slog.Logger: production ghi JSON, dev ghi dạng pretty.
//
// Gọi New một lần trong main, rồi cả app dùng thẳng slog. Attr gắn vào ctx
// bằng With (request ID, actor ID...) tự có trong mọi log ghi bằng ctx đó:
//
//	slog.SetDefault(logger.New(os.Stdout, cfg.Log))
//
//	ctx = logger.With(ctx, slog.String("request_id", id))
//	slog.InfoContext(ctx, "asset checked out", "asset_id", a.ID)
//
// Attr chỉ biết được ở bên trong request (actor sau khi verify token) thì thêm
// bằng AddToScope, để cả log ghi bằng ctx bên ngoài (access log) cũng có:
//
//	logger.AddToScope(r.Context(), slog.String("actor_id", id))
package logger
