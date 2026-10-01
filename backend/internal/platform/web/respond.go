package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"storeit/internal/platform/errs"
)

func JSON(w http.ResponseWriter, status int, v any) error {
	buf, err := json.Marshal(v)
	if err != nil {
		return errs.ErrInternal.With(
			errs.WithCause(fmt.Errorf("encode response: %w", err)))
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write(buf); err != nil {
		return errs.ErrInternal.With(
			errs.WithCause(fmt.Errorf("write response: %w", err)))
	}
	return nil
}

func NoContent(w http.ResponseWriter) error {
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func Text(w http.ResponseWriter, status int, body string) error {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write([]byte(body)); err != nil {
		return errs.ErrInternal.With(
			errs.WithCause(fmt.Errorf("write response: %w", err)))
	}
	return nil
}
