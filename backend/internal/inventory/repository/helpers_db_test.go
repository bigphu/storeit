package repository_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"storeit/internal/inventory/domain"
	"storeit/internal/inventory/repository"
	"storeit/internal/platform/auth"
	"storeit/internal/platform/database/dbtest"
	"storeit/internal/platform/events"
	"storeit/internal/platform/jobs"
)

type repos struct {
	pool     *pgxpool.Pool
	types    *repository.TypeRepository
	statuses *repository.StatusRepository
	assets   *repository.AssetRepository
	profiles *repository.ExportProfileRepository
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
		types:    repository.NewTypeRepository(pool, outbox),
		statuses: repository.NewStatusRepository(pool, outbox),
		assets:   repository.NewAssetRepository(pool, outbox),
		profiles: repository.NewExportProfileRepository(pool, outbox),
	}
}

func actorCtx() context.Context {
	return auth.WithActor(context.Background(), auth.Actor{AccountID: uuid.New()})
}

// uniq trả hậu tố ngẫu nhiên viết hoa (DB test dùng chung giữa các test)
func uniq() string { return strings.ToUpper(uuid.NewString()[:8]) }

// laptop tạo một loại có đủ năm kiểu thuộc tính; trả loại đã đọc lại (Get)
func laptop(t *testing.T, r repos) domain.AssetType {
	t.Helper()
	u := uniq()
	created, err := r.types.Create(actorCtx(), domain.NewAssetType{
		Code: "LAP" + u, Name: "Laptop " + u, Description: "Portable computers",
		Attributes: []domain.NewAttribute{
			{Key: "serial", Label: "Serial", DataType: domain.TypeText, Required: true, Position: 1},
			{Key: "ram_gb", Label: "RAM", DataType: domain.TypeNumber, Unit: "GB", Position: 2},
			{Key: "warranty_end", Label: "Warranty end", DataType: domain.TypeDate, Position: 3},
			{Key: "has_dock", Label: "Dock", DataType: domain.TypeBoolean, Position: 4},
			{Key: "os", Label: "OS", DataType: domain.TypeSelect, Position: 5, Options: []string{"Windows", "macOS"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.types.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func attr(t *testing.T, typ domain.AssetType, key string) domain.Attribute {
	t.Helper()
	for _, a := range typ.Attributes {
		if a.Key == key {
			return a
		}
	}
	t.Fatalf("type %s has no attribute %s", typ.Code, key)
	return domain.Attribute{}
}

func countEvents(t *testing.T, r repos, typ string, aggregateID uuid.UUID) int {
	t.Helper()
	var n int
	if err := r.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM platform.events WHERE type = $1 AND aggregate_id = $2`, typ, aggregateID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// lastPayload trả payload JSON của event mới nhất loại typ cho aggregate
func lastPayload(t *testing.T, r repos, typ string, aggregateID uuid.UUID) string {
	t.Helper()
	var p string
	if err := r.pool.QueryRow(context.Background(), `
		SELECT payload::text FROM platform.events WHERE type = $1 AND aggregate_id = $2
		ORDER BY occurred_at DESC, id DESC LIMIT 1`, typ, aggregateID).Scan(&p); err != nil {
		t.Fatal(err)
	}
	return p
}

func ptr[T any](v T) *T { return &v }
