package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"

	chimw "github.com/go-chi/chi/v5/middleware"

	"storeit/internal/platform/errs"
	"storeit/internal/platform/logger"
)

// WriteProblem trả err về client dạng problem+json. Lỗi 5xx được log kèm
// nguyên nhân (bằng logger trong ctx, xem logger.FromContext), còn client chỉ
// thấy thông báo chung. Response đã bắt đầu gửi thì không trả được lỗi nữa,
// nên lỗi nào cũng chỉ được log.
//
// Dùng làm ResponseErrorHandlerFunc của strict server (xem RequestError).
func WriteProblem(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil { // gọi nhầm: vẫn trả 500 thay vì panic
		err = errs.ErrInternal.With(errs.WithCause(errors.New("web: WriteProblem called with nil error")))
	}
	problem := errs.NewFrom(err)
	started := responseStarted(w)
	if started || problem.Status() >= http.StatusInternalServerError {
		logger.FromContext(r.Context()).ErrorContext(r.Context(), "request failed",
			"err", err, "status", problem.Status(), "method", r.Method, "path", r.URL.Path,
			"response_started", started)
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
//	api.HandlerWithOptions(strict, api.ChiServerOptions{BaseRouter: r, ErrorHandlerFunc: web.RequestError, ...})
//
// Body, param sai spec đã bị ValidateRequests chặn trước; RequestError chỉ còn
// gặp lỗi đọc body hay param mà code sinh ra phát hiện.
func RequestError(w http.ResponseWriter, r *http.Request, err error) {
	if isBodyError(err) {
		WriteProblem(w, r, decodeError(err))
		return
	}
	if name, detail, ok := generatedParamError(err); ok {
		WriteProblem(w, r, ErrInvalidRequest.With(
			errs.WithFields(errs.FieldError{Field: name, Detail: detail}),
			errs.WithDetail("The request has invalid parameters."),
			errs.WithCause(err)))
		return
	}
	// Lỗi khác của code sinh (form, multipart, đọc body...): message gốc có
	// chi tiết nội bộ, chỉ để log
	WriteProblem(w, r, ErrInvalidRequest.With(
		errs.WithDetail("The request could not be read."), errs.WithCause(err)))
}

// generatedParamError nhận lỗi param do wrapper oapi-codegen sinh ra
// (InvalidParamFormatError, RequiredParamError...): mỗi package api có kiểu
// riêng nhưng đều có field ParamName, nên đọc bằng reflect. Message gốc có tên
// kiểu Go bên trong (vd *uuid.UUID), chỉ để log; client nhận câu chung.
func generatedParamError(err error) (name, detail string, ok bool) {
	for e := err; e != nil; e = errors.Unwrap(e) {
		v := reflect.ValueOf(e)
		if v.Kind() == reflect.Pointer {
			v = v.Elem()
		}
		if v.Kind() != reflect.Struct {
			continue
		}
		f := v.FieldByName("ParamName")
		if !f.IsValid() || f.Kind() != reflect.String {
			continue
		}
		detail = "has an invalid format"
		if strings.HasPrefix(v.Type().Name(), "Required") {
			detail = "is required"
		}
		return f.String(), detail, true
	}
	return "", "", false
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

// RenderProblem chỉ ghi response, không log. Dùng khi đã log rồi, vd Recoverer.
// Response đã bắt đầu gửi (header đã đi) thì không ghi gì, để problem+json
// không bị nối vào phần response đã gửi.
func RenderProblem(w http.ResponseWriter, r *http.Request, err error) {
	if responseStarted(w) {
		return
	}
	if err == nil {
		err = errs.ErrInternal
	}
	problem := errs.NewFrom(err)

	// Header handler đã đặt cho response định trả (vd file tải về) không còn
	// đúng với problem+json: Content-Length sai làm body bị cắt
	h := w.Header()
	for _, k := range []string{"Content-Length", "Content-Disposition", "Content-Encoding",
		"Etag", "Last-Modified", "Cache-Control"} {
		h.Del(k)
	}
	h.Set("Content-Type", ProblemContentType)
	w.WriteHeader(problem.Status())

	if encErr := json.NewEncoder(w).Encode(problem); encErr != nil {
		logger.FromContext(r.Context()).WarnContext(r.Context(), "write problem response", "err", encErr)
	}
}

// WrapWriter bọc w để biết response đã bắt đầu gửi chưa, giữ nguyên
// Flusher/Hijacker của w. w đã được bọc sẵn (vd bởi RequestLogger) thì dùng lại.
func WrapWriter(w http.ResponseWriter, r *http.Request) chimw.WrapResponseWriter {
	if ww, ok := w.(chimw.WrapResponseWriter); ok {
		return ww
	}
	return chimw.NewWrapResponseWriter(w, r.ProtoMajor)
}

// responseStarted báo header đã được gửi. Chỉ biết được khi w được bọc bằng
// WrapWriter (Handle, RequestLogger, Recoverer đều bọc); không thì coi như chưa.
func responseStarted(w http.ResponseWriter) bool {
	ww, ok := w.(chimw.WrapResponseWriter)
	return ok && ww.Status() != 0
}
