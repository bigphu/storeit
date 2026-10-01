package handler

import (
	"net/http"
	"time"
)

// Cookie refresh. Path giới hạn ở /api/v1/auth là phần quan trọng nhất:
// cookie không đi kèm mọi lời gọi API thường, nên không lộ qua log hay proxy
// cấu hình sai. Chỉ login, refresh, logout và đổi mật khẩu thấy nó.
const (
	refreshCookieName = "storeit_refresh"
	refreshCookiePath = "/api/v1/auth"
)

// CookieSettings khác nhau giữa môi trường: Secure phải tắt được khi dev chạy
// http, vì trình duyệt bỏ cookie Secure trên http, kể cả localhost
type CookieSettings struct {
	Secure bool
	Domain string
}

// set trả giá trị header Set-Cookie cho refresh token mới
func (c CookieSettings) set(raw string, expires time.Time) *string {
	return c.header(&http.Cookie{
		Value:   raw,
		Expires: expires,
		MaxAge:  max(int(time.Until(expires).Seconds()), 1),
	})
}

// clear ghi đè cookie bằng bản đã hết hạn. Domain và Path phải khớp lúc đặt,
// không thì trình duyệt coi là cookie khác và cookie cũ vẫn nằm đó
func (c CookieSettings) clear() *string {
	return c.header(&http.Cookie{Expires: time.Unix(0, 0), MaxAge: -1})
}

func (c CookieSettings) header(ck *http.Cookie) *string {
	ck.Name = refreshCookieName
	ck.Path = refreshCookiePath
	ck.Domain = c.Domain
	ck.HttpOnly = true
	ck.Secure = c.Secure
	// Strict: không luồng nào cần cookie này sống sót qua điều hướng từ site
	// khác. Nếu frontend chuyển sang cross-site thật, thêm CSRF token chứ đừng hạ cờ.
	ck.SameSite = http.SameSiteStrictMode
	s := ck.String()
	return &s
}
