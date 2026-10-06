// cmd/seed ghi dữ liệu demo vào DB dev trống rồi thoát (make seed). DB đã có dữ
// liệu thì không làm gì. Không dùng ở production.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"storeit/internal/identity"
	"storeit/internal/inventory"
	"storeit/internal/platform/config"
	"storeit/internal/platform/database"
	"storeit/internal/platform/events"
	"storeit/internal/platform/jobs"
	"storeit/internal/platform/logger"
	"storeit/internal/seed"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var cfg seedConfig
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

	insertClient, err := jobs.NewInsertClient(pool, log)
	if err != nil {
		return err
	}
	outbox := events.NewOutbox(events.NewRegistry(), insertClient)
	identityMod, err := identity.New(identity.Deps{
		Pool: pool, Outbox: outbox, Jobs: jobs.NewRiver(insertClient), Config: cfg.Identity,
	})
	if err != nil {
		return err
	}
	inventoryMod, err := inventory.New(inventory.Deps{
		Pool: pool, Outbox: outbox, Accounts: identityMod.AccountReader(), Config: cfg.Inventory,
	})
	if err != nil {
		return err
	}

	sum, err := seed.Run(ctx, seed.Deps{
		Accounts: identityMod, Inventory: inventoryMod.Service(), Password: cfg.Seed.Password,
	})
	if err != nil {
		return err
	}
	if sum.Skipped {
		log.Info(`already seeded — run "make db-reset CONFIRM=yes" to start over`)
		return nil
	}
	log.Info("seeded demo data",
		slog.Int("accounts", sum.Accounts), slog.Int("statuses", sum.Statuses), slog.Int("types", sum.Types),
		slog.Int("assets", sum.Assets), slog.Int("retired", sum.Retired), slog.Int("profiles", sum.Profiles))
	return nil
}
