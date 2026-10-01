package worker

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"storeit/internal/identity/job"
	"storeit/internal/platform/mail"
)

type fakeMailer struct{ err error }

func (f fakeMailer) SendAccountEmail(context.Context, job.SendAccountEmailArgs) error { return f.err }

func TestSendAccountEmailWork(t *testing.T) {
	work := func(err error) error {
		w := NewSendAccountEmail(fakeMailer{err: err})
		return w.Work(context.Background(), &river.Job[job.SendAccountEmailArgs]{})
	}
	if err := work(nil); err != nil {
		t.Errorf("success: %v", err)
	}

	// Vĩnh viễn (domain chưa xác thực...): huỷ job, không thử lại hàng giờ
	permanent := fmt.Errorf("identity: send: %w", &mail.StatusError{Status: 422, Reason: "domain not verified"})
	var cancel *rivertype.JobCancelError
	if err := work(permanent); !errors.As(err, &cancel) {
		t.Errorf("permanent: %v (%T), want JobCancel", err, err)
	}

	// Tạm thời: trả nguyên lỗi để River thử lại
	temp := errors.New("connection reset")
	if err := work(temp); errors.As(err, &cancel) || !errors.Is(err, temp) {
		t.Errorf("temporary: %v, want the error itself", err)
	}
}
