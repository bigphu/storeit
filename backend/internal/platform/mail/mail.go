package mail

import (
	"context"
	"fmt"
	"log/slog"
	netmail "net/mail"
)

// Address là một người nhận. Name có thể rỗng.
type Address struct {
	Name  string
	Email string
}

// Message là một lá thư đã dựng xong. Người gửi và Reply-To lấy từ Config.
type Message struct {
	To      Address
	Subject string
	Text    string
	HTML    string // rỗng là thư chỉ có text
	// IdempotencyKey: Resend bỏ qua lần gửi thứ hai cùng key trong 24 giờ (job
	// chạy lại sau khi đã gửi xong). smtp và log bỏ qua field này.
	IdempotencyKey string
}

// Sender gửi một lá thư. Lỗi mà thử lại cũng vô ích thì IsPermanent(err) đúng.
type Sender interface {
	Send(ctx context.Context, m Message) error
}

// New dựng Sender theo cfg.Transport. Không mở kết nối, không gọi mạng.
// log nil thì log transport ghi qua logger của ctx.
func New(cfg Config, log *slog.Logger) (Sender, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	cfg = cfg.normalized()
	from, err := netmail.ParseAddress(cfg.From)
	if err != nil {
		return nil, fmt.Errorf("mail: MAIL_FROM: %w", err)
	}
	switch cfg.Transport {
	case TransportResend:
		return newResend(cfg, *from), nil
	case TransportSMTP:
		return newSMTP(cfg, *from)
	default:
		return logSender{log: log}, nil
	}
}

// formatAddress dựng "Tên <a@b>" theo RFC 5322 bằng net/mail: tên có dấu
// phẩy hay tiếng Việt nối tay sẽ thành header hỏng
func formatAddress(name, email string) string {
	if name == "" {
		return email
	}
	return (&netmail.Address{Name: name, Address: email}).String()
}
