package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"storeit/internal/identity/domain"
	"storeit/internal/platform/errs"
)

// Session là kết quả đăng nhập hoặc refresh: access token cho header
// Authorization, refresh token cho cookie
type Session struct {
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
	Account          domain.Account
}

// Login: sai email hay sai mật khẩu đều ErrBadCredentials, và email lạ vẫn tốn
// một lần bcrypt. Account bị khoá chỉ lộ ra khi mật khẩu đúng.
func (s *Service) Login(ctx context.Context, email, password string, dev Device) (Session, error) {
	e, err := domain.NormalizeEmail(email)
	if err != nil {
		s.hasher.Compare(s.dummy(), password)
		return Session{}, domain.ErrBadCredentials
	}
	a, err := s.accounts.GetByEmail(ctx, e)
	if errors.Is(err, domain.ErrAccountNotFound) {
		s.hasher.Compare(s.dummy(), password)
		return Session{}, domain.ErrBadCredentials
	}
	if err != nil {
		return Session{}, err
	}
	if !s.hasher.Compare(a.PasswordHash, password) {
		return Session{}, domain.ErrBadCredentials
	}
	if err := a.CanLogin(); err != nil {
		return Session{}, err
	}
	return s.startSession(ctx, a, dev)
}

// Refresh xoay refresh token. Mọi thất bại (thiếu, sai, hết hạn, thu hồi, dùng
// lại, account bị khoá) đều ErrInvalidRefreshToken. Quyền được nạp lại ở đây,
// nên đổi role có hiệu lực ở lần refresh kế tiếp.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (Session, error) {
	if refreshToken == "" {
		return Session{}, domain.ErrInvalidRefreshToken
	}
	next, nextHash, err := newSecret()
	if err != nil {
		return Session{}, errs.ErrInternal.With(errs.WithCause(err))
	}
	res, err := s.sessions.Refresh(ctx, domain.RefreshInput{
		TokenHash: hashSecret(refreshToken),
		NextHash:  nextHash,
		Now:       s.now(),
		Grace:     s.settings.Grace,
		Sliding:   s.settings.SlidingTTL,
	})
	if err != nil {
		return Session{}, err
	}
	if res.Decision != domain.RefreshRotate && res.Decision != domain.RefreshGrace {
		return Session{}, domain.ErrInvalidRefreshToken
	}
	a, err := s.accounts.Get(ctx, res.AccountID)
	if err != nil {
		return Session{}, err
	}
	return s.issue(ctx, a, next, res.ExpiresAt)
}

// Logout thu hồi phiên của refresh token. Idempotent: thiếu hay sai token vẫn
// thành công, vì kết quả người dùng muốn (không còn đăng nhập) đã đạt.
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}
	_, err := s.sessions.Revoke(ctx, hashSecret(refreshToken), domain.RevokeLogout)
	return err
}

// Me là account đang đăng nhập cùng role và quyền hiện tại của nó
type Me struct {
	Account     domain.Account
	Roles       []domain.Role
	Permissions []string
}

func (s *Service) Me(ctx context.Context) (Me, error) {
	actor, err := currentActor(ctx)
	if err != nil {
		return Me{}, err
	}
	a, err := s.accounts.Get(ctx, actor.AccountID)
	if errors.Is(err, domain.ErrAccountNotFound) {
		return Me{}, domain.ErrBadCredentials
	}
	if err != nil {
		return Me{}, err
	}
	roles, err := s.accounts.Roles(ctx, a.ID)
	if err != nil {
		return Me{}, err
	}
	perms, err := s.accounts.Permissions(ctx, a.ID)
	if err != nil {
		return Me{}, err
	}
	return Me{Account: a, Roles: roles, Permissions: perms}, nil
}

// ChangePassword: người dùng tự đổi mật khẩu. Giữ phiên đang dùng (family của
// refreshToken), thu hồi mọi phiên khác.
func (s *Service) ChangePassword(ctx context.Context, current, next, refreshToken string) error {
	actor, err := currentActor(ctx)
	if err != nil {
		return err
	}
	if err := domain.ValidatePassword(next); err != nil {
		return err
	}
	a, err := s.accounts.Get(ctx, actor.AccountID)
	if err != nil {
		return err
	}
	if !s.hasher.Compare(a.PasswordHash, current) {
		return domain.ErrWrongPassword
	}
	var keep *uuid.UUID
	if refreshToken != "" {
		if keep, err = s.sessions.FamilyOf(ctx, hashSecret(refreshToken)); err != nil {
			return err
		}
	}
	h, err := s.hasher.Hash(next)
	if err != nil {
		return err
	}
	return s.accounts.SetPassword(ctx, a.ID, h, keep)
}

func (s *Service) startSession(ctx context.Context, a domain.Account, dev Device) (Session, error) {
	raw, h, err := newSecret()
	if err != nil {
		return Session{}, errs.ErrInternal.With(errs.WithCause(err))
	}
	now := s.now()
	expires := now.Add(s.settings.SlidingTTL)
	absolute := now.Add(s.settings.AbsoluteTTL)
	if expires.After(absolute) {
		expires = absolute
	}
	if _, err := s.sessions.Start(ctx, domain.NewSession{
		AccountID: a.ID, TokenHash: h, UserAgent: dev.UserAgent, IP: dev.IP,
		ExpiresAt: expires, AbsoluteExpiresAt: absolute,
	}); err != nil {
		return Session{}, err
	}
	return s.issue(ctx, a, raw, expires)
}

// issue phát access token với quyền hiện tại của account
func (s *Service) issue(ctx context.Context, a domain.Account, refresh string, refreshExpires time.Time) (Session, error) {
	perms, err := s.accounts.Permissions(ctx, a.ID)
	if err != nil {
		return Session{}, err
	}
	tok, err := s.tokens.Issue(a.ID, perms)
	if err != nil {
		return Session{}, errs.ErrInternal.With(errs.WithCause(err))
	}
	return Session{
		AccessToken:      tok.Value,
		AccessExpiresAt:  tok.ExpiresAt,
		RefreshToken:     refresh,
		RefreshExpiresAt: refreshExpires,
		Account:          a,
	}, nil
}

func (s *Service) dummy() string {
	s.dummyOnce.Do(func() {
		h, err := s.hasher.Hash("storeit-timing-equalizer")
		if err == nil {
			s.dummyHash = h
		}
	})
	return s.dummyHash
}
