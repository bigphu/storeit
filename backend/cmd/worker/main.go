// cmd/worker chạy job nền trên River: giao event cho subscriber và job định kỳ
// (dọn phiên đăng nhập đã chết).
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/riverqueue/river"

	"storeit/internal/identity"
	"storeit/internal/platform/config"
	"storeit/internal/platform/database"
	"storeit/internal/platform/events"
	"storeit/internal/platform/jobs"
	"storeit/internal/platform/logger"
)

// Thời gian cho job đang chạy hoàn tất khi tắt; compose cho 30s
const stopTimeout = 25 * time.Second

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var cfg workerConfig
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

	// Subscriber và job có thể ghi thay đổi kèm event, nên worker cũng cần outbox
	insertClient, err := jobs.NewInsertClient(pool, log)
	if err != nil {
		return err
	}
	registry := events.NewRegistry()
	outbox := events.NewOutbox(registry, insertClient)

	identityMod, err := identity.New(identity.Deps{Pool: pool, Outbox: outbox, Config: cfg.Identity})
	if err != nil {
		return err
	}

	workers := river.NewWorkers()
	events.RegisterWorker(workers, pool, registry, identityMod.LoadActor)
	identityMod.RegisterWorkers(workers)

	client, err := jobs.NewWorkerClient(pool, log, cfg.Jobs, workers, identity.PeriodicJobs())
	if err != nil {
		return err
	}
	if err := client.Start(ctx); err != nil {
		return fmt.Errorf("worker: start: %w", err)
	}
	log.Info("worker started")
	<-ctx.Done()

	log.Info("worker stopping", slog.Duration("timeout", stopTimeout))
	stopCtx, cancel := context.WithTimeout(context.Background(), stopTimeout)
	defer cancel()
	if err := client.Stop(stopCtx); err != nil {
		return fmt.Errorf("worker: stop: %w", err)
	}
	return nil
}
