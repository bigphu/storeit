package server

import (
	"time"
)

// Config của HTTP server, đọc bằng config.Load. Vd với envPrefix:"HTTP_" thì
// là HTTP_ADDR, HTTP_IDLE_TIMEOUT... Field nào bằng 0 (dựng tay trong test) thì
// lấy mặc định.
//
// Không có WriteTimeout: nó cắt cả response dài như tải file export. Route nào
// cần giới hạn thì tự gắn chimw.Timeout.
type Config struct {
	Addr string `env:"ADDR" envDefault:":8080"`

	// Thời gian tối đa để client gửi xong header (chống slowloris)
	ReadHeaderTimeout time.Duration `env:"READ_HEADER_TIMEOUT" envDefault:"10s"`
	// Thời gian tối đa để đọc cả request, gồm body
	ReadTimeout time.Duration `env:"READ_TIMEOUT" envDefault:"30s"`
	// Kết nối keep-alive rảnh quá lâu thì đóng. Để 0 thì net/http lấy
	// ReadTimeout, nên luôn đặt riêng
	IdleTimeout time.Duration `env:"IDLE_TIMEOUT" envDefault:"60s"`
	// Thời gian chờ request đang chạy xong khi tắt, quá thì cắt ngang
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"15s"`
}

func (c Config) withDefaults() Config {
	c.Addr = cmpOr(c.Addr, ":8080")
	c.ReadHeaderTimeout = cmpOr(c.ReadHeaderTimeout, 10*time.Second)
	c.ReadTimeout = cmpOr(c.ReadTimeout, 30*time.Second)
	c.IdleTimeout = cmpOr(c.IdleTimeout, 60*time.Second)
	c.ShutdownTimeout = cmpOr(c.ShutdownTimeout, 15*time.Second)
	return c
}

// cmpOr như cmp.Or nhưng coi giá trị âm cũng là chưa đặt
func cmpOr[T string | time.Duration](v, def T) T {
	var zero T
	if v <= zero {
		return def
	}
	return v
}
