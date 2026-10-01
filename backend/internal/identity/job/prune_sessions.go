// Package job là args của job nền của identity, dùng chung giữa chỗ enqueue
// và worker. Chỉ dữ liệu.
package job

import (
	"github.com/riverqueue/river"

	"storeit/internal/platform/jobs"
)

// PruneSessionsArgs: job định kỳ dọn phiên đăng nhập đã chết
type PruneSessionsArgs struct{}

func (PruneSessionsArgs) Kind() string { return "identity.prune_sessions" }

// Một lần dọn đang chờ là đủ: chạy định kỳ không cần xếp thêm
func (PruneSessionsArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: jobs.QueueDefault, UniqueOpts: river.UniqueOpts{ByArgs: true}}
}
