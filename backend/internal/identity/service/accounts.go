package service

import (
	"context"
	"slices"
	"time"

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
	v, err := s.view(ctx, a)
	if err != nil {
		return AccountView{}, err
	}
	// trang chi tiết: số phiên còn sống và hạn link mời
	n, err := s.accounts.LiveSessions(ctx, id)
	if err != nil {
		return AccountView{}, err
	}
	v.Sessions = &n
	exp, err := s.accounts.InviteExpiries(ctx, []uuid.UUID{id})
	if err != nil {
		return AccountView{}, err
	}
	if t, ok := exp[id]; ok {
		v.InviteExpiresAt = &t
	}
	return v, nil
}

// AccountListItem: account trong danh sách, kèm role và hạn link mời đang chờ
type AccountListItem struct {
	domain.Account
	Roles           []domain.Role
	InviteExpiresAt *time.Time
}

// AccountPage: một trang account, tổng theo bộ lọc và số account theo trạng thái
// (cùng tìm kiếm và role, bỏ qua lọc trạng thái) cho các nút lọc
type AccountPage struct {
	Items        []AccountListItem
	Total        int64
	StatusCounts map[domain.AccountStatus]int64
}

func (s *Service) ListAccounts(ctx context.Context, f domain.AccountFilter) (AccountPage, error) {
	if _, err := auth.Require(ctx, domain.PermAccountRead); err != nil {
		return AccountPage{}, err
	}
	accounts, total, err := s.accounts.List(ctx, f)
	if err != nil {
		return AccountPage{}, err
	}
	counts, err := s.accounts.StatusCounts(ctx, f)
	if err != nil {
		return AccountPage{}, err
	}
	ids := make([]uuid.UUID, len(accounts))
	for i, a := range accounts {
		ids[i] = a.ID
	}
	roles, err := s.accounts.RolesOf(ctx, ids)
	if err != nil {
		return AccountPage{}, err
	}
	exp, err := s.accounts.InviteExpiries(ctx, ids)
	if err != nil {
		return AccountPage{}, err
	}
	page := AccountPage{Items: make([]AccountListItem, len(accounts)), Total: total, StatusCounts: counts}
	for i, a := range accounts {
		page.Items[i] = AccountListItem{Account: a, Roles: roles[a.ID]}
		if t, ok := exp[a.ID]; ok && a.Status() == domain.StatusInvited {
			page.Items[i].InviteExpiresAt = &t
		}
	}
	return page, nil
}

// SignOutEverywhere thu hồi mọi phiên của account. Như khoá account, người làm phải
// có đủ quyền của account đó. Trả số phiên đã thu hồi.
func (s *Service) SignOutEverywhere(ctx context.Context, id uuid.UUID) (int64, error) {
	actor, err := auth.Require(ctx, domain.PermAccountManage)
	if err != nil {
		return 0, err
	}
	if err := s.requireHoldsAccount(ctx, actor, id); err != nil {
		return 0, err
	}
	return s.accounts.SignOutEverywhere(ctx, id)
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
