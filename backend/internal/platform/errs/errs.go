package errs

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
)

type FieldError struct {
	Field  string `json:"field"`
	Detail string `json:"detail"`
}

// Field chỉ đọc được qua getter
type Error struct {
	cause error

	typ    string
	title  string
	status int
	detail string
	fields []FieldError
}

var _ error = (*Error)(nil)

var _ json.Marshaler = (*Error)(nil)

func (e *Error) Error() string {
	msg := e.title
	if e.detail != "" {
		msg += ": " + e.detail
	}
	if e.cause != nil {
		msg += ": " + e.cause.Error()
	}
	return msg
}

func (e *Error) Unwrap() error {
	return e.cause
}

func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && e.typ != "" && e.typ == t.typ
}

// New khai báo một loại lỗi. typ là URI định danh (so bằng errors.Is), title là
// câu ngắn không đổi giữa các lần xảy ra. Status hay dùng thì có sẵn hàm bên
// dưới (NotFound, Conflict...); status khác thì gọi New:
//
//	var ErrTooLarge = errs.New(http.StatusRequestEntityTooLarge,
//		"/errors/request-too-large", "Request body too large")
func New(status int, typ, title string, opts ...Option) *Error {
	e := &Error{status: status, typ: typ, title: title}
	e.apply(opts)
	return e
}

// NewFrom lấy *Error nằm trong chuỗi wrap của err. Không có thì trả
// ErrInternal, để client không thấy message nội bộ.
func NewFrom(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return ErrInternal
}

// With trả bản sao có thêm thông tin, lỗi gốc không đổi. Dùng trên biến lỗi
// khai báo sẵn:
//
//	return ErrAssetNotFound.With(errs.WithDetailf("asset %s", id), errs.WithCause(err))
func (e *Error) With(opts ...Option) *Error {
	clone := *e
	clone.fields = slices.Clone(e.fields)
	clone.apply(opts)
	return &clone
}

func (e *Error) Type() string {
	return e.typ
}

func (e *Error) Title() string {
	return e.title
}

func (e *Error) Status() int {
	if e.status != 0 {
		return e.status
	}
	return http.StatusInternalServerError
}

func (e *Error) Detail() string {
	return e.detail
}

func (e *Error) Fields() []FieldError {
	return append([]FieldError(nil), e.fields...) // copy
}

func (e *Error) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type   string       `json:"type,omitempty"`
		Title  string       `json:"title,omitempty"`
		Status int          `json:"status,omitempty"`
		Detail string       `json:"detail,omitempty"`
		Fields []FieldError `json:"errors,omitempty"`
	}{e.typ, e.title, e.Status(), e.detail, e.fields})
}
