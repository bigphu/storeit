package job

import (
	"time"

	"github.com/riverqueue/river"

	"storeit/internal/platform/jobs"
)

// ForgotPasswordArgs: form "quên mật khẩu" công khai. Request chỉ xếp job này
// (cùng một việc dù email có tài khoản hay không, nên thời gian phản hồi không
// lộ gì); worker tra account và phát link.
type ForgotPasswordArgs struct {
	Email string `json:"email"` // đã NormalizeEmail
}

func (ForgotPasswordArgs) Kind() string { return "identity.forgot_password" }

// Cùng email trong cùng một phút chỉ giữ một job: lớp chặn phụ, cooldown thật
// nằm ở câu upsert token
func (ForgotPasswordArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:      jobs.QueueDefault,
		UniqueOpts: river.UniqueOpts{ByArgs: true, ByPeriod: time.Minute},
	}
}
