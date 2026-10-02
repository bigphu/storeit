package service

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"storeit/internal/identity/domain"
	"storeit/internal/platform/auth"
)

// CreateAccountInput: không có mật khẩu. Người dùng tự đặt qua link mời.
type CreateAccountInput struct {
	Email    string
	Name     string
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
	invite, err := s.newToken(domain.PurposeInvite)
	if err != nil {
		return AccountView{}, err
	}
	a, err := s.accounts.Create(ctx, domain.NewAccount{
		Email: email, Name: name, MemberID: in.MemberID, RoleIDs: in.RoleIDs, Invite: &invite,
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
	if ch.ClearMember && ch.MemberID != nil {
		return AccountView{}, domain.ErrMemberConflict
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
	if err := s.requireHoldsAccount(ctx, actor, id); err != nil {
		return AccountView{}, err
	}
	a, err := s.accounts.SetActive(ctx, id, false)
	if err != nil {
		return AccountView{}, err
	}
	return s.view(ctx, a)
}

// EnableAccount mở khoá: trả lại cho account mọi quyền của nó, nên người mở
// cũng phải có đủ các quyền đó
func (s *Service) EnableAccount(ctx context.Context, id uuid.UUID) (AccountView, error) {
	actor, err := auth.Require(ctx, domain.PermAccountManage)
	if err != nil {
		return AccountView{}, err
	}
	if err := s.requireHoldsAccount(ctx, actor, id); err != nil {
		return AccountView{}, err
	}
	a, err := s.accounts.SetActive(ctx, id, true)
	if err != nil {
		return AccountView{}, err
	}
	return s.view(ctx, a)
}

// AssignRoles thay toàn bộ role của account. Không tự bỏ role Administrator
// của mình; role được thêm hay bị gỡ chỉ được mang quyền người gán có.
func (s *Service) AssignRoles(ctx context.Context, id uuid.UUID, roleIDs []uuid.UUID) (AccountView, error) {
	actor, err := auth.Require(ctx, domain.PermAccountManage)
	if err != nil {
		return AccountView{}, err
	}
	current, err := s.accounts.Roles(ctx, id)
	if err != nil {
		return AccountView{}, err
	}
	currentIDs := make([]uuid.UUID, len(current))
	for i, r := range current {
		currentIDs[i] = r.ID
	}
	if actor.AccountID == id && slices.Contains(currentIDs, domain.AdministratorRoleID) &&
		!slices.Contains(roleIDs, domain.AdministratorRoleID) {
		return AccountView{}, domain.ErrLockout
	}
	all, err := s.roles.List(ctx)
	if err != nil {
		return AccountView{}, err
	}
	perms := map[uuid.UUID][]string{}
	for _, r := range all {
		perms[r.ID] = r.Permissions
	}
	// Role không tồn tại không mang quyền nào; ReplaceRoles trả ErrUnknownRoles
	for _, rid := range symmetricDiff(currentIDs, roleIDs) {
		if err := requireHolds(actor, perms[rid]); err != nil {
			return AccountView{}, err
		}
	}
	a, err := s.accounts.ReplaceRoles(ctx, id, roleIDs)
	if err != nil {
		return AccountView{}, err
	}
	return s.view(ctx, a)
}

// requireHoldsAccount: tác động lên account (khoá, mở khoá) đòi người làm có mọi
// quyền account đó đang có
func (s *Service) requireHoldsAccount(ctx context.Context, actor auth.Actor, id uuid.UUID) error {
	perms, err := s.accounts.Permissions(ctx, id)
	if err != nil {
		return err
	}
	return requireHolds(actor, perms)
}
