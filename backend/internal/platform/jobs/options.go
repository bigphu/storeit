package jobs

import (
	"time"
)

// Options là cấu hình cho một lần insert job, gom từ các Option. Field bằng 0
// thì River lấy từ InsertOpts() của job, rồi tới default của client.
type Options struct {
	Queue       string
	Priority    int
	ScheduledAt time.Time
	MaxAttempts int

	// UniqueByArgs gộp một insert vào job CHƯA XONG (đang chờ, đang chạy, chờ
	// retry, hẹn giờ) có cùng kind và cùng args, thay vì tạo thêm một job nữa
	//
	// Mặc định TẮT: hai job cùng kind nhưng khác args là hai việc khác nhau
	UniqueByArgs bool
}

type Option func(*Options)

// WithQueue chọn queue. Queue phải được khai báo trong Config.Queues của River
// client chạy worker, nếu không job nằm chờ mãi mà không ai lấy.
func WithQueue(queue string) Option {
	return func(o *Options) {
		o.Queue = queue
	}
}

// WithPriority nhận 1 (cao nhất) đến 4, ngoài khoảng này thì insert lỗi
func WithPriority(priority int) Option {
	return func(o *Options) {
		o.Priority = priority
	}
}

func WithSchedule(t time.Time) Option {
	return func(o *Options) {
		o.ScheduledAt = t
	}
}

func WithMaxAttempts(n int) Option {
	return func(o *Options) {
		o.MaxAttempts = n
	}
}

func WithUniqueArgs() Option {
	return func(o *Options) {
		o.UniqueByArgs = true
	}
}
