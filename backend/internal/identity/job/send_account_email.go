package job

import (
	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"storeit/internal/platform/jobs"
)

// SendAccountEmailArgs: gửi thư chứa link đặt mật khẩu (lời mời hoặc đặt lại).
// Repository xếp job này trong cùng transaction phát token.
//
// Token là token thô: DB chỉ lưu hash, nên đây là đường duy nhất để link tới
// được thư. Nó nằm trong river_job.args tới khi River xoá job đã xong (24 giờ).
type SendAccountEmailArgs struct {
	TokenID uuid.UUID `json:"token_id"`
	Purpose string    `json:"purpose"`
	Token   string    `json:"token"`
}

func (SendAccountEmailArgs) Kind() string { return "identity.send_account_email" }

func (SendAccountEmailArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: jobs.QueueDefault}
}
