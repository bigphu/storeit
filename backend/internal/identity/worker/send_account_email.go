package worker

import (
	"context"

	"github.com/riverqueue/river"

	"storeit/internal/identity/job"
	"storeit/internal/platform/mail"
)

// accountMailer là phần service worker cần; *service.Service thoả interface này
type accountMailer interface {
	SendAccountEmail(ctx context.Context, args job.SendAccountEmailArgs) error
}

type SendAccountEmail struct {
	river.WorkerDefaults[job.SendAccountEmailArgs]
	svc accountMailer
}

func NewSendAccountEmail(svc accountMailer) *SendAccountEmail {
	return &SendAccountEmail{svc: svc}
}

// Work gửi thư; lỗi vĩnh viễn (domain chưa xác thực, địa chỉ hỏng, SMTP 5xx)
// thì huỷ job thay vì thử lại hàng giờ. Không cần actor: link trong job đã là
// quyền hạn của việc này.
func (w *SendAccountEmail) Work(ctx context.Context, j *river.Job[job.SendAccountEmailArgs]) error {
	err := w.svc.SendAccountEmail(ctx, j.Args)
	if mail.IsPermanent(err) {
		return river.JobCancel(err)
	}
	return err
}
