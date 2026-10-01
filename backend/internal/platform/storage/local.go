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

// NewLocal mở (tạo nếu chưa có) thư mục cfg.Dir làm gốc
func NewLocal(cfg Config) (*Local, error) {
	dir := cfg.Dir
	if dir == "" {
		dir = defaultDir
	}
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
	if err := l.write(tmp, ctxReader{ctx, r}); err != nil {
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
	// Sync trước khi rename: máy sập ngay sau rename cũng không để lại file rỗng
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// ctxReader dừng đọc khi ctx bị huỷ (vd client ngắt giữa lúc upload), để Put
// không chép nốt cả file rồi mới biết
type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (c ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
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
