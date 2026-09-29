package logger

import (
	"bytes"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

func newTestLogger(buf *bytes.Buffer, level slog.Level) *slog.Logger {
	h := NewPrettyHandler(buf, &slog.HandlerOptions{Level: level})
	h.color = false
	return slog.New(h)
}

// dropTime bỏ phần giờ ở đầu dòng để so sánh ổn định
func dropTime(s string) string {
	var out []string
	for _, line := range strings.Split(strings.TrimSuffix(s, "\n"), "\n") {
		if len(line) > len(timeFormat) && line[2] == ':' {
			line = line[len(timeFormat)+1:]
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func TestPrettyHandler_SingleLine(t *testing.T) {
	var buf bytes.Buffer
	log := newTestLogger(&buf, slog.LevelInfo)

	log.Info("request", "method", "GET", "status", 200, "dur", 12*time.Millisecond, "note", "hello world")

	want := `INFO  request method=GET status=200 dur=12ms note="hello world"`
	if got := dropTime(buf.String()); got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestPrettyHandler_NoAttrs(t *testing.T) {
	var buf bytes.Buffer
	newTestLogger(&buf, slog.LevelInfo).Warn("disk almost full")

	if got, want := dropTime(buf.String()), "WARN  disk almost full"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPrettyHandler_GroupsAndWith(t *testing.T) {
	var buf bytes.Buffer
	log := newTestLogger(&buf, slog.LevelInfo).
		With("req_id", "abc").
		WithGroup("http").
		With("path", "/x")

	log.Info("done", slog.Group("resp", "status", 201))

	want := `INFO  done req_id=abc http.path=/x http.resp.status=201`
	if got := dropTime(buf.String()); got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestPrettyHandler_Level(t *testing.T) {
	var buf bytes.Buffer
	log := newTestLogger(&buf, slog.LevelWarn)

	log.Info("hidden")
	log.Error("shown")

	if got, want := dropTime(buf.String()), "ERROR shown"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPrettyHandler_MultilineBlock(t *testing.T) {
	var buf bytes.Buffer
	newTestLogger(&buf, slog.LevelInfo).Error("panic recovered", "path", "/x", "stack", "line1\nline2\n")

	want := "ERROR panic recovered path=/x\n" +
		blockIndent + "stack:\n" +
		blockIndent + "  line1\n" +
		blockIndent + "  line2"
	if got := dropTime(buf.String()); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

type valueStringer struct{ s string }

func (v valueStringer) String() string { return v.s }

// Con trỏ nil tới kiểu có String() nhận receiver giá trị sẽ panic khi gọi;
// TextHandler bên trong phải nuốt được panic đó
func TestPrettyHandler_NilPointerStringer(t *testing.T) {
	var buf bytes.Buffer
	var p *valueStringer

	newTestLogger(&buf, slog.LevelInfo).Info("x", "v", p)

	if got, want := dropTime(buf.String()), "INFO  x v=<nil>"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPrettyHandler_Color(t *testing.T) {
	var buf bytes.Buffer
	h := NewPrettyHandler(&buf, nil)
	h.color = true
	slog.New(h).Info("hi", "k", "v")

	if !strings.Contains(buf.String(), ansiCyan+"INFO "+ansiReset) {
		t.Errorf("level not colorized: %q", buf.String())
	}
}

// Các logger tách ra từ With dùng chung buf; chạy với -race để bắt lỗi khoá
func TestPrettyHandler_Concurrent(t *testing.T) {
	var buf bytes.Buffer
	base := newTestLogger(&buf, slog.LevelInfo)

	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			base.With("worker", i).Info("tick", "n", i)
		}()
	}
	wg.Wait()

	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(lines) != 20 {
		t.Fatalf("got %d lines, want 20", len(lines))
	}
	for _, l := range lines {
		if !strings.Contains(l, "INFO  tick worker=") {
			t.Errorf("garbled line: %q", l)
		}
	}
}
