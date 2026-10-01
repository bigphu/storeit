package middleware

import (
	"net/http"
	"time"

	"storeit/internal/platform/logger"
)

// ReadTimeout đổi hạn đọc request (gồm body) của route thành d, tính từ lúc
// request vào tới đây. Server giữ HTTP_READ_TIMEOUT ngắn cho API JSON; route
// upload file lớn tự gắn, thường cùng BodyLimit:
//
//	r.With(middleware.BodyLimit(50<<20), middleware.ReadTimeout(10*time.Minute)).Post("/imports", h)
//
// d <= 0 là bỏ hạn đọc (không giới hạn), không phải hết hạn ngay.
func ReadTimeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var deadline time.Time // zero = không có hạn
			if d > 0 {
				deadline = time.Now().Add(d)
			}
			// ResponseController đi xuyên writer đã bọc (chimw.WrapResponseWriter
			// có Unwrap) tới kết nối thật
			if err := http.NewResponseController(w).SetReadDeadline(deadline); err != nil {
				logger.FromContext(r.Context()).WarnContext(r.Context(),
					"cannot extend read deadline", "err", err, "timeout", d)
			}
			next.ServeHTTP(w, r)
		})
	}
}
