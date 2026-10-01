package repository

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"storeit/internal/identity/contract"
	"storeit/internal/identity/domain"
	"storeit/internal/identity/repository/db"
	"storeit/internal/platform/database"
	"storeit/internal/platform/events"
)

// RoleRepository cài đặt domain.RoleRepository
type RoleRepository struct {
	pool   *pgxpool.Pool
	q      *db.Queries
	outbox *events.Outbox
}

var _ domain.RoleRepository = (*RoleRepository)(nil)

func NewRoleRepository(pool *pgxpool.Pool, outbox *events.Outbox) *RoleRepository {
	return &RoleRepository{pool: pool, q: db.New(pool), outbox: outbox}
}

func (r *RoleRepository) List(ctx context.Context) ([]domain.Role, error) {
	rows, err := r.q.ListRoles(ctx)
	if err != nil {
		return nil, fmt.Errorf("identity: list roles: %w", err)
	}
	return withPermissions(ctx, r.q, rows)
}

func (r *RoleRepository) Get(ctx context.Context, id uuid.UUID) (domain.Role, error) {
	return getRole(ctx, r.q, id)
}

func (r *RoleRepository) Create(ctx context.Context, in domain.Role) (domain.Role, error) {
	id, err := newID()
	if err != nil {
		return domain.Role{}, err
	}
	perms := uniqueStrings(in.Permissions)
	var out domain.Role
	err = database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		row, err := q.CreateRole(ctx, db.CreateRoleParams{ID: id, Name: in.Name, Description: in.Description})
		if err != nil {
			return roleWriteError(err)
		}
		if err := insertPermissions(ctx, q, id, perms); err != nil {
			return err
		}
		out = toRole(row, perms)
		return r.append(ctx, tx, contract.EventRoleCreated, id, contract.RoleCreated{
			RoleID: id, Name: row.Name, Description: row.Description, Permissions: out.Permissions,
		})
	})
	return out, err
}

func (r *RoleRepository) Update(ctx context.Context, id uuid.UUID, name, description string) (domain.Role, error) {
	var out domain.Role
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		cur, err := getRole(ctx, q, id)
		if err != nil {
			return err
		}
		var changes []contract.FieldChange
		if name != cur.Name {
			changes = append(changes, contract.FieldChange{Field: "name", From: cur.Name, To: name})
		}
		if description != cur.Description {
			changes = append(changes, contract.FieldChange{Field: "description", From: cur.Description, To: description})
		}
		if len(changes) == 0 {
			out = cur
			return nil
		}
		row, err := q.UpdateRole(ctx, db.UpdateRoleParams{ID: id, Name: name, Description: description})
		if err != nil {
			return roleWriteError(err)
		}
		out = toRole(row, cur.Permissions)
		return r.append(ctx, tx, contract.EventRoleUpdated, id, contract.RoleUpdated{RoleID: id, Changes: changes})
	})
	return out, err
}

func (r *RoleRepository) ReplacePermissions(ctx context.Context, id uuid.UUID, perms []string) (domain.Role, error) {
	perms = uniqueStrings(perms)
	var out domain.Role
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		cur, err := getRole(ctx, q, id)
		if err != nil {
			return err
		}
		if err := q.DeleteRolePermissions(ctx, id); err != nil {
			return fmt.Errorf("identity: clear role permissions: %w", err)
		}
		if err := insertPermissions(ctx, q, id, perms); err != nil {
			return err
		}
		out = cur
		out.Permissions = slices.Clip(perms)
		if out.Permissions == nil {
			out.Permissions = []string{}
		}
		if slices.Equal(cur.Permissions, perms) {
			return nil
		}
		if err := q.TouchRole(ctx, id); err != nil {
			return fmt.Errorf("identity: touch role: %w", err)
		}
		return r.append(ctx, tx, contract.EventRolePermissionsUpdated, id, contract.RolePermissionsUpdated{
			RoleID: id, From: cur.Permissions, To: out.Permissions,
		})
	})
	return out, err
}

func (r *RoleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		cur, err := getRole(ctx, q, id)
		if err != nil {
			return err
		}
		if _, err := q.DeleteRole(ctx, id); err != nil {
			// Còn account giữ role này (service đã kiểm tra, đây là lớp chặn cuối)
			if code, _ := pgCode(err); code == codeForeignKeyViolation {
				return domain.ErrRoleInUse
			}
			return fmt.Errorf("identity: delete role: %w", err)
		}
		return r.append(ctx, tx, contract.EventRoleDeleted, id, contract.RoleDeleted{RoleID: id, Name: cur.Name})
	})
}

func (r *RoleRepository) CountAssignments(ctx context.Context, id uuid.UUID) (int64, error) {
	n, err := r.q.CountRoleAssignments(ctx, id)
	if err != nil {
		return 0, fmt.Errorf("identity: count role assignments: %w", err)
	}
	return n, nil
}

func (r *RoleRepository) Permissions(ctx context.Context) ([]domain.Permission, error) {
	rows, err := r.q.ListPermissions(ctx)
	if err != nil {
		return nil, fmt.Errorf("identity: list permissions: %w", err)
	}
	out := make([]domain.Permission, len(rows))
	for i, p := range rows {
		out[i] = domain.Permission{Code: p.Code, Description: p.Description}
	}
	return out, nil
}

func (r *RoleRepository) append(ctx context.Context, tx pgx.Tx, typ string, id uuid.UUID, payload any) error {
	e, err := events.New(typ, contract.AggregateRole, id, payload)
	if err != nil {
		return err
	}
	return r.outbox.Append(ctx, tx, e)
}

func getRole(ctx context.Context, q *db.Queries, id uuid.UUID) (domain.Role, error) {
	row, err := q.GetRole(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Role{}, domain.ErrRoleNotFound
	}
	if err != nil {
		return domain.Role{}, fmt.Errorf("identity: get role: %w", err)
	}
	perms, err := q.RolePermissions(ctx, id)
	if err != nil {
		return domain.Role{}, fmt.Errorf("identity: role permissions: %w", err)
	}
	return toRole(row, perms), nil
}

// insertPermissions: mã quyền không có trong danh mục là ErrUnknownPermissions
// (FK role_permissions → permissions)
func insertPermissions(ctx context.Context, q *db.Queries, roleID uuid.UUID, perms []string) error {
	for _, p := range perms {
		if err := q.InsertRolePermission(ctx, db.InsertRolePermissionParams{RoleID: roleID, Permission: p}); err != nil {
			if code, _ := pgCode(err); code == codeForeignKeyViolation {
				return domain.ErrUnknownPermissions
			}
			return fmt.Errorf("identity: grant permission: %w", err)
		}
	}
	return nil
}

func roleWriteError(err error) error {
	if code, constraint := pgCode(err); code == codeUniqueViolation && constraint == "roles_name_key" {
		return domain.ErrRoleNameTaken
	}
	return fmt.Errorf("identity: write role: %w", err)
}
