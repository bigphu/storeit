package events

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"storeit/internal/platform/events/db"
	"storeit/internal/platform/jobs"
	"storeit/internal/platform/logger"
)

// HandleEventArgs là job giao một event cho một subscriber. Mỗi subscriber
// một job, nên subscriber này lỗi không chặn hay chạy lại subscriber khác.
type HandleEventArgs struct {
	EventID    uuid.UUID `json:"event_id"`
	Subscriber string    `json:"subscriber"`
}

func (HandleEventArgs) Kind() string { return "platform.handle_event" }

// Unique theo args: cùng event, cùng subscriber thì chỉ có một job, kể cả khi
// job trước đã xong
func (HandleEventArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:      jobs.QueueEvents,
		UniqueOpts: river.UniqueOpts{ByArgs: true},
	}
}

type HandleEventWorker struct {
	river.WorkerDefaults[HandleEventArgs]

	q         *db.Queries
	registry  *Registry
	loadActor jobs.ActorLoader
}

// RegisterWorker thêm HandleEventWorker vào workers của cmd/worker.
// loadActor nạp quyền hiện tại của account đã gây ra event (identity cung cấp).
func RegisterWorker(workers *river.Workers, pool *pgxpool.Pool, registry *Registry, loadActor jobs.ActorLoader) {
	river.AddWorker(workers, &HandleEventWorker{
		q:         db.New(pool),
		registry:  registry,
		loadActor: loadActor,
	})
}

// Work nạp event, đặt lại actor gốc vào ctx rồi gọi subscriber, để việc làm
// tiếp theo được ghi là của người đã gây ra thay đổi. Lỗi thì River retry.
func (w *HandleEventWorker) Work(ctx context.Context, job *river.Job[HandleEventArgs]) error {
	h, ok := w.registry.handler(job.Args.Subscriber)
	if !ok {
		// Không huỷ job: có thể API đã deploy bản có subscriber mới mà worker
		// chưa. Lỗi thường để River retry (DefaultMaxAttempts, khoảng 7 tiếng);
		// subscriber đã bị gỡ hẳn thì job bị discard sau lần thử cuối.
		return fmt.Errorf("events: subscriber %q not registered in this worker", job.Args.Subscriber)
	}

	row, err := w.q.GetEvent(ctx, job.Args.EventID)
	if errors.Is(err, pgx.ErrNoRows) {
		return river.JobCancel(fmt.Errorf("events: event %s not found", job.Args.EventID))
	}
	if err != nil {
		return fmt.Errorf("events: load %s: %w", job.Args.EventID, err)
	}
	e := fromRow(row)

	ctx = logger.With(ctx,
		slog.String("event_id", e.ID.String()),
		slog.String("event_type", e.Type),
		slog.String("subscriber", job.Args.Subscriber))
	ctx, err = jobs.RestoreActor(ctx, e.ActorID, w.loadActor)
	if err != nil {
		return err
	}
	return h(ctx, e)
}

func fromRow(r db.GetEventRow) Event {
	e := Event{
		ID:            r.ID,
		Type:          r.Type,
		AggregateType: r.AggregateType,
		AggregateID:   r.AggregateID,
		OccurredAt:    r.OccurredAt,
		Payload:       r.Payload,
	}
	if r.ActorID != nil {
		e.ActorID = *r.ActorID
	}
	return e
}
