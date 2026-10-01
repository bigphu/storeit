package jwt

import (
	"encoding/base64"
	"strings"
	"testing"
)

// kid trùng (vd copy dòng khi xoay key rồi quên đổi tên) thì key sau đè key
// trước mà không ai biết; phải báo lỗi lúc khởi động
func TestParseKeys_DuplicateKID(t *testing.T) {
	a := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("a", minSecretLen)))
	b := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("b", minSecretLen)))

	if _, err := parseKeys("v1:"+a+",v1:"+b, "v1"); err == nil {
		t.Error("want error for duplicate kid v1")
	}
}

// Xoay key: thêm key mới trên dòng riêng là cách viết tự nhiên, phải đọc được
// như khi ngăn cách bằng dấu phẩy
func TestParseKeys_NewlineSeparated(t *testing.T) {
	a := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("a", minSecretLen)))
	b := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("b", minSecretLen)))

	ring, err := parseKeys("v1:"+a+"\r\nv2:"+b+"\n", "v2")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := ring.lookup("v1"); !ok {
		t.Error("v1 missing from keyring")
	}
	if kid, _ := ring.active(); kid != "v2" {
		t.Errorf("active kid = %q, want v2", kid)
	}
}
