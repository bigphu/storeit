package mail

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testKey = "re_secret_key_123"

func newResendForTest(t *testing.T, h http.HandlerFunc, mod ...func(*Config)) (Sender, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	cfg := Config{
		Transport: TransportResend, From: "StoreIt <no-reply@storeit.test>",
		ResendAPIKey: testKey, ResendBaseURL: srv.URL,
	}
	for _, f := range mod {
		f(&cfg)
	}
	s, err := New(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	return s, srv
}

var testMsg = Message{
	To:             Address{Name: "Nguyễn Lan", Email: "lan@storeit.test"},
	Subject:        "Your StoreIt invitation",
	Text:           "plain body",
	HTML:           "<p>html body</p>",
	IdempotencyKey: "identity/invite/0193",
}

func TestResendSendsRequest(t *testing.T) {
	var got struct {
		method, path, auth, idem, ctype string
		body                            map[string]any
	}
	s, _ := newResendForTest(t, func(w http.ResponseWriter, r *http.Request) {
		got.method, got.path = r.Method, r.URL.Path
		got.auth, got.idem, got.ctype = r.Header.Get("Authorization"), r.Header.Get("Idempotency-Key"), r.Header.Get("Content-Type")
		_ = json.NewDecoder(r.Body).Decode(&got.body)
		_, _ = io.WriteString(w, `{"id":"49a3999c"}`)
	})
	if err := s.Send(context.Background(), testMsg); err != nil {
		t.Fatalf("Send() = %v", err)
	}
	if got.method != http.MethodPost || got.path != "/emails" {
		t.Errorf("request = %s %s, want POST /emails", got.method, got.path)
	}
	if got.auth != "Bearer "+testKey {
		t.Errorf("Authorization = %q", got.auth)
	}
	if got.idem != testMsg.IdempotencyKey {
		t.Errorf("Idempotency-Key = %q, want %q", got.idem, testMsg.IdempotencyKey)
	}
	if got.ctype != "application/json" {
		t.Errorf("Content-Type = %q", got.ctype)
	}
	want := map[string]any{
		"from":    `"StoreIt" <no-reply@storeit.test>`,
		"to":      []any{`=?utf-8?q?Nguy=E1=BB=85n_Lan?= <lan@storeit.test>`},
		"subject": testMsg.Subject,
		"text":    testMsg.Text,
		"html":    testMsg.HTML,
	}
	for k, v := range want {
		if fmt.Sprint(got.body[k]) != fmt.Sprint(v) {
			t.Errorf("body[%s] = %v, want %v", k, got.body[k], v)
		}
	}
	if _, ok := got.body["reply_to"]; ok {
		t.Error("reply_to sent although MAIL_REPLY_TO is empty")
	}
}

func TestResendReplyTo(t *testing.T) {
	var body map[string]any
	s, _ := newResendForTest(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = io.WriteString(w, `{"id":"x"}`)
	}, func(c *Config) { c.ReplyTo = "support@storeit.test" })
	if err := s.Send(context.Background(), testMsg); err != nil {
		t.Fatal(err)
	}
	if body["reply_to"] != "support@storeit.test" {
		t.Errorf("reply_to = %v", body["reply_to"])
	}
}

// Không có idempotency key thì không gửi header rỗng
func TestResendWithoutIdempotencyKey(t *testing.T) {
	var present bool
	s, _ := newResendForTest(t, func(w http.ResponseWriter, r *http.Request) {
		_, present = r.Header["Idempotency-Key"]
		_, _ = io.WriteString(w, `{"id":"x"}`)
	})
	m := testMsg
	m.IdempotencyKey = ""
	if err := s.Send(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	if present {
		t.Error("Idempotency-Key header sent with empty value")
	}
}

func TestResendErrorStatus(t *testing.T) {
	cases := []struct {
		status     int
		body       string
		permanent  bool
		wantReason string
	}{
		{422, `{"name":"validation_error","message":"The storeit.test domain is not verified."}`, true, "validation_error: The storeit.test domain is not verified."},
		{403, `{"message":"forbidden"}`, true, "forbidden"},
		{429, `{"name":"rate_limit_exceeded","message":"Too many requests"}`, false, "rate_limit_exceeded: Too many requests"},
		{500, `<html>bad gateway</html>`, false, "no readable reason in response"},
		{401, `{"name":"invalid_api_key","message":"API key ` + testKey + ` is invalid"}`, true, "invalid_api_key: API key [redacted] is invalid"},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprint(tc.status), func(t *testing.T) {
			s, _ := newResendForTest(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			})
			err := s.Send(context.Background(), testMsg)
			var se *StatusError
			if !errors.As(err, &se) {
				t.Fatalf("Send() = %v (%T), want *StatusError", err, err)
			}
			if se.Status != tc.status || se.Reason != tc.wantReason {
				t.Errorf("StatusError = %d %q, want %d %q", se.Status, se.Reason, tc.status, tc.wantReason)
			}
			if IsPermanent(err) != tc.permanent {
				t.Errorf("IsPermanent() = %v, want %v", IsPermanent(err), tc.permanent)
			}
			if strings.Contains(err.Error(), testKey) {
				t.Errorf("error leaks the API key: %v", err)
			}
		})
	}
}

func TestResendLongReasonTruncated(t *testing.T) {
	s, _ := newResendForTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = fmt.Fprintf(w, `{"message":%q}`, strings.Repeat("ồ", 500))
	})
	var se *StatusError
	if !errors.As(s.Send(context.Background(), testMsg), &se) {
		t.Fatal("want *StatusError")
	}
	if n := len([]rune(se.Reason)); n > 301 {
		t.Errorf("reason has %d runes, want at most 301", n)
	}
}

// Lỗi giao vận không được mang URL hay khoá API vào log
func TestResendTransportErrorHidesURL(t *testing.T) {
	s, srv := newResendForTest(t, func(w http.ResponseWriter, r *http.Request) {})
	srv.Close()
	err := s.Send(context.Background(), testMsg)
	if err == nil {
		t.Fatal("Send() to a closed server = nil")
	}
	if strings.Contains(err.Error(), srv.URL) || strings.Contains(err.Error(), testKey) {
		t.Errorf("error leaks URL or key: %v", err)
	}
	if IsPermanent(err) {
		t.Error("a network error must be retried")
	}
}

func TestStatusErrorPermanent(t *testing.T) {
	for status, want := range map[int]bool{
		400: true, 401: true, 403: true, 404: true, 422: true,
		408: false, 429: false, 500: false, 502: false, 503: false,
	} {
		if got := (&StatusError{Status: status}).Permanent(); got != want {
			t.Errorf("Permanent(%d) = %v, want %v", status, got, want)
		}
	}
}

func TestIsPermanentUnwraps(t *testing.T) {
	err := fmt.Errorf("identity: send invite: %w", &StatusError{Status: 422})
	if !IsPermanent(err) {
		t.Error("IsPermanent does not see through %w")
	}
	if IsPermanent(errors.New("boom")) || IsPermanent(nil) {
		t.Error("IsPermanent true for a plain error or nil")
	}
}

// MAIL_FROM chỉ có địa chỉ (không tên): gửi đúng địa chỉ trần, không phải "<a@b>"
func TestResendFromWithoutName(t *testing.T) {
	var body map[string]any
	s, _ := newResendForTest(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = io.WriteString(w, `{"id":"x"}`)
	}, func(c *Config) { c.From = "no-reply@storeit.test" })
	m := testMsg
	m.To = Address{Email: "lan@storeit.test"}
	if err := s.Send(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	if body["from"] != "no-reply@storeit.test" {
		t.Errorf("from = %v, want the bare address", body["from"])
	}
	if fmt.Sprint(body["to"]) != "[lan@storeit.test]" {
		t.Errorf("to = %v, want the bare address", body["to"])
	}
}
