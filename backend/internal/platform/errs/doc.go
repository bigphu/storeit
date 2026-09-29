// Package errs là lỗi có kiểu: mỗi lỗi mang sẵn HTTP status và được trả về
// client dạng problem+json.
//
// Khai báo mỗi loại lỗi một lần trong domain, bằng hàm theo status:
//
//	var ErrAssetNotFound = errs.NotFound("/errors/asset-not-found", "Asset not found")
//
// Khi trả về, dùng nguyên biến đó hoặc gắn thêm thông tin của lần này bằng
// With. With trả bản sao nên biến gốc dùng chung được:
//
//	return ErrAssetNotFound.With(errs.WithDetailf("asset %s", id), errs.WithCause(err))
//
// Kiểm tra bằng errors.Is(err, ErrAssetNotFound): so theo type, nên bản sao từ
// With và lỗi đã bị fmt.Errorf("...: %w") bọc ngoài vẫn khớp. Lỗi thường,
// không phải *errs.Error, sẽ thành 500 và client chỉ thấy thông báo chung.
package errs
