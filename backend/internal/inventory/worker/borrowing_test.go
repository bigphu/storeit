package worker

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"storeit/internal/inventory/job"
	"storeit/internal/platform/mail"
)

type fakeBorrowingMailer struct {
	err   error
	got   job.BorrowingEmailArgs
	scans int
}

func (f *fakeBorrowingMailer) SendBorrowingEmail(_ context.Context, args job.BorrowingEmailArgs) error {
	f.got = args
	return f.err
}

func (f *fakeBorrowingMailer) ScanOverdueLoans(context.Context) error {
	f.scans++
	return f.err
}

func TestBorrowingEmailWork(t *testing.T) {
	work := func(err error) (*fakeBorrowingMailer, error) {
		f := &fakeBorrowingMailer{err: err}
		args := job.BorrowingEmailArgs{EmailKind: job.EmailTicketApproved}
		return f, NewBorrowingEmail(f).Work(context.Background(), &river.Job[job.BorrowingEmailArgs]{Args: args})
	}
	if f, err := work(nil); err != nil || f.got.EmailKind != job.EmailTicketApproved {
		t.Errorf("success: %v, service got %+v", err, f.got)
	}
	// Vĩnh viễn (domain chưa xác thực...): huỷ job, không thử lại hàng giờ
	permanent := fmt.Errorf("inventory: send: %w", &mail.StatusError{Status: 422, Reason: "domain not verified"})
	var cancel *rivertype.JobCancelError
	if _, err := work(permanent); !errors.As(err, &cancel) {
		t.Errorf("permanent: %v (%T), want JobCancel", err, err)
	}
	temp := errors.New("connection reset")
	if _, err := work(temp); errors.As(err, &cancel) || !errors.Is(err, temp) {
		t.Errorf("temporary: %v, want the error itself", err)
	}
}

func TestOverdueScanWork(t *testing.T) {
	f := &fakeBorrowingMailer{}
	if err := NewOverdueScan(f).Work(context.Background(), &river.Job[job.OverdueScanArgs]{}); err != nil || f.scans != 1 {
		t.Errorf("scan: %v, scans %d", err, f.scans)
	}
}
