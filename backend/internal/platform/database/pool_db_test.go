package database_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"storeit/internal/platform/database"
	"storeit/internal/platform/database/dbtest"
)

// Open không có DB: cấu hình sai báo ngay, DB không tới được thì lỗi trong
// ConnectTimeout chứ không treo
func TestOpen_Errors(t *testing.T) {
	ctx := context.Background()
	if _, err := database.Open(ctx, database.Config{MaxConns: 2, MinConns: 3}); err == nil {
		t.Error("want error for MinConns > MaxConns")
	}

	start := time.Now()
	_, err := database.Open(ctx, database.Config{
		URL:            "postgres://u:p@127.0.0.1:1/none?sslmode=disable",
		ConnectTimeout: time.Second,
	})
	if err == nil {
		t.Fatal("want error for unreachable database")
	}
	if d := time.Since(start); d > 10*time.Second {
		t.Errorf("Open took %v, want it bounded by ConnectTimeout", d)
	}
}

// newTable tạo bảng riêng cho test (DB test dùng chung giữa các test)
func newTable(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	name := "wtx_" + uuid.NewString()[:8]
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf("CREATE TABLE %s (v int)", name)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DROP TABLE "+name) })
	return name
}

func count(t *testing.T, pool *pgxpool.Pool, table string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM "+table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestWithTx_RealPostgres(t *testing.T) {
	pool := dbtest.Pool(t)
	ctx := context.Background()
	errDomain := errors.New("asset changed")

	t.Run("commit", func(t *testing.T) {
		table := newTable(t, pool)
		err := database.WithTx(ctx, pool, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, "INSERT INTO "+table+" VALUES (1)")
			return err
		})
		if err != nil || count(t, pool, table) != 1 {
			t.Errorf("err = %v, rows = %d, want committed row", err, count(t, pool, table))
		}
	})

	t.Run("error rolls back and keeps domain error", func(t *testing.T) {
		table := newTable(t, pool)
		err := database.WithTx(ctx, pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, "INSERT INTO "+table+" VALUES (1)"); err != nil {
				return err
			}
			return errDomain
		})
		if !errors.Is(err, errDomain) || count(t, pool, table) != 0 {
			t.Errorf("err = %v, rows = %d, want domain error and no row", err, count(t, pool, table))
		}
	})

	// Panic: rollback và trả kết nối về pool (không bị giữ mãi)
	t.Run("panic rolls back and releases connection", func(t *testing.T) {
		table := newTable(t, pool)
		func() {
			defer func() { _ = recover() }()
			_ = database.WithTx(ctx, pool, func(tx pgx.Tx) error {
				_, _ = tx.Exec(ctx, "INSERT INTO "+table+" VALUES (1)")
				panic("boom")
			})
		}()
		if n := count(t, pool, table); n != 0 {
			t.Errorf("rows = %d, want rollback", n)
		}
		if acquired := pool.Stat().AcquiredConns(); acquired != 0 {
			t.Errorf("acquired conns = %d, want connection returned to pool", acquired)
		}
	})

	// Request bị huỷ giữa chừng: vẫn rollback sạch, kết nối dùng lại được
	t.Run("cancelled ctx still rolls back", func(t *testing.T) {
		table := newTable(t, pool)
		cctx, cancel := context.WithCancel(ctx)
		err := database.WithTx(cctx, pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(cctx, "INSERT INTO "+table+" VALUES (1)"); err != nil {
				return err
			}
			cancel()
			return cctx.Err()
		})
		if !errors.Is(err, context.Canceled) || count(t, pool, table) != 0 {
			t.Errorf("err = %v, rows = %d, want canceled and no row", err, count(t, pool, table))
		}
	})
}
