package events

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"storeit/internal/platform/auth"
	"storeit/internal/platform/events/db"
)

// Outbox ghi event cùng transaction với thay đổi sinh ra nó (transactional
// outbox): thay đổi, dòng event và job giao event cùng commit hoặc cùng mất.
type Outbox struct {
	registry *Registry
	river    *river.Client[pgx.Tx]
	q        *db.Queries
}

// NewOutbox nhận client River chỉ để insert (jobs.NewInsertClient)
func NewOutbox(registry *Registry, client *river.Client[pgx.Tx]) *Outbox {
	return &Outbox{registry: registry, river: client, q: db.New(nil)}
}

// Append ghi evts vào platform.events và một job HandleEvent cho mỗi
// subscriber, tất cả trong tx. Chỉ repository gọi, bên trong database.WithTx.
// ActorID của event lấy từ actor trong ctx.
func (o *Outbox) Append(ctx context.Context, tx pgx.Tx, evts ...Event) error {
	actorID := auth.ActorIDFrom(ctx)
	q := o.q.WithTx(tx)

	var jobs []river.InsertManyParams
	for _, e := range evts {
		e.ActorID = actorID
		if err := q.InsertEvent(ctx, toParams(e)); err != nil {
			return fmt.Errorf("events: insert %s: %w", e.Type, err)
		}
		for _, sub := range o.registry.For(e.Type) {
			jobs = append(jobs, river.InsertManyParams{
				Args: HandleEventArgs{EventID: e.ID, Subscriber: sub},
			})
		}
	}
	if len(jobs) == 0 {
		return nil
	}
	if _, err := o.river.InsertManyTx(ctx, tx, jobs); err != nil {
		return fmt.Errorf("events: enqueue: %w", err)
	}
	return nil
}

func toParams(e Event) db.InsertEventParams {
	var actorID *uuid.UUID
	if e.ActorID != uuid.Nil {
		actorID = &e.ActorID
	}
	return db.InsertEventParams{
		ID:            e.ID,
		Type:          e.Type,
		AggregateType: e.AggregateType,
		AggregateID:   e.AggregateID,
		ActorID:       actorID,
		OccurredAt:    e.OccurredAt,
		Payload:       e.Payload,
	}
}
