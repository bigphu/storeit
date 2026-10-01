package jwt

import (
	"errors"
	"time"
)

type Config struct {
	Keys      string `env:"JWT_KEYS_FILE,file,required,notEmpty"` // file content: "v1:<base64>,v2:<base64>" (phẩy hay xuống dòng)
	ActiveKID string `env:"JWT_ACTIVE_KID,required,notEmpty"`
	Issuer    string `env:"JWT_ISSUER,required,notEmpty"`
	Audience  string `env:"JWT_AUDIENCE" envDefault:"storeit-api"`

	AccessTokenTTL time.Duration `env:"JWT_ACCESS_TOKEN_TTL" envDefault:"15m"`
}

func (c Config) Validate() error {
	switch {
	case c.Issuer == "":
		return errors.New("jwt: missing issuer")
	case c.Audience == "":
		return errors.New("jwt: missing audience")
	case c.AccessTokenTTL <= 0:
		return errors.New("jwt: ttl must be greater than 0")
	}
	return nil
}
