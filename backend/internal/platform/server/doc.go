// Package server chạy http.Server có timeout và tắt êm khi ctx bị huỷ.
//
// Server có sẵn middleware chung (request ID, access log, bắt panic) và trả
// 404/405 dạng problem+json. Module nghiệp vụ chỉ việc gắn route lên Router().
//
// Run không tự bắt signal, main phải đổi SIGINT/SIGTERM thành ctx bị huỷ:
//
//	type Config struct {
//		HTTP server.Config `envPrefix:"HTTP_"` // HTTP_ADDR, HTTP_IDLE_TIMEOUT...
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
// Trong test dùng Handler() với httptest, hoặc Serve với listener ":0".
package server
