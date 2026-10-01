package domain

// Giới hạn mật khẩu tính theo byte: bcrypt chỉ dùng 72 byte đầu, mật khẩu dài
// hơn sẽ bị cắt lặng lẽ, nên từ chối luôn thay vì cắt
const (
	MinPasswordBytes = 12
	MaxPasswordBytes = 72
)

func ValidatePassword(p string) error {
	if len(p) < MinPasswordBytes || len(p) > MaxPasswordBytes {
		return ErrWeakPassword
	}
	return nil
}
