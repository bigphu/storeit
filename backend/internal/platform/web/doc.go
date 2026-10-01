// Package web gom phần HTTP dùng chung: đọc JSON, trả JSON, trả lỗi problem+json,
// validate request theo spec OpenAPI (ValidateRequests) và kiểm danh sách
// operation với spec (MustOperations).
//
// Route sinh từ OpenAPI (strict server) được ValidateRequests kiểm tra theo spec
// trước khi vào handler, handler không tự validate. Handler viết tay thì trả
// về error, web.Handle đổi error đó thành response, web.Decode đọc và validate
// body theo tag validate:
//
//	r.Post("/things", web.Handle(func(w http.ResponseWriter, r *http.Request) error {
//		var in CreateThing
//		if err := web.Decode(r, &in); err != nil { // JSON sai, thiếu field → 4xx
//			return err
//		}
//		return web.JSON(w, http.StatusCreated, out)
//	}))
package web
