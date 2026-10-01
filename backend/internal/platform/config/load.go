package config

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/caarlos0/env/v11"
)

// Validator là khối cấu hình có ràng buộc riêng (vd database.Config:
// MIN_CONNS <= MAX_CONNS). Validate chỉ kiểm tra field của chính nó, không gọi
// Validate của khối con: Load lo việc đó.
type Validator interface {
	Validate() error
}

// Load đọc biến môi trường vào cfg (con trỏ tới struct cấu hình của binary)
// rồi gọi Validate của mọi khối bên trong có method này, kể cả cfg. Binary chỉ
// liệt kê khối nó dùng, không phải nhớ gọi Validate của từng khối.
//
// Lỗi của mọi khối được gom lại, mỗi lỗi kèm đường dẫn field (vd "DB: ..."),
// để thấy hết chỗ sai trong một lần khởi động.
func Load(cfg any) error {
	if err := env.Parse(cfg); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	if err := validate(cfg); err != nil {
		return fmt.Errorf("config: %w", err)
	}
	return nil
}

func validate(cfg any) error {
	return validateValue(reflect.ValueOf(cfg), "", true)
}

// validateValue duyệt struct: khối con trước, rồi tới chính nó (self), để
// Validate của struct ngoài được biết các khối con đã hợp lệ từng cái.
func validateValue(v reflect.Value, path string, self bool) error {
	for v.Kind() == reflect.Pointer {
		if v.IsNil() { // khối tuỳ chọn không được đặt
			return nil
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}

	var errs []error
	for i := range v.NumField() {
		f := v.Type().Field(i)
		if !f.IsExported() {
			continue
		}
		name := f.Name
		if path != "" {
			name = path + "." + f.Name
		}
		// Khối nhúng (anonymous) thì Validate của nó đã được promote lên struct
		// ngoài và chạy ở bước self của struct ngoài; chỉ duyệt khối con của nó
		if err := validateValue(v.Field(i), name, !f.Anonymous); err != nil {
			errs = append(errs, err)
		}
	}

	if self {
		if err := callValidate(v); err != nil {
			if path != "" {
				err = fmt.Errorf("%s: %w", path, err)
			}
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// callValidate gọi Validate với cả receiver con trỏ lẫn giá trị
func callValidate(v reflect.Value) error {
	if v.CanAddr() {
		if val, ok := v.Addr().Interface().(Validator); ok {
			return val.Validate()
		}
	}
	if v.CanInterface() {
		if val, ok := v.Interface().(Validator); ok {
			return val.Validate()
		}
	}
	return nil
}
