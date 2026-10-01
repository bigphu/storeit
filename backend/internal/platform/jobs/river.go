package jobs

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

type River struct {
	client *river.Client[pgx.Tx]
}

var _ Enqueuer = (*River)(nil)

func NewRiver(client *river.Client[pgx.Tx]) *River {
	return &River{client: client}
}

// Insert một job vào queue worker
func (r *River) Enqueue(ctx context.Context, job Job, opts ...Option) error {
	if _, err := r.client.Insert(ctx, job, parseOpts(opts...)); err != nil {
		return fmt.Errorf("jobs: enqueue %s: %w", job.Kind(), err)
	}
	return nil
}

// Insert một job vào queue worker trong một transaction
func (r *River) EnqueueTx(ctx context.Context, tx pgx.Tx, job Job, opts ...Option) error {
	if _, err := r.client.InsertTx(ctx, tx, job, parseOpts(opts...)); err != nil {
		return fmt.Errorf("jobs: enqueue %s: %w", job.Kind(), err)
	}
	return nil
}

// Field nào bằng 0 thì River tự lấy từ InsertOpts() của job, rồi tới default
// của client, nên không cần kiểm tra trước khi gán
func parseOpts(opts ...Option) *river.InsertOpts {
	o := &Options{}
	for _, opt := range opts {
		if opt != nil {
			opt(o)
		}
	}

	riverOpts := &river.InsertOpts{
		Queue:       o.Queue,
		Priority:    o.Priority,
		ScheduledAt: o.ScheduledAt,
		MaxAttempts: o.MaxAttempts,
	}
	if o.UniqueByArgs {
		riverOpts.UniqueOpts = river.UniqueOpts{
			ByArgs: true,
			// River bắt buộc có available, pending, running, scheduled
			ByState: []rivertype.JobState{
				rivertype.JobStateAvailable,
				rivertype.JobStatePending,
				rivertype.JobStateRunning,
				rivertype.JobStateRetryable,
				rivertype.JobStateScheduled,
			},
		}
	}

	return riverOpts
}
