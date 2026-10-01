package events

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/riverqueue/river"

	"storeit/internal/platform/auth"
	"storeit/internal/platform/events/db"
	"storeit/internal/platform/jobs"
)

// fakeDB thay Postgres cho GetEvent: QueryRow trả row (hoặc lỗi) cố định
type fakeDB struct {
	row GetEventRowData
	err error
}

type GetEventRowData = db.GetEventRow

func (f *fakeDB) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	panic("not used")
}
func (f *fakeDB) Query(context.Context, string, ...any) (pgx.Rows, error) { panic("not used") }
func (f *fakeDB) QueryRow(context.Context, string, ...any) pgx.Row {
	return fakeRow{f}
}

type fakeRow struct{ f *fakeDB }

// Scan theo đúng thứ tự cột của GetEvent
func (r fakeRow) Scan(dest ...any) error {
	if r.f.err != nil {
		return r.f.err
	}
	row := r.f.row
	*dest[0].(*uuid.UUID) = row.ID
	*dest[1].(*string) = row.Type
	*dest[2].(*string) = row.AggregateType
	*dest[3].(*uuid.UUID) = row.AggregateID
	*dest[4].(**uuid.UUID) = row.ActorID
	*dest[5].(*time.Time) = row.OccurredAt
	*dest[6].(*json.RawMessage) = row.Payload
	return nil
}

type delivered struct {
	event Event
	actor auth.Actor
}

func newWorker(t *testing.T, fdb *fakeDB, load jobs.ActorLoader) (*HandleEventWorker, *[]delivered) {
	t.Helper()
	var got []delivered
	r := NewRegistry()
	r.On("test.sub", func(ctx context.Context, e Event) error {
		a, _ := auth.FromContext(ctx)
		got = append(got, delivered{e, a})
		return nil
	}, "inventory.asset_checked_out")
	return &HandleEventWorker{q: db.New(fdb), registry: r, loadActor: load}, &got
}

func work(w *HandleEventWorker, id uuid.UUID) error {
	return w.Work(context.Background(), &river.Job[HandleEventArgs]{
		Args: HandleEventArgs{EventID: id, Subscriber: "test.sub"},
	})
}

func isCancel(err error) bool {
	var c *river.JobCancelError
	return errors.As(err, &c)
}

func TestHandleEventWorker_DeliversEventWithActor(t *testing.T) {
	accountID := uuid.New()
	row := db.GetEventRow{
		ID: uuid.New(), Type: "inventory.asset_checked_out", AggregateType: "asset",
		AggregateID: uuid.New(), ActorID: &accountID, OccurredAt: time.Now().UTC(),
		Payload: json.RawMessage(`{"tag":"A-1"}`),
	}
	load := func(_ context.Context, id uuid.UUID) (auth.Actor, error) {
		return auth.Actor{AccountID: id, Permissions: []string{"p"}}, nil
	}
	w, got := newWorker(t, &fakeDB{row: row}, load)

	if err := work(w, row.ID); err != nil {
		t.Fatal(err)
	}

	if len(*got) != 1 {
		t.Fatalf("handler called %d times, want 1", len(*got))
	}
	d := (*got)[0]
	if d.event.ID != row.ID || d.event.ActorID != accountID || string(d.event.Payload) != `{"tag":"A-1"}` {
		t.Errorf("event = %+v", d.event)
	}
	if d.actor.AccountID != accountID || !d.actor.Can("p") {
		t.Errorf("actor = %+v, want loaded account %v", d.actor, accountID)
	}
}

// Event do SystemActor gây ra (actor_id NULL) thì subscriber chạy bằng SystemActor
func TestHandleEventWorker_NullActorRunsAsSystem(t *testing.T) {
	row := db.GetEventRow{ID: uuid.New(), Type: "inventory.asset_checked_out", Payload: json.RawMessage(`{}`)}
	load := func(context.Context, uuid.UUID) (auth.Actor, error) {
		t.Error("loader must not be called for system events")
		return auth.Actor{}, nil
	}
	w, got := newWorker(t, &fakeDB{row: row}, load)

	if err := work(w, row.ID); err != nil {
		t.Fatal(err)
	}
	if len(*got) != 1 || !(*got)[0].actor.IsSystem() {
		t.Errorf("delivered = %+v, want one delivery as SystemActor", *got)
	}
}

func TestHandleEventWorker_Failures(t *testing.T) {
	gone := func(context.Context, uuid.UUID) (auth.Actor, error) { return auth.Actor{}, jobs.ErrActorNotFound }
	down := func(context.Context, uuid.UUID) (auth.Actor, error) { return auth.Actor{}, errors.New("identity down") }
	someone := uuid.New()
	withActor := db.GetEventRow{ID: uuid.New(), ActorID: &someone, Payload: json.RawMessage(`{}`)}

	tests := []struct {
		name       string
		db         *fakeDB
		load       jobs.ActorLoader
		wantCancel bool
	}{
		// Event đã bị xoá: retry không giúp gì
		{"event not found", &fakeDB{err: pgx.ErrNoRows}, nil, true},
		// DB tạm lỗi: để River retry
		{"db error", &fakeDB{err: errors.New("conn reset")}, nil, false},
		{"actor deleted", &fakeDB{row: withActor}, gone, true},
		{"actor loader down", &fakeDB{row: withActor}, down, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, got := newWorker(t, tt.db, tt.load)

			err := work(w, uuid.New())

			if err == nil || isCancel(err) != tt.wantCancel {
				t.Errorf("err = %v, want cancel=%v", err, tt.wantCancel)
			}
			if len(*got) != 0 {
				t.Error("handler must not run")
			}
		})
	}
}

func TestDecode_BadPayload(t *testing.T) {
	type payload struct{ Tag string }
	_, err := Decode[payload](Event{Type: "x.y", Payload: json.RawMessage(`{"Tag":1}`)})
	if err == nil {
		t.Error("want error for payload of the wrong shape")
	}
}
