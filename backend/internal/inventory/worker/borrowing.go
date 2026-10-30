package worker

import (
	"context"

	"github.com/riverqueue/river"

	"storeit/internal/inventory/job"
	"storeit/internal/platform/mail"
)

// borrowingMailer là phần service worker cần; *service.Service thoả interface này
type borrowingMailer interface {
	SendBorrowingEmail(ctx context.Context, args job.BorrowingEmailArgs) error
	ScanOverdueLoans(ctx context.Context) error
}

// BorrowingEmail gửi thư của việc mượn (US-10)
type BorrowingEmail struct {
	river.WorkerDefaults[job.BorrowingEmailArgs]
	svc borrowingMailer
}

func NewBorrowingEmail(svc borrowingMailer) *BorrowingEmail { return &BorrowingEmail{svc: svc} }

// Work gửi thư; lỗi vĩnh viễn (domain chưa xác thực, địa chỉ hỏng) thì huỷ job thay vì thử lại
// hàng giờ. Không cần actor: thư chỉ đọc phiếu/khoản mượn.
func (w *BorrowingEmail) Work(ctx context.Context, j *river.Job[job.BorrowingEmailArgs]) error {
	err := w.svc.SendBorrowingEmail(ctx, j.Args)
	if mail.IsPermanent(err) {
		return river.JobCancel(err)
	}
	return err
}

// OverdueScan xếp thư nhắc cho các khoản mượn quá hạn (job định kỳ mỗi giờ)
type OverdueScan struct {
	river.WorkerDefaults[job.OverdueScanArgs]
	svc borrowingMailer
}

func NewOverdueScan(svc borrowingMailer) *OverdueScan { return &OverdueScan{svc: svc} }

func (w *OverdueScan) Work(ctx context.Context, _ *river.Job[job.OverdueScanArgs]) error {
	return w.svc.ScanOverdueLoans(ctx)
}
