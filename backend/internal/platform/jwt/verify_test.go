package jwt

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var testSecret = []byte(strings.Repeat("k", minSecretLen))

// sign ký token tay để thử các trường hợp Issue không bao giờ tạo ra
func sign(t *testing.T, method gojwt.SigningMethod, key any, header map[string]any, claims gojwt.MapClaims) string {
	t.Helper()
	tok := gojwt.NewWithClaims(method, claims)
	tok.Header["kid"] = "v1"
	tok.Header["typ"] = typeHeader
	for k, v := range header {
		tok.Header[k] = v
	}
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func validClaims() gojwt.MapClaims {
	now := time.Now()
	return gojwt.MapClaims{
		"iss": "storeit", "aud": "storeit-api", "sub": uuid.NewString(), "jti": uuid.NewString(),
		"iat": now.Unix(), "nbf": now.Unix(), "exp": now.Add(time.Minute).Unix(),
	}
}

func with(c gojwt.MapClaims, k string, v any) gojwt.MapClaims {
	c[k] = v
	return c
}

func without(c gojwt.MapClaims, k string) gojwt.MapClaims {
	delete(c, k)
	return c
}

// Mọi token sai phải bị từ chối với lỗi đúng loại (auth trả 401 cho tất cả,
// loại lỗi để log biết vì sao)
func TestVerify_Rejects(t *testing.T) {
	p := newTestProvider(t)
	hour := time.Hour
	tests := []struct {
		name  string
		token string
		want  error
	}{
		{"malformed", "not.a.jwt", ErrMalformed},
		{"expired beyond leeway", sign(t, gojwt.SigningMethodHS256, testSecret, nil,
			with(validClaims(), "exp", time.Now().Add(-hour).Unix())), ErrExpired},
		{"not yet valid", sign(t, gojwt.SigningMethodHS256, testSecret, nil,
			with(validClaims(), "nbf", time.Now().Add(hour).Unix())), ErrNotYetValid},
		{"signed with another secret", sign(t, gojwt.SigningMethodHS256, []byte(strings.Repeat("x", 32)), nil,
			validClaims()), ErrSignature},
		{"unknown kid", sign(t, gojwt.SigningMethodHS256, testSecret, map[string]any{"kid": "v9"},
			validClaims()), ErrUnknownKID},
		// Thuật toán khác HS256 (kể cả HMAC khác) bị chặn trước khi kiểm chữ ký
		{"HS512 not allowed", sign(t, gojwt.SigningMethodHS512, testSecret, nil, validClaims()), ErrSignature},
		{"alg none", sign(t, gojwt.SigningMethodNone, gojwt.UnsafeAllowNoneSignatureType, nil, validClaims()), ErrSignature},
		{"wrong issuer", sign(t, gojwt.SigningMethodHS256, testSecret, nil,
			with(validClaims(), "iss", "evil")), ErrClaims},
		{"wrong audience", sign(t, gojwt.SigningMethodHS256, testSecret, nil,
			with(validClaims(), "aud", "other-api")), ErrClaims},
		{"no expiry", sign(t, gojwt.SigningMethodHS256, testSecret, nil,
			without(validClaims(), "exp")), ErrClaims},
		// Token không phải access token (vd id token typ JWT) không dùng thay được
		{"wrong typ", sign(t, gojwt.SigningMethodHS256, testSecret, map[string]any{"typ": "JWT"},
			validClaims()), ErrBadType},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims, err := p.Verify(tt.token)
			if claims != nil || !errors.Is(err, tt.want) {
				t.Errorf("Verify = %v, %v; want error %v", claims, err, tt.want)
			}
		})
	}
}

// Leeway 30s: token vừa hết hạn vài giây vẫn qua (lệch đồng hồ giữa các máy)
func TestVerify_LeewayOnExpiry(t *testing.T) {
	p := newTestProvider(t)
	tok := sign(t, gojwt.SigningMethodHS256, testSecret, nil,
		with(validClaims(), "exp", time.Now().Add(-10*time.Second).Unix()))
	if _, err := p.Verify(tok); err != nil {
		t.Errorf("Verify = %v, want token within leeway accepted", err)
	}
}

// Xoay key: token ký bằng kid cũ vẫn verify được khi kid mới đã active
func TestVerify_KeyRotation(t *testing.T) {
	enc := base64.StdEncoding.EncodeToString
	cfg := Config{
		Keys:           "v1:" + enc(testSecret),
		ActiveKID:      "v1",
		Issuer:         "storeit",
		Audience:       "storeit-api",
		AccessTokenTTL: time.Minute,
	}
	old, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	tok, err := old.Issue(uuid.New(), nil)
	if err != nil {
		t.Fatal(err)
	}

	cfg.Keys += "," + "v2:" + enc([]byte(strings.Repeat("n", minSecretLen)))
	cfg.ActiveKID = "v2"
	rotated, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := rotated.Verify(tok.Value); err != nil {
		t.Errorf("token from old kid rejected after rotation: %v", err)
	}
	fresh, err := rotated.Issue(uuid.New(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := old.Verify(fresh.Value); !errors.Is(err, ErrUnknownKID) {
		t.Errorf("old provider verifying v2 token: err = %v, want ErrUnknownKID", err)
	}
}

func TestClaims_IDs(t *testing.T) {
	p := newTestProvider(t)
	tok, err := p.Issue(uuid.New(), nil)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := p.Verify(tok.Value)
	if err != nil {
		t.Fatal(err)
	}
	if id, err := claims.TokenID(); err != nil || id != tok.ID {
		t.Errorf("TokenID = %v, %v; want %v", id, err, tok.ID)
	}

	// sub không phải UUID: token hợp lệ về chữ ký nhưng không dùng được
	bad := &Claims{RegisteredClaims: gojwt.RegisteredClaims{Subject: "admin", ID: "x"}}
	if _, err := bad.UserID(); !errors.Is(err, ErrClaims) {
		t.Errorf("UserID err = %v, want ErrClaims", err)
	}
	if _, err := bad.TokenID(); !errors.Is(err, ErrClaims) {
		t.Errorf("TokenID err = %v, want ErrClaims", err)
	}
}

func TestConfig_Validate(t *testing.T) {
	ok := Config{Issuer: "storeit", Audience: "storeit-api", AccessTokenTTL: time.Minute}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*Config){
		"no issuer":   func(c *Config) { c.Issuer = "" },
		"no audience": func(c *Config) { c.Audience = "" },
		"zero ttl":    func(c *Config) { c.AccessTokenTTL = 0 },
	} {
		c := ok
		mutate(&c)
		if err := c.Validate(); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}
