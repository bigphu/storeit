package events

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"storeit/internal/platform/auth"
	"storeit/internal/platform/database"
	"storeit/internal/platform/database/dbtest"
	"storeit/internal/platform/jobs"
)

func newOutbox(t *testing.T, pool *pgxpool.Pool, r *Registry) *Outbox {
	t.Helper()
	client, err := jobs.NewInsertClient(pool)
	if err != nil {
		t.Fatal(err)
	}
	return NewOutbox(r, client)
}

func noop(context.Context, Event) error { return nil }

// eventActor và subscribersOf đọc thẳng bảng, không qua code đang test
func eventActor(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) (actor *uuid.UUID, found bool) {
	t.Helper()
	err := pool.QueryRow(context.Background(),
		`SELECT actor_id FROM platform.events WHERE id = $1`, id).Scan(&actor)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false
	}
	if err != nil {
		t.Fatal(err)
	}
	return actor, true
}

func subscribersOf(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT args->>'subscriber' FROM river_job
		WHERE kind = $1 AND args->>'event_id' = $2
		ORDER BY 1`, HandleEventArgs{}.Kind(), id.String())
	if err != nil {
		t.Fatal(err)
	}
	subs, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return subs
}

func TestAppend_CommitWritesEventAndOneJobPerSubscriber(t *testing.T) {
	pool := dbtest.Pool(t)
	r := NewRegistry()
	r.On("inventory.asset_checked_out", "notifications.checkout_email", noop)
	r.OnAll("activity.record", noop)
	outbox := newOutbox(t, pool, r)
	actorID := uuid.New()
	ctx := auth.WithActor(context.Background(), auth.Actor{AccountID: actorID})
	e, err := New("inventory.asset_checked_out", "asset", uuid.New(), assetCheckedOut{})
	if err != nil {
		t.Fatal(err)
	}

	err = database.WithTx(ctx, pool, func(tx pgx.Tx) error {
		return outbox.Append(ctx, tx, e)
	})
	if err != nil {
		t.Fatal(err)
	}

	actor, found := eventActor(t, pool, e.ID)
	if !found {
		t.Fatal("event row missing after commit")
	}
	if actor == nil || *actor != actorID {
		t.Errorf("actor_id = %v, want %v (from ctx)", actor, actorID)
	}
	if got, want := subscribersOf(t, pool, e.ID), []string{"activity.record", "notifications.checkout_email"}; !slices.Equal(got, want) {
		t.Errorf("jobs for subscribers %v, want %v", got, want)
	}
}

func TestAppend_RollbackLeavesNoEventAndNoJob(t *testing.T) {
	pool := dbtest.Pool(t)
	r := NewRegistry()
	r.OnAll("activity.record", noop)
	outbox := newOutbox(t, pool, r)
	ctx := auth.WithActor(context.Background(), auth.Actor{AccountID: uuid.New()})
	e, err := New("inventory.asset_checked_out", "asset", uuid.New(), assetCheckedOut{})
	if err != nil {
		t.Fatal(err)
	}
	errChanged := errors.New("asset changed")

	err = database.WithTx(ctx, pool, func(tx pgx.Tx) error {
		if err := outbox.Append(ctx, tx, e); err != nil {
			return err
		}
		return errChanged // vd UPDATE ... WHERE version = @version không khớp
	})
	if !errors.Is(err, errChanged) {
		t.Fatalf("err = %v", err)
	}

	if _, found := eventActor(t, pool, e.ID); found {
		t.Error("event row present after rollback")
	}
	if subs := subscribersOf(t, pool, e.ID); len(subs) != 0 {
		t.Errorf("jobs present after rollback: %v", subs)
	}
}

func TestAppend_SystemActorHasNoActorID(t *testing.T) {
	pool := dbtest.Pool(t)
	outbox := newOutbox(t, pool, NewRegistry())
	ctx := auth.WithActor(context.Background(), auth.SystemActor)
	e, err := New("inventory.warranty_expiring", "asset", uuid.New(), struct{}{})
	if err != nil {
		t.Fatal(err)
	}

	if err := database.WithTx(ctx, pool, func(tx pgx.Tx) error { return outbox.Append(ctx, tx, e) }); err != nil {
		t.Fatal(err)
	}

	actor, found := eventActor(t, pool, e.ID)
	if !found || actor != nil {
		t.Errorf("found=%v actor_id=%v, want row with NULL actor_id", found, actor)
	}
	// Không ai subscribe thì vẫn ghi event (để replay sau), không có job
	if subs := subscribersOf(t, pool, e.ID); len(subs) != 0 {
		t.Errorf("jobs = %v, want none", subs)
	}
}

// Từ đầu tới cuối: Append trong API, worker River giao event cho subscriber
// với actor gốc (quyền nạp lại qua loader)
func TestHandleEventWorker_DeliversWithOriginalActor(t *testing.T) {
	pool := dbtest.Pool(t)
	actorID := uuid.New()
	perms := []string{"inventory.asset.read"}
	load := func(_ context.Context, id uuid.UUID) (auth.Actor, error) {
		return auth.Actor{AccountID: id, Permissions: perms}, nil
	}

	type delivery struct {
		event Event
		actor auth.Actor
	}
	delivered := make(chan delivery, 1)
	r := NewRegistry()
	r.On("inventory.asset_checked_out", "test.deliver", func(ctx context.Context, e Event) error {
		a, _ := auth.FromContext(ctx)
		delivered <- delivery{e, a}
		return nil
	})

	workers := river.NewWorkers()
	RegisterWorker(workers, pool, r, load)
	client, err := jobs.NewWorkerClient(pool, workers, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Stop(context.Background()) })

	payload := assetCheckedOut{AssetID: uuid.New(), MemberID: uuid.New()}
	e, err := New("inventory.asset_checked_out", "asset", payload.AssetID, payload)
	if err != nil {
		t.Fatal(err)
	}
	ctx := auth.WithActor(context.Background(), auth.Actor{AccountID: actorID})
	outbox := newOutbox(t, pool, r)
	if err := database.WithTx(ctx, pool, func(tx pgx.Tx) error { return outbox.Append(ctx, tx, e) }); err != nil {
		t.Fatal(err)
	}

	select {
	case d := <-delivered:
		if d.event.ID != e.ID || d.event.ActorID != actorID {
			t.Errorf("event = %+v, want ID %v actor %v", d.event, e.ID, actorID)
		}
		if got, err := Decode[assetCheckedOut](d.event); err != nil || got != payload {
			t.Errorf("payload = %+v, %v, want %+v", got, err, payload)
		}
		if d.actor.AccountID != actorID || !d.actor.Can("inventory.asset.read") {
			t.Errorf("actor in subscriber ctx = %+v", d.actor)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("subscriber not called")
	}
}
