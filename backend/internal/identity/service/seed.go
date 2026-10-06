package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"storeit/internal/identity/domain"
	"storeit/internal/platform/auth"
)

// SeedAccount tạo account có sẵn mật khẩu cho dữ liệu demo (chỉ cmd/seed gọi,
// như Bootstrap). Email đã có thì trả id của account đó, không đổi gì.
func (s *Service) SeedAccount(ctx context.Context, email, name, password string, roleIDs []uuid.UUID) (uuid.UUID, error) {
	e, err := domain.NormalizeEmail(email)
	if err != nil {
		return uuid.Nil, err
	}
	n, err := cleanName(name)
	if err != nil {
		return uuid.Nil, err
	}
	if err := domain.ValidatePassword(password); err != nil {
		return uuid.Nil, err
	}
	if a, err := s.accounts.GetByEmail(ctx, e); err == nil {
		return a.ID, nil
	} else if !errors.Is(err, domain.ErrAccountNotFound) {
		return uuid.Nil, err
	}
	h, err := s.hasher.Hash(password)
	if err != nil {
		return uuid.Nil, err
	}
	// Không do ai yêu cầu: event ghi actor là hệ thống
	ctx = auth.WithActor(ctx, auth.SystemActor)
	a, err := s.accounts.Create(ctx, domain.NewAccount{Email: e, Name: n, PasswordHash: h, RoleIDs: roleIDs})
	if errors.Is(err, domain.ErrEmailTaken) {
		// chạy song song: bên kia vừa tạo
		got, gerr := s.accounts.GetByEmail(ctx, e)
		if gerr != nil {
			return uuid.Nil, gerr
		}
		return got.ID, nil
	}
	if err != nil {
		return uuid.Nil, err
	}
	return a.ID, nil
}
