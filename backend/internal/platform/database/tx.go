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

// WithTx chạy fn trong một transaction: fn trả nil thì commit, trả lỗi, panic
// hay runtime.Goexit thì rollback (panic vẫn được ném tiếp). Lỗi của fn được
// trả nguyên (errors.Is vẫn khớp lỗi domain), kể cả khi rollback cũng lỗi.
//
// Khác pgx.BeginFunc ở chỗ đó: BeginFunc để lỗi rollback đè lên lỗi của fn,
// nên vd ErrAssetChanged (409) sẽ thành 500.
func WithTx(ctx context.Context, db Beginner, fn func(pgx.Tx) error) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("db: begin: %w", err)
	}

	// Rollback dùng ctx không bị huỷ: request bị huỷ giữa chừng thì pgx đóng
	// luôn kết nối nếu rollback bằng ctx đã huỷ, thay vì rollback rồi trả về pool
	rbCtx := context.WithoutCancel(ctx)

	// fn panic hay thoát bằng runtime.Goexit (t.Fatal trong test) thì không
	// về tới dưới; defer này vẫn rollback để kết nối không bị giữ mãi
	finished := false
	defer func() {
		if !finished {
			_ = tx.Rollback(rbCtx)
		}
	}()

	err = fn(tx)
	finished = true
	if err != nil {
		if rbErr := tx.Rollback(rbCtx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			return errors.Join(err, fmt.Errorf("db: rollback: %w", rbErr))
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("db: commit: %w", err)
	}
	return nil
}
