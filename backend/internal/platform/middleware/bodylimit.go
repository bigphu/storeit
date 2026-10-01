package middleware

import (
	"context"
	"io"
	"net/http"
)

type origBodyKey struct{}

// BodyLimit giới hạn body của request ở n byte: đọc quá thì gặp
// *http.MaxBytesError, web đổi thành 413. server.New gắn một lần cho mọi route
// (HTTP_MAX_BODY_BYTES), vì strict server của oapi-codegen đọc thẳng r.Body,
// không qua giới hạn nào khác.
//
// Gắn thêm ở một route thì thay hẳn giới hạn của server (nới hoặc siết), vì
// luôn bọc body gốc chứ không bọc chồng lên MaxBytesReader trước đó:
//
//	r.With(middleware.BodyLimit(50<<20), middleware.ReadTimeout(10*time.Minute)).Post("/imports", h)
//
// n <= 0 là không giới hạn.
func BodyLimit(n int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			orig, ok := r.Context().Value(origBodyKey{}).(io.ReadCloser)
			if !ok {
				orig = r.Body
			}
			// WithContext trả bản sao, không sửa r của middleware phía ngoài
			r = r.WithContext(context.WithValue(r.Context(), origBodyKey{}, orig))
			r.Body = orig
			if n > 0 && orig != nil && orig != http.NoBody {
				r.Body = http.MaxBytesReader(innermost(w), orig, n)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// innermost bóc các lớp bọc (chimw.WrapResponseWriter...) tới writer của
// net/http: chỉ writer đó nhận được tín hiệu "body quá lớn" để đóng kết nối
// thay vì đọc nốt phần thừa
func innermost(w http.ResponseWriter) http.ResponseWriter {
	for {
		u, ok := w.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			return w
		}
		w = u.Unwrap()
	}
}
