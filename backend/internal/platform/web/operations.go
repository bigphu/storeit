package web

import (
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
)

// MustOperations kiểm danh sách operation dạng "METHOD <route pattern>" (cho
// auth.Public, middleware.ForOperations) với spec lúc khởi động, rồi trả lại
// nguyên danh sách. Gõ sai (thiếu baseURL, sai tên param, method viết thường...)
// thì panic, thay vì lặng lẽ không khớp request nào:
//
//	public := web.MustOperations(spec, "/api/v1", "POST /api/v1/auth/login")
//	authMW := auth.Middleware(tokens, auth.Public(public...))
func MustOperations(spec *openapi3.T, baseURL string, ops ...string) []string {
	known := operationKeys(spec, baseURL)
	for _, op := range ops {
		if !known[op] {
			panic(fmt.Sprintf("web: operation %q is not in the spec (want \"METHOD %s/path\", path and param names as in the spec)", op, baseURL))
		}
	}
	return ops
}

// operationKey là khoá của một operation, dùng chung cho ValidateRequests và
// MustOperations để hai bên không lệch nhau
func operationKey(method, baseURL, path string) string {
	return method + " " + baseURL + path
}

// operationKeys là khoá "METHOD baseURL+path" của mọi operation trong spec,
// cùng dạng với r.Method + " " + route pattern của chi
func operationKeys(spec *openapi3.T, baseURL string) map[string]bool {
	keys := map[string]bool{}
	for path, item := range spec.Paths.Map() {
		for method := range item.Operations() {
			keys[operationKey(method, baseURL, path)] = true
		}
	}
	return keys
}
