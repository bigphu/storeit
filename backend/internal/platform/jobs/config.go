package jobs

import "errors"

// Config của worker, đọc bằng config.Load từ JOBS_*; binary nhúng nguyên khối,
// không thêm envPrefix. Chỉ cmd/worker cần: client chỉ insert (cmd/api) không
// chạy job. Field nào bằng 0 (dựng tay trong test) thì lấy mặc định, âm là lỗi.
//
// Tổng số worker cộng thêm vài kết nối riêng của River phải nằm trong
// DB_MAX_CONNS của worker, nếu không job sẽ phải chờ kết nối.
type Config struct {
	// Số job chạy song song của queue default (import, job nền) trên một process
	DefaultMaxWorkers int `env:"JOBS_DEFAULT_MAX_WORKERS" envDefault:"10"`
	// Số job giao event chạy song song (queue events) trên một process
	EventsMaxWorkers int `env:"JOBS_EVENTS_MAX_WORKERS" envDefault:"10"`
}

const defaultMaxWorkers = 10

// Validate từ chối số worker âm; 0 là lấy mặc định
func (c Config) Validate() error {
	if c.DefaultMaxWorkers < 0 || c.EventsMaxWorkers < 0 {
		return errors.New("jobs: JOBS_*_MAX_WORKERS must not be negative")
	}
	return nil
}

func (c Config) withDefaults() Config {
	if c.DefaultMaxWorkers <= 0 {
		c.DefaultMaxWorkers = defaultMaxWorkers
	}
	if c.EventsMaxWorkers <= 0 {
		c.EventsMaxWorkers = defaultMaxWorkers
	}
	return c
}
