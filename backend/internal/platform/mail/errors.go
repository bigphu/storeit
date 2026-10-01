package mail

import (
	"errors"
	"fmt"
	"net/http"
)

// IsPermanent đúng khi thử gửi lại không thể cho kết quả khác: domain chưa
// xác thực, khoá API sai, máy chủ SMTP từ chối hẳn (5xx), địa chỉ hỏng. Job
// gửi thư dùng nó để huỷ thay vì thử lại hàng giờ.
func IsPermanent(err error) bool {
	var p interface{ Permanent() bool }
	return errors.As(err, &p) && p.Permanent()
}

// StatusError là lỗi HTTP từ Resend, giữ nguyên mã trạng thái qua mọi lớp bọc
type StatusError struct {
	Status int
	// Reason chỉ gồm name và message trong JSON lỗi của Resend, đã cắt ngắn và
	// che khoá API; không phải thân response thô
	Reason string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("mail: resend returned status %d: %s", e.Status, e.Reason)
}

// Permanent: 4xx là lỗi ở request, thử lại vô ích. Trừ 408 và 429, nói về
// thời điểm chứ không phải nội dung. 5xx là phía Resend hỏng, thử lại.
func (e *StatusError) Permanent() bool {
	switch e.Status {
	case http.StatusRequestTimeout, http.StatusTooManyRequests:
		return false
	}
	return e.Status >= 400 && e.Status < 500
}

// permanentError đánh dấu một lỗi là vĩnh viễn (SMTP 5xx, địa chỉ hỏng)
type permanentError struct{ err error }

func (e permanentError) Error() string   { return e.err.Error() }
func (e permanentError) Unwrap() error   { return e.err }
func (e permanentError) Permanent() bool { return true }
