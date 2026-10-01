// cmd/server chạy HTTP API.
//
//	go run ./cmd/server   // hoặc make run-api; cần .env.local và Postgres đã migrate
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"storeit/internal/identity"
	"storeit/internal/platform/config"
	"storeit/internal/platform/database"
	"storeit/internal/platform/events"
	"storeit/internal/platform/jobs"
	"storeit/internal/platform/jwt"
	"storeit/internal/platform/logger"
	"storeit/internal/platform/server"
	"storeit/internal/platform/web"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var cfg serverConfig
	if err := config.Load(&cfg); err != nil {
		return err
	}
	log := logger.New(os.Stdout, cfg.Log)
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.Open(ctx, cfg.DB)
	if err != nil {
		return err
	}
	defer database.Close(pool)

	tokens, err := jwt.New(cfg.JWT)
	if err != nil {
		return err
	}
	insertClient, err := jobs.NewInsertClient(pool, log)
	if err != nil {
		return err
	}
	// API và worker dựng cùng registry: Append cần biết enqueue job nào.
	// Module có subscriber sẽ đăng ký vào đây (activity, M2).
	registry := events.NewRegistry()
	outbox := events.NewOutbox(registry, insertClient)

	identityMod, err := identity.New(identity.Deps{Pool: pool, Tokens: tokens, Outbox: outbox, Config: cfg.Identity})
	if err != nil {
		return err
	}
	if err := identityMod.Bootstrap(ctx); err != nil {
		return err
	}

	srv := server.New(cfg.HTTP, log)
	srv.Router().Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			_ = web.Text(w, http.StatusServiceUnavailable, "database unavailable")
			return
		}
		_ = web.Text(w, http.StatusOK, "ok")
	})
	if err := identityMod.Mount(srv.Router()); err != nil {
		return err
	}
	return srv.Run(ctx)
}
