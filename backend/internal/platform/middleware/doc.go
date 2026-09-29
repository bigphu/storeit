// Package middleware chứa middleware HTTP riêng của app. Middleware chung
// (RequestID, RealIP, Timeout...) thì lấy thẳng từ chi.
//
// Thứ tự quan trọng, gắn một lần ở router gốc:
//
//	r.Use(
//		chimw.RequestID,                // tạo request ID
//		middleware.RequestLogger(log),  // gắn request_id vào ctx, log mỗi request
//		middleware.Recoverer(log),      // panic => 500 problem+json
//	)
//
// RequestLogger phải đứng sau RequestID (để có ID mà gắn) và bọc ngoài
// Recoverer: panic khi đó vẫn hiện thành một dòng 500 trong access log, và log
// panic của Recoverer cũng có request_id.
package middleware
