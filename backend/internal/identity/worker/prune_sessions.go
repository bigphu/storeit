// Package worker chạy job nền của identity: đọc args từ identity/job, gọi đúng
// một method của service.
package worker

import (
	"context"

	"github.com/riverqueue/river"

	"storeit/internal/identity/job"
	"storeit/internal/identity/service"
)

type PruneSessions struct {
	river.WorkerDefaults[job.PruneSessionsArgs]
	svc *service.Service
}

func NewPruneSessions(svc *service.Service) *PruneSessions {
	return &PruneSessions{svc: svc}
}

// Work gọi service dọn phiên; dọn rác không kiểm tra quyền nên không cần actor
func (w *PruneSessions) Work(ctx context.Context, _ *river.Job[job.PruneSessionsArgs]) error {
	_, err := w.svc.PruneSessions(ctx)
	return err
}
