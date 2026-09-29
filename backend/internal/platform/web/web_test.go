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
		{"too large", `{"email":"` + strings.Repeat("a", maxBodyBytes) + `"}`, 413, "/errors/request-too-large", nil},
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
			"param error from chi wrapper",
			errors.New("Invalid format for parameter thingID: error unmarshaling 'abc' text as *uuid.UUID"),
			400, "/errors/invalid-request", "parameter thingID",
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
