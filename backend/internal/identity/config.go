package identity

import (
	"errors"
	"time"
)

// Config của module identity, đọc bằng config.Load; binary nhúng nguyên khối,
// không thêm envPrefix. Field thời lượng bằng 0 (dựng tay trong test) thì lấy
// mặc định, âm là lỗi.
type Config struct {
	// Hạn trượt của mỗi refresh token: không dùng tới trong khoảng này thì phiên chết
	RefreshSlidingTTL time.Duration `env:"IDENTITY_REFRESH_SLIDING_TTL" envDefault:"336h"`
	// Hạn tuyệt đối của một lần đăng nhập, refresh bao nhiêu cũng không vượt qua
	RefreshAbsoluteTTL time.Duration `env:"IDENTITY_REFRESH_ABSOLUTE_TTL" envDefault:"720h"`
	// Cửa sổ coi token vừa dùng là client gửi lại (hai tab, mạng chập chờn)
	// thay vì bị đánh cắp
	RefreshGracePeriod time.Duration `env:"IDENTITY_REFRESH_GRACE_PERIOD" envDefault:"30s"`
	// Giữ hàng đã chết bao lâu trước khi job dọn rác xoá
	RefreshRetention time.Duration `env:"IDENTITY_REFRESH_RETENTION" envDefault:"720h"`

	// Cookie refresh: Secure phải tắt được khi dev chạy http (trình duyệt bỏ
	// cookie Secure trên http, kể cả localhost)
	CookieSecure bool   `env:"IDENTITY_COOKIE_SECURE" envDefault:"true"`
	CookieDomain string `env:"IDENTITY_COOKIE_DOMAIN"`

	// Tài khoản Administrator đầu tiên, tạo lúc khởi động nếu chưa có account
	// nào. Để trống cả hai thì bỏ qua.
	AdminEmail    string `env:"ADMIN_EMAIL"`
	AdminPassword string `env:"ADMIN_PASSWORD"`
}

func (c Config) Validate() error {
	if c.RefreshSlidingTTL < 0 || c.RefreshAbsoluteTTL < 0 || c.RefreshGracePeriod < 0 || c.RefreshRetention < 0 {
		return errors.New("identity: durations must not be negative")
	}
	d := c.withDefaults()
	if d.RefreshSlidingTTL > d.RefreshAbsoluteTTL {
		return errors.New("identity: IDENTITY_REFRESH_SLIDING_TTL must not exceed IDENTITY_REFRESH_ABSOLUTE_TTL")
	}
	if (c.AdminEmail == "") != (c.AdminPassword == "") {
		return errors.New("identity: set both ADMIN_EMAIL and ADMIN_PASSWORD, or neither")
	}
	return nil
}

func (c Config) withDefaults() Config {
	if c.RefreshSlidingTTL <= 0 {
		c.RefreshSlidingTTL = 336 * time.Hour
	}
	if c.RefreshAbsoluteTTL <= 0 {
		c.RefreshAbsoluteTTL = 720 * time.Hour
	}
	if c.RefreshGracePeriod <= 0 {
		c.RefreshGracePeriod = 30 * time.Second
	}
	if c.RefreshRetention <= 0 {
		c.RefreshRetention = 720 * time.Hour
	}
	return c
}
