package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"storeit/internal/platform/errs"
)

// Decode đọc body JSON của r vào dst rồi validate, cho handler viết tay (route
// sinh từ OpenAPI đã được ValidateRequests kiểm tra). Giới hạn kích thước do
// middleware.BodyLimit đặt (server.New gắn cho mọi route); vượt thì 413.
func Decode(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		return decodeError(err)
	}

	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		var errMaxBytes *http.MaxBytesError
		if errors.As(err, &errMaxBytes) {
			return decodeError(err)
		}
		return ErrMalformedJSON.With(
			errs.WithDetail("The request body must contain exactly one JSON value."))
	}
	return Validate(dst)
}

func decodeError(err error) error {
	var errMaxBytes *http.MaxBytesError
	if errors.As(err, &errMaxBytes) {
		return ErrRequestTooLarge.With(
			errs.WithDetailf("The request body must not exceed %d bytes.", errMaxBytes.Limit),
			errs.WithCause(err))
	}

	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return ErrMalformedJSON.With(
			errs.WithDetailf("The request body contains invalid JSON at byte %d.", syntaxErr.Offset),
			errs.WithCause(err))
	}

	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		field := typeErr.Field
		if field == "" {
			field = "(body)"
		}
		return validationError([]errs.FieldError{{
			Field:  field,
			Detail: fmt.Sprintf("must be of type %s", typeErr.Type.String()),
		}}).With(errs.WithCause(err))
	}

	if errors.Is(err, io.EOF) {
		return ErrMalformedJSON.With(
			errs.WithDetail("The request body must not be empty."))
	}

	return ErrMalformedJSON.With(
		errs.WithCause(err))
}
