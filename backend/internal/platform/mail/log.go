package mail

import (
	"context"
	"log/slog"

	"storeit/internal/platform/logger"
)

// logSender không gửi gì, chỉ in thư ra log ở mức WARN. Thân thư thường chứa
// link có token, nên chỉ dùng khi dev hoặc test; production chọn smtp/resend.
type logSender struct{ log *slog.Logger }

func (s logSender) Send(ctx context.Context, m Message) error {
	log := s.log
	if log == nil {
		log = logger.FromContext(ctx)
	}
	log.WarnContext(ctx, "mail transport is log, email not sent",
		slog.String("to", m.To.Email),
		slog.String("subject", m.Subject),
		slog.String("text", m.Text),
	)
	return nil
}
