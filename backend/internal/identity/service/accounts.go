package service

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"storeit/internal/identity/domain"
	"storeit/internal/platform/auth"
)

type CreateAccountInput struct {
	Email    string
	Name     string
	Password string
	MemberID *uuid.UUID
	RoleIDs  []uuid.UUID
}

func (s *Service) CreateAccount(ctx context.Context, in CreateAccountInput) (AccountView, error) {
	if _, err := auth.Require(ctx, domain.PermAccountManage); err != nil {
		return AccountView{}, err
	}
	email, err := domain.NormalizeEmail(in.Email)
	if err != nil {
		return AccountView{}, err
	}
	name, err := cleanName(in.Name)
	if err != nil {
		return AccountView{}, err
	}
	if err := domain.ValidatePassword(in.Password); err != nil {
		return AccountView{}, err
	}
	h, err := s.hasher.Hash(in.Password)
	if err != nil {
		return AccountView{}, err
	}
	a, err := s.accounts.Create(ctx, domain.NewAccount{
		Email: email, Name: name, PasswordHash: h, MemberID: in.MemberID, RoleIDs: in.RoleIDs,
	})
	if err != nil {
		return AccountView{}, err
	}
	return s.view(ctx, a)
}

func (s *Service) GetAccount(ctx context.Context, id uuid.UUID) (AccountView, error) {
	if _, err := auth.Require(ctx, domain.PermAccountRead); err != nil {
		return AccountView{}, err
	}
	a, err := s.accounts.Get(ctx, id)
	if err != nil {
		return AccountView{}, err
	}
	return s.view(ctx, a)
}

// ListAccounts trả account không kèm role (tránh N+1); chi tiết dùng GetAccount
func (s *Service) ListAccounts(ctx context.Context, f domain.AccountFilter) ([]domain.Account, int64, error) {
	if _, err := auth.Require(ctx, domain.PermAccountRead); err != nil {
		return nil, 0, err
	}
	return s.accounts.List(ctx, f)
}

func (s *Service) UpdateAccount(ctx context.Context, id uuid.UUID, ch domain.ProfileChange) (AccountView, error) {
	if _, err := auth.Require(ctx, domain.PermAccountManage); err != nil {
		return AccountView{}, err
	}
	if ch.Name != nil {
		n, err := cleanName(*ch.Name)
		if err != nil {
			return AccountView{}, err
		}
		ch.Name = &n
	}
	a, err := s.accounts.UpdateProfile(ctx, id, ch)
	if err != nil {
		return AccountView{}, err
	}
	return s.view(ctx, a)
}

// DisableAccount khoá account và thu hồi mọi phiên của nó. Không tự khoá mình.
func (s *Service) DisableAccount(ctx context.Context, id uuid.UUID) (AccountView, error) {
	actor, err := auth.Require(ctx, domain.PermAccountManage)
	if err != nil {
		return AccountView{}, err
	}
	if actor.AccountID == id {
		return AccountView{}, domain.ErrLockout
	}
	a, err := s.accounts.SetActive(ctx, id, false)
	if err != nil {
		return AccountView{}, err
	}
	return s.view(ctx, a)
}

func (s *Service) EnableAccount(ctx context.Context, id uuid.UUID) (AccountView, error) {
	if _, err := auth.Require(ctx, domain.PermAccountManage); err != nil {
		return AccountView{}, err
	}
	a, err := s.accounts.SetActive(ctx, id, true)
	if err != nil {
		return AccountView{}, err
	}
	return s.view(ctx, a)
}

// ResetPassword: quản trị đặt lại mật khẩu, thu hồi mọi phiên của account đó
func (s *Service) ResetPassword(ctx context.Context, id uuid.UUID, password string) error {
	if _, err := auth.Require(ctx, domain.PermAccountManage); err != nil {
		return err
	}
	if err := domain.ValidatePassword(password); err != nil {
		return err
	}
	if _, err := s.accounts.Get(ctx, id); err != nil {
		return err
	}
	h, err := s.hasher.Hash(password)
	if err != nil {
		return err
	}
	return s.accounts.SetPassword(ctx, id, h, nil, true)
}

// AssignRoles thay toàn bộ role của account. Không tự bỏ role Administrator của mình.
func (s *Service) AssignRoles(ctx context.Context, id uuid.UUID, roleIDs []uuid.UUID) (AccountView, error) {
	actor, err := auth.Require(ctx, domain.PermAccountManage)
	if err != nil {
		return AccountView{}, err
	}
	if actor.AccountID == id && !slices.Contains(roleIDs, domain.AdministratorRoleID) {
		current, err := s.accounts.Roles(ctx, id)
		if err != nil {
			return AccountView{}, err
		}
		if slices.ContainsFunc(current, func(r domain.Role) bool { return r.ID == domain.AdministratorRoleID }) {
			return AccountView{}, domain.ErrLockout
		}
	}
	a, err := s.accounts.ReplaceRoles(ctx, id, roleIDs)
	if err != nil {
		return AccountView{}, err
	}
	return s.view(ctx, a)
}
