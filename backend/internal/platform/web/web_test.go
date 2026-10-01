package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"storeit/internal/platform/errs"
	"storeit/internal/platform/logger"
)

type address struct {
	City string `json:"city" validate:"required"`
}

type createThing struct {
	Email   string  `json:"email,omitempty" validate:"required,email"`
	Age     int     `json:"age" validate:"min=18"`
	Note    string  `validate:"max=3"` // không có tag json => dùng tên field Go
	Address address `json:"address"`
}

type problemBody struct {
	Type   string            `json:"type"`
	Title  string            `json:"title"`
	Status int               `json:"status"`
	Detail string            `json:"detail"`
	Fields []errs.FieldError `json:"errors"`
}

func request(body string) *http.Request {
	return httptest.NewRequest(http.MethodPost, "/things", strings.NewReader(body))
}

func asProblem(t *testing.T, err error) *errs.Error {
	t.Helper()
	var e *errs.Error
	if !errors.As(err, &e) {
		t.Fatalf("want *errs.Error, got %T: %v", err, err)
	}
	return e
}

func TestDecode(t *testing.T) {
	valid := `{"email":"a@b.co","age":20,"Note":"ok","address":{"city":"HN"}}`

	tests := []struct {
		name       string
		body       string
		wantStatus int // 0 = không lỗi
		wantType   string
		wantFields []string
	}{
		{"valid", valid, 0, "", nil},
		{"empty body", ``, 400, "/errors/malformed-json", nil},
		{"syntax error", `{"email":`, 400, "/errors/malformed-json", nil},
		{"two values", valid + valid, 400, "/errors/malformed-json", nil},
		{"wrong type", `{"age":"old"}`, 422, "/errors/validation-failed", []string{"age"}},
		{
			"validation uses json names",
			`{"email":"nope","age":3,"Note":"long","address":{}}`,
			422, "/errors/validation-failed",
			[]string{"email", "age", "Note", "address.city"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var in createThing
			err := Decode(request(tt.body), &in)
			if tt.wantStatus == 0 {
				if err != nil {
					t.Fatalf("want no error, got %v", err)
				}
				return
			}
			e := asProblem(t, err)
			if e.Status() != tt.wantStatus || e.Type() != tt.wantType {
				t.Fatalf("got %d %s, want %d %s", e.Status(), e.Type(), tt.wantStatus, tt.wantType)
			}
			var got []string
			for _, f := range e.Fields() {
				got = append(got, f.Field)
			}
			if fmt.Sprint(got) != fmt.Sprint(tt.wantFields) {
				t.Errorf("fields = %v, want %v", got, tt.wantFields)
			}
		})
	}
}

// captureLogs gom log của slog.Default vào buf trong lúc test chạy
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	old := slog.Default()
	slog.SetDefault(logger.New(buf, logger.Config{Level: slog.LevelDebug, Format: logger.FormatJSON}))
	t.Cleanup(func() { slog.SetDefault(old) })
	return buf
}

func decodeProblem(t *testing.T, rec *httptest.ResponseRecorder) problemBody {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); ct != ProblemContentType {
		t.Errorf("Content-Type = %q, want %q", ct, ProblemContentType)
	}
	var p problemBody
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("body is not JSON: %v\n%s", err, rec.Body.String())
	}
	return p
}

func TestWriteProblem(t *testing.T) {
	t.Run("500 is logged with request context, cause hidden from client", func(t *testing.T) {
		logs := captureLogs(t)
		ctx := logger.With(context.Background(), slog.String("request_id", "req-1"))
		r := httptest.NewRequest(http.MethodGet, "/things/1", nil).WithContext(ctx)
		rec := httptest.NewRecorder()

		WriteProblem(rec, r, errors.New("pq: connection refused"))

		p := decodeProblem(t, rec)
		if rec.Code != 500 || p.Status != 500 {
			t.Fatalf("status = %d / %d, want 500", rec.Code, p.Status)
		}
		if strings.Contains(rec.Body.String(), "connection refused") {
			t.Errorf("cause leaked to client: %s", rec.Body.String())
		}
		for _, want := range []string{`"level":"ERROR"`, "connection refused", `"request_id":"req-1"`, `"path":"/things/1"`} {
			if !strings.Contains(logs.String(), want) {
				t.Errorf("log missing %s:\n%s", want, logs.String())
			}
		}
	})

	t.Run("4xx is not logged and keeps its detail", func(t *testing.T) {
		logs := captureLogs(t)
		rec := httptest.NewRecorder()
		notFound := errs.NotFound("/errors/thing-not-found", "Thing not found", errs.WithDetail("thing 7"))

		WriteProblem(rec, httptest.NewRequest(http.MethodGet, "/", nil), fmt.Errorf("get thing: %w", notFound))

		p := decodeProblem(t, rec)
		if rec.Code != 404 || p.Type != "/errors/thing-not-found" || p.Detail != "thing 7" {
			t.Errorf("got %d %+v", rec.Code, p)
		}
		if logs.Len() != 0 {
			t.Errorf("4xx should not be logged, got:\n%s", logs.String())
		}
	})
}

func TestRequestError(t *testing.T) {
	var syntaxErr error
	{
		var v any
		syntaxErr = json.Unmarshal([]byte(`{"name":`), &v)
	}

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantType   string
		wantDetail string // chuỗi con
	}{
		{
			"strict server body decode error",
			fmt.Errorf("can't decode JSON body: %w", syntaxErr),
			400, "/errors/malformed-json", "",
		},
		{
			// Lỗi khác của code sinh (form, multipart...): không trả message gốc
			"other generated error",
			errors.New("can't decode formdata: mime: invalid boundary in /tmp/upload-123"),
			400, "/errors/invalid-request", "The request could not be read.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			RequestError(rec, httptest.NewRequest(http.MethodPost, "/things/abc", nil), tt.err)

			p := decodeProblem(t, rec)
			if rec.Code != tt.wantStatus || p.Type != tt.wantType {
				t.Fatalf("got %d %s, want %d %s", rec.Code, p.Type, tt.wantStatus, tt.wantType)
			}
			if !strings.Contains(p.Detail, tt.wantDetail) {
				t.Errorf("detail = %q, want it to contain %q", p.Detail, tt.wantDetail)
			}
		})
	}
}

// Handler trả lỗi sau khi đã ghi response (vd Write lỗi vì client ngắt): không
// ghi problem+json nối vào phần đã gửi, chỉ log
func TestHandle_ErrorAfterResponseStarted(t *testing.T) {
	logs := captureLogs(t)
	h := Handle(func(w http.ResponseWriter, r *http.Request) error {
		if err := JSON(w, http.StatusCreated, map[string]int{"id": 1}); err != nil {
			return err
		}
		return errors.New("late failure")
	})

	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodPost, "/things", nil))

	if rec.Code != http.StatusCreated || rec.Body.String() != `{"id":1}` {
		t.Errorf("got %d %q, want 201 %q", rec.Code, rec.Body.String(), `{"id":1}`)
	}
	if !strings.Contains(logs.String(), "late failure") {
		t.Errorf("error not logged:\n%s", logs.String())
	}
}

// Lỗi 5xx được log bằng logger trong ctx của request (RequestLogger đặt vào),
// không phải slog.Default()
func TestWriteProblem_LogsToContextLogger(t *testing.T) {
	global := captureLogs(t)
	var buf bytes.Buffer
	ctx := logger.NewContext(context.Background(), logger.New(&buf, logger.Config{}))
	r := httptest.NewRequest(http.MethodGet, "/things/1", nil).WithContext(ctx)

	WriteProblem(httptest.NewRecorder(), r, errors.New("db down"))

	if !strings.Contains(buf.String(), "db down") {
		t.Errorf("ctx logger missing error:\n%s", buf.String())
	}
	if global.Len() != 0 {
		t.Errorf("slog.Default() should not be used:\n%s", global.String())
	}
}

// Giới hạn body do middleware.BodyLimit đặt (r.Body là MaxBytesReader); Decode
// đổi MaxBytesError thành 413 và nêu đúng giới hạn của route
func TestDecode_TooLarge(t *testing.T) {
	valid := `{"email":"a@b.co","age":20,"Note":"ok","address":{"city":"HN"}}`
	for name, body := range map[string]string{
		"value too large": `{"email":"` + strings.Repeat("a", 200) + `"}`,
		// Giá trị đầu hợp lệ, phần thừa phía sau mới làm body vượt giới hạn
		"too large after value": valid + strings.Repeat(" ", 200),
	} {
		t.Run(name, func(t *testing.T) {
			req := request(body)
			req.Body = http.MaxBytesReader(httptest.NewRecorder(), req.Body, 100)

			var in createThing
			e := asProblem(t, Decode(req, &in))

			if e.Status() != http.StatusRequestEntityTooLarge || !strings.Contains(e.Detail(), "100 bytes") {
				t.Errorf("got %d %q, want 413 naming the 100-byte limit", e.Status(), e.Detail())
			}
		})
	}
}

// Handler đã đặt header cho file tải về rồi mới lỗi: problem+json không được
// mang Content-Length, Content-Disposition... của file đó
func TestWriteProblem_DropsHandlerHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	for k, v := range map[string]string{
		"Content-Length":      "999999",
		"Content-Disposition": `attachment; filename="export.xlsx"`,
		"Etag":                `"abc"`,
		"Last-Modified":       "Wed, 01 Oct 2026 00:00:00 GMT",
		"Cache-Control":       "public, max-age=3600",
	} {
		rec.Header().Set(k, v)
	}

	WriteProblem(rec, httptest.NewRequest(http.MethodGet, "/exports/1", nil), ErrRouteNotFound)

	for _, k := range []string{"Content-Length", "Content-Disposition", "Etag", "Last-Modified", "Cache-Control"} {
		if v := rec.Header().Get(k); v != "" {
			t.Errorf("%s = %q, want removed", k, v)
		}
	}
	if decodeProblem(t, rec).Status != http.StatusNotFound {
		t.Errorf("body = %s", rec.Body)
	}
}

// Giống các kiểu lỗi param oapi-codegen sinh trong từng package api
type InvalidParamFormatError struct {
	ParamName string
	Err       error
}

func (e *InvalidParamFormatError) Error() string {
	return "Invalid format for parameter " + e.ParamName + ": " + e.Err.Error()
}

type RequiredParamError struct{ ParamName string }

func (e *RequiredParamError) Error() string {
	return "Query argument " + e.ParamName + " is required, but not found"
}

// Lỗi param do wrapper sinh ra bắt (trước ValidateRequests) phải cùng dạng với
// lỗi param của validator: 400 kèm field, không lộ kiểu Go bên trong
func TestRequestError_GeneratedParamErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantField  string
		wantDetail string
	}{
		{
			"bad format",
			&InvalidParamFormatError{ParamName: "thingID", Err: errors.New("error unmarshaling 'abc' text as *uuid.UUID: invalid UUID length: 3")},
			"thingID", "has an invalid format",
		},
		{"required", &RequiredParamError{ParamName: "page"}, "page", "is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			RequestError(rec, httptest.NewRequest(http.MethodGet, "/things/abc", nil), tt.err)

			p := decodeProblem(t, rec)
			if rec.Code != 400 || p.Type != "/errors/invalid-request" {
				t.Fatalf("got %d %s", rec.Code, p.Type)
			}
			if len(p.Fields) != 1 || p.Fields[0].Field != tt.wantField || p.Fields[0].Detail != tt.wantDetail {
				t.Errorf("fields = %+v, want %s %q", p.Fields, tt.wantField, tt.wantDetail)
			}
			if strings.Contains(rec.Body.String(), "uuid.UUID") {
				t.Errorf("Go type leaked to client: %s", rec.Body)
			}
		})
	}
}

func TestRequestError_OtherErrorsHideInternals(t *testing.T) {
	rec := httptest.NewRecorder()
	RequestError(rec, httptest.NewRequest(http.MethodPost, "/things", nil),
		errors.New("can't decode formdata: mime: invalid boundary in /tmp/upload-123"))

	if strings.Contains(rec.Body.String(), "/tmp/upload-123") || strings.Contains(rec.Body.String(), "mime:") {
		t.Errorf("internal error text leaked: %s", rec.Body)
	}
}

// Gọi nhầm với err nil không được panic: trả 500 chung
func TestWriteProblem_NilError(t *testing.T) {
	captureLogs(t)
	rec := httptest.NewRecorder()

	WriteProblem(rec, httptest.NewRequest(http.MethodGet, "/", nil), nil)

	if p := decodeProblem(t, rec); rec.Code != 500 || p.Type != "/errors/internal" {
		t.Errorf("got %d %+v, want 500 internal", rec.Code, p)
	}
}

type rules struct {
	Email   string   `json:"email" validate:"omitempty,email"`
	Site    string   `json:"site" validate:"omitempty,url"`
	ID      string   `json:"id" validate:"omitempty,uuid"`
	Name    string   `json:"name" validate:"omitempty,min=2,max=4"`
	Qty     int      `json:"qty" validate:"omitempty,min=1,max=9"`
	Code    string   `json:"code" validate:"omitempty,len=3"`
	Kind    string   `json:"kind" validate:"omitempty,oneof=a b"`
	Pass    string   `json:"pass"`
	Confirm string   `json:"confirm" validate:"eqfield=Pass"`
	Tags    []string `json:"tags" validate:"omitempty,unique"`
}

// Mỗi tag validate có câu dễ đọc; tag lạ vẫn có câu chung nêu tên rule
func TestValidate_RuleMessages(t *testing.T) {
	tests := []struct {
		in    rules
		field string
		want  string
	}{
		{rules{Email: "x"}, "email", "must be a valid email address"},
		{rules{Site: "x"}, "site", "must be a valid URL"},
		{rules{ID: "x"}, "id", "must be a valid UUID"},
		{rules{Name: "x"}, "name", "must be at least 2 characters"},
		{rules{Name: "xxxxx"}, "name", "must be at most 4 characters"},
		{rules{Qty: -1}, "qty", "must be at least 1"},
		{rules{Qty: 10}, "qty", "must be at most 9"},
		{rules{Code: "x"}, "code", "must be exactly 3 characters"},
		{rules{Kind: "c"}, "kind", "must be one of: a, b"},
		{rules{Pass: "a", Confirm: "b"}, "confirm", "must match Pass"},
		{rules{Tags: []string{"a", "a"}}, "tags", `failed the "unique" rule`},
	}
	for _, tt := range tests {
		t.Run(tt.field+" "+tt.want, func(t *testing.T) {
			e := asProblem(t, Validate(&tt.in))
			fields := e.Fields()
			if len(fields) != 1 || fields[0].Field != tt.field || fields[0].Detail != tt.want {
				t.Errorf("fields = %+v, want %s %q", fields, tt.field, tt.want)
			}
		})
	}
}

func TestRespond(t *testing.T) {
	rec := httptest.NewRecorder()
	if err := NoContent(rec); err != nil || rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Errorf("NoContent: %v %d %q", err, rec.Code, rec.Body)
	}

	rec = httptest.NewRecorder()
	if err := Text(rec, http.StatusOK, "ok"); err != nil || rec.Body.String() != "ok" ||
		rec.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Errorf("Text: %v %q %q", err, rec.Body, rec.Header().Get("Content-Type"))
	}

	// Giá trị không encode được: lỗi 500, chưa ghi gì ra response
	rec = httptest.NewRecorder()
	err := JSON(rec, http.StatusOK, make(chan int))
	if asProblem(t, err).Status() != 500 || rec.Body.Len() != 0 {
		t.Errorf("JSON(chan): err = %v, body %q", err, rec.Body)
	}
}

func TestNotFoundAndMethodNotAllowed(t *testing.T) {
	for _, tt := range []struct {
		h    http.HandlerFunc
		code int
		typ  string
	}{
		{NotFoundHandler(), 404, "/errors/route-not-found"},
		{MethodNotAllowedHandler(), 405, "/errors/method-not-allowed"},
	} {
		rec := httptest.NewRecorder()
		tt.h(rec, httptest.NewRequest(http.MethodDelete, "/things", nil))
		if p := decodeProblem(t, rec); rec.Code != tt.code || p.Type != tt.typ || !strings.Contains(p.Detail, "/things") {
			t.Errorf("got %d %+v", rec.Code, p)
		}
	}
}
