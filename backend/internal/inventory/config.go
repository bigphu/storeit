package inventory

import (
	"fmt"
	"net/url"
	"time"
)

// Config của inventory; binary liệt kê khối này trong cmd/<bin>/config.go
type Config struct {
	// Số dòng tối đa một lần export Excel; 0 là mặc định của service (50.000)
	ExportMaxRows int `env:"INVENTORY_EXPORT_MAX_ROWS" envDefault:"50000"`
	// Địa chỉ frontend cho link trong thư mượn tài sản (US-10); rỗng là http://localhost:3000
	AppURL string `env:"INVENTORY_APP_URL" envDefault:"http://localhost:3000"`
	// Múi giờ IANA của ngày nghiệp vụ (ngày mượn, quá hạn); rỗng là Asia/Ho_Chi_Minh
	TimeZone string `env:"INVENTORY_TIME_ZONE" envDefault:"Asia/Ho_Chi_Minh"`
}

const (
	defaultAppURL   = "http://localhost:3000"
	defaultTimeZone = "Asia/Ho_Chi_Minh"
)

func (c Config) Validate() error {
	if c.ExportMaxRows < 0 || c.ExportMaxRows > 1_000_000 {
		return fmt.Errorf("inventory: INVENTORY_EXPORT_MAX_ROWS must be between 1 and 1000000")
	}
	// Link trong thư nối thêm "/<trang>": query hay fragment sẵn có sẽ làm hỏng nó
	u, err := url.Parse(c.appURL())
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("inventory: INVENTORY_APP_URL %q must be an http(s) URL without query or fragment", c.AppURL)
	}
	if _, err := time.LoadLocation(c.timeZone()); err != nil {
		return fmt.Errorf("inventory: INVENTORY_TIME_ZONE %q is not an IANA time zone such as Asia/Ho_Chi_Minh", c.TimeZone)
	}
	return nil
}

// Location: múi giờ của ngày nghiệp vụ; Validate đã kiểm, lỗi thì UTC
func (c Config) Location() *time.Location {
	loc, err := time.LoadLocation(c.timeZone())
	if err != nil {
		return time.UTC
	}
	return loc
}

func (c Config) appURL() string {
	if c.AppURL == "" {
		return defaultAppURL
	}
	return c.AppURL
}

func (c Config) timeZone() string {
	if c.TimeZone == "" {
		return defaultTimeZone
	}
	return c.TimeZone
}
