// Tạm thời chỉ chạy thử package logger, sẽ thay bằng API server (M0):
//
//	go run ./cmd/server
//	LOG_FORMAT=pretty LOG_LEVEL=debug go run ./cmd/server
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"time"

	"storeit/internal/platform/config"
	"storeit/internal/platform/logger"
)

func main() {
	var cfg serverConfig
	if err := config.Load(&cfg); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	slog.SetDefault(logger.New(os.Stdout, cfg.Log))

	// Đừng đặt key trùng time/level/msg/source: JSON bị lặp key, pretty thì mất attr
	slog.Info("server starting", "addr", ":8080", "log_format", cfg.Log.Format, "log_level", cfg.Log.Level)
	slog.Debug("chỉ hiện khi LOG_LEVEL=debug")

	// Giả lập middleware: gắn request ID và actor ID vào ctx
	ctx := logger.With(context.Background(), slog.String("request_id", "req-8f3a"))
	ctx = logger.With(ctx, slog.String("actor_id", "acc-42"))
	checkOutAsset(ctx, "asset-17")

	// Group gom attr dưới một tiền tố
	slog.WarnContext(ctx, "slow query",
		slog.Group("db", "table", "inventory.assets", "dur", 730*time.Millisecond))

	// Chuỗi nhiều dòng (stack trace) được pretty in thành khối riêng
	slog.ErrorContext(ctx, "panic recovered",
		"err", errors.New("index out of range"),
		"stack", string(debug.Stack()))
}

// checkOutAsset đóng vai service: chỉ nhận ctx, không nhận logger, mà log vẫn
// có request_id và actor_id
func checkOutAsset(ctx context.Context, assetID string) {
	slog.InfoContext(ctx, "asset checked out", "asset_id", assetID, "to_member", "mem-5")
}
