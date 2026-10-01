// Package storage lưu file upload và file export: ổ đĩa khi dev, S3 khi production.
//
// Service nhận Store (interface), main chọn bản cài đặt:
//
//	store, err := storage.NewLocal(cfg.Storage) // STORAGE_DIR
//
// Key do code tạo từ ID, không lấy tên file người dùng gửi lên:
//
//	key := "imports/" + im.ID.String() + "/source.xlsx"
//	err := store.Put(ctx, key, file)
package storage
