package events

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Event là phong bì chung của mọi event. Payload là struct công khai trong
// contract/events.go của module phát ra, đã encode JSON.
type Event struct {
	ID            uuid.UUID
	Type          string    // "<module>.<việc đã xảy ra>", vd inventory.asset_checked_out
	AggregateType string    // loại entity bị đổi, vd asset
	AggregateID   uuid.UUID // entity bị đổi
	ActorID       uuid.UUID // Outbox.Append điền từ ctx; uuid.Nil là SystemActor
	OccurredAt    time.Time
	Payload       json.RawMessage
}

// New dựng event với ID UUIDv7 (sắp theo thời gian) và thời điểm hiện tại.
// Repository gọi khi đổi domain event sang kiểu trong contract/events.go:
//
//	e, err := events.New(contract.EventAssetCheckedOut, "asset", a.ID,
//		contract.AssetCheckedOut{AssetID: a.ID, Tag: a.Tag, MemberID: *a.HolderID})
func New(eventType, aggregateType string, aggregateID uuid.UUID, payload any) (Event, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return Event{}, fmt.Errorf("events: new id: %w", err)
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return Event{}, fmt.Errorf("events: encode %s payload: %w", eventType, err)
	}
	return Event{
		ID:            id,
		Type:          eventType,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		OccurredAt:    time.Now(),
		Payload:       b,
	}, nil
}

// Decode đọc payload thành kiểu T lấy từ contract của module phát ra:
//
//	p, err := events.Decode[invcontract.AssetCheckedOut](e)
func Decode[T any](e Event) (T, error) {
	var v T
	if err := json.Unmarshal(e.Payload, &v); err != nil {
		return v, fmt.Errorf("events: decode %s payload: %w", e.Type, err)
	}
	return v, nil
}
