package events

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"storeit/internal/platform/auth"
	"storeit/internal/platform/database"
	"storeit/internal/platform/database/dbtest"
)

// Dòng thời gian của một aggregate: mới nhất trước, trang sau bắt đầu ngay sau event cuối
// của trang trước, không lẫn event của aggregate khác
func TestListForAggregate_NewestFirstAndPaging(t *testing.T) {
	pool := dbtest.Pool(t)
	outbox := newOutbox(t, pool, NewRegistry())
	user := auth.WithActor(context.Background(), auth.Actor{AccountID: uuid.New()})
	system := auth.WithActor(context.Background(), auth.SystemActor)
	a, other := uuid.New(), uuid.New()
	base := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)

	appendAt := func(ctx context.Context, agg uuid.UUID, at time.Time) Event {
		t.Helper()
		e, err := New("inventory.asset_updated", "asset", agg, struct{}{})
		if err != nil {
			t.Fatal(err)
		}
		e.OccurredAt = at
		if err := database.WithTx(ctx, pool, func(tx pgx.Tx) error { return outbox.Append(ctx, tx, e) }); err != nil {
			t.Fatal(err)
		}
		return e
	}
	var ids []uuid.UUID // theo thứ tự cũ → mới
	for i := range 4 {
		ids = append(ids, appendAt(user, a, base.Add(time.Duration(i)*time.Minute)).ID)
	}
	ids = append(ids, appendAt(system, a, base.Add(10*time.Minute)).ID) // mới nhất, của hệ thống
	appendAt(user, other, base.Add(20*time.Minute))

	page1, err := ListForAggregate(context.Background(), pool, "asset", a, nil, 3)
	if err != nil {
		t.Fatal(err)
	}
	if got := idsOf(page1); !equalIDs(got, []uuid.UUID{ids[4], ids[3], ids[2]}) {
		t.Fatalf("page 1 = %v, want newest three", got)
	}
	if page1[0].ActorID != uuid.Nil {
		t.Errorf("system event ActorID = %v, want uuid.Nil", page1[0].ActorID)
	}
	if page1[1].ActorID == uuid.Nil {
		t.Errorf("user event lost its actor")
	}

	page2, err := ListForAggregate(context.Background(), pool, "asset", a, &page1[2].ID, 3)
	if err != nil {
		t.Fatal(err)
	}
	if got := idsOf(page2); !equalIDs(got, []uuid.UUID{ids[1], ids[0]}) {
		t.Fatalf("page 2 = %v, want the two oldest", got)
	}

	page3, err := ListForAggregate(context.Background(), pool, "asset", a, &page2[1].ID, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(page3) != 0 {
		t.Errorf("page 3 = %v, want empty", idsOf(page3))
	}
}

func idsOf(evts []Event) []uuid.UUID {
	out := make([]uuid.UUID, len(evts))
	for i, e := range evts {
		out[i] = e.ID
	}
	return out
}

func equalIDs(a, b []uuid.UUID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
