package errs

import (
	"fmt"
)

// Option gắn thông tin của một lần xảy ra lỗi: chi tiết, nguyên nhân, field
// sai. Status, type, title là danh tính của lỗi nên truyền thẳng vào New, không
// có option để đổi, nhờ vậy With không làm errors.Is mất tác dụng.
type Option func(*Error)

func (e *Error) apply(opts []Option) {
	for _, opt := range opts {
		if opt != nil {
			opt(e)
		}
	}
}

// WithCause giữ lỗi gốc để log và errors.Is/As. Client không thấy cause.
func WithCause(cause error) Option {
	return func(e *Error) {
		e.cause = cause
	}
}

// WithDetail là câu giải thích cho client. Chuỗi giữ nguyên, không format,
// nên truyền được cả chuỗi có dấu %.
func WithDetail(detail string) Option {
	return func(e *Error) {
		e.detail = detail
	}
}

// WithDetailf như WithDetail nhưng format theo fmt.Sprintf
func WithDetailf(format string, args ...any) Option {
	return WithDetail(fmt.Sprintf(format, args...))
}

// WithFields thêm field sai vào danh sách (không thay thế field đã có)
func WithFields(fields ...FieldError) Option {
	return func(e *Error) {
		e.fields = append(e.fields, fields...)
	}
}
