// Package web gom phần HTTP dùng chung: đọc JSON, trả JSON, trả lỗi problem+json.
//
// Handler trả về error, web.Handle đổi error đó thành response:
//
//	r.Post("/things", web.Handle(func(w http.ResponseWriter, r *http.Request) error {
//		var in CreateThing
//		if err := web.Decode(r, &in); err != nil { // JSON sai, thiếu field → 4xx
//			return err
//		}
//		return web.JSON(w, http.StatusCreated, out)
//	}))
package web
