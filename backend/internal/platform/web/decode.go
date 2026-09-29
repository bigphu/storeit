package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"storeit/internal/platform/errs"
)

const maxBodyBytes = 1 << 20 // 1 MB

func Decode(r *http.Request, dst any) error {
	reader := http.MaxBytesReader(nil, r.Body, maxBodyBytes)
	dec := json.NewDecoder(reader)
	if err := dec.Decode(dst); err != nil {
		return decodeError(err)
	}

	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ErrMalformedJSON.With(
			errs.WithDetail("The request body must contain exactly one JSON value."))
	}
	return Validate(dst)
}

func decodeError(err error) error {
	var errMaxBytes *http.MaxBytesError
	if errors.As(err, &errMaxBytes) {
		return ErrRequestTooLarge.With(
			errs.WithDetailf("The request body must not exceed %d bytes.", maxBodyBytes),
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
