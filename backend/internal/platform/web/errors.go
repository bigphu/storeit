package web

import (
	"net/http"
	"storeit/internal/platform/errs"
)

var (
	ErrMalformedJSON = errs.Invalid(
		"/errors/malformed-json", "Malformed JSON body",
		errs.WithDetail("The request body could not be parsed as JSON."))

	ErrRequestTooLarge = errs.New(http.StatusRequestEntityTooLarge,
		"/errors/request-too-large", "Request body too large")

	ErrValidation = errs.Unprocessable(
		"/errors/validation-failed", "Validation failed")

	ErrInvalidRequest = errs.Invalid(
		"/errors/invalid-request", "Invalid request")

	ErrRouteNotFound = errs.NotFound(
		"/errors/route-not-found", "Route not found")

	ErrMethodNotAllowed = errs.New(http.StatusMethodNotAllowed,
		"/errors/method-not-allowed", "Method not allowed")

	ErrUnsupportedMediaType = errs.New(http.StatusUnsupportedMediaType,
		"/errors/unsupported-media-type", "Unsupported media type")
)
