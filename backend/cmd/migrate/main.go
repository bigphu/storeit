// cmd/migrate chạy migration goose (migrations/) rồi migration của River, sau
// đó thoát. Compose chỉ start app và worker khi migrate thoát 0.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"storeit/internal/platform/config"
	"storeit/internal/platform/database"
	"storeit/internal/platform/logger"
	"storeit/migrations"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var cfg migrateConfig
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

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer func() { _ = sqlDB.Close() }()
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	if err != nil {
		return fmt.Errorf("migrate: goose: %w", err)
	}
	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("migrate: goose up: %w", err)
	}
	for _, r := range results {
		log.Info("applied migration", slog.String("source", r.Source.Path), slog.Duration("dur", r.Duration))
	}

	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return fmt.Errorf("migrate: river: %w", err)
	}
	res, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
	if err != nil {
		return fmt.Errorf("migrate: river up: %w", err)
	}
	log.Info("migrations up to date", slog.Int("goose_applied", len(results)), slog.Int("river_applied", len(res.Versions)))
	return nil
}
