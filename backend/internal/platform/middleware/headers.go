package middleware

import "net/http"

// NoSniff đặt X-Content-Type-Options: nosniff cho mọi response, để trình duyệt
// không đoán lại kiểu nội dung (vd coi JSON có chứa HTML là trang HTML)
func NoSniff(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}
