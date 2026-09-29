package storage

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Local lưu file trên ổ đĩa, dùng khi dev. Mọi thao tác đi qua os.Root nên
// không key nào ra được ngoài thư mục gốc, kể cả qua symlink.
type Local struct {
	root *os.Root
}

var _ Store = (*Local)(nil)

func NewLocal(dir string) (*Local, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("storage: %w", err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("storage: %w", err)
	}
	return &Local{root: root}, nil
}

func (l *Local) Close() error {
	return l.root.Close()
}

func (l *Local) Put(ctx context.Context, key string, r io.Reader) error {
	name, err := localName(key)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := l.root.MkdirAll(filepath.Dir(name), 0o750); err != nil {
		return fmt.Errorf("storage: put %q: %w", key, err)
	}

	// Ghi vào file tạm rồi rename, để người đọc không bao giờ thấy file dở
	tmp := name + ".tmp-" + rand.Text()
	if err := l.write(tmp, r); err != nil {
		_ = l.root.Remove(tmp)
		return fmt.Errorf("storage: put %q: %w", key, err)
	}
	if err := l.root.Rename(tmp, name); err != nil {
		_ = l.root.Remove(tmp)
		return fmt.Errorf("storage: put %q: %w", key, err)
	}
	return nil
}

func (l *Local) write(name string, r io.Reader) error {
	f, err := l.root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func (l *Local) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	name, err := localName(key)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f, err := l.root.Open(name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: %q", ErrNotFound, key)
	}
	if err != nil {
		return nil, fmt.Errorf("storage: get %q: %w", key, err)
	}
	return f, nil
}

func (l *Local) Delete(ctx context.Context, key string) error {
	name, err := localName(key)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := l.root.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("storage: delete %q: %w", key, err)
	}
	return nil
}

// localName kiểm tra key rồi đổi sang đường dẫn của OS. Key hợp lệ là đường
// dẫn tương đối đã sạch, chỉ dùng "/": không rỗng, không "..", không "\".
func localName(key string) (string, error) {
	if !fs.ValidPath(key) || key == "." || strings.Contains(key, `\`) || path.Clean(key) != key {
		return "", fmt.Errorf("%w: %q", ErrInvalidKey, key)
	}
	return filepath.FromSlash(key), nil
}
