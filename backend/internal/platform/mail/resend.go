package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	netmail "net/mail"
	"strings"
	"time"

	"storeit/internal/platform/logger"
)

// resendSender gửi qua HTTP API của Resend (port từ mimir-2.0). Khoá API nằm
// cạnh URL trong mọi request, nên không lỗi nào ở đây được bọc nguyên URL.
type resendSender struct {
	baseURL string
	apiKey  string
	from    string
	replyTo string
	http    *http.Client
}

func newResend(cfg Config, from netmail.Address) *resendSender {
	return &resendSender{
		baseURL: strings.TrimSuffix(cfg.ResendBaseURL, "/"),
		apiKey:  cfg.ResendAPIKey,
		from:    formatAddress(from.Name, from.Address),
		replyTo: cfg.ReplyTo,
		// Chặn trên cho một lần gửi, không phải hạn của cả job
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

// resendRequest là thân POST /emails. Gửi cả text lẫn html: thư chỉ có html
// bị bộ lọc spam chấm điểm cao hơn.
type resendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Text    string   `json:"text"`
	HTML    string   `json:"html,omitempty"`
	// omitempty: "reply_to": "" là đặt header rỗng, khác với không đặt
	ReplyTo string `json:"reply_to,omitempty"`
}

func (r *resendSender) Send(ctx context.Context, m Message) error {
	body, err := json.Marshal(resendRequest{
		From:    r.from,
		To:      []string{formatAddress(m.To.Name, m.To.Email)},
		Subject: m.Subject,
		Text:    m.Text,
		HTML:    m.HTML,
		ReplyTo: r.replyTo,
	})
	if err != nil {
		return permanentError{err: errors.New("mail: encode resend request")}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL+"/emails", bytes.NewReader(body))
	if err != nil {
		// *url.Error mang nguyên URL: thay bằng chuỗi tĩnh, không bọc
		return errors.New("mail: build resend request failed")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	if m.IdempotencyKey != "" {
		req.Header.Set("Idempotency-Key", m.IdempotencyKey)
	}

	resp, err := r.http.Do(req)
	if err != nil {
		// Cùng lý do: lỗi giao vận bọc URL, không đưa vào error chain
		return errors.New("mail: resend request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return errors.New("mail: read resend response failed")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &StatusError{Status: resp.StatusCode, Reason: r.reason(raw)}
	}

	var out struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(raw, &out)
	// id để lần vết trong dashboard Resend khi có người báo không nhận được thư
	logger.FromContext(ctx).InfoContext(ctx, "resend accepted email", slog.String("resend_id", out.ID))
	return nil
}

// reason chỉ đọc hai trường đã biết tên trong JSON lỗi của Resend. Không đưa
// cả thân response vào lỗi (nó đi thẳng vào log của River), nhưng cũng không
// bỏ trống: 403 trần không phân biệt nổi "domain chưa xác thực" với "khoá sai".
func (r *resendSender) reason(raw []byte) string {
	var e struct {
		Name    string `json:"name"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &e); err != nil || e.Message == "" {
		return "no readable reason in response"
	}
	msg := e.Message
	if e.Name != "" {
		msg = e.Name + ": " + msg
	}
	// Cắt theo rune: cắt giữa ký tự nhiều byte cho ra chuỗi hỏng
	if runes := []rune(msg); len(runes) > 300 {
		msg = string(runes[:300]) + "…"
	}
	// Khoá API không được vào log kể cả khi Resend tự echo nó ra
	if r.apiKey != "" {
		msg = strings.ReplaceAll(msg, r.apiKey, "[redacted]")
	}
	return msg
}
