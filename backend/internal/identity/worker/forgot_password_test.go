package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/riverqueue/river"

	"storeit/internal/identity/job"
)

type fakeForgot struct {
	got job.ForgotPasswordArgs
	err error
}

func (f *fakeForgot) ProcessForgotPassword(_ context.Context, args job.ForgotPasswordArgs) error {
	f.got = args
	return f.err
}

func TestForgotPasswordWork(t *testing.T) {
	f := &fakeForgot{}
	w := NewForgotPassword(f)
	args := job.ForgotPasswordArgs{Email: "lan@storeit.test"}
	if err := w.Work(context.Background(), &river.Job[job.ForgotPasswordArgs]{Args: args}); err != nil || f.got != args {
		t.Errorf("Work = %v, service got %+v", err, f.got)
	}
	// Lỗi hệ thống (DB) thì River thử lại
	f.err = errors.New("db down")
	if err := w.Work(context.Background(), &river.Job[job.ForgotPasswordArgs]{Args: args}); !errors.Is(err, f.err) {
		t.Errorf("Work = %v, want the service error", err)
	}
}
