package database

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Open mở pool rồi ping. url rỗng thì pgx đọc biến PG* (PGHOST, PGUSER...).
func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := parseConfig(url)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("db: pool connection: %w", err)
	}

	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: pool ping: %w", err)
	}

	return pool, nil
}

// parseConfig như pgxpool.ParseConfig, thêm PGPASSWORD_FILE (Docker secret),
// vì pgx không đọc biến này. Mật khẩu có sẵn trong url hay PGPASSWORD thì thắng.
func parseConfig(url string) (*pgxpool.Config, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("db: parse config: %w", err)
	}

	if file := os.Getenv("PGPASSWORD_FILE"); file != "" && cfg.ConnConfig.Password == "" {
		b, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("db: PGPASSWORD_FILE: %w", err)
		}
		cfg.ConnConfig.Password = strings.TrimRight(string(b), "\r\n")
	}
	return cfg, nil
}

// type Pingers map[string]*pgxpool.Pool

// func (p Pingers) Ping(ctx context.Context) error {
// 	names := slices.Sorted(maps.Keys(p))
// 	var errs []error
// 	for _, name := range names {
// 		if err := p[name].Ping(ctx); err != nil {
// 			errs = append(errs, fmt.Errorf("db: %s pool: %w", name, err))
// 		}
// 	}
// 	return errors.Join(errs...)
// }

func Close(p *pgxpool.Pool) {
	if p != nil {
		p.Close()
	}
}
