package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

// fakeClock cho test điều khiển thời gian
type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

func limited(every time.Duration, burst int, clock *fakeClock) (*rateLimiter, http.Handler) {
	rl := newRateLimiter(every, burst, clock.now)
	return rl, rl.middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
}

// from gửi request như thể ClientIP đã chạy và xác định IP client là ip
func from(h http.Handler, ip string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req = req.WithContext(context.WithValue(req.Context(), clientIPKey{}, ip))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRateLimit_BurstThen429(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)}
	_, h := limited(6*time.Second, 3, clock)

	for i := range 3 {
		if rec := from(h, "203.0.113.5"); rec.Code != http.StatusNoContent {
			t.Fatalf("request %d: %d, want 204", i+1, rec.Code)
		}
	}
	rec := from(h, "203.0.113.5")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("over the burst: %d, want 429", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type = %q", ct)
	}
	var p struct {
		Type   string
		Status int
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil || p.Type != "/errors/rate-limited" || p.Status != 429 {
		t.Errorf("problem = %+v, %v", p, err)
	}
	// Một token mới sau 6 giây: báo client chờ 6 giây, làm tròn lên
	if ra := rec.Header().Get("Retry-After"); ra != "6" {
		t.Errorf("Retry-After = %q, want 6", ra)
	}

	// IP khác có bucket riêng
	if rec := from(h, "198.51.100.7"); rec.Code != http.StatusNoContent {
		t.Errorf("other client: %d, want 204", rec.Code)
	}

	// Chờ đủ một nhịp thì được thêm đúng một request
	clock.advance(6 * time.Second)
	if rec := from(h, "203.0.113.5"); rec.Code != http.StatusNoContent {
		t.Errorf("after refill: %d, want 204", rec.Code)
	}
	if rec := from(h, "203.0.113.5"); rec.Code != http.StatusTooManyRequests {
		t.Errorf("second after refill: %d, want 429", rec.Code)
	}
}

// Request bị từ chối không được tiêu token: client chờ đúng Retry-After là qua
func TestRateLimit_RejectedRequestsDoNotDrain(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)}
	_, h := limited(10*time.Second, 1, clock)
	from(h, "203.0.113.5")
	for range 5 {
		from(h, "203.0.113.5")
	}
	rec := from(h, "203.0.113.5")
	retry, _ := strconv.Atoi(rec.Header().Get("Retry-After"))
	clock.advance(time.Duration(retry) * time.Second)
	if rec := from(h, "203.0.113.5"); rec.Code != http.StatusNoContent {
		t.Errorf("after waiting Retry-After: %d, want 204", rec.Code)
	}
}

// Không có IP từ ClientIP (middleware chưa chạy): dùng RemoteAddr
func TestRateLimit_FallsBackToRemoteAddr(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)}
	_, h := limited(time.Minute, 1, clock)
	send := func(remote string) int {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.RemoteAddr = remote
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	if send("192.0.2.1:5000") != http.StatusNoContent || send("192.0.2.1:6000") != http.StatusTooManyRequests {
		t.Error("same host on another port should share the bucket")
	}
	if send("192.0.2.2:5000") != http.StatusNoContent {
		t.Error("another host should have its own bucket")
	}
}

// Bucket đã đầy lại (client im lặng đủ lâu) bị xoá: map không phình mãi
func TestRateLimit_EvictsIdleClients(t *testing.T) {
	clock := &fakeClock{t: time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)}
	rl, h := limited(time.Second, 5, clock)
	for i := range 100 {
		from(h, "10.0.0."+strconv.Itoa(i))
	}
	if n := rl.size(); n != 100 {
		t.Fatalf("tracked clients = %d, want 100", n)
	}
	clock.advance(time.Hour)
	from(h, "10.0.1.1")
	if n := rl.size(); n != 1 {
		t.Errorf("tracked clients after an idle hour = %d, want 1", n)
	}
}

// Mỗi lần gọi RateLimit là một bộ đếm riêng: giới hạn login không ăn vào giới hạn forgot
func TestRateLimit_InstancesAreIndependent(t *testing.T) {
	a, b := RateLimit(time.Minute, 1), RateLimit(time.Minute, 1)
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	ha, hb := a(ok), b(ok)
	if from(ha, "203.0.113.5").Code != http.StatusOK || from(hb, "203.0.113.5").Code != http.StatusOK {
		t.Error("first request through each limiter should pass")
	}
	if from(ha, "203.0.113.5").Code != http.StatusTooManyRequests {
		t.Error("second request through the same limiter should be limited")
	}
}
