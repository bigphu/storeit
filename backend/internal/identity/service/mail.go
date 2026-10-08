package service

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	htmltemplate "html/template"
	"log/slog"
	texttemplate "text/template"
	"time"

	"storeit/internal/identity/domain"
	"storeit/internal/identity/job"
	"storeit/internal/platform/logger"
	"storeit/internal/platform/mail"
)

//go:embed templates
var templateFS embed.FS

// Template parse một lần lúc khởi động; hỏng là lỗi lập trình nên Must
var (
	textTemplates = texttemplate.Must(texttemplate.ParseFS(templateFS, "templates/*.txt"))
	htmlTemplates = htmltemplate.Must(htmltemplate.ParseFS(templateFS, "templates/*.html"))
)

// accountEmail là nội dung riêng của từng loại thư
var accountEmail = map[domain.TokenPurpose]struct {
	subject string
	page    string // trang frontend nhận token
}{
	domain.PurposeInvite: {"You're invited to StoreIt", "accept-invite"},
	domain.PurposeReset:  {"Reset your StoreIt password", "reset-password"},
}

type emailData struct {
	Name      string
	Link      string
	ExpiresIn string
	// Logo PNG do frontend phục vụ (public/email-logo.png), cùng địa chỉ với link trong thư
	LogoURL string
}

// SendAccountEmail là việc của job identity.send_account_email: dựng thư chứa
// link và gửi. Link đã chết (dùng rồi, bị thay, hết hạn, account bị khoá) thì
// không gửi gì và trả nil: job chạy trễ hay chạy lại không được gửi link hỏng.
// Lỗi gửi được bọc bằng %w, để mail.IsPermanent nhìn xuyên qua.
func (s *Service) SendAccountEmail(ctx context.Context, args job.SendAccountEmailArgs) error {
	if s.mail == nil {
		return errors.New("identity: no mail sender configured")
	}
	log := logger.FromContext(ctx)
	tok, err := s.pwTokens.Get(ctx, args.TokenID)
	if errors.Is(err, domain.ErrInvalidPasswordToken) {
		log.InfoContext(ctx, "account email skipped, link no longer exists", slog.String("token_id", args.TokenID.String()))
		return nil
	}
	if err != nil {
		return err
	}
	now := s.now()
	if !tok.ExpiresAt.After(now) {
		log.InfoContext(ctx, "account email skipped, link expired", slog.String("token_id", tok.ID.String()))
		return nil
	}
	a, err := s.accounts.Get(ctx, tok.AccountID)
	if errors.Is(err, domain.ErrAccountNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if !a.Active {
		log.InfoContext(ctx, "account email skipped, account disabled", slog.String("account_id", a.ID.String()))
		return nil
	}

	kind, ok := accountEmail[tok.Purpose]
	if !ok {
		return fmt.Errorf("identity: no email for token purpose %q", tok.Purpose)
	}
	data := emailData{
		Name: a.Name,
		// Token nằm trong fragment: trình duyệt không gửi fragment lên server,
		// nên token không vào access log hay header Referer
		Link:      fmt.Sprintf("%s/%s#token=%s", s.appURL, kind.page, args.Token),
		ExpiresIn: humanDuration(s.ttl(tok.Purpose)),
		LogoURL:   s.appURL + "/email-logo.png",
	}
	var text, html bytes.Buffer
	name := string(tok.Purpose)
	if err := textTemplates.ExecuteTemplate(&text, name+".txt", data); err != nil {
		return fmt.Errorf("identity: render %s email: %w", name, err)
	}
	if err := htmlTemplates.ExecuteTemplate(&html, name+".html", data); err != nil {
		return fmt.Errorf("identity: render %s email: %w", name, err)
	}
	err = s.mail.Send(ctx, mail.Message{
		To:      mail.Address{Name: a.Name, Email: a.Email},
		Subject: kind.subject,
		Text:    text.String(),
		HTML:    html.String(),
		// Mỗi token một key: job chạy lại sau khi đã gửi thì Resend bỏ qua
		IdempotencyKey: "identity/" + name + "/" + tok.ID.String(),
	})
	if err != nil {
		return fmt.Errorf("identity: send %s email: %w", name, err)
	}
	log.InfoContext(ctx, "sent account email", slog.String("purpose", name), slog.String("account_id", a.ID.String()))
	return nil
}

// humanDuration viết thời hạn cho người đọc: "72 hours", "1 hour", "30 minutes"
func humanDuration(d time.Duration) string {
	unit, n := "minute", int(d/time.Minute)
	if d >= time.Hour && d%time.Hour == 0 {
		unit, n = "hour", int(d/time.Hour)
	}
	if n == 1 {
		return "1 " + unit
	}
	return fmt.Sprintf("%d %ss", n, unit)
}
