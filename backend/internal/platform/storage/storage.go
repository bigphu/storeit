package storage

import (
	"context"
	"errors"
	"io"
)

var (
	ErrNotFound   = errors.New("storage: file not found")
	ErrInvalidKey = errors.New("storage: invalid key")
)

// Store lưu file theo key dạng đường dẫn có "/", vd "imports/<id>/source.xlsx".
// Key do code tạo (từ ID), không lấy từ tên file người dùng upload.
type Store interface {
	// Put ghi toàn bộ r dưới key, ghi đè nếu đã có. Lỗi giữa chừng thì key
	// giữ nguyên như trước, không có file dở dang.
	Put(ctx context.Context, key string, r io.Reader) error

	// Get mở file của key, người gọi phải Close. Không có thì ErrNotFound.
	Get(ctx context.Context, key string) (io.ReadCloser, error)

	// Delete xoá file của key. Không có cũng không lỗi, nên retry được.
	Delete(ctx context.Context, key string) error
}

// Config đọc bằng config.Load, vd với envPrefix:"STORAGE_" là STORAGE_DIR.
// Hiện chỉ có ổ đĩa; bản S3 thêm vào lúc deploy.
type Config struct {
	Dir string `env:"DIR" envDefault:"./tmp/storage"`
}
