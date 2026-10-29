package events

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"storeit/internal/platform/events/db"
)

// ListForAggregate đọc event của một aggregate, mới nhất trước (occurred_at, id giảm dần).
// before là ID event cuối của trang trước: chỉ trả event cũ hơn event đó; nil là từ đầu.
// Chỉ đọc. ActorID là uuid.Nil với SystemActor, như khi worker giao event.
func ListForAggregate(ctx context.Context, q db.DBTX, aggregateType string, id uuid.UUID, before *uuid.UUID, limit int) ([]Event, error) {
	rows, err := db.New(q).ListAggregateEvents(ctx, db.ListAggregateEventsParams{
		AggregateType: aggregateType, AggregateID: id, Before: before, Lim: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("events: list %s %s: %w", aggregateType, id, err)
	}
	out := make([]Event, len(rows))
	for i, r := range rows {
		out[i] = fromRow(db.GetEventRow(r))
	}
	return out, nil
}
