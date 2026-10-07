package repository

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"storeit/internal/identity/contract"
	"storeit/internal/identity/domain"
	"storeit/internal/identity/repository/db"
	"storeit/internal/platform/database"
	"storeit/internal/platform/events"
	"storeit/internal/platform/jobs"
)

// AccountRepository cài đặt domain.AccountRepository. Mọi method ghi chạy
// trong một database.WithTx, gồm cả event vào outbox.
type AccountRepository struct {
	pool   *pgxpool.Pool
	q      *db.Queries
	outbox *events.Outbox
	jobs   jobs.Enqueuer
}

var _ domain.AccountRepository = (*AccountRepository)(nil)

func NewAccountRepository(pool *pgxpool.Pool, outbox *events.Outbox, enq jobs.Enqueuer) *AccountRepository {
	return &AccountRepository{pool: pool, q: db.New(pool), outbox: outbox, jobs: enq}
}

func (r *AccountRepository) Create(ctx context.Context, in domain.NewAccount) (domain.Account, error) {
	id, err := newID()
	if err != nil {
		return domain.Account{}, err
	}
	roleIDs := uniqueIDs(in.RoleIDs)
	var out domain.Account
	err = database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		row, err := q.CreateAccount(ctx, db.CreateAccountParams{
			ID: id, Email: in.Email, Name: in.Name, PasswordHash: nilIfEmpty(in.PasswordHash), MemberID: in.MemberID,
		})
		if err != nil {
			if code, constraint := pgCode(err); code == codeUniqueViolation && constraint == "accounts_email_lower" {
				return domain.ErrEmailTaken
			}
			return fmt.Errorf("identity: create account: %w", err)
		}
		if err := insertRoles(ctx, q, id, roleIDs); err != nil {
			return err
		}
		if in.Invite != nil {
			if _, err := issueToken(ctx, tx, q, r.jobs, id, *in.Invite, 0); err != nil {
				return err
			}
		}
		out = toAccount(row)
		return r.append(ctx, tx, contract.EventAccountCreated, id, contract.AccountCreated{
			AccountID: id, Email: row.Email, Name: row.Name, MemberID: row.MemberID, RoleIDs: roleIDs,
		})
	})
	return out, err
}

// Get kèm lần đăng nhập gần nhất (bảng account_sign_ins)
func (r *AccountRepository) Get(ctx context.Context, id uuid.UUID) (domain.Account, error) {
	a, err := accountOrNotFound(r.q.GetAccount(ctx, id))
	if err != nil {
		return domain.Account{}, err
	}
	out := []domain.Account{a}
	if err := r.attachSignIns(ctx, out); err != nil {
		return domain.Account{}, err
	}
	return out[0], nil
}

// attachSignIns điền LastSignInAt cho các account trong một truy vấn
func (r *AccountRepository) attachSignIns(ctx context.Context, accounts []domain.Account) error {
	ids := make([]uuid.UUID, len(accounts))
	for i, a := range accounts {
		ids[i] = a.ID
	}
	rows, err := r.q.ListLastSignIns(ctx, ids)
	if err != nil {
		return fmt.Errorf("identity: last sign-ins: %w", err)
	}
	at := make(map[uuid.UUID]time.Time, len(rows))
	for _, row := range rows {
		at[row.AccountID] = row.LastAt
	}
	for i := range accounts {
		if t, ok := at[accounts[i].ID]; ok {
			accounts[i].LastSignInAt = &t
		}
	}
	return nil
}

func (r *AccountRepository) GetByEmail(ctx context.Context, email string) (domain.Account, error) {
	row, err := r.q.GetAccountByEmail(ctx, email)
	return accountOrNotFound(row, err)
}

func (r *AccountRepository) GetMany(ctx context.Context, ids []uuid.UUID) ([]domain.Account, error) {
	rows, err := r.q.GetAccountsByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("identity: get accounts: %w", err)
	}
	out := make([]domain.Account, len(rows))
	for i, row := range rows {
		out[i] = toAccount(row)
	}
	return out, nil
}

func (r *AccountRepository) List(ctx context.Context, f domain.AccountFilter) ([]domain.Account, int64, error) {
	var q *string
	if f.Query != "" {
		// % và _ người dùng gõ là chữ, không phải ký tự đại diện của ILIKE
		// (ký tự escape mặc định của ILIKE là dấu gạch ngược)
		esc := likeEscaper.Replace(f.Query)
		q = &esc
	}
	var status *string
	if f.Status != nil {
		status = ptr(string(*f.Status))
	}
	rows, err := r.q.ListAccounts(ctx, db.ListAccountsParams{
		Q: q, Active: f.Active, Status: status, RoleID: f.RoleID, Lim: f.Limit, Off: f.Offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("identity: list accounts: %w", err)
	}
	total, err := r.q.CountAccounts(ctx, db.CountAccountsParams{Q: q, Active: f.Active, Status: status, RoleID: f.RoleID})
	if err != nil {
		return nil, 0, fmt.Errorf("identity: count accounts: %w", err)
	}
	out := make([]domain.Account, len(rows))
	for i, row := range rows {
		out[i] = toAccount(row)
	}
	if err := r.attachSignIns(ctx, out); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (r *AccountRepository) UpdateProfile(ctx context.Context, id uuid.UUID, ch domain.ProfileChange) (domain.Account, error) {
	var out domain.Account
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		cur, err := accountOrNotFound(q.GetAccountForUpdate(ctx, id))
		if err != nil {
			return err
		}
		name, member := cur.Name, cur.MemberID
		var changes []contract.FieldChange
		if ch.Name != nil && *ch.Name != cur.Name {
			changes = append(changes, contract.FieldChange{Field: "name", From: cur.Name, To: *ch.Name})
			name = *ch.Name
		}
		switch {
		case ch.ClearMember && cur.MemberID != nil:
			changes = append(changes, contract.FieldChange{Field: "member_id", From: cur.MemberID, To: nil})
			member = nil
		case ch.MemberID != nil && (cur.MemberID == nil || *cur.MemberID != *ch.MemberID):
			changes = append(changes, contract.FieldChange{Field: "member_id", From: cur.MemberID, To: ch.MemberID})
			member = ch.MemberID
		}
		row, err := q.UpdateAccountProfile(ctx, db.UpdateAccountProfileParams{
			ID: id, Name: name, MemberID: member, Version: ch.Version,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrAccountChanged
		}
		if err != nil {
			return fmt.Errorf("identity: update account: %w", err)
		}
		out = toAccount(row)
		if len(changes) == 0 {
			return nil
		}
		return r.append(ctx, tx, contract.EventAccountUpdated, id, contract.AccountUpdated{AccountID: id, Changes: changes})
	})
	return out, err
}

func (r *AccountRepository) SetActive(ctx context.Context, id uuid.UUID, active bool) (domain.Account, error) {
	var out domain.Account
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		if !active {
			if err := lockAdminRole(ctx, q); err != nil {
				return err
			}
		}
		cur, err := accountOrNotFound(q.GetAccountForUpdate(ctx, id))
		if err != nil {
			return err
		}
		if cur.Active == active { // không đổi gì, không event
			out = cur
			return nil
		}
		row, err := q.SetAccountActive(ctx, db.SetAccountActiveParams{ID: id, Active: active})
		if err != nil {
			return fmt.Errorf("identity: set active: %w", err)
		}
		out = toAccount(row)
		if active {
			return r.append(ctx, tx, contract.EventAccountEnabled, id, contract.AccountEnabled{AccountID: id})
		}
		roles, err := q.AccountRoleIDs(ctx, id)
		if err != nil {
			return fmt.Errorf("identity: account roles: %w", err)
		}
		if err := ensureAdminRemains(ctx, q, slices.Contains(roles, domain.AdministratorRoleID)); err != nil {
			return err
		}
		// Khoá account: đăng xuất mọi thiết bị ngay, không đợi refresh token hết hạn
		if _, err := q.RevokeAccountFamilies(ctx, db.RevokeAccountFamiliesParams{
			AccountID: id, Reason: ptr(string(domain.RevokeAdmin)),
		}); err != nil {
			return fmt.Errorf("identity: revoke sessions: %w", err)
		}
		// Link đang chờ (lời mời, đặt lại) cũng chết; mở khoá không hồi lại chúng
		if err := q.DeleteAccountPasswordTokens(ctx, id); err != nil {
			return fmt.Errorf("identity: delete password tokens: %w", err)
		}
		return r.append(ctx, tx, contract.EventAccountDisabled, id, contract.AccountDisabled{AccountID: id})
	})
	return out, err
}

func (r *AccountRepository) SetPassword(ctx context.Context, id uuid.UUID, hash string, keepFamily *uuid.UUID) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		if _, err := accountOrNotFound(q.GetAccountForUpdate(ctx, id)); err != nil {
			return err
		}
		if err := q.SetAccountPassword(ctx, db.SetAccountPasswordParams{ID: id, PasswordHash: &hash}); err != nil {
			return fmt.Errorf("identity: set password: %w", err)
		}
		if _, err := q.RevokeAccountFamilies(ctx, db.RevokeAccountFamiliesParams{
			AccountID: id, Reason: ptr(string(domain.RevokePasswordChange)), Keep: keepFamily,
		}); err != nil {
			return fmt.Errorf("identity: revoke sessions: %w", err)
		}
		return nil
	})
}

func (r *AccountRepository) ReplaceRoles(ctx context.Context, id uuid.UUID, roleIDs []uuid.UUID) (domain.Account, error) {
	roleIDs = uniqueIDs(roleIDs)
	var out domain.Account
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		if err := lockAdminRole(ctx, q); err != nil {
			return err
		}
		cur, err := accountOrNotFound(q.GetAccountForUpdate(ctx, id))
		if err != nil {
			return err
		}
		out = cur
		before, err := q.AccountRoleIDs(ctx, id)
		if err != nil {
			return fmt.Errorf("identity: account roles: %w", err)
		}
		before = uniqueIDs(before)
		if err := q.DeleteAccountRoles(ctx, id); err != nil {
			return fmt.Errorf("identity: clear roles: %w", err)
		}
		if err := insertRoles(ctx, q, id, roleIDs); err != nil {
			return err
		}
		if slices.Equal(before, roleIDs) {
			return nil
		}
		lostAdmin := cur.Active && slices.Contains(before, domain.AdministratorRoleID) &&
			!slices.Contains(roleIDs, domain.AdministratorRoleID)
		if err := ensureAdminRemains(ctx, q, lostAdmin); err != nil {
			return err
		}
		return r.append(ctx, tx, contract.EventRolesAssigned, id, contract.RolesAssigned{
			AccountID: id, From: before, To: roleIDs,
		})
	})
	return out, err
}

func (r *AccountRepository) Roles(ctx context.Context, id uuid.UUID) ([]domain.Role, error) {
	ids, err := r.q.AccountRoleIDs(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("identity: account roles: %w", err)
	}
	return loadRoles(ctx, r.q, ids)
}

func (r *AccountRepository) Permissions(ctx context.Context, id uuid.UUID) ([]string, error) {
	perms, err := r.q.AccountPermissions(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("identity: account permissions: %w", err)
	}
	return perms, nil
}

func (r *AccountRepository) Count(ctx context.Context) (int64, error) {
	n, err := r.q.CountAllAccounts(ctx)
	if err != nil {
		return 0, fmt.Errorf("identity: count accounts: %w", err)
	}
	return n, nil
}

func (r *AccountRepository) append(ctx context.Context, tx pgx.Tx, typ string, id uuid.UUID, payload any) error {
	return appendAccountEvent(ctx, tx, r.outbox, typ, id, payload)
}

// insertRoles gán role cho account; role không tồn tại là ErrUnknownRoles
func insertRoles(ctx context.Context, q *db.Queries, accountID uuid.UUID, roleIDs []uuid.UUID) error {
	if len(roleIDs) == 0 {
		return nil
	}
	found, err := q.LockActiveRolesByIDs(ctx, roleIDs)
	if err != nil {
		return fmt.Errorf("identity: lock roles: %w", err)
	}
	if len(found) != len(roleIDs) {
		return domain.ErrUnknownRoles
	}
	for _, rid := range roleIDs {
		if err := q.InsertAccountRole(ctx, db.InsertAccountRoleParams{AccountID: accountID, RoleID: rid}); err != nil {
			return fmt.Errorf("identity: assign role: %w", err)
		}
	}
	return nil
}

// loadRoles đọc các role cùng quyền của chúng, không N+1
func loadRoles(ctx context.Context, q *db.Queries, ids []uuid.UUID) ([]domain.Role, error) {
	if len(ids) == 0 {
		return []domain.Role{}, nil
	}
	rows, err := q.GetRolesByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("identity: load roles: %w", err)
	}
	return withPermissions(ctx, q, rows)
}

func withPermissions(ctx context.Context, q *db.Queries, rows []db.IdentityRole) ([]domain.Role, error) {
	ids := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	perms, err := q.RolesPermissions(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("identity: role permissions: %w", err)
	}
	byRole := map[uuid.UUID][]string{}
	for _, p := range perms {
		byRole[p.RoleID] = append(byRole[p.RoleID], p.Permission)
	}
	out := make([]domain.Role, len(rows))
	for i, r := range rows {
		out[i] = toRole(r, byRole[r.ID])
	}
	return out, nil
}

func accountOrNotFound(row db.IdentityAccount, err error) (domain.Account, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Account{}, domain.ErrAccountNotFound
	}
	if err != nil {
		return domain.Account{}, fmt.Errorf("identity: get account: %w", err)
	}
	return toAccount(row), nil
}

func (r *AccountRepository) StatusCounts(ctx context.Context, f domain.AccountFilter) (map[domain.AccountStatus]int64, error) {
	var q *string
	if f.Query != "" {
		esc := likeEscaper.Replace(f.Query)
		q = &esc
	}
	rows, err := r.q.CountAccountsByStatus(ctx, db.CountAccountsByStatusParams{Q: q, RoleID: f.RoleID})
	if err != nil {
		return nil, fmt.Errorf("identity: count accounts by status: %w", err)
	}
	out := make(map[domain.AccountStatus]int64, len(rows))
	for _, row := range rows {
		out[domain.AccountStatus(row.Status)] = row.N
	}
	return out, nil
}

func (r *AccountRepository) RolesOf(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]domain.Role, error) {
	rows, err := r.q.ListRolesOfAccounts(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("identity: roles of accounts: %w", err)
	}
	out := make(map[uuid.UUID][]domain.Role)
	for _, row := range rows {
		out[row.AccountID] = append(out[row.AccountID], domain.Role{ID: row.ID, Name: row.Name})
	}
	return out, nil
}

func (r *AccountRepository) InviteExpiries(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]time.Time, error) {
	rows, err := r.q.ListInviteExpiries(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("identity: invite expiries: %w", err)
	}
	out := make(map[uuid.UUID]time.Time, len(rows))
	for _, row := range rows {
		out[row.AccountID] = row.ExpiresAt
	}
	return out, nil
}

func (r *AccountRepository) LiveSessions(ctx context.Context, id uuid.UUID) (int64, error) {
	n, err := r.q.CountLiveFamilies(ctx, id)
	if err != nil {
		return 0, fmt.Errorf("identity: count sessions: %w", err)
	}
	return n, nil
}

func (r *AccountRepository) SignOutEverywhere(ctx context.Context, id uuid.UUID) (int64, error) {
	var n int64
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		// khoá account trước family, cùng thứ tự với SetActive
		if _, err := accountOrNotFound(q.GetAccountForUpdate(ctx, id)); err != nil {
			return err
		}
		var err error
		n, err = q.RevokeAccountFamilies(ctx, db.RevokeAccountFamiliesParams{AccountID: id, Reason: ptr(string(domain.RevokeAdmin))})
		if err != nil {
			return fmt.Errorf("identity: revoke sessions: %w", err)
		}
		if n == 0 {
			return nil
		}
		return r.append(ctx, tx, contract.EventAccountSignedOut, id, contract.AccountSignedOut{AccountID: id, Sessions: n})
	})
	return n, err
}

func ptr[T any](v T) *T { return &v }

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// lockAdminRole xếp hàng mọi thao tác có thể làm mất một admin; gọi trước khi
// khoá hàng account (cùng một thứ tự ở mọi nơi, tránh deadlock)
func lockAdminRole(ctx context.Context, q *db.Queries) error {
	if _, err := q.LockRole(ctx, domain.AdministratorRoleID); err != nil {
		return fmt.Errorf("identity: lock administrator role: %w", err)
	}
	return nil
}

// ensureAdminRemains: account vừa bị khoá hoặc vừa mất role Administrator
// trong tx này. Nếu nó từng là admin đang hoạt động thì vẫn phải còn ít nhất
// một Administrator đang hoạt động; account thường không bị chặn.
func ensureAdminRemains(ctx context.Context, q *db.Queries, wasAdmin bool) error {
	if !wasAdmin {
		return nil
	}
	n, err := q.CountActiveAccountsWithRole(ctx, domain.AdministratorRoleID)
	if err != nil {
		return fmt.Errorf("identity: count administrators: %w", err)
	}
	if n == 0 {
		return domain.ErrLockout
	}
	return nil
}
