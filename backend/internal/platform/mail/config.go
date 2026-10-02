package mail

import (
	"errors"
	"fmt"
	netmail "net/mail"
	"strings"
)

// Transport chọn đường gửi thư. Chọn tường minh, không suy ra từ "khối nào
// được điền": điền cả khối SMTP lẫn Resend là bình thường (đổi transport
// không phải sửa gì khác), và production quên đặt thì phải lỗi khởi động chứ
// không lặng lẽ rơi về log (log in cả link chứa token).
type Transport string

const (
	TransportSMTP   Transport = "smtp"
	TransportResend Transport = "resend"
	TransportLog    Transport = "log"
)

// Chế độ TLS của SMTP
const (
	TLSStartTLS = "starttls" // STARTTLS bắt buộc, thường là port 587
	TLSImplicit = "tls"      // TLS ngay từ đầu, thường là port 465
	TLSNone     = "none"     // không mã hoá: chỉ cho Mailpit khi dev
)

const defaultSMTPPort = 587

// Config của mail. Chỉ cmd/worker dùng: API không gửi thư, chỉ xếp job.
type Config struct {
	Transport Transport `env:"MAIL_TRANSPORT,required"`
	// Người gửi theo RFC 5322: "StoreIt <no-reply@example.com>". Với Resend phải
	// thuộc domain đã xác thực.
	From string `env:"MAIL_FROM,required"`
	// Nơi thư trả lời đi tới; rỗng là không đặt header
	ReplyTo string `env:"MAIL_REPLY_TO"`

	SMTPHost     string `env:"MAIL_SMTP_HOST"`
	SMTPPort     int    `env:"MAIL_SMTP_PORT" envDefault:"587"`
	SMTPUsername string `env:"MAIL_SMTP_USERNAME"`
	// Đọc từ file (Docker secret); biến chứa đường dẫn file
	SMTPPassword string `env:"MAIL_SMTP_PASSWORD_FILE,file"`
	SMTPTLS      string `env:"MAIL_SMTP_TLS" envDefault:"starttls"`

	// Đọc từ file (Docker secret); biến chứa đường dẫn file
	ResendAPIKey string `env:"MAIL_RESEND_API_KEY_FILE,file"`
	// Chỉ để test trỏ vào httptest; rỗng là https://api.resend.com
	ResendBaseURL string
}

func (c Config) Validate() error {
	c = c.normalized()
	switch c.Transport {
	case TransportSMTP, TransportResend, TransportLog:
	case "":
		return errors.New("mail: MAIL_TRANSPORT is required (smtp, resend or log)")
	default:
		return fmt.Errorf("mail: MAIL_TRANSPORT %q is not smtp, resend or log", c.Transport)
	}
	if c.From == "" {
		return errors.New("mail: MAIL_FROM is required")
	}
	if _, err := netmail.ParseAddress(c.From); err != nil {
		return fmt.Errorf("mail: MAIL_FROM %q is not an email address: %w", c.From, err)
	}
	if c.ReplyTo != "" {
		if _, err := netmail.ParseAddress(c.ReplyTo); err != nil {
			return fmt.Errorf("mail: MAIL_REPLY_TO %q is not an email address: %w", c.ReplyTo, err)
		}
	}

	switch c.Transport {
	case TransportSMTP:
		switch {
		case c.SMTPHost == "":
			return errors.New("mail: MAIL_SMTP_HOST is required for the smtp transport")
		case c.SMTPPort < 1 || c.SMTPPort > 65535:
			return fmt.Errorf("mail: MAIL_SMTP_PORT %d is out of range", c.SMTPPort)
		case c.SMTPTLS != TLSStartTLS && c.SMTPTLS != TLSImplicit && c.SMTPTLS != TLSNone:
			return fmt.Errorf("mail: MAIL_SMTP_TLS %q is not starttls, tls or none", c.SMTPTLS)
		case c.SMTPUsername != "" && c.SMTPPassword == "":
			return errors.New("mail: MAIL_SMTP_USERNAME is set but MAIL_SMTP_PASSWORD_FILE is empty")
		case c.SMTPUsername == "" && c.SMTPPassword != "":
			return errors.New("mail: MAIL_SMTP_PASSWORD_FILE is set but MAIL_SMTP_USERNAME is empty")
		}
	case TransportResend:
		if c.ResendAPIKey == "" {
			return errors.New("mail: MAIL_RESEND_API_KEY_FILE is required for the resend transport")
		}
	}
	return nil
}

// normalized chuẩn hoá giá trị gõ tay và áp mặc định cho Config dựng tay
// trong test (field bằng 0)
func (c Config) normalized() Config {
	c.Transport = Transport(strings.ToLower(strings.TrimSpace(string(c.Transport))))
	c.From = strings.TrimSpace(c.From)
	c.ReplyTo = strings.TrimSpace(c.ReplyTo)
	c.SMTPTLS = strings.ToLower(strings.TrimSpace(c.SMTPTLS))
	if c.SMTPTLS == "" {
		c.SMTPTLS = TLSStartTLS
	}
	if c.SMTPPort == 0 {
		c.SMTPPort = defaultSMTPPort
	}
	// Đọc từ file nên thường dính xuống dòng cuối
	c.SMTPPassword = strings.TrimSpace(c.SMTPPassword)
	c.ResendAPIKey = strings.TrimSpace(c.ResendAPIKey)
	if c.ResendBaseURL == "" {
		c.ResendBaseURL = "https://api.resend.com"
	}
	return c
}
