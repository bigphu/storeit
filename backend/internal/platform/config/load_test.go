package config

import (
	"errors"
	"strings"
	"testing"
)

// Khối cấu hình giả, như database.Config hay jwt.Config: tự Validate field
// của mình. pointer receiver để kiểm tra Load gọi được cả hai kiểu receiver.
type dbBlock struct {
	MaxConns int `env:"T_DB_MAX_CONNS" envDefault:"10"`
}

func (c dbBlock) Validate() error {
	if c.MaxConns > 100 {
		return errors.New("db: too many conns")
	}
	return nil
}

type jwtBlock struct {
	Issuer string `env:"T_JWT_ISSUER"`
	calls  *int
}

func (c *jwtBlock) Validate() error {
	if c.calls != nil {
		*c.calls++
	}
	if c.Issuer == "" {
		return errors.New("jwt: missing issuer")
	}
	return nil
}

// Binary chỉ liệt kê khối nó dùng, không phải tự gọi Validate của từng khối
type binaryConfig struct {
	DB  dbBlock
	JWT jwtBlock
	Opt *dbBlock // khối tuỳ chọn để nil thì bỏ qua
}

func TestLoad_ValidatesEveryBlock(t *testing.T) {
	t.Setenv("T_DB_MAX_CONNS", "500")
	t.Setenv("T_JWT_ISSUER", "")

	err := Load(&binaryConfig{})

	if err == nil {
		t.Fatal("want errors from both blocks")
	}
	// Lỗi mang đường dẫn field để biết khối nào sai
	for _, want := range []string{"DB: db: too many conns", "JWT: jwt: missing issuer"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, want it to contain %q", err, want)
		}
	}
}

func TestLoad_ValidConfig(t *testing.T) {
	t.Setenv("T_JWT_ISSUER", "storeit")

	var cfg binaryConfig
	if err := Load(&cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.DB.MaxConns != 10 || cfg.JWT.Issuer != "storeit" {
		t.Errorf("cfg = %+v", cfg)
	}
}

// Binary vẫn được có Validate riêng cho ràng buộc giữa các khối; nó chạy sau
// khi các khối con đã hợp lệ từng cái
type crossConfig struct {
	DB  dbBlock
	JWT jwtBlock
}

func (c *crossConfig) Validate() error {
	if c.DB.MaxConns < 20 && c.JWT.Issuer == "big" {
		return errors.New("big issuer needs more conns")
	}
	return nil
}

func TestLoad_TopLevelValidate(t *testing.T) {
	t.Setenv("T_JWT_ISSUER", "big")

	err := Load(&crossConfig{})

	if err == nil || !strings.Contains(err.Error(), "big issuer needs more conns") {
		t.Errorf("err = %v", err)
	}
}

// Khối nhúng (anonymous) có Validate được promote lên struct ngoài: chỉ gọi
// một lần, không phải hai
type embedsJWT struct {
	jwtBlock
}

func TestValidate_EmbeddedBlockCalledOnce(t *testing.T) {
	calls := 0
	cfg := embedsJWT{jwtBlock{Issuer: "storeit", calls: &calls}}

	if err := validate(&cfg); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("Validate called %d times, want 1", calls)
	}
}
