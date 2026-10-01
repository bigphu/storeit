package repository_test

import (
	"context"
	"crypto/sha256"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"storeit/internal/identity/domain"
	"storeit/internal/identity/repository"
	"storeit/internal/platform/auth"
	"storeit/internal/platform/database/dbtest"
	"storeit/internal/platform/events"
	"storeit/internal/platform/jobs"
)

type repos struct {
	pool     *pgxpool.Pool
	accounts *repository.AccountRepository
	roles    *repository.RoleRepository
	sessions *repository.SessionRepository
}

func newRepos(t *testing.T) repos {
	t.Helper()
	pool := dbtest.Pool(t)
	client, err := jobs.NewInsertClient(pool, nil)
	if err != nil {
		t.Fatal(err)
	}
	outbox := events.NewOutbox(events.NewRegistry(), client)
	return repos{
		pool:     pool,
		accounts: repository.NewAccountRepository(pool, outbox),
		roles:    repository.NewRoleRepository(pool, outbox),
		sessions: repository.NewSessionRepository(pool),
	}
}

// actorCtx: ctx có actor để event ghi actor_id
func actorCtx(id uuid.UUID) context.Context {
	return auth.WithActor(context.Background(), auth.Actor{AccountID: id})
}

// newAccount tạo account với email ngẫu nhiên (DB test dùng chung giữa các test)
func newAccount(t *testing.T, r repos, roleIDs ...uuid.UUID) domain.Account {
	t.Helper()
	a, err := r.accounts.Create(context.Background(), domain.NewAccount{
		Email:        "u-" + uuid.NewString()[:8] + "@storeit.test",
		Name:         "Test User",
		PasswordHash: "$2a$04$hash",
		RoleIDs:      roleIDs,
	})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func hash(s string) []byte {
	h := sha256.Sum256([]byte(s))
	return h[:]
}

func countEvents(t *testing.T, r repos, typ string, aggregateID uuid.UUID) (n int, actor *uuid.UUID) {
	t.Helper()
	err := r.pool.QueryRow(context.Background(), `
		SELECT count(*), max(actor_id::text)::uuid FROM platform.events
		WHERE type = $1 AND aggregate_id = $2`, typ, aggregateID).Scan(&n, &actor)
	if err != nil {
		t.Fatal(err)
	}
	return n, actor
}
