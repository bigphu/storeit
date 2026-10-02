package mail

import (
	"context"
	"errors"
	"fmt"
	netmail "net/mail"
	"time"

	gomail "github.com/wneessen/go-mail"
)

// smtpSender gửi qua một máy chủ SMTP bằng go-mail. Mỗi lần gửi mở một kết
// nối: worker gửi ít thư, giữ kết nối lâu chỉ thêm lỗi timeout phía máy chủ.
type smtpSender struct {
	client  *gomail.Client
	from    netmail.Address
	replyTo string
}

func newSMTP(cfg Config, from netmail.Address) (*smtpSender, error) {
	// WithPort phải đứng trước chính sách TLS, không thì go-mail tự đổi port
	opts := []gomail.Option{
		gomail.WithPort(cfg.SMTPPort),
		// go-mail chỉ đưa ctx vào bước dial; timeout chặn phần còn lại
		gomail.WithTimeout(30 * time.Second),
	}
	switch cfg.SMTPTLS {
	case TLSImplicit:
		opts = append(opts, gomail.WithSSL())
	case TLSNone:
		opts = append(opts, gomail.WithTLSPolicy(gomail.NoTLS))
	default:
		opts = append(opts, gomail.WithTLSPortPolicy(gomail.TLSMandatory))
	}
	if cfg.SMTPUsername != "" {
		opts = append(opts,
			gomail.WithSMTPAuth(gomail.SMTPAuthAutoDiscover),
			gomail.WithUsername(cfg.SMTPUsername),
			gomail.WithPassword(cfg.SMTPPassword),
		)
	}
	client, err := gomail.NewClient(cfg.SMTPHost, opts...)
	if err != nil {
		return nil, fmt.Errorf("mail: smtp client: %w", err)
	}
	return &smtpSender{client: client, from: from, replyTo: cfg.ReplyTo}, nil
}

func (s *smtpSender) Send(ctx context.Context, m Message) error {
	msg := gomail.NewMsg()
	if err := msg.FromFormat(s.from.Name, s.from.Address); err != nil {
		return permanentError{fmt.Errorf("mail: invalid sender: %w", err)}
	}
	if err := msg.AddToFormat(m.To.Name, m.To.Email); err != nil {
		return permanentError{fmt.Errorf("mail: invalid recipient: %w", err)}
	}
	if s.replyTo != "" {
		if err := msg.ReplyTo(s.replyTo); err != nil {
			return permanentError{fmt.Errorf("mail: invalid reply-to: %w", err)}
		}
	}
	msg.Subject(m.Subject)
	msg.SetMessageID()
	msg.SetDate()
	msg.SetBodyString(gomail.TypeTextPlain, m.Text)
	if m.HTML != "" {
		msg.AddAlternativeString(gomail.TypeTextHTML, m.HTML)
	}

	if err := s.client.DialAndSendWithContext(ctx, msg); err != nil {
		err = fmt.Errorf("mail: smtp send: %w", err)
		// Chỉ mã 5xx của máy chủ là từ chối hẳn. IsTemp() của go-mail không
		// dùng được: lỗi mạng giữa chừng (không có mã) cũng cho IsTemp false.
		var se *gomail.SendError
		if errors.As(err, &se) && se.ErrorCode() >= 500 && se.ErrorCode() < 600 {
			return permanentError{err}
		}
		return err
	}
	return nil
}
