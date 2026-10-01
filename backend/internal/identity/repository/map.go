package repository

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"storeit/internal/identity/domain"
	"storeit/internal/identity/repository/db"
)

func toAccount(a db.IdentityAccount) domain.Account {
	return domain.Account{
		ID:           a.ID,
		Email:        a.Email,
		Name:         a.Name,
		PasswordHash: a.PasswordHash,
		MemberID:     a.MemberID,
		Active:       a.Active,
		Version:      a.Version,
		CreatedAt:    a.CreatedAt,
		UpdatedAt:    a.UpdatedAt,
	}
}

func toRole(r db.IdentityRole, perms []string) domain.Role {
	if perms == nil {
		perms = []string{}
	}
	return domain.Role{
		ID:          r.ID,
		Name:        r.Name,
		Description: r.Description,
		IsSystem:    r.IsSystem,
		Permissions: perms,
	}
}

// pgCode trả mã lỗi Postgres và tên constraint, rỗng nếu không phải lỗi Postgres
func pgCode(err error) (code, constraint string) {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return pe.Code, pe.ConstraintName
	}
	return "", ""
}

const (
	codeUniqueViolation     = "23505"
	codeForeignKeyViolation = "23503"
)

// uniqueIDs bỏ trùng và sắp xếp, để insert không vi phạm khoá chính và event
// so sánh được trước/sau
func uniqueIDs(ids []uuid.UUID) []uuid.UUID {
	out := slices.Clone(ids)
	slices.SortFunc(out, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
	return slices.Compact(out)
}

func uniqueStrings(s []string) []string {
	out := slices.Clone(s)
	slices.Sort(out)
	return slices.Compact(out)
}

func newID() (uuid.UUID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, fmt.Errorf("identity: new id: %w", err)
	}
	return id, nil
}
