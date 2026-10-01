// Package middleware chứa middleware HTTP riêng của app.
//
// server.New gắn sẵn chuỗi chung ở router gốc, theo thứ tự:
//
//	BodyLimit(HTTP_MAX_BODY_BYTES)  // giới hạn body mọi route (strict server đọc thẳng r.Body)
//	NoSniff                         // X-Content-Type-Options: nosniff
//	ClientIP(HTTP_TRUSTED_PROXIES)  // IP client, chỉ tin X-Forwarded-For từ proxy đã khai báo
//	RequestID                       // X-Request-Id hợp lệ thì giữ, không thì sinh UUID
//	RequestLogger(log)              // request_id + logger vào ctx, một dòng access log
//	Recoverer()                     // panic => 500 problem+json (chưa gửi gì) hoặc chỉ log
//
// RequestLogger đứng sau RequestID (có ID để gắn) và bọc ngoài Recoverer: panic
// vẫn thành một dòng 500 trong access log, log panic cũng có request_id.
//
// Thông số riêng một operation sinh từ OpenAPI (body lớn hơn, đọc lâu hơn) gắn
// qua ForOperations trong Middlewares của module, không dùng r.With:
//
//	upload := middleware.ForOperations(web.MustOperations(spec, "/api/v1", "POST /api/v1/imports"),
//		middleware.BodyLimit(50<<20), middleware.ReadTimeout(10*time.Minute))
package middleware
