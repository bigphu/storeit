package worker

import (
	"context"

	"github.com/riverqueue/river"

	"storeit/internal/identity/job"
)

// forgotProcessor là phần service worker cần; *service.Service thoả interface này
type forgotProcessor interface {
	ProcessForgotPassword(ctx context.Context, args job.ForgotPasswordArgs) error
}

type ForgotPassword struct {
	river.WorkerDefaults[job.ForgotPasswordArgs]
	svc forgotProcessor
}

func NewForgotPassword(svc forgotProcessor) *ForgotPassword {
	return &ForgotPassword{svc: svc}
}

// Work tra account theo email và phát link; không cần actor (form công khai)
func (w *ForgotPassword) Work(ctx context.Context, j *river.Job[job.ForgotPasswordArgs]) error {
	return w.svc.ProcessForgotPassword(ctx, j.Args)
}
