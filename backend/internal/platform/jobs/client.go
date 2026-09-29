package jobs

import (
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

// Queue của app. Job chọn queue qua InsertOpts() của args hoặc WithQueue;
// queue mới phải thêm vào NewWorkerClient, nếu không job nằm chờ mãi.
const (
	QueueDefault = river.QueueDefault
	// Giao event cho subscriber (events.HandleEventArgs), tách riêng để job
	// nặng như import không làm chậm audit trail
	QueueEvents = "events"
)

// DefaultMaxAttempts: số lần chạy tối đa của một job trước khi River bỏ
// (discard). River lùi thời gian retry theo luỹ thừa bậc 4 của số lần thử, nên
// 10 lần trải ra khoảng 7 tiếng, đủ cho sự cố tạm thời mà không kéo dài hàng
// tuần như mặc định 25 của River. Job cần khác thì tự đặt trong InsertOpts().
const DefaultMaxAttempts = 10

// Số job chạy song song mỗi queue trên một process worker
const maxWorkersPerQueue = 10

// NewInsertClient dựng client chỉ để insert job, cho cmd/api (không chạy job).
// MaxAttempts gắn vào job lúc insert, nên default cũng phải đặt ở đây.
func NewInsertClient(pool *pgxpool.Pool) (*river.Client[pgx.Tx], error) {
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Logger:      slog.Default(),
		MaxAttempts: DefaultMaxAttempts,
	})
	if err != nil {
		return nil, fmt.Errorf("jobs: insert client: %w", err)
	}
	return client, nil
}

// NewWorkerClient dựng client chạy job cho cmd/worker, nghe mọi queue của app.
// periodic là scheduled job (nhắc hạn trả, hết bảo hành...), có thể nil.
func NewWorkerClient(pool *pgxpool.Pool, workers *river.Workers, periodic []*river.PeriodicJob) (*river.Client[pgx.Tx], error) {
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Logger:      slog.Default(),
		MaxAttempts: DefaultMaxAttempts,
		Queues: map[string]river.QueueConfig{
			QueueDefault: {MaxWorkers: maxWorkersPerQueue},
			QueueEvents:  {MaxWorkers: maxWorkersPerQueue},
		},
		Workers:      workers,
		PeriodicJobs: periodic,
	})
	if err != nil {
		return nil, fmt.Errorf("jobs: worker client: %w", err)
	}
	return client, nil
}
