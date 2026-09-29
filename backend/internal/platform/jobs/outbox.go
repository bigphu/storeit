package jobs

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Payload của job cần được enqueue
type Job interface {
	Kind() string
}

// Enqueuer đưa job vào queue. Service và repository nhận interface này thay
// vì River, nên test có thể thay bằng fake.
type Enqueuer interface {
	// Enqueue insert job ngay, độc lập với transaction nào đang chạy
	Enqueue(ctx context.Context, job Job, opts ...Option) error

	// EnqueueTx insert job trong tx: commit thì job mới được chạy, rollback
	// thì job mất theo. Dùng khi job đi kèm một thay đổi dữ liệu; chỉ
	// repository gọi, bên trong database.WithTx.
	EnqueueTx(ctx context.Context, tx pgx.Tx, job Job, opts ...Option) error
}
