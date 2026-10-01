package logger

import (
	"io"
	"log/slog"
)

// New dựng logger ghi ra w theo cfg
func New(w io.Writer, cfg Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: cfg.Level}

	var h slog.Handler
	if cfg.Format == FormatPretty {
		h = NewPrettyHandler(w, opts)
	} else {
		h = slog.NewJSONHandler(w, opts)
	}
	return slog.New(newContextHandler(h))
}
