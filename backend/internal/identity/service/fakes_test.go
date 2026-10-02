package service

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"storeit/internal/identity/domain"
	"storeit/internal/platform/jwt"
)

// Fake repository trong bộ nhớ: đủ để kiểm tra logic của service. Tính đúng
// của SQL, transaction và event được kiểm ở repository (test với Postgres).

type fakeRoles struct {
	mu    sync.Mutex
	roles map[uuid.UUID]domain.Role
}

func newFakeRoles() *fakeRoles {
	all := []string{domain.PermAccountRead, domain.PermAccountManage, domain.PermRoleRead, domain.PermRoleManage}
	return &fakeRoles{roles: map[uuid.UUID]domain.Role{
		domain.AdministratorRoleID: {ID: domain.AdministratorRoleID, Name: "Administrator", IsSystem: true, Permissions: all},
		domain.EmployeeRoleID:      {ID: domain.EmployeeRoleID, Name: "Employee", IsSystem: true, Permissions: []string{}},
	}}
}

func (f *fakeRoles) List(context.Context) ([]domain.Role, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.Role
	for _, r := range f.roles {
		out = append(out, r)
	}
	slices.SortFunc(out, func(a, b domain.Role) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

func (f *fakeRoles) Get(_ context.Context, id uuid.UUID) (domain.Role, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.roles[id]
	if !ok {
		return domain.Role{}, domain.ErrRoleNotFound
	}
	return r, nil
}

func (f *fakeRoles) Create(_ context.Context, r domain.Role) (domain.Role, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r.ID = uuid.New()
	f.roles[r.ID] = r
	return r, nil
}

func (f *fakeRoles) Update(_ context.Context, id uuid.UUID, name, description string) (domain.Role, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r := f.roles[id]
	r.Name, r.Description = name, description
	f.roles[id] = r
	return r, nil
}

func (f *fakeRoles) ReplacePermissions(_ context.Context, id uuid.UUID, perms []string) (domain.Role, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r := f.roles[id]
	r.Permissions = perms
	f.roles[id] = r
	return r, nil
}

func (f *fakeRoles) Delete(_ context.Context, id uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.roles, id)
	return nil
}

func (f *fakeRoles) CountAssignments(context.Context, uuid.UUID) (int64, error) { return 0, nil }

func (f *fakeRoles) Permissions(context.Context) ([]domain.Permission, error) {
	return []domain.Permission{{Code: domain.PermAccountRead, Description: "View accounts"}}, nil
}

type passwordCall struct {
	id   uuid.UUID
	keep *uuid.UUID
}

type fakeAccounts struct {
	mu        sync.Mutex
	accounts  map[uuid.UUID]domain.Account
	roleIDs   map[uuid.UUID][]uuid.UUID
	roles     *fakeRoles
	passwords []passwordCall

	// staleCount: Count trả 0 dù đã có account, giả lập hai server khởi động
	// cùng lúc trên DB trống
	staleCount bool
	// invites nhận lời mời phát cùng lúc tạo account (Create với Invite)
	invites *fakePasswordTokens
}

func newFakeAccounts(roles *fakeRoles) *fakeAccounts {
	return &fakeAccounts{accounts: map[uuid.UUID]domain.Account{}, roleIDs: map[uuid.UUID][]uuid.UUID{}, roles: roles}
}

func (f *fakeAccounts) Create(_ context.Context, in domain.NewAccount) (domain.Account, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, a := range f.accounts {
		if a.Email == in.Email {
			return domain.Account{}, domain.ErrEmailTaken
		}
	}
	a := domain.Account{
		ID: uuid.New(), Email: in.Email, Name: in.Name, PasswordHash: in.PasswordHash,
		MemberID: in.MemberID, Active: true, Version: 1,
	}
	f.accounts[a.ID] = a
	f.roleIDs[a.ID] = in.RoleIDs
	if in.Invite != nil && f.invites != nil {
		_, _ = f.invites.Issue(context.Background(), a.ID, *in.Invite, domain.TokenEventNone, 0)
	}
	return a, nil
}

func (f *fakeAccounts) Get(_ context.Context, id uuid.UUID) (domain.Account, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.accounts[id]
	if !ok {
		return domain.Account{}, domain.ErrAccountNotFound
	}
	return a, nil
}

func (f *fakeAccounts) GetByEmail(_ context.Context, email string) (domain.Account, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, a := range f.accounts {
		if a.Email == strings.ToLower(email) {
			return a, nil
		}
	}
	return domain.Account{}, domain.ErrAccountNotFound
}

func (f *fakeAccounts) GetMany(ctx context.Context, ids []uuid.UUID) ([]domain.Account, error) {
	var out []domain.Account
	for _, id := range ids {
		if a, err := f.Get(ctx, id); err == nil {
			out = append(out, a)
		}
	}
	return out, nil
}

func (f *fakeAccounts) List(context.Context, domain.AccountFilter) ([]domain.Account, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.Account
	for _, a := range f.accounts {
		out = append(out, a)
	}
	return out, int64(len(out)), nil
}

func (f *fakeAccounts) UpdateProfile(_ context.Context, id uuid.UUID, ch domain.ProfileChange) (domain.Account, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a := f.accounts[id]
	if a.Version != ch.Version {
		return domain.Account{}, domain.ErrAccountChanged
	}
	if ch.Name != nil {
		a.Name = *ch.Name
	}
	a.Version++
	f.accounts[id] = a
	return a, nil
}

func (f *fakeAccounts) SetActive(_ context.Context, id uuid.UUID, active bool) (domain.Account, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.accounts[id]
	if !ok {
		return domain.Account{}, domain.ErrAccountNotFound
	}
	a.Active = active
	f.accounts[id] = a
	return a, nil
}

func (f *fakeAccounts) SetPassword(_ context.Context, id uuid.UUID, hash string, keep *uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	a := f.accounts[id]
	a.PasswordHash = hash
	f.accounts[id] = a
	f.passwords = append(f.passwords, passwordCall{id: id, keep: keep})
	return nil
}

func (f *fakeAccounts) ReplaceRoles(_ context.Context, id uuid.UUID, roleIDs []uuid.UUID) (domain.Account, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.roleIDs[id] = roleIDs
	return f.accounts[id], nil
}

func (f *fakeAccounts) Roles(ctx context.Context, id uuid.UUID) ([]domain.Role, error) {
	f.mu.Lock()
	ids := slices.Clone(f.roleIDs[id])
	f.mu.Unlock()
	var out []domain.Role
	for _, rid := range ids {
		if r, err := f.roles.Get(ctx, rid); err == nil {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeAccounts) Permissions(ctx context.Context, id uuid.UUID) ([]string, error) {
	roles, _ := f.Roles(ctx, id)
	var out []string
	for _, r := range roles {
		out = append(out, r.Permissions...)
	}
	slices.Sort(out)
	return slices.Compact(out), nil
}

func (f *fakeAccounts) Count(context.Context) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.staleCount {
		return 0, nil
	}
	return int64(len(f.accounts)), nil
}

type fakeSessions struct {
	mu       sync.Mutex
	started  []domain.NewSession
	refresh  domain.RefreshResult
	inputs   []domain.RefreshInput
	revoked  [][]byte
	family   *uuid.UUID
	familyOf map[string]uuid.UUID

	pruneCutoff time.Time
}

func (f *fakeSessions) Start(_ context.Context, s domain.NewSession) (uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.started = append(f.started, s)
	return uuid.New(), nil
}

func (f *fakeSessions) Refresh(_ context.Context, in domain.RefreshInput) (domain.RefreshResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.inputs = append(f.inputs, in)
	return f.refresh, nil
}

func (f *fakeSessions) Revoke(_ context.Context, h []byte, _ domain.RevokeReason) (*uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.revoked = append(f.revoked, h)
	return f.family, nil
}

func (f *fakeSessions) FamilyOf(_ context.Context, h []byte) (*uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if id, ok := f.familyOf[string(h)]; ok {
		return &id, nil
	}
	return nil, nil
}

func (f *fakeSessions) Prune(_ context.Context, cutoff time.Time) (int64, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pruneCutoff = cutoff
	return 2, 5, nil
}

// fakeTokens ghi lại quyền được đưa vào access token
type fakeTokens struct {
	mu    sync.Mutex
	perms [][]string
}

func (f *fakeTokens) Issue(accountID uuid.UUID, perms []string) (jwt.Token, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.perms = append(f.perms, perms)
	return jwt.Token{Value: "access-" + accountID.String(), ID: uuid.New(), ExpiresAt: time.Now().Add(15 * time.Minute)}, nil
}

// countingHasher đếm số lần so mật khẩu, để kiểm tra email lạ vẫn tốn một lần bcrypt
type countingHasher struct {
	Hasher
	mu       sync.Mutex
	compares int
}

func (h *countingHasher) Compare(hash, pw string) bool {
	h.mu.Lock()
	h.compares++
	h.mu.Unlock()
	return h.Hasher.Compare(hash, pw)
}
