package middleware

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"storeit/internal/platform/errs"
	"storeit/internal/platform/web"
)

// ErrRateLimited: client gửi quá nhanh; header Retry-After nói phải chờ bao lâu
var ErrRateLimited = errs.New(http.StatusTooManyRequests, "/errors/rate-limited", "Too many requests",
	errs.WithDetail("Too many attempts. Wait a moment and try again."))

// RateLimit giới hạn số request theo IP client (ClientIPFrom, rơi về
// RemoteAddr): mỗi IP có một bucket chứa tối đa burst lượt, hồi một lượt sau
// mỗi every. Hết lượt thì 429 problem+json kèm Retry-After; request bị từ chối
// không tiêu lượt.
//
// Mỗi lần gọi là một bộ đếm riêng. Gắn cho từng operation bằng ForOperations:
//
//	login := middleware.ForOperations(web.MustOperations(spec, base, "POST /api/v1/auth/login"),
//		middleware.RateLimit(6*time.Second, 10))
//
// Bộ đếm nằm trong bộ nhớ của tiến trình: chạy nhiều replica API thì mỗi
// replica đếm riêng (giới hạn thật nhân lên theo số replica).
func RateLimit(every time.Duration, burst int) func(http.Handler) http.Handler {
	return newRateLimiter(every, burst, time.Now).middleware
}

type rateLimiter struct {
	limit rate.Limit
	burst int
	// Bucket im lặng lâu hơn refill đã đầy lại, xoá cũng như giữ
	refill time.Duration
	now    func() time.Time

	mu        sync.Mutex
	clients   map[string]*client
	lastSweep time.Time
}

type client struct {
	lim      *rate.Limiter
	lastSeen time.Time
}

func newRateLimiter(every time.Duration, burst int, now func() time.Time) *rateLimiter {
	if burst < 1 {
		burst = 1
	}
	return &rateLimiter{
		limit:     rate.Every(every),
		burst:     burst,
		refill:    every * time.Duration(burst),
		now:       now,
		clients:   map[string]*client{},
		lastSweep: now(),
	}
}

func (rl *rateLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if wait := rl.take(clientKey(r)); wait > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(wait.Seconds()))))
			web.RenderProblem(w, r, ErrRateLimited)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// take lấy một lượt cho key; hết lượt thì trả thời gian phải chờ (> 0)
func (rl *rateLimiter) take(key string) time.Duration {
	now := rl.now()
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.sweep(now)
	c, ok := rl.clients[key]
	if !ok {
		c = &client{lim: rate.NewLimiter(rl.limit, rl.burst)}
		rl.clients[key] = c
	}
	c.lastSeen = now
	res := c.lim.ReserveN(now, 1)
	if wait := res.DelayFrom(now); wait > 0 {
		// Trả lượt lại: request bị từ chối không được làm client chờ lâu hơn
		res.CancelAt(now)
		return wait
	}
	return 0
}

// sweep xoá client im lặng đủ lâu để bucket đầy lại; chạy tối đa mỗi
// max(refill, 1 phút) một lần, không cần goroutine riêng
func (rl *rateLimiter) sweep(now time.Time) {
	if now.Sub(rl.lastSweep) < max(rl.refill, time.Minute) {
		return
	}
	rl.lastSweep = now
	for k, c := range rl.clients {
		if now.Sub(c.lastSeen) >= rl.refill {
			delete(rl.clients, k)
		}
	}
}

func (rl *rateLimiter) size() int {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	return len(rl.clients)
}

// clientKey: IP do ClientIP xác định; middleware đó chưa chạy thì host của RemoteAddr
func clientKey(r *http.Request) string {
	if ip := ClientIPFrom(r.Context()); ip != "" {
		return ip
	}
	if ip := parseAddr(r.RemoteAddr); ip.IsValid() {
		return ip.String()
	}
	return r.RemoteAddr
}
