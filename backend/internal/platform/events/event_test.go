package events

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

type assetCheckedOut struct {
	AssetID  uuid.UUID `json:"asset_id"`
	MemberID uuid.UUID `json:"member_id"`
}

func TestNew(t *testing.T) {
	assetID := uuid.New()
	payload := assetCheckedOut{AssetID: assetID, MemberID: uuid.New()}
	before := time.Now()

	e, err := New("inventory.asset_checked_out", "asset", assetID, payload)
	if err != nil {
		t.Fatal(err)
	}

	if e.ID.Version() != 7 {
		t.Errorf("ID version = %d, want 7 (sorts by time)", e.ID.Version())
	}
	if e.Type != "inventory.asset_checked_out" || e.AggregateType != "asset" || e.AggregateID != assetID {
		t.Errorf("event = %+v", e)
	}
	if e.OccurredAt.Before(before) || e.OccurredAt.After(time.Now()) {
		t.Errorf("OccurredAt = %v, want now", e.OccurredAt)
	}
	got, err := Decode[assetCheckedOut](e)
	if err != nil || got != payload {
		t.Errorf("Decode = %+v, %v, want %+v", got, err, payload)
	}
}

func TestNew_UnencodablePayload(t *testing.T) {
	if _, err := New("x.y", "x", uuid.New(), make(chan int)); err == nil {
		t.Error("want error for payload that is not JSON")
	}
}

func TestRegistry_For(t *testing.T) {
	r := NewRegistry()
	noop := func(context.Context, Event) error { return nil }
	r.On("inventory.asset_checked_out", "notifications.checkout_email", noop)
	r.On("inventory.asset_created", "finance.cost_rollup", noop)
	r.OnAll("activity.record", noop)

	if got, want := r.For("inventory.asset_checked_out"), []string{"activity.record", "notifications.checkout_email"}; !slices.Equal(got, want) {
		t.Errorf("checked_out: got %v, want %v", got, want)
	}
	if got, want := r.For("directory.member_created"), []string{"activity.record"}; !slices.Equal(got, want) {
		t.Errorf("unrelated type: got %v, want %v", got, want)
	}
}

// Tên subscriber là khoá của job: trùng tên thì job của hai subscriber lẫn nhau
func TestRegistry_DuplicateSubscriberPanics(t *testing.T) {
	r := NewRegistry()
	noop := func(context.Context, Event) error { return nil }
	r.On("a.x", "activity.record", noop)

	defer func() {
		if recover() == nil {
			t.Error("want panic on duplicate subscriber name")
		}
	}()
	r.OnAll("activity.record", noop)
}

// Subscriber đã bị gỡ khỏi code thì job cũ của nó bị huỷ, không retry vô ích
func TestHandleEventWorker_UnknownSubscriberCancels(t *testing.T) {
	w := &HandleEventWorker{registry: NewRegistry()}

	err := w.Work(context.Background(), &river.Job[HandleEventArgs]{
		Args: HandleEventArgs{EventID: uuid.New(), Subscriber: "gone.subscriber"},
	})

	var cancel *river.JobCancelError
	if !errors.As(err, &cancel) {
		t.Errorf("err = %v, want JobCancel", err)
	}
}
