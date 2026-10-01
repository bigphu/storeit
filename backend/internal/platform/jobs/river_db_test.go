package jobs

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"storeit/internal/platform/database"
	"storeit/internal/platform/database/dbtest"
)

type testArgs struct {
	Key string `json:"key"`
}

func (testArgs) Kind() string { return "test.jobs_args" }

func newEnqueuer(t *testing.T) (*River, func(key string) int) {
	t.Helper()
	pool := dbtest.Pool(t)
	client, err := NewInsertClient(pool, nil)
	if err != nil {
		t.Fatal(err)
	}
	countByKey := func(key string) int {
		var n int
		err := pool.QueryRow(context.Background(),
			`SELECT count(*) FROM river_job WHERE kind = $1 AND args->>'key' = $2`,
			testArgs{}.Kind(), key).Scan(&n)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	return NewRiver(client), countByKey
}

func TestRiver_Enqueue(t *testing.T) {
	r, count := newEnqueuer(t)
	ctx := context.Background()
	key := uuid.NewString()

	if err := r.Enqueue(ctx, testArgs{Key: key}, WithQueue(QueueDefault), WithPriority(2)); err != nil {
		t.Fatal(err)
	}
	if n := count(key); n != 1 {
		t.Errorf("jobs = %d, want 1", n)
	}
}

// WithUniqueArgs gộp vào job chưa xong cùng args, không tạo thêm
func TestRiver_EnqueueUniqueArgs(t *testing.T) {
	r, count := newEnqueuer(t)
	ctx := context.Background()
	key := uuid.NewString()

	for range 3 {
		if err := r.Enqueue(ctx, testArgs{Key: key}, WithUniqueArgs()); err != nil {
			t.Fatal(err)
		}
	}
	if n := count(key); n != 1 {
		t.Errorf("jobs = %d, want 1 (deduplicated)", n)
	}
}

// EnqueueTx: job đi theo transaction, rollback thì không có job
func TestRiver_EnqueueTx(t *testing.T) {
	r, count := newEnqueuer(t)
	pool := dbtest.Pool(t)
	ctx := context.Background()
	committed, rolledBack := uuid.NewString(), uuid.NewString()
	errDomain := errors.New("domain")

	if err := database.WithTx(ctx, pool, func(tx pgx.Tx) error {
		return r.EnqueueTx(ctx, tx, testArgs{Key: committed})
	}); err != nil {
		t.Fatal(err)
	}
	_ = database.WithTx(ctx, pool, func(tx pgx.Tx) error {
		if err := r.EnqueueTx(ctx, tx, testArgs{Key: rolledBack}); err != nil {
			return err
		}
		return errDomain
	})

	if count(committed) != 1 || count(rolledBack) != 0 {
		t.Errorf("committed = %d, rolled back = %d, want 1 and 0", count(committed), count(rolledBack))
	}
}

// Priority ngoài 1-4 là lỗi của River, được bọc kèm kind
func TestRiver_EnqueueInvalidPriority(t *testing.T) {
	r, _ := newEnqueuer(t)
	err := r.Enqueue(context.Background(), testArgs{Key: uuid.NewString()}, WithPriority(9))
	if err == nil {
		t.Fatal("want error for priority 9")
	}
}
