package web

import (
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/go-chi/chi/v5"

	"storeit/internal/platform/errs"
)

// ValidateRequests kiểm tra mọi request theo spec OpenAPI của module trước khi
// vào handler: path/query param, Content-Type và body (required, kiểu,
// minLength, enum, format...). Handler sinh từ spec không phải tự validate.
// Spec lấy từ code sinh (embedded-spec: true trong oapi.yaml); baseURL là
// BaseURL đã truyền cho HandlerWithOptions, vd "/api/v1".
//
// Tìm operation theo route pattern của chi, nên phải chạy sau khi chi route
// xong: gắn qua Middlewares của oapi-codegen, cùng chỗ với auth. oapi-codegen
// áp middleware cuối danh sách ở ngoài cùng, nên auth đứng sau để chạy trước
// (thiếu token là 401, không phải 422):
//
//	spec, err := api.GetSwagger()
//	...
//	api.HandlerWithOptions(strict, api.ChiServerOptions{
//		BaseRouter:       r,
//		BaseURL:          "/api/v1",
//		Middlewares:      []api.MiddlewareFunc{web.ValidateRequests(spec, "/api/v1"), authMW},
//		ErrorHandlerFunc: web.RequestError,
//	})
//
// Lỗi trả về dạng problem+json: param sai 400, body sai schema 422 (kèm danh
// sách field), JSON hỏng 400, Content-Type sai 415, body quá giới hạn 413.
// Security trong spec bỏ qua ở đây, auth.Middleware lo.
//
// Chỉ body JSON được đọc để validate theo schema. Body khác (file upload) chỉ
// được kiểm Content-Type rồi để nguyên cho handler stream, không đọc hết vào RAM.
// Operation cần body lớn hơn HTTP_MAX_BODY_BYTES thì nới bằng
// middleware.ForOperations, đặt sau ValidateRequests trong Middlewares.
//
// baseURL không khớp servers trong spec thì panic ngay lúc dựng (khởi động),
// thay vì mọi request đều 500.
func ValidateRequests(spec *openapi3.T, baseURL string) func(http.Handler) http.Handler {
	registerFormats()
	checkBaseURL(spec, baseURL)
	routes := map[string]*routers.Route{}
	for path, item := range spec.Paths.Map() {
		for method, op := range item.Operations() {
			// Bỏ security của operation (auth.Middleware lo): kin-openapi
			// kiểm security bằng cách đọc hết body vào RAM cho mọi request,
			// kể cả upload. Bản sao nông, spec gốc không đổi.
			noSecurity := *op
			noSecurity.Security = &openapi3.SecurityRequirements{}
			routes[operationKey(method, baseURL, path)] = &routers.Route{
				Spec: spec, Path: path, PathItem: item, Method: method, Operation: &noSecurity,
			}
		}
	}
	opts := &openapi3filter.Options{
		MultiError:         true,
		AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
		// Không chèn default của schema vào request: body bị viết lại (với
		// merge-patch, field vắng mặt là "giữ nguyên", chèn default là ghi đè
		// dữ liệu; +json khác application/json thì không viết lại được và
		// request hợp lệ bị từ chối), còn param thì code sinh đã đọc trước khi
		// middleware chạy nên default không tới handler. Default xử lý trong Go.
		SkipSettingDefaults: true,
	}
	// Cho body không phải JSON: chỉ validate param
	paramsOnly := *opts
	paramsOnly.ExcludeRequestBody = true

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rc := chi.RouteContext(r.Context())
			var pattern string
			if rc != nil {
				pattern = rc.RoutePattern()
			}
			route, ok := routes[r.Method+" "+pattern]
			if !ok {
				// Middleware gắn cho route không có trong spec (sai baseURL, gắn
				// bằng r.Use...): báo ngay thay vì lặng lẽ bỏ qua validate
				WriteProblem(w, r, errs.ErrInternal.With(errs.WithCause(
					fmt.Errorf("openapi: no operation for %s %q in spec", r.Method, pattern))))
				return
			}

			params := make(map[string]string, len(rc.URLParams.Keys))
			for i, k := range rc.URLParams.Keys {
				params[k] = pathParamValue(r, rc.URLParams.Values[i])
			}

			// Tự kiểm Content-Type theo spec trước khi gọi kin-openapi, để không
			// phải đoán loại lỗi qua message của nó
			ct := normalizeContentType(r)
			if !contentTypeAllowed(route, ct, r) {
				WriteProblem(w, r, unsupportedMediaType(route, nil))
				return
			}
			useOpts := opts
			if ct != "" && !validatesBody(ct) {
				// File upload...: Content-Type đã đúng, để body nguyên cho handler.
				// Không đọc body nên tự kiểm required: không có body là sai
				if rb := route.Operation.RequestBody; rb != nil && rb.Value.Required && !hasBody(r) {
					WriteProblem(w, r, validationError([]errs.FieldError{{Field: "(body)", Detail: "is required"}}))
					return
				}
				useOpts = &paramsOnly
			}

			// Với body JSON, ValidateRequest đọc body rồi đặt lại vào r.Body
			err := openapi3filter.ValidateRequest(r.Context(), &openapi3filter.RequestValidationInput{
				Request:    r,
				PathParams: params,
				Route:      route,
				Options:    useOpts,
			})
			if err != nil {
				WriteProblem(w, r, requestValidationError(err))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// checkBaseURL: baseURL phải là path của một server trong spec (nếu spec có
// khai báo servers không chứa biến)
func checkBaseURL(spec *openapi3.T, baseURL string) {
	if len(spec.Servers) == 0 {
		return
	}
	want := strings.TrimSuffix(baseURL, "/")
	var paths []string
	for _, s := range spec.Servers {
		if strings.Contains(s.URL, "{") { // URL có biến, không so được
			return
		}
		u, err := url.Parse(s.URL)
		if err != nil {
			continue
		}
		if p := strings.TrimSuffix(u.Path, "/"); p == want {
			return
		} else {
			paths = append(paths, p)
		}
	}
	panic(fmt.Sprintf("web: ValidateRequests baseURL %q does not match spec servers %q", baseURL, paths))
}

// contentTypeAllowed: operation có body khai báo Content-Type trong spec thì
// request phải gửi một trong các kiểu đó; có body mà không có Content-Type cũng
// sai. Body rỗng không có Content-Type thì để kin-openapi báo thiếu body nếu
// body là required.
func contentTypeAllowed(route *routers.Route, ct string, r *http.Request) bool {
	rb := route.Operation.RequestBody
	if rb == nil || len(rb.Value.Content) == 0 {
		return true
	}
	if ct == "" {
		return !hasBody(r)
	}
	return rb.Value.Content.Get(ct) != nil
}

// normalizeContentType đưa Content-Type về dạng chuẩn (kiểu và tên tham số viết
// thường, vd "Application/JSON; Charset=UTF-8" thành "application/json;
// charset=UTF-8") rồi ghi lại vào header. Media type không phân biệt hoa thường,
// nhưng kin-openapi so khớp với spec và code sinh chọn nhánh body (strings.HasPrefix)
// đều phân biệt, nên header hợp lệ sẽ bị 415 hoặc body bị bỏ qua. Header không
// đọc được thì để nguyên cho bước kiểm sau báo 415.
func normalizeContentType(r *http.Request) string {
	ct := r.Header.Get("Content-Type")
	if ct == "" {
		return ""
	}
	mt, params, err := mime.ParseMediaType(ct)
	if err != nil {
		return ct
	}
	if canon := mime.FormatMediaType(mt, params); canon != "" {
		r.Header.Set("Content-Type", canon)
		return canon
	}
	return ct
}

// hasBody: request có body (ContentLength -1 là chưa biết độ dài, coi như có)
func hasBody(r *http.Request) bool {
	return r.Body != nil && r.Body != http.NoBody && r.ContentLength != 0
}

// validatesBody: body có được đọc để validate theo schema không. Chỉ JSON
// (application/json, các kiểu +json) mà kin-openapi có decoder; +json tự đặt
// (vd application/vnd.storeit.report+json) kin không đọc được, coi như upload:
// chỉ kiểm Content-Type.
func validatesBody(contentType string) bool {
	mt, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	isJSON := mt == "application/json" || strings.HasSuffix(mt, "+json")
	return isJSON && openapi3filter.RegisteredBodyDecoder(mt) != nil
}

// pathParamValue unescape giá trị path param như code sinh làm trước khi đưa
// cho handler. chi trả đoạn path thô (còn %XX) khi URL có RawPath, kin-openapi
// không unescape, nên không làm thì maxLength, pattern... kiểm sai chuỗi.
func pathParamValue(r *http.Request, v string) string {
	if r.URL.RawPath == "" {
		return v
	}
	if u, err := url.PathUnescape(v); err == nil {
		return u
	}
	return v
}

// unsupportedMediaType nêu các Content-Type operation nhận theo spec
func unsupportedMediaType(route *routers.Route, cause error) error {
	var types []string
	if rb := route.Operation.RequestBody; rb != nil {
		for t := range rb.Value.Content {
			types = append(types, t)
		}
	}
	slices.Sort(types)
	detail := "The request body must not have a Content-Type."
	if len(types) > 0 {
		detail = "The request body must be one of: " + strings.Join(types, ", ") + "."
	}
	return ErrUnsupportedMediaType.With(errs.WithDetail(detail), errs.WithCause(cause))
}

var registerFormatsOnce sync.Once

// registerFormats khai báo format "uuid" cho kin-openapi (mặc định không kiểm
// tra). Dùng pattern dạng chuẩn chứ không FormatOfStringForUUIDOfRFC4122 có sẵn:
// pattern đó chỉ nhận version 1-5, trong khi ID của app là UUIDv7. Khai báo
// toàn cục vì param không nhận format validator riêng theo từng lần validate.
func registerFormats() {
	registerFormatsOnce.Do(func() {
		openapi3.DefineStringFormatValidator("uuid", openapi3.NewRegexpFormatValidator(
			`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`))
	})
}

// requestValidationError đổi lỗi của kin-openapi thành *errs.Error. Phân loại
// theo kiểu lỗi (RequestError, ParseError, SchemaError, ErrInvalidRequired),
// không theo message, để nâng cấp kin-openapi không làm đổi mã lỗi; Content-Type
// đã được kiểm trước (contentTypeAllowed).
func requestValidationError(err error) error {
	var (
		paramFields, bodyFields []errs.FieldError
		malformed               bool
		other                   []error
	)
	for _, e := range flatten(err) {
		var maxErr *http.MaxBytesError
		if errors.As(e, &maxErr) {
			return decodeError(maxErr)
		}
		var re *openapi3filter.RequestError
		if !errors.As(e, &re) {
			other = append(other, e)
			continue
		}
		switch {
		case re.Parameter != nil:
			for _, d := range details(re) {
				paramFields = append(paramFields, errs.FieldError{Field: re.Parameter.Name, Detail: d})
			}
		case re.RequestBody != nil && isParseError(re.Err):
			malformed = true
		case errors.Is(re.Err, openapi3filter.ErrInvalidRequired):
			bodyFields = append(bodyFields, errs.FieldError{Field: "(body)", Detail: "is required"})
		default:
			for _, se := range schemaErrors(re.Err) {
				bodyFields = append(bodyFields, errs.FieldError{Field: fieldOf(se), Detail: se.Reason})
			}
			if len(schemaErrors(re.Err)) == 0 {
				other = append(other, re)
			}
		}
	}

	switch {
	case malformed:
		return ErrMalformedJSON.With(errs.WithCause(err))
	case len(paramFields) > 0:
		// Body cũng sai thì báo luôn, để client sửa một lần
		return ErrInvalidRequest.With(errs.WithFields(append(paramFields, bodyFields...)...),
			errs.WithDetail("The request has invalid parameters."), errs.WithCause(err))
	case len(bodyFields) > 0:
		return validationError(bodyFields).With(errs.WithCause(err))
	}
	// Lỗi lạ: message của kin-openapi có cả schema lẫn giá trị gửi lên, không
	// trả cho client; nguyên nhân vẫn được giữ để log
	return ErrInvalidRequest.With(errs.WithDetail("The request does not match the API specification."),
		errs.WithCause(errors.Join(other...)))
}

// isParseError: body không đọc được theo Content-Type đã khai báo (JSON hỏng)
func isParseError(err error) bool {
	var pe *openapi3filter.ParseError
	return errors.As(err, &pe)
}

// flatten tách openapi3.MultiError (lồng nhau) thành danh sách lỗi đơn. Ép
// kiểu trực tiếp chứ không errors.As: errors.As đi xuyên RequestError tới
// MultiError của schema bên trong, làm mất RequestError (param nào, body hay không)
func flatten(err error) []error {
	if me, ok := err.(openapi3.MultiError); ok {
		var out []error
		for _, e := range me {
			out = append(out, flatten(e)...)
		}
		return out
	}
	return []error{err}
}

func schemaErrors(err error) []*openapi3.SchemaError {
	var out []*openapi3.SchemaError
	for _, e := range flatten(err) {
		var se *openapi3.SchemaError
		if errors.As(e, &se) {
			out = append(out, se)
		}
	}
	return out
}

// details là các câu lỗi của một param: lý do của schema nếu có, không thì lỗi
// bên trong (vd không đọc được "maybe" thành boolean). Không dùng re.Error():
// nó lặp lại tên param đã có ở field.
func details(re *openapi3filter.RequestError) []string {
	var out []string
	for _, se := range schemaErrors(re.Err) {
		out = append(out, se.Reason)
	}
	if len(out) == 0 {
		switch {
		case re.Err != nil:
			out = append(out, re.Err.Error())
		case re.Reason != "":
			out = append(out, re.Reason)
		default:
			out = append(out, "is invalid")
		}
	}
	return out
}

// fieldOf trả đường dẫn field dạng address.city, giống web.Validate
func fieldOf(se *openapi3.SchemaError) string {
	if p := se.JSONPointer(); len(p) > 0 {
		return strings.Join(p, ".")
	}
	return "(body)"
}
