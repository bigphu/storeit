package errs

import (
	"net/http"
)

func Invalid(typ, title string, opts ...Option) *Error {
	return New(http.StatusBadRequest, typ, title, opts...)
}

func Unauthorized(typ, title string, opts ...Option) *Error {
	return New(http.StatusUnauthorized, typ, title, opts...)
}

func Forbidden(typ, title string, opts ...Option) *Error {
	return New(http.StatusForbidden, typ, title, opts...)
}

func NotFound(typ, title string, opts ...Option) *Error {
	return New(http.StatusNotFound, typ, title, opts...)
}

func Conflict(typ, title string, opts ...Option) *Error {
	return New(http.StatusConflict, typ, title, opts...)
}

func Unprocessable(typ, title string, opts ...Option) *Error {
	return New(http.StatusUnprocessableEntity, typ, title, opts...)
}

func Internal(typ, title string, opts ...Option) *Error {
	return New(http.StatusInternalServerError, typ, title, opts...)
}

var ErrInternal = Internal(
	"/errors/internal", "Internal server error",
	WithDetail("An unexpected error occurred."))
