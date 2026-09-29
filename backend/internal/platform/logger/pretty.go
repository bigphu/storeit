package logger

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

const (
	ansiReset  = "\033[0m"
	ansiDim    = "\033[2m"
	ansiRed    = "\033[31m"
	ansiYellow = "\033[33m"
	ansiCyan   = "\033[36m"
	ansiGray   = "\033[90m"
)

const timeFormat = "15:04:05.000"

// blockIndent đẩy khối nhiều dòng thẳng cột với message (bỏ qua phần giờ)
const blockIndent = "             "

// PrettyHandler in log dễ đọc cho dev:
//
//	15:04:05.000 INFO  request method=GET status=200 dur=12ms
//
// Phần attr do một slog.TextHandler bên trong format, handler này chỉ lo giờ,
// level có màu và message. Luôn có màu, trừ khi đặt NO_COLOR (container dev
// không có TTY nên không dò terminal được).
type PrettyHandler struct {
	inner slog.Handler  // TextHandler, chỉ ghi phần attr vào buf
	buf   *bytes.Buffer // chỗ inner ghi ra, đọc lại trong Handle
	mu    *sync.Mutex   // khoá buf và out, mọi bản sao dùng chung
	out   io.Writer
	color bool
}

var _ slog.Handler = (*PrettyHandler)(nil)

// NewPrettyHandler nhận tham số giống slog.NewTextHandler, opts có thể nil
func NewPrettyHandler(w io.Writer, opts *slog.HandlerOptions) *PrettyHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	buf := &bytes.Buffer{}
	return &PrettyHandler{
		inner: slog.NewTextHandler(buf, &slog.HandlerOptions{
			Level:       opts.Level,
			AddSource:   opts.AddSource,
			ReplaceAttr: suppressDefaults(opts.ReplaceAttr),
		}),
		buf:   buf,
		mu:    &sync.Mutex{},
		out:   w,
		color: os.Getenv("NO_COLOR") == "",
	}
}

func (h *PrettyHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.inner.Enabled(ctx, l)
}

func (h *PrettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h2 := *h
	h2.inner = h.inner.WithAttrs(attrs)
	return &h2
}

func (h *PrettyHandler) WithGroup(name string) slog.Handler {
	h2 := *h
	h2.inner = h.inner.WithGroup(name)
	return &h2
}

func (h *PrettyHandler) Handle(ctx context.Context, r slog.Record) error {
	r, blocks := splitMultiline(r)

	h.mu.Lock()
	defer h.mu.Unlock()

	attrs, err := h.formatAttrs(ctx, r)
	if err != nil {
		return err
	}

	var b strings.Builder
	if !r.Time.IsZero() {
		b.WriteString(h.colorize(ansiDim, r.Time.Format(timeFormat)))
		b.WriteByte(' ')
	}
	// Level đủ 5 ký tự để message các dòng thẳng cột
	b.WriteString(h.colorize(levelColor(r.Level), fmt.Sprintf("%-5s", r.Level)))
	b.WriteByte(' ')
	b.WriteString(r.Message)
	if attrs != "" {
		b.WriteByte(' ')
		b.WriteString(h.colorize(ansiGray, attrs))
	}
	b.WriteByte('\n')

	for _, a := range blocks {
		b.WriteString(blockIndent)
		b.WriteString(h.colorize(ansiGray, a.Key+":"))
		b.WriteByte('\n')
		for _, line := range strings.Split(strings.TrimRight(a.Value.String(), "\n"), "\n") {
			b.WriteString(blockIndent + "  ")
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}

	_, err = io.WriteString(h.out, b.String())
	return err
}

// formatAttrs lấy phần key=value do inner ghi vào buf. Gọi khi đang giữ h.mu
func (h *PrettyHandler) formatAttrs(ctx context.Context, r slog.Record) (string, error) {
	h.buf.Reset()
	if err := h.inner.Handle(ctx, r); err != nil {
		return "", err
	}
	return strings.TrimSuffix(h.buf.String(), "\n"), nil
}

// suppressDefaults bỏ time, level, msg khỏi output của inner (Handle tự in ở
// đầu dòng), attr còn lại mới qua ReplaceAttr của người gọi nếu có
func suppressDefaults(next func([]string, slog.Attr) slog.Attr) func([]string, slog.Attr) slog.Attr {
	return func(groups []string, a slog.Attr) slog.Attr {
		if len(groups) == 0 {
			switch a.Key {
			case slog.TimeKey, slog.LevelKey, slog.MessageKey:
				return slog.Attr{}
			}
		}
		if next == nil {
			return a
		}
		return next(groups, a)
	}
}

// splitMultiline tách attr nhiều dòng (stack trace...) để in thành khối riêng,
// vì TextHandler sẽ escape "\n" thành một dòng dài khó đọc. Chỉ xét attr của
// chính record, attr gắn qua With đã được format sẵn
func splitMultiline(r slog.Record) (slog.Record, []slog.Attr) {
	var blocks []slog.Attr
	rest := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		if a.Value.Kind() == slog.KindString && strings.Contains(a.Value.String(), "\n") {
			blocks = append(blocks, a)
		} else {
			rest.AddAttrs(a)
		}
		return true
	})
	return rest, blocks
}

func (h *PrettyHandler) colorize(color, s string) string {
	if !h.color {
		return s
	}
	return color + s + ansiReset
}

func levelColor(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return ansiRed
	case l >= slog.LevelWarn:
		return ansiYellow
	case l >= slog.LevelInfo:
		return ansiCyan
	default:
		return ansiDim
	}
}
