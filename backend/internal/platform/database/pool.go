package database

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Open mở pool theo cfg rồi ping. cfg.URL rỗng thì pgx đọc biến PG* (PGHOST,
// PGUSER...).
func Open(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	pc, err := parseConfig(cfg)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		return nil, fmt.Errorf("db: pool connection: %w", err)
	}

	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: pool ping: %w", err)
	}

	return pool, nil
}

// parseConfig như pgxpool.ParseConfig(cfg.URL) rồi áp thông số pool của cfg,
// thêm PGPASSWORD_FILE (Docker secret) vì pgx không đọc biến này. Mật khẩu có
// sẵn trong URL hay PGPASSWORD thì skip.
func parseConfig(cfg Config) (*pgxpool.Config, error) {
	pc, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("db: parse config: %w", err)
	}

	if file := os.Getenv("PGPASSWORD_FILE"); file != "" && pc.ConnConfig.Password == "" {
		b, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("db: PGPASSWORD_FILE: %w", err)
		}
		pc.ConnConfig.Password = strings.TrimRight(string(b), "\r\n")
	}

	cfg = cfg.withDefaults()
	pc.MaxConns = cfg.MaxConns
	pc.MinConns = cfg.MinConns
	pc.MaxConnLifetime = cfg.MaxConnLifetime
	pc.MaxConnLifetimeJitter = cfg.MaxConnLifetimeJitter
	pc.MaxConnIdleTime = cfg.MaxConnIdleTime
	pc.HealthCheckPeriod = cfg.HealthCheckPeriod
	pc.ConnConfig.ConnectTimeout = cfg.ConnectTimeout
	return pc, nil
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
