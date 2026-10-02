package mail

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestLogSenderWritesMessage(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewTextHandler(&buf, nil))
	s, err := New(Config{Transport: TransportLog, From: "no-reply@storeit.test"}, log)
	if err != nil {
		t.Fatal(err)
	}
	m := testMsg
	m.Text = "Open http://app/accept-invite#token=abc"
	if err := s.Send(context.Background(), m); err != nil {
		t.Fatalf("Send() = %v", err)
	}
	out := buf.String()
	for _, want := range []string{"level=WARN", "lan@storeit.test", m.Subject, "accept-invite#token=abc"} {
		if !strings.Contains(out, want) {
			t.Errorf("log %q does not contain %q", out, want)
		}
	}
}

// nil logger: rơi về slog.Default, không panic
func TestLogSenderNilLogger(t *testing.T) {
	s, err := New(Config{Transport: TransportLog, From: "no-reply@storeit.test"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Send(context.Background(), testMsg); err != nil {
		t.Fatal(err)
	}
}
