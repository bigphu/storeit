package errs

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

var errThing = NotFound("/errors/thing-not-found", "Thing not found")

func TestWith_KeepsIdentityAndOriginal(t *testing.T) {
	cause := errors.New("no rows")
	got := errThing.With(WithDetailf("thing %d", 7), WithCause(cause), nil)

	if got.Status() != http.StatusNotFound || got.Type() != errThing.Type() || got.Title() != errThing.Title() {
		t.Errorf("identity changed: %d %s %s", got.Status(), got.Type(), got.Title())
	}
	if got.Detail() != "thing 7" {
		t.Errorf("detail = %q", got.Detail())
	}
	// bọc thêm một lớp vẫn khớp cả lỗi gốc lẫn cause
	wrapped := fmt.Errorf("get thing: %w", got)
	if !errors.Is(wrapped, errThing) || !errors.Is(wrapped, cause) {
		t.Error("errors.Is lost errThing or cause")
	}
	// biến gốc không bị đổi
	if errThing.Detail() != "" || errThing.Unwrap() != nil {
		t.Errorf("original mutated: %q %v", errThing.Detail(), errThing.Unwrap())
	}
}

func TestWith_FieldsNotShared(t *testing.T) {
	base := errThing.With(WithFields(FieldError{"a", "x"}))
	one := base.With(WithFields(FieldError{"b", "y"}))
	two := base.With(WithFields(FieldError{"c", "z"}))

	if n := len(base.Fields()); n != 1 {
		t.Errorf("base has %d fields, want 1", n)
	}
	if f := one.Fields(); len(f) != 2 || f[1].Field != "b" {
		t.Errorf("one = %+v", f)
	}
	if f := two.Fields(); len(f) != 2 || f[1].Field != "c" {
		t.Errorf("two = %+v", f)
	}
}

func TestWithDetail_NotFormatted(t *testing.T) {
	if got := errThing.With(WithDetail("100% sure")).Detail(); got != "100% sure" {
		t.Errorf("detail = %q", got)
	}
}

func TestIs_DifferentType(t *testing.T) {
	other := NotFound("/errors/other", "Other")
	if errors.Is(errThing, other) {
		t.Error("different types matched")
	}
	// type rỗng không khớp với gì, kể cả lỗi type rỗng khác
	if errors.Is(New(http.StatusTeapot, "", "a"), New(http.StatusTeapot, "", "b")) {
		t.Error("empty types matched")
	}
}

func TestNewFrom(t *testing.T) {
	if NewFrom(nil) != nil {
		t.Error("nil should stay nil")
	}
	if got := NewFrom(fmt.Errorf("wrap: %w", errThing)); got != errThing {
		t.Errorf("got %v, want errThing", got)
	}
	if got := NewFrom(errors.New("db down")); got != ErrInternal {
		t.Errorf("plain error = %v, want ErrInternal", got)
	}
}

func TestStatus_DefaultsTo500(t *testing.T) {
	if got := New(0, "/errors/x", "X").Status(); got != http.StatusInternalServerError {
		t.Errorf("status = %d", got)
	}
}
