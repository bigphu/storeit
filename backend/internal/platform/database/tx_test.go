package database

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

// fakeTx chỉ ghi lại Commit/Rollback; method khác của pgx.Tx không được gọi
type fakeTx struct {
	pgx.Tx
	commitErr, rollbackErr error
	committed, rolledBack  bool
}

func (t *fakeTx) Commit(context.Context) error {
	t.committed = true
	return t.commitErr
}

func (t *fakeTx) Rollback(context.Context) error {
	t.rolledBack = true
	return t.rollbackErr
}

type fakeDB struct {
	tx       *fakeTx
	beginErr error
}

func (d *fakeDB) Begin(context.Context) (pgx.Tx, error) {
	if d.beginErr != nil {
		return nil, d.beginErr
	}
	return d.tx, nil
}

var errDomain = errors.New("asset changed")

func TestWithTx_CommitsWhenFnSucceeds(t *testing.T) {
	db := &fakeDB{tx: &fakeTx{}}
	var got pgx.Tx

	err := WithTx(context.Background(), db, func(tx pgx.Tx) error {
		got = tx
		return nil
	})

	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != db.tx {
		t.Error("fn did not receive the transaction")
	}
	if !db.tx.committed || db.tx.rolledBack {
		t.Errorf("committed=%v rolledBack=%v, want commit only", db.tx.committed, db.tx.rolledBack)
	}
}

func TestWithTx_RollsBackAndReturnsFnError(t *testing.T) {
	db := &fakeDB{tx: &fakeTx{}}

	err := WithTx(context.Background(), db, func(pgx.Tx) error { return errDomain })

	if !errors.Is(err, errDomain) {
		t.Fatalf("err = %v, want %v", err, errDomain)
	}
	if db.tx.committed || !db.tx.rolledBack {
		t.Errorf("committed=%v rolledBack=%v, want rollback only", db.tx.committed, db.tx.rolledBack)
	}
}

// Rollback lỗi (vd mất kết nối) không được che lỗi domain của fn, nếu không
// 409 sẽ thành 500
func TestWithTx_RollbackErrorKeepsFnError(t *testing.T) {
	rbErr := errors.New("conn closed")
	db := &fakeDB{tx: &fakeTx{rollbackErr: rbErr}}

	err := WithTx(context.Background(), db, func(pgx.Tx) error { return errDomain })

	if !errors.Is(err, errDomain) {
		t.Errorf("err = %v, want it to wrap %v", err, errDomain)
	}
	if !errors.Is(err, rbErr) {
		t.Errorf("err = %v, want it to wrap %v", err, rbErr)
	}
}

func TestWithTx_RollsBackOnPanic(t *testing.T) {
	db := &fakeDB{tx: &fakeTx{}}

	defer func() {
		if p := recover(); p != "boom" {
			t.Errorf("recovered %v, want panic to propagate", p)
		}
		if db.tx.committed || !db.tx.rolledBack {
			t.Errorf("committed=%v rolledBack=%v, want rollback only", db.tx.committed, db.tx.rolledBack)
		}
	}()
	_ = WithTx(context.Background(), db, func(pgx.Tx) error { panic("boom") })
}

func TestWithTx_BeginError(t *testing.T) {
	beginErr := errors.New("pool closed")
	db := &fakeDB{beginErr: beginErr}
	called := false

	err := WithTx(context.Background(), db, func(pgx.Tx) error {
		called = true
		return nil
	})

	if !errors.Is(err, beginErr) {
		t.Errorf("err = %v, want %v", err, beginErr)
	}
	if called {
		t.Error("fn called although Begin failed")
	}
}

func TestWithTx_CommitError(t *testing.T) {
	commitErr := errors.New("serialization failure")
	db := &fakeDB{tx: &fakeTx{commitErr: commitErr}}

	err := WithTx(context.Background(), db, func(pgx.Tx) error { return nil })

	if !errors.Is(err, commitErr) {
		t.Errorf("err = %v, want %v", err, commitErr)
	}
}
