package mail

import (
	"context"
	"log/slog"
	"strings"

	"storeit/internal/platform/logger"
)

// reservedTLDs: miền cấp cao dành cho test và tài liệu (RFC 2606, RFC 6761), không bao giờ
// là hộp thư thật. Tài khoản demo dùng @storeit.test.
var reservedTLDs = []string{"test", "example", "invalid", "localhost"}

// IsReservedTestDomain: địa chỉ thuộc một miền dành cho test (.test, .example, .invalid,
// .localhost, hay chính các nhãn đó)
func IsReservedTestDomain(email string) bool {
	at := strings.LastIndexByte(email, '@')
	if at < 0 {
		return false
	}
	domain := strings.ToLower(strings.TrimSuffix(email[at+1:], "."))
	for _, tld := range reservedTLDs {
		if domain == tld || strings.HasSuffix(domain, "."+tld) {
			return true
		}
	}
	return false
}

// guarded bọc transport: thư tới miền dành cho test được ghi log rồi bỏ, không gửi. Dev cũng
// gửi qua Resend thật, mà tài khoản demo là @storeit.test.
type guarded struct {
	next Sender
	log  *slog.Logger
}

func (g guarded) Send(ctx context.Context, m Message) error {
	if IsReservedTestDomain(m.To.Email) {
		log := g.log
		if log == nil {
			log = logger.FromContext(ctx)
		}
		domain := m.To.Email[strings.LastIndexByte(m.To.Email, '@')+1:]
		log.WarnContext(ctx, "mail dropped: reserved test domain", slog.String("domain", domain))
		return nil
	}
	return g.next.Send(ctx, m)
}
