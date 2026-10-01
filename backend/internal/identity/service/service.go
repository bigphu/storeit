// Package service là use case của identity: đăng nhập, phiên, account, role.
// Mọi use case quản trị bắt đầu bằng auth.Require; service không thấy HTTP,
// SQL hay transaction.
package service

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"storeit/internal/identity/domain"
	"storeit/internal/platform/auth"
	"storeit/internal/platform/jwt"
)

// TokenIssuer phát access token; *jwt.Provider thoả interface này
type TokenIssuer interface {
	Issue(accountID uuid.UUID, permissions []string) (jwt.Token, error)
}

// Settings là thời hạn phiên (lấy từ identity.Config)
type Settings struct {
	SlidingTTL  time.Duration
	AbsoluteTTL time.Duration
	Grace       time.Duration
}

type Deps struct {
	Accounts domain.AccountRepository
	Roles    domain.RoleRepository
	Sessions domain.SessionRepository
	Hasher   Hasher
	Tokens   TokenIssuer
	Settings Settings
	// Now cho test điều khiển thời gian; nil là time.Now
	Now func() time.Time
}

type Service struct {
	accounts domain.AccountRepository
	roles    domain.RoleRepository
	sessions domain.SessionRepository
	hasher   Hasher
	tokens   TokenIssuer
	settings Settings
	now      func() time.Time

	// Hash giả để email lạ vẫn tốn đúng một lần bcrypt (chống dò email theo thời gian)
	dummyOnce sync.Once
	dummyHash string
}

func New(d Deps) *Service {
	now := d.Now
	if now == nil {
		now = time.Now
	}
	return &Service{
		accounts: d.Accounts, roles: d.Roles, sessions: d.Sessions,
		hasher: d.Hasher, tokens: d.Tokens, settings: d.Settings, now: now,
	}
}

// Device là ngữ cảnh thiết bị ghi vào phiên đăng nhập
type Device struct {
	UserAgent string
	IP        string
}

// AccountView là account kèm role của nó
type AccountView struct {
	domain.Account
	Roles []domain.Role
}

// currentActor trả actor của request; không có (hoặc là SystemActor, không
// phải người) thì 401
func currentActor(ctx context.Context) (auth.Actor, error) {
	a, ok := auth.FromContext(ctx)
	if !ok || a.IsSystem() {
		return auth.Actor{}, auth.ErrUnauthenticated
	}
	return a, nil
}

func (s *Service) view(ctx context.Context, a domain.Account) (AccountView, error) {
	roles, err := s.accounts.Roles(ctx, a.ID)
	if err != nil {
		return AccountView{}, err
	}
	return AccountView{Account: a, Roles: roles}, nil
}

func cleanName(name string) (string, error) {
	n := strings.TrimSpace(name)
	if n == "" {
		return "", domain.ErrInvalidName
	}
	return n, nil
}
