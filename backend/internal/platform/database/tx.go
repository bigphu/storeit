package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Beginner mở transaction: *pgxpool.Pool, hoặc pgx.Tx (khi đó là savepoint)
type Beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// WithTx chạy fn trong một transaction: fn trả nil thì commit, trả lỗi hoặc
// panic thì rollback. Lỗi của fn được trả nguyên (errors.Is vẫn khớp lỗi
// domain), kể cả khi rollback cũng lỗi.
//
// Khác pgx.BeginFunc ở chỗ đó: BeginFunc để lỗi rollback đè lên lỗi của fn,
// nên vd ErrAssetChanged (409) sẽ thành 500.
func WithTx(ctx context.Context, db Beginner, fn func(pgx.Tx) error) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("db: begin: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			return errors.Join(err, fmt.Errorf("db: rollback: %w", rbErr))
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("db: commit: %w", err)
	}
	return nil
}
