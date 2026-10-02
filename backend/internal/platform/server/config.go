package server

import (
	"cmp"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"
)

// Config của HTTP server, đọc bằng config.Load từ HTTP_ADDR,
// HTTP_IDLE_TIMEOUT...; binary nhúng nguyên khối, không thêm envPrefix. Field
// nào bằng 0 (dựng tay trong test) thì lấy mặc định; giá trị âm là lỗi.
//
// Không có WriteTimeout: nó cắt cả response dài như tải file export. Route nào
// cần giới hạn thì tự gắn chimw.Timeout.
type Config struct {
	Addr string `env:"HTTP_ADDR" envDefault:":8080"`

	// Thời gian tối đa để client gửi xong header (chống slowloris)
	ReadHeaderTimeout time.Duration `env:"HTTP_READ_HEADER_TIMEOUT" envDefault:"10s"`
	// Thời gian tối đa để đọc cả request, gồm body. Giữ ngắn cho API JSON;
	// route upload file lớn tự nới bằng middleware.ReadTimeout
	ReadTimeout time.Duration `env:"HTTP_READ_TIMEOUT" envDefault:"30s"`
	// Kết nối keep-alive rảnh quá lâu thì đóng. Để 0 thì net/http lấy
	// ReadTimeout, nên luôn đặt riêng
	IdleTimeout time.Duration `env:"HTTP_IDLE_TIMEOUT" envDefault:"60s"`
	// Thời gian chờ request đang chạy xong khi tắt, quá thì cắt ngang
	ShutdownTimeout time.Duration `env:"HTTP_SHUTDOWN_TIMEOUT" envDefault:"15s"`

	// Body tối đa của mọi request (byte). Route upload tự nới bằng
	// middleware.BodyLimit
	MaxBodyBytes int64 `env:"HTTP_MAX_BODY_BYTES" envDefault:"1048576"`

	// Proxy được tin để đọc IP client từ X-Forwarded-For, dạng CIDR cách nhau
	// bằng dấu phẩy, vd "172.18.0.0/16" (mạng Docker mà reverse proxy đi vào).
	// Rỗng thì luôn lấy địa chỉ kết nối: header do client tự viết được.
	TrustedProxies Prefixes `env:"HTTP_TRUSTED_PROXIES"`

	// Trang tài liệu API (Swagger UI) ở /api/docs. Trang công khai, không cần
	// đăng nhập: chỉ bật khi dev
	APIDocs bool `env:"HTTP_API_DOCS"`
}

// Prefixes là danh sách CIDR cách nhau bằng dấu phẩy; khoảng trắng quanh mỗi
// phần tử được bỏ qua (env không tự trim khi tách slice)
type Prefixes []netip.Prefix

func (p *Prefixes) UnmarshalText(text []byte) error {
	var out Prefixes
	for _, s := range strings.Split(string(text), ",") {
		if s = strings.TrimSpace(s); s == "" {
			continue
		}
		pfx, err := netip.ParsePrefix(s)
		if err != nil {
			return fmt.Errorf("trusted proxy %q: want CIDR like 10.0.0.0/8 or 10.0.0.5/32: %w", s, err)
		}
		out = append(out, pfx.Masked())
	}
	*p = out
	return nil
}

// Validate chỉ từ chối giá trị âm; 0 là lấy mặc định (xem withDefaults)
func (c Config) Validate() error {
	switch {
	case c.ReadHeaderTimeout < 0, c.ReadTimeout < 0, c.IdleTimeout < 0, c.ShutdownTimeout < 0:
		return errors.New("http: timeouts must not be negative")
	case c.MaxBodyBytes < 0:
		return errors.New("http: HTTP_MAX_BODY_BYTES must not be negative")
	}
	return nil
}

func (c Config) withDefaults() Config {
	c.Addr = cmpOr(c.Addr, ":8080")
	c.ReadHeaderTimeout = cmpOr(c.ReadHeaderTimeout, 10*time.Second)
	c.ReadTimeout = cmpOr(c.ReadTimeout, 30*time.Second)
	c.IdleTimeout = cmpOr(c.IdleTimeout, 60*time.Second)
	c.ShutdownTimeout = cmpOr(c.ShutdownTimeout, 15*time.Second)
	c.MaxBodyBytes = cmpOr(c.MaxBodyBytes, 1<<20)
	return c
}

// cmpOr như cmp.Or nhưng coi giá trị âm cũng là chưa đặt
func cmpOr[T cmp.Ordered](v, def T) T {
	var zero T
	if v <= zero {
		return def
	}
	return v
}
