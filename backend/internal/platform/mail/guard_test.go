package mail

import (
	"context"
	"testing"
)

func TestIsReservedTestDomain(t *testing.T) {
	for _, e := range []string{"an@storeit.test", "x@a.example", "y@foo.invalid", "z@localhost", "Q@Mail.TEST"} {
		if !IsReservedTestDomain(e) {
			t.Errorf("%s: want reserved", e)
		}
	}
	for _, e := range []string{"an@storeit.vn", "a@test.com", "b@example.com", "c@mytest", "not-an-email"} {
		if IsReservedTestDomain(e) {
			t.Errorf("%s: want not reserved", e)
		}
	}
}

type fakeSender struct{ got []Message }

func (f *fakeSender) Send(_ context.Context, m Message) error {
	f.got = append(f.got, m)
	return nil
}

// Thư tới miền dành cho test (tài khoản demo @storeit.test) bị bỏ, transport không thấy gì
func TestSenderDropsReservedDomains(t *testing.T) {
	f := &fakeSender{}
	s := guarded{next: f}
	if err := s.Send(context.Background(), Message{To: Address{Email: "an@storeit.test"}, Subject: "x"}); err != nil {
		t.Fatal(err)
	}
	if len(f.got) != 0 {
		t.Fatalf("transport got %d messages for a .test address, want 0", len(f.got))
	}
	if err := s.Send(context.Background(), Message{To: Address{Email: "an@storeit.vn"}, Subject: "x"}); err != nil {
		t.Fatal(err)
	}
	if len(f.got) != 1 {
		t.Errorf("transport got %d messages for a real address, want 1", len(f.got))
	}
}

// New bọc transport gửi thật (Resend, SMTP có TLS) bằng guard; log và Mailpit (SMTP không
// TLS) giữ nguyên để dev vẫn thấy thư gửi tài khoản demo
func TestNewGuardsRealTransports(t *testing.T) {
	from := "StoreIt <noreply@storeit.vn>"
	for _, c := range []struct {
		cfg  Config
		want bool
	}{
		{Config{Transport: TransportResend, From: from, ResendAPIKey: "re_test"}, true},
		{Config{Transport: TransportSMTP, From: from, SMTPHost: "smtp.example.com", SMTPPort: 587, SMTPTLS: TLSStartTLS}, true},
		{Config{Transport: TransportSMTP, From: from, SMTPHost: "mailpit", SMTPPort: 1025, SMTPTLS: TLSNone}, false},
		{Config{Transport: TransportLog, From: from}, false},
	} {
		s, err := New(c.cfg, nil)
		if err != nil {
			t.Fatalf("%s: %v", c.cfg.Transport, err)
		}
		if _, ok := s.(guarded); ok != c.want {
			t.Errorf("%s (tls %q): guarded = %v, want %v", c.cfg.Transport, c.cfg.SMTPTLS, ok, c.want)
		}
	}
}
