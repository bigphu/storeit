package database

import (
	"cmp"
	"errors"
	"time"
)

// Config của pool kết nối, đọc bằng config.Load từ DB_URL, DB_MAX_CONNS...;
// binary nhúng nguyên khối, không thêm envPrefix. Field nào bằng 0 (dựng tay
// trong test) thì lấy mặc định, giá trị âm là lỗi; mặc định ở tag envDefault
// và defaultConfig phải giống nhau.
//
// Thông số pool lấy từ đây, đè lên pool_max_conns, connect_timeout... nếu có
// trong URL, để mọi thông số pool nằm một chỗ.
type Config struct {
	// URL rỗng thì pgx đọc biến PG* (PGHOST, PGUSER...), cách compose đang dùng.
	// Mật khẩu đọc thêm từ PGPASSWORD_FILE, xem parseConfig.
	URL string `env:"DB_URL"`

	// Số kết nối tối đa của pool. Worker cần nhiều hơn tổng số job chạy song
	// song (JOBS_*_MAX_WORKERS, mặc định 2 queue x 10) vì River giữ thêm kết
	// nối riêng. Tổng MaxConns của mọi process (api, worker) phải dưới
	// max_connections của Postgres (100).
	MaxConns int32 `env:"DB_MAX_CONNS" envDefault:"25"`
	// Số kết nối luôn mở sẵn, để request đầu tiên sau lúc rảnh không phải chờ
	// mở kết nối
	MinConns int32 `env:"DB_MIN_CONNS" envDefault:"0"`

	// Kết nối sống quá lâu thì đóng để mở mới, vd sau khi Postgres failover
	// hay đổi cấu hình. Jitter cộng thêm ngẫu nhiên để các kết nối không cùng
	// hết hạn một lúc.
	MaxConnLifetime       time.Duration `env:"DB_MAX_CONN_LIFETIME" envDefault:"1h"`
	MaxConnLifetimeJitter time.Duration `env:"DB_MAX_CONN_LIFETIME_JITTER" envDefault:"5m"`
	// Kết nối rảnh quá lâu thì đóng (nhưng vẫn giữ đủ MinConns)
	MaxConnIdleTime time.Duration `env:"DB_MAX_CONN_IDLE_TIME" envDefault:"30m"`
	// Chu kỳ pool kiểm tra và dọn kết nối hỏng, hết hạn
	HealthCheckPeriod time.Duration `env:"DB_HEALTH_CHECK_PERIOD" envDefault:"1m"`
	// Thời gian tối đa để mở một kết nối. Không đặt thì DB treo làm request
	// treo theo tới khi ctx của nó bị huỷ
	ConnectTimeout time.Duration `env:"DB_CONNECT_TIMEOUT" envDefault:"5s"`
}

func defaultConfig() Config {
	return Config{
		MaxConns:              25,
		MinConns:              0,
		MaxConnLifetime:       time.Hour,
		MaxConnLifetimeJitter: 5 * time.Minute,
		MaxConnIdleTime:       30 * time.Minute,
		HealthCheckPeriod:     time.Minute,
		ConnectTimeout:        5 * time.Second,
	}
}

// Validate từ chối giá trị âm (0 là lấy mặc định) và MinConns > MaxConns
func (c Config) Validate() error {
	switch {
	case c.MaxConns < 0, c.MinConns < 0:
		return errors.New("db: DB_MAX_CONNS, DB_MIN_CONNS must not be negative")
	case c.MaxConnLifetime < 0, c.MaxConnLifetimeJitter < 0, c.MaxConnIdleTime < 0,
		c.HealthCheckPeriod < 0, c.ConnectTimeout < 0:
		return errors.New("db: pool durations must not be negative")
	}
	c = c.withDefaults()
	if c.MinConns > c.MaxConns {
		return errors.New("db: MIN_CONNS must not exceed MAX_CONNS")
	}
	return nil
}

func (c Config) withDefaults() Config {
	def := defaultConfig()
	c.MaxConns = cmpOr(c.MaxConns, def.MaxConns)
	c.MinConns = cmpOr(c.MinConns, def.MinConns)
	c.MaxConnLifetime = cmpOr(c.MaxConnLifetime, def.MaxConnLifetime)
	c.MaxConnLifetimeJitter = cmpOr(c.MaxConnLifetimeJitter, def.MaxConnLifetimeJitter)
	c.MaxConnIdleTime = cmpOr(c.MaxConnIdleTime, def.MaxConnIdleTime)
	c.HealthCheckPeriod = cmpOr(c.HealthCheckPeriod, def.HealthCheckPeriod)
	c.ConnectTimeout = cmpOr(c.ConnectTimeout, def.ConnectTimeout)
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
