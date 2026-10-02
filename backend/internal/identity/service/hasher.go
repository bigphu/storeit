package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// Hasher băm và so mật khẩu. Production dùng bcrypt cost mặc định; test dùng
// cost thấp nhất cho nhanh.
type Hasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) bool
}

type bcryptHasher struct{ cost int }

// NewBcrypt: cost <= 0 là bcrypt.DefaultCost
func NewBcrypt(cost int) Hasher {
	if cost <= 0 {
		cost = bcrypt.DefaultCost
	}
	return bcryptHasher{cost: cost}
}

func (h bcryptHasher) Hash(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		return "", fmt.Errorf("identity: hash password: %w", err)
	}
	return string(b), nil
}

func (h bcryptHasher) Compare(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// newSecret tạo refresh token: 32 byte ngẫu nhiên base64url (gửi cho client)
// và SHA-256 của nó (lưu DB)
func newSecret() (raw string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, fmt.Errorf("identity: random token: %w", err)
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, hashSecret(raw), nil
}

// hashSecret: token đủ ngẫu nhiên nên SHA-256 không cần salt hay bcrypt
func hashSecret(raw string) []byte {
	h := sha256.Sum256([]byte(raw))
	return h[:]
}
