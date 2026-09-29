package jwt

import (
	"fmt"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	gojwt.RegisteredClaims

	// Quyền của account lúc phát token (hợp của các role). Đổi role thì token
	// cũ vẫn giữ quyền cũ tới khi hết hạn, nên TTL phải ngắn
	Permissions []string `json:"perms"`
}

func (c *Claims) UserID() (uuid.UUID, error) {
	id, err := uuid.Parse(c.Subject)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: sub is not uuid: %v", ErrClaims, err)
	}
	return id, nil
}

func (c *Claims) TokenID() (uuid.UUID, error) {
	id, err := uuid.Parse(c.ID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: jti is not uuid: %v", ErrClaims, err)
	}
	return id, nil
}
