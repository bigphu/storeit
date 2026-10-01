// Package server chạy http.Server có timeout và tắt êm khi ctx bị huỷ.
//
// Router có sẵn chuỗi middleware chung (giới hạn body, nosniff, IP client,
// request ID, access log, bắt panic; xem package middleware) và trả 404/405
// dạng problem+json (405 kèm header Allow). Module nghiệp vụ chỉ việc gắn route
// lên Router().
//
// Run không tự bắt signal, main phải đổi SIGINT/SIGTERM thành ctx bị huỷ:
//
//	type serverConfig struct {
//		HTTP server.Config // HTTP_ADDR, HTTP_MAX_BODY_BYTES, HTTP_TRUSTED_PROXIES...
//	}
//
//	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
//	defer stop()
//
//	srv := server.New(cfg.HTTP, slog.Default())
//	inventory.Routes(srv.Router())
//	if err := srv.Run(ctx); err != nil {
//		log.Fatal(err)
//	}
//
// chi chỉ dựng chuỗi middleware khi có route đầu tiên: router chưa có route nào
// thì request bỏ qua mọi middleware (chỉ gặp trong test).
//
// Trong test dùng Handler() với httptest, hoặc Serve với listener ":0".
package server
