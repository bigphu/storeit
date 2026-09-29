package storage

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func newLocal(t *testing.T) *Local {
	t.Helper()
	s, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func read(t *testing.T, s Store, key string) string {
	t.Helper()
	rc, err := s.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("Get(%q): %v", key, err)
	}
	defer func() { _ = rc.Close() }()
	b, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestLocal_PutGetNestedKey(t *testing.T) {
	s := newLocal(t)
	ctx := context.Background()

	if err := s.Put(ctx, "imports/2026/assets.xlsx", strings.NewReader("rows")); err != nil {
		t.Fatal(err)
	}

	if got := read(t, s, "imports/2026/assets.xlsx"); got != "rows" {
		t.Errorf("got %q, want %q", got, "rows")
	}
}

func TestLocal_PutOverwrites(t *testing.T) {
	s := newLocal(t)
	ctx := context.Background()

	for _, body := range []string{"v1", "v2"} {
		if err := s.Put(ctx, "exports/a.csv", strings.NewReader(body)); err != nil {
			t.Fatal(err)
		}
	}

	if got := read(t, s, "exports/a.csv"); got != "v2" {
		t.Errorf("got %q, want v2", got)
	}
}

func TestLocal_GetMissing(t *testing.T) {
	_, err := newLocal(t).Get(context.Background(), "nope.xlsx")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestLocal_Delete(t *testing.T) {
	s := newLocal(t)
	ctx := context.Background()
	if err := s.Put(ctx, "a/b.txt", strings.NewReader("x")); err != nil {
		t.Fatal(err)
	}

	if err := s.Delete(ctx, "a/b.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, "a/b.txt"); !errors.Is(err, ErrNotFound) {
		t.Errorf("after delete: err = %v, want ErrNotFound", err)
	}
	// Xoá lần hai (retry) không lỗi
	if err := s.Delete(ctx, "a/b.txt"); err != nil {
		t.Errorf("delete missing: %v", err)
	}
}

// Reader lỗi giữa chừng thì không được để lại file dở dang dưới key đó
func TestLocal_FailedPutLeavesNothing(t *testing.T) {
	s := newLocal(t)
	ctx := context.Background()
	broken := io.MultiReader(strings.NewReader("half"), iotestErrReader{})

	if err := s.Put(ctx, "imports/x.xlsx", broken); err == nil {
		t.Fatal("want error from broken reader")
	}
	if _, err := s.Get(ctx, "imports/x.xlsx"); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestLocal_RejectsKeysOutsideRoot(t *testing.T) {
	s := newLocal(t)
	ctx := context.Background()

	for _, key := range []string{"", ".", "../x", "a/../../x", "/etc/passwd", `a\..\..\x`, "a//b"} {
		if err := s.Put(ctx, key, strings.NewReader("x")); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("Put(%q) err = %v, want ErrInvalidKey", key, err)
		}
		if _, err := s.Get(ctx, key); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("Get(%q) err = %v, want ErrInvalidKey", key, err)
		}
		if err := s.Delete(ctx, key); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("Delete(%q) err = %v, want ErrInvalidKey", key, err)
		}
	}
}

type iotestErrReader struct{}

func (iotestErrReader) Read([]byte) (int, error) { return 0, errors.New("disk on fire") }
