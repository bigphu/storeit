package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi/v5"
)

// Spec nhỏ giống spec của module: servers /api/v1, security toàn cục (auth do
// auth.Middleware lo, validator bỏ qua)
const testSpec = `
openapi: 3.0.3
info: { title: test, version: 0.0.0 }
servers: [ { url: /api/v1 } ]
security: [ { bearerAuth: [] } ]
paths:
  /things:
    post:
      operationId: createThing
      parameters:
        - { name: dryRun, in: query, schema: { type: boolean } }
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [name, address]
              properties:
                name: { type: string, minLength: 1 }
                age: { type: integer, minimum: 0 }
                color: { type: string, default: red }
                address:
                  type: object
                  required: [city]
                  properties:
                    city: { type: string }
      responses: { "201": { description: created } }
  /files:
    post:
      operationId: uploadFile
      requestBody:
        required: true
        content:
          application/octet-stream:
            schema: { type: string, format: binary }
      responses: { "204": { description: ok } }
  /things/{thingID}:
    get:
      operationId: getThing
      parameters:
        - { name: thingID, in: path, required: true, schema: { type: string, format: uuid } }
        - { name: limit, in: query, schema: { type: integer, maximum: 10 } }
      responses: { "200": { description: ok } }
    patch:
      operationId: patchThing
      parameters:
        - { name: thingID, in: path, required: true, schema: { type: string, format: uuid } }
      requestBody:
        required: true
        content:
          application/merge-patch+json:
            schema:
              type: object
              properties:
                name: { type: string, minLength: 1 }
                status: { type: string, default: active }
      responses: { "204": { description: ok } }
  /reports:
    post:
      operationId: createReport
      requestBody:
        required: true
        content:
          application/vnd.storeit.report+json:
            schema:
              type: object
              required: [title]
              properties:
                title: { type: string }
      responses: { "204": { description: ok } }
  /tags/{tag}:
    get:
      operationId: getTag
      parameters:
        - { name: tag, in: path, required: true, schema: { type: string, maxLength: 3 } }
      responses: { "204": { description: ok } }
components:
  securitySchemes:
    bearerAuth: { type: http, scheme: bearer }
`

func loadTestSpec(t *testing.T) *openapi3.T {
	t.Helper()
	spec, err := openapi3.NewLoader().LoadFromData([]byte(testSpec))
	if err != nil {
		t.Fatal(err)
	}
	return spec
}

// specRouter gắn validator như Middlewares của oapi-codegen (chạy sau khi chi
// route xong); handler ghi lại body nó đọc được
func specRouter(t *testing.T, gotBody *string) *chi.Mux {
	t.Helper()
	mw := ValidateRequests(loadTestSpec(t), "/api/v1")
	h := func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		*gotBody = string(b)
		w.WriteHeader(http.StatusNoContent)
	}
	r := chi.NewRouter()
	r.With(mw).Post("/api/v1/things", h)
	r.With(mw).Get("/api/v1/things/{thingID}", h)
	r.With(mw).Get("/api/v1/not-in-spec", h)
	r.With(mw).Post("/api/v1/files", h)
	r.With(mw).Patch("/api/v1/things/{thingID}", h)
	r.With(mw).Post("/api/v1/reports", h)
	r.With(mw).Get("/api/v1/tags/{tag}", h)
	return r
}

func TestValidateRequests(t *testing.T) {
	const okBody = `{"name":"drill","age":3,"address":{"city":"HN"}}`
	const thing = "/api/v1/things/7f0c4e1a-2b3c-4d5e-8f90-123456789abc"
	tests := []struct {
		name, method, path, contentType, body string
		wantStatus                            int
		wantType                              string
		wantFields                            []string
	}{
		{"valid body reaches handler", "POST", "/api/v1/things", "application/json", okBody, 204, "", nil},
		{"valid params", "GET", thing + "?limit=5", "", "", 204, "", nil},
		{
			"body violates schema", "POST", "/api/v1/things", "application/json",
			`{"name":"","age":-1,"address":{}}`,
			422, "/errors/validation-failed", []string{"address.city", "age", "name"},
		},
		{"missing required body", "POST", "/api/v1/things", "application/json", "", 422, "/errors/validation-failed", nil},
		{"malformed JSON", "POST", "/api/v1/things", "application/json", `{"name":`, 400, "/errors/malformed-json", nil},
		{"wrong content type", "POST", "/api/v1/things", "text/plain", okBody, 415, "/errors/unsupported-media-type", nil},
		{"body without content type", "POST", "/api/v1/things", "", okBody, 415, "/errors/unsupported-media-type", nil},
		// Param và body cùng sai: báo hết, không bỏ lỗi body
		{
			"param and body errors together", "POST", "/api/v1/things?dryRun=maybe", "application/json",
			`{"name":"","address":{"city":"HN"}}`,
			400, "/errors/invalid-request", []string{"dryRun", "name"},
		},
		{"upload accepted", "POST", "/api/v1/files", "application/octet-stream", "raw bytes", 204, "", nil},
		// Body upload là required: Content-Type đúng nhưng không có body vẫn sai
		{"upload with empty body", "POST", "/api/v1/files", "application/octet-stream", "", 422, "/errors/validation-failed", []string{"(body)"}},
		// Không chèn default của schema vào body: với merge-patch, field vắng
		// mặt nghĩa là "giữ nguyên", chèn default là ghi đè dữ liệu. Body tới
		// handler phải y nguyên (bảng kiểm gotBody == body)
		{"merge-patch body passed through unchanged", "PATCH", thing, "application/merge-patch+json", `{"name":"x"}`, 204, "", nil},
		// +json kin-openapi không có decoder: không báo JSON hỏng, chỉ kiểm
		// Content-Type rồi để handler đọc
		{"custom +json type accepted", "POST", "/api/v1/reports", "application/vnd.storeit.report+json", `{"title":"q3"}`, 204, "", nil},
		// %41 là "A": path param phải được validate sau khi unescape ("Ab",
		// dài 2), giống giá trị handler nhận, không phải "%41b" (dài 4)
		{"escaped path param validated unescaped", "GET", "/api/v1/tags/%41b", "", "", 204, "", nil},
		{"escaped path param too long", "GET", "/api/v1/tags/%41bcd", "", "", 400, "/errors/invalid-request", []string{"tag"}},
		{"upload wrong content type", "POST", "/api/v1/files", "text/plain", "raw bytes", 415, "/errors/unsupported-media-type", nil},
		{"query param out of range", "GET", thing + "?limit=50", "", "", 400, "/errors/invalid-request", []string{"limit"}},
		{"path param bad format", "GET", "/api/v1/things/not-a-uuid", "", "", 400, "/errors/invalid-request", []string{"thingID"}},
		// Route có middleware nhưng không có trong spec: gắn sai chỗ, báo 500
		// ngay thay vì lặng lẽ bỏ qua validate
		{"route missing from spec", "GET", "/api/v1/not-in-spec", "", "", 500, "/errors/internal", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotBody string
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			rec := httptest.NewRecorder()

			specRouter(t, &gotBody).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.wantStatus == http.StatusNoContent {
				// Validator đọc body rồi phải trả lại nguyên vẹn cho handler
				if gotBody != tt.body {
					t.Errorf("handler body = %q, want %q", gotBody, tt.body)
				}
				return
			}
			var p problemBody
			if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
				t.Fatalf("not problem JSON: %s", rec.Body)
			}
			if p.Type != tt.wantType {
				t.Errorf("type = %q, want %q", p.Type, tt.wantType)
			}
			if tt.wantFields != nil {
				var got []string
				for _, f := range p.Fields {
					got = append(got, f.Field)
				}
				slices.Sort(got)
				if !slices.Equal(got, tt.wantFields) {
					t.Errorf("fields = %v, want %v (%s)", got, tt.wantFields, rec.Body)
				}
			}
		})
	}
}

// Body vượt giới hạn (middleware.BodyLimit bọc r.Body bằng MaxBytesReader) thì
// validator đọc body gặp MaxBytesError: trả 413, không phải 400
func TestValidateRequests_BodyTooLarge(t *testing.T) {
	var gotBody string
	r := specRouter(t, &gotBody)
	body := `{"name":"` + strings.Repeat("a", 100) + `","address":{"city":"HN"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/things", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	req.Body = http.MaxBytesReader(rec, req.Body, 32)

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413; body: %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "32 bytes") {
		t.Errorf("detail should name the 32-byte limit: %s", rec.Body)
	}
}

// 415 nêu đúng Content-Type mà operation nhận theo spec
func TestValidateRequests_UnsupportedMediaTypeNamesSpecTypes(t *testing.T) {
	for path, want := range map[string]string{
		"/api/v1/things": "application/json",
		"/api/v1/files":  "application/octet-stream",
	} {
		var gotBody string
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader("x"))
		req.Header.Set("Content-Type", "text/plain")
		rec := httptest.NewRecorder()

		specRouter(t, &gotBody).ServeHTTP(rec, req)

		if rec.Code != http.StatusUnsupportedMediaType || !strings.Contains(rec.Body.String(), want) {
			t.Errorf("%s: got %d %s, want 415 naming %s", path, rec.Code, rec.Body, want)
		}
	}
}

// countingReader đếm số byte đã bị đọc khỏi body
type countingReader struct {
	r io.Reader
	n int
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += n
	return n, err
}

// Body không phải JSON (file upload) không bị validator đọc hết vào RAM: handler
// nhận body gốc, chưa ai đọc byte nào, để stream thẳng xuống storage
func TestValidateRequests_UploadBodyNotBuffered(t *testing.T) {
	body := &countingReader{r: strings.NewReader(strings.Repeat("x", 4096))}
	readBeforeHandler := -1
	r := chi.NewRouter()
	r.With(ValidateRequests(loadTestSpec(t), "/api/v1")).Post("/api/v1/files", func(w http.ResponseWriter, r *http.Request) {
		readBeforeHandler = body.n
		_, _ = io.Copy(io.Discard, r.Body)
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/files", io.NopCloser(body))
	req.Header.Set("Content-Type", "application/octet-stream")

	r.ServeHTTP(httptest.NewRecorder(), req)

	if readBeforeHandler != 0 {
		t.Errorf("validator read %d bytes of the upload before the handler, want 0", readBeforeHandler)
	}
	if body.n != 4096 {
		t.Errorf("handler read %d bytes, want 4096", body.n)
	}
}

// baseURL lệch với servers trong spec thì mọi request đều 500: phải báo ngay
// lúc dựng middleware (khởi động), không đợi tới request đầu tiên
func TestValidateRequests_BaseURLMustMatchServers(t *testing.T) {
	spec := loadTestSpec(t)
	defer func() {
		if recover() == nil {
			t.Error("want panic for baseURL not in spec servers")
		}
	}()
	ValidateRequests(spec, "/api/v2")
}

func TestValidateRequests_SpecWithoutServersAcceptsAnyBaseURL(t *testing.T) {
	spec := loadTestSpec(t)
	spec.Servers = nil
	ValidateRequests(spec, "/anything") // không panic
}

// Danh sách operation (cho auth.Public, middleware.ForOperations) được kiểm
// với spec lúc khởi động: gõ sai thì panic, không lặng lẽ không khớp gì
func TestMustOperations(t *testing.T) {
	spec := loadTestSpec(t)
	ops := []string{"POST /api/v1/files", "GET /api/v1/things/{thingID}"}
	if got := MustOperations(spec, "/api/v1", ops...); !slices.Equal(got, ops) {
		t.Errorf("got %v, want %v", got, ops)
	}

	for _, bad := range []string{
		"POST /api/v1/file",       // thiếu s
		"PUT /api/v1/files",       // sai method
		"POST /files",             // thiếu baseURL
		"post /api/v1/files",      // method phải viết hoa như r.Method
		"GET /api/v1/things/{id}", // tên param khác spec
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%q: want panic", bad)
				}
			}()
			MustOperations(spec, "/api/v1", bad)
		}()
	}
}

// Content-Type không phân biệt hoa thường, nhưng code sinh cho operation nhiều
// kiểu body so bằng strings.HasPrefix (phân biệt hoa thường): header phải tới
// handler ở dạng chuẩn, nếu không body bị bỏ qua mà không báo lỗi
func TestValidateRequests_NormalizesContentType(t *testing.T) {
	var gotCT string
	r := chi.NewRouter()
	r.With(ValidateRequests(loadTestSpec(t), "/api/v1")).Post("/api/v1/things", func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/things",
		strings.NewReader(`{"name":"drill","address":{"city":"HN"}}`))
	req.Header.Set("Content-Type", "Application/JSON; Charset=UTF-8")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	if gotCT != "application/json; charset=UTF-8" {
		t.Errorf("handler Content-Type = %q, want %q", gotCT, "application/json; charset=UTF-8")
	}
}
