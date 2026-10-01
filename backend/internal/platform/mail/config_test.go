package mail

import (
	"strings"
	"testing"
)

func TestConfigValidate(t *testing.T) {
	smtp := Config{Transport: TransportSMTP, From: "StoreIt <no-reply@storeit.test>", SMTPHost: "mailpit", SMTPPort: 1025, SMTPTLS: TLSNone}
	resend := Config{Transport: TransportResend, From: "no-reply@storeit.test", ResendAPIKey: "re_test"}
	logc := Config{Transport: TransportLog, From: "no-reply@storeit.test"}

	cases := []struct {
		name    string
		cfg     Config
		wantErr string // rỗng là hợp lệ
	}{
		{"smtp hợp lệ", smtp, ""},
		{"resend hợp lệ", resend, ""},
		{"log hợp lệ", logc, ""},
		{"thiếu transport", with(logc, func(c *Config) { c.Transport = "" }), "MAIL_TRANSPORT"},
		{"transport lạ", with(logc, func(c *Config) { c.Transport = "sendgrid" }), "MAIL_TRANSPORT"},
		{"thiếu from", with(logc, func(c *Config) { c.From = "" }), "MAIL_FROM"},
		{"from sai dạng", with(logc, func(c *Config) { c.From = "not an address" }), "MAIL_FROM"},
		{"reply-to sai dạng", with(logc, func(c *Config) { c.ReplyTo = "x@" }), "MAIL_REPLY_TO"},
		{"smtp thiếu host", with(smtp, func(c *Config) { c.SMTPHost = "" }), "MAIL_SMTP_HOST"},
		{"smtp port âm", with(smtp, func(c *Config) { c.SMTPPort = -1 }), "MAIL_SMTP_PORT"},
		{"smtp port quá lớn", with(smtp, func(c *Config) { c.SMTPPort = 70000 }), "MAIL_SMTP_PORT"},
		{"smtp tls lạ", with(smtp, func(c *Config) { c.SMTPTLS = "ssl" }), "MAIL_SMTP_TLS"},
		{"smtp username thiếu password", with(smtp, func(c *Config) { c.SMTPUsername = "u" }), "MAIL_SMTP_PASSWORD_FILE"},
		{"smtp password thiếu username", with(smtp, func(c *Config) { c.SMTPPassword = "p" }), "MAIL_SMTP_USERNAME"},
		{"smtp tls rỗng là starttls", with(smtp, func(c *Config) { c.SMTPTLS = "" }), ""},
		{"smtp port 0 là mặc định", with(smtp, func(c *Config) { c.SMTPPort = 0 }), ""},
		{"resend thiếu key", with(resend, func(c *Config) { c.ResendAPIKey = "" }), "MAIL_RESEND_API_KEY_FILE"},
		{"resend key chỉ khoảng trắng", with(resend, func(c *Config) { c.ResendAPIKey = " \n" }), "MAIL_RESEND_API_KEY_FILE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			switch {
			case tc.wantErr == "" && err != nil:
				t.Fatalf("Validate() = %v, want nil", err)
			case tc.wantErr != "" && err == nil:
				t.Fatalf("Validate() = nil, want error mentioning %s", tc.wantErr)
			case tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr):
				t.Fatalf("Validate() = %v, want it to mention %s", err, tc.wantErr)
			}
		})
	}
}

// Transport viết hoa hay có khoảng trắng (gõ tay trong .env) vẫn hiểu được
func TestConfigTransportNormalized(t *testing.T) {
	c := Config{Transport: " Log ", From: "a@b.test"}
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
	if _, err := New(c, nil); err != nil {
		t.Fatalf("New() = %v", err)
	}
}

func TestNewRejectsInvalidConfig(t *testing.T) {
	if _, err := New(Config{Transport: TransportResend, From: "a@b.test"}, nil); err == nil {
		t.Fatal("New() with resend and no key = nil error")
	}
}

func with(c Config, f func(*Config)) Config {
	f(&c)
	return c
}
