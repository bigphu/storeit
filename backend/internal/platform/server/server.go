package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/go-chi/chi/v5"

	"storeit/internal/platform/middleware"
	"storeit/internal/platform/web"
)

type Server struct {
	cfg    Config
	log    *slog.Logger
	router *chi.Mux
}

// New dựng router có sẵn middleware chung (giới hạn body, request ID, access
// log, bắt panic) và trả 404/405 dạng problem+json. log nil thì dùng
// slog.Default().
func New(cfg Config, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	cfg = cfg.withDefaults()

	r := chi.NewRouter()
	// Giới hạn body cho mọi route: strict server của oapi-codegen đọc thẳng
	// r.Body. Operation upload tự nới bằng middleware.ForOperations + BodyLimit
	r.Use(middleware.BodyLimit(cfg.MaxBodyBytes))
	r.Use(middleware.NoSniff)
	r.Use(middleware.ClientIP(cfg.TrustedProxies))
	r.Use(middleware.RequestID)
	r.Use(middleware.RequestLogger(log))
	r.Use(middleware.Recoverer())

	// Để 404 và 405 cũng ra problem+json như mọi lỗi khác, thay vì trang text
	// mặc định của chi -- client chỉ phải hiểu một format lỗi duy nhất
	r.NotFound(web.NotFoundHandler())
	r.MethodNotAllowed(web.MethodNotAllowedHandler())

	return &Server{
		cfg:    cfg,
		log:    log,
		router: r,
	}
}

// Router để các module nghiệp vụ tự đăng ký route của mình lên đó
func (s *Server) Router() chi.Router {
	return s.router
}

// Handler dùng cho httptest, không cần mở port
func (s *Server) Handler() http.Handler {
	return s.router
}

// Run mở cổng cfg.Addr rồi phục vụ cho tới khi ctx bị huỷ. Cổng bận thì trả
// lỗi ngay, không log "listening" trước.
func (s *Server) Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return fmt.Errorf("server: %w", err)
	}
	return s.Serve(ctx, ln)
}

// Serve phục vụ trên ln (Serve sẽ đóng ln). Khi ctx bị huỷ thì ngừng nhận
// request mới, chờ request đang chạy tối đa ShutdownTimeout, quá thì cắt ngang.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	srv := &http.Server{
		Handler:           s.router,
		ReadHeaderTimeout: s.cfg.ReadHeaderTimeout,
		ReadTimeout:       s.cfg.ReadTimeout,
		IdleTimeout:       s.cfg.IdleTimeout,
		// Lỗi nội bộ của net/http (TLS handshake...) cũng đi qua slog
		ErrorLog: slog.NewLogLogger(s.log.Handler(), slog.LevelWarn),
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	s.log.Info("server listening", slog.String("addr", ln.Addr().String()))

	select {
	case err := <-serveErr:
		// Chưa gọi Shutdown nên Serve dừng là lỗi thật
		return fmt.Errorf("server: %w", err)
	case <-ctx.Done():
	}

	s.log.Info("server shutting down", slog.Duration("timeout", s.cfg.ShutdownTimeout))
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		// Hết giờ mà còn request chạy: đóng hẳn kết nối thay vì bỏ lại
		err = fmt.Errorf("server: shutdown: %w", err)
		if closeErr := srv.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("server: close: %w", closeErr))
		}
		return err
	}
	s.log.Info("server stopped")
	return nil
}
