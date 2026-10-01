package logger

import (
	"fmt"
	"log/slog"
	"strings"
)

// Config đọc từ LOG_LEVEL, LOG_FORMAT; binary nhúng nguyên khối, không thêm
// envPrefix. Mặc định là info + JSON, hợp cho production
type Config struct {
	Level  slog.Level `env:"LOG_LEVEL" envDefault:"info"` // debug|info|warn|error
	Format Format     `env:"LOG_FORMAT" envDefault:"json"`
}

type Format string

const (
	FormatJSON   Format = "json"   // production: một object JSON mỗi dòng
	FormatPretty Format = "pretty" // dev: một dòng có màu cho người đọc
)

// UnmarshalText để LOG_FORMAT sai là báo lỗi ngay lúc khởi động
func (f *Format) UnmarshalText(text []byte) error {
	switch v := Format(strings.ToLower(string(text))); v {
	case FormatJSON, FormatPretty:
		*f = v
		return nil
	}
	return fmt.Errorf("unknown log format %q (want json|pretty)", text)
}
