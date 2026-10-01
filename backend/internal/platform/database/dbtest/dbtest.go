// Package dbtest dựng Postgres thật cho integration test (testcontainers-go),
// đã chạy mọi goose migration và migration của River.
//
//	func TestSaveCheckOut(t *testing.T) {
//		pool := dbtest.Pool(t)
//		...
//	}
//
// Container dựng một lần cho mỗi test binary (mỗi package) và dùng chung giữa
// các test, nên test không được giả định DB trống: tạo dữ liệu với ID mới và
// chỉ kiểm tra dữ liệu của mình. Ryuk của testcontainers xoá container khi
// test binary thoát.
//
// Máy không có Docker thì test bị skip; trên CI (CI=true) thì fail, để test
// DB không lặng lẽ bị bỏ qua.
package dbtest

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"storeit/internal/platform/database"
	"storeit/migrations"
)

// Cùng bản với compose.yml
const image = "postgres:18-alpine"

var (
	once    sync.Once
	pool    *pgxpool.Pool
	initErr error
)

// Pool trả pool tới DB test dùng chung của package
func Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if os.Getenv("CI") == "" {
		testcontainers.SkipIfProviderIsNotHealthy(t)
	}
	once.Do(func() { pool, initErr = start(context.Background()) })
	if initErr != nil {
		t.Fatalf("dbtest: %v", initErr)
	}
	return pool
}

func start(ctx context.Context) (*pgxpool.Pool, error) {
	ctr, err := postgres.Run(ctx, image,
		postgres.WithDatabase("storeit"),
		postgres.WithUsername("app_user"),
		postgres.WithPassword("app_pw"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		return nil, fmt.Errorf("start postgres: %w", err)
	}
	url, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, fmt.Errorf("connection string: %w", err)
	}
	// Mở qua database.Open để test chạy đúng đường của app (pool cùng thông
	// số mặc định, đủ kết nối cho River worker trong test)
	p, err := database.Open(ctx, database.Config{URL: url})
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}
	if err := migrate(ctx, p); err != nil {
		p.Close()
		return nil, err
	}
	return p, nil
}

// migrate làm đúng việc cmd/migrate làm: goose rồi River
func migrate(ctx context.Context, p *pgxpool.Pool) error {
	sqlDB := stdlib.OpenDBFromPool(p)
	defer func() { _ = sqlDB.Close() }()

	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	if err != nil {
		return fmt.Errorf("goose: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}

	migrator, err := rivermigrate.New(riverpgxv5.New(p), nil)
	if err != nil {
		return fmt.Errorf("river migrate: %w", err)
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return fmt.Errorf("river migrate up: %w", err)
	}
	return nil
}
