package identity_test

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"storeit/internal/identity"
	"storeit/internal/identity/job"
	"storeit/internal/identity/worker"
	"storeit/internal/platform/database/dbtest"
	"storeit/internal/platform/events"
	"storeit/internal/platform/jobs"
	"storeit/internal/platform/jwt"
)

func newModule(t *testing.T, cfg identity.Config) *identity.Module {
	t.Helper()
	pool := dbtest.Pool(t)
	client, err := jobs.NewInsertClient(pool, nil)
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := jwt.New(jwt.Config{
		Keys:      "v1:" + base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32))),
		ActiveKID: "v1", Issuer: "storeit", Audience: "storeit-api", AccessTokenTTL: 15 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	m, err := identity.New(identity.Deps{
		Pool: pool, Tokens: tokens, Outbox: events.NewOutbox(events.NewRegistry(), client), Config: cfg,
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// Bootstrap chỉ tạo admin khi bảng account trống. DB test dùng chung nên chỉ
// kiểm tra được: gọi hai lần không lỗi, và cấu hình rỗng thì bỏ qua.
func TestModule_Bootstrap(t *testing.T) {
	m := newModule(t, identity.Config{
		AdminEmail: "root-" + uuid.NewString()[:8] + "@storeit.test", AdminPassword: "correct-horse-battery",
	})
	for range 2 {
		if err := m.Bootstrap(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if err := newModule(t, identity.Config{}).Bootstrap(context.Background()); err != nil {
		t.Errorf("bootstrap without admin config: %v", err)
	}
}

func TestModule_PruneWorkerAndPeriodicJob(t *testing.T) {
	m := newModule(t, identity.Config{})

	w := worker.NewPruneSessions(m.Service())
	if err := w.Work(context.Background(), &river.Job[job.PruneSessionsArgs]{}); err != nil {
		t.Errorf("prune worker: %v", err)
	}

	workers := river.NewWorkers()
	m.RegisterWorkers(workers)
	if len(identity.PeriodicJobs()) != 1 {
		t.Errorf("periodic jobs = %d, want 1", len(identity.PeriodicJobs()))
	}
}
