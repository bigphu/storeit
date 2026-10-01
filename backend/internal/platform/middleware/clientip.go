package middleware

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

type clientIPKey struct{}

// ClientIP xác định IP của client rồi đặt vào ctx (đọc bằng ClientIPFrom).
//
// Mặc định là địa chỉ của kết nối (r.RemoteAddr). Chỉ khi kết nối đến từ một
// proxy trong trusted (vd reverse proxy trước app) mới đọc X-Forwarded-For:
// đi từ phải sang trái, bỏ qua các hop thuộc trusted, lấy IP đầu tiên không
// thuộc trusted. Phần bên trái do client tự viết được nên không bao giờ tin
// thẳng IP đầu danh sách. trusted rỗng thì không tin header nào.
func ClientIP(trusted []netip.Prefix) func(http.Handler) http.Handler {
	isTrusted := func(a netip.Addr) bool {
		for _, p := range trusted {
			if p.Contains(a) {
				return true
			}
		}
		return false
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientAddr(r, isTrusted)
			var s string
			if ip.IsValid() {
				s = ip.String()
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), clientIPKey{}, s)))
		})
	}
}

// ClientIPFrom trả IP client do ClientIP đặt, rỗng nếu không có
func ClientIPFrom(ctx context.Context) string {
	s, _ := ctx.Value(clientIPKey{}).(string)
	return s
}

func clientAddr(r *http.Request, isTrusted func(netip.Addr) bool) netip.Addr {
	ip := parseAddr(r.RemoteAddr)
	if !ip.IsValid() || !isTrusted(ip) {
		return ip
	}
	// Header có thể lặp nhiều dòng; gộp theo thứ tự rồi tách theo dấu phẩy
	hops := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		a := parseAddr(strings.TrimSpace(hops[i]))
		if !a.IsValid() {
			break // hop hỏng: không đi tiếp qua phần không đọc được
		}
		ip = a
		if !isTrusted(a) {
			break
		}
	}
	return ip
}

// parseAddr đọc "ip", "ip:port" hay "[ipv6]:port"; IPv4 bọc trong IPv6
// (::ffff:a.b.c.d) được bóc ra để so với prefix IPv4
func parseAddr(s string) netip.Addr {
	if a, err := netip.ParseAddr(s); err == nil {
		return a.Unmap()
	}
	if host, _, err := net.SplitHostPort(s); err == nil {
		if a, err := netip.ParseAddr(host); err == nil {
			return a.Unmap()
		}
	}
	return netip.Addr{}
}
