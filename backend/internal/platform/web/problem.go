package web

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"storeit/internal/platform/errs"
)

// WriteProblem trả err về client dạng problem+json. Lỗi 5xx được log kèm
// nguyên nhân, còn client chỉ thấy thông báo chung.
//
// Dùng làm ResponseErrorHandlerFunc của strict server (xem RequestError).
func WriteProblem(w http.ResponseWriter, r *http.Request, err error) {
	problem := errs.NewFrom(err)
	if problem.Status() >= http.StatusInternalServerError {
		slog.ErrorContext(r.Context(), "request failed",
			"err", err, "status", problem.Status(), "method", r.Method, "path", r.URL.Path)
	}
	RenderProblem(w, r, problem)
}

// RequestError trả lỗi cho request hỏng trước khi vào handler: body không đọc
// được, hoặc path/query param sai. Gắn vào cả hai chỗ oapi-codegen gọi:
//
//	strict := api.NewStrictHandlerWithOptions(h, nil, api.StrictHTTPServerOptions{
//		RequestErrorHandlerFunc:  web.RequestError,
//		ResponseErrorHandlerFunc: web.WriteProblem,
//	})
//	api.HandlerWithOptions(strict, api.ChiServerOptions{BaseRouter: r, ErrorHandlerFunc: web.RequestError})
func RequestError(w http.ResponseWriter, r *http.Request, err error) {
	if isBodyError(err) {
		WriteProblem(w, r, decodeError(err))
		return
	}
	// Còn lại là lỗi param do code sinh ra, message đủ rõ để trả thẳng
	WriteProblem(w, r, ErrInvalidRequest.With(errs.WithDetail(err.Error()), errs.WithCause(err)))
}

// isBodyError: lỗi khi đọc JSON body (strict server bọc bằng "can't decode JSON body: %w")
func isBodyError(err error) bool {
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	var maxErr *http.MaxBytesError
	return errors.As(err, &syntaxErr) || errors.As(err, &typeErr) || errors.As(err, &maxErr) ||
		errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
}

const ProblemContentType = "application/problem+json"

// RenderProblem chỉ ghi response, không log. Dùng khi đã log rồi, vd Recoverer
func RenderProblem(w http.ResponseWriter, r *http.Request, err error) {
	problem := errs.NewFrom(err)

	w.Header().Set("Content-Type", ProblemContentType)
	w.WriteHeader(problem.Status())

	if encErr := json.NewEncoder(w).Encode(problem); encErr != nil {
		slog.WarnContext(r.Context(), "write problem response", "err", encErr)
	}
}
