package middleware

import (
	"context"
	"net/http"
	"regexp"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

// validRequestID: ID nhận từ header phải ngắn và chỉ có ký tự an toàn, để
// client không chèn được dòng log giả hay làm log phình ra
var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// RequestID thay chimw.RequestID: dùng X-Request-Id của request nếu hợp lệ (vd
// reverse proxy đã đặt), không thì sinh UUID mới. ID nằm ở chỗ chimw.GetReqID
// đọc, và được trả lại trong header X-Request-Id của response để client đối
// chiếu với log.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(chimw.RequestIDHeader)
		if !validRequestID.MatchString(id) {
			id = uuid.NewString()
		}
		w.Header().Set(chimw.RequestIDHeader, id)
		ctx := context.WithValue(r.Context(), chimw.RequestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
