package identity_test

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"storeit/internal/identity"
	"storeit/internal/identity/domain"
	"storeit/internal/identity/job"
	"storeit/internal/identity/service"
	"storeit/internal/identity/worker"
	"storeit/internal/platform/database/dbtest"
	"storeit/internal/platform/events"
	"storeit/internal/platform/jobs"
	"storeit/internal/platform/jwt"
	"storeit/internal/platform/mail"
	"storeit/internal/platform/web"
)

func newModule(t *testing.T, cfg identity.Config) *identity.Module {
	t.Helper()
	return newModuleWith(t, cfg, nil)
}

// newModuleWith dựng module với sender cho trước (nil: như cmd/server, không gửi thư)
func newModuleWith(t *testing.T, cfg identity.Config, sender mail.Sender) *identity.Module {
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
		Pool: pool, Tokens: tokens, Outbox: events.NewOutbox(events.NewRegistry(), client),
		Jobs: jobs.NewRiver(client), Mail: sender, Config: cfg,
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

	if len(identity.PeriodicJobs()) != 1 {
		t.Errorf("periodic jobs = %d, want 1", len(identity.PeriodicJobs()))
	}
}

// Worker cần sender; API thì không. Thiếu job queue là lỗi dựng module.
func TestModule_RequiresJobsAndMail(t *testing.T) {
	if err := newModule(t, identity.Config{}).RegisterWorkers(river.NewWorkers()); err == nil {
		t.Error("RegisterWorkers without Deps.Mail = nil error")
	}
	sender, err := mail.New(mail.Config{Transport: mail.TransportLog, From: "no-reply@storeit.test"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := newModuleWith(t, identity.Config{}, sender).RegisterWorkers(river.NewWorkers()); err != nil {
		t.Errorf("RegisterWorkers with a sender: %v", err)
	}
	if _, err := identity.New(identity.Deps{Pool: dbtest.Pool(t), Outbox: &events.Outbox{}}); err == nil {
		t.Error("New without Jobs = nil error")
	}
}

// Spec cho trang tài liệu: đủ route mới, ref sang api/common.yaml đã resolve
func TestModule_APIDoc(t *testing.T) {
	doc, err := newModule(t, identity.Config{}).APIDoc()
	if err != nil {
		t.Fatal(err)
	}
	if doc.Name != "identity" || doc.Spec == nil {
		t.Fatalf("doc = %q, %v", doc.Name, doc.Spec)
	}
	for _, p := range []string{"/auth/login", "/auth/password/forgot", "/accounts/{accountID}/invitation"} {
		if doc.Spec.Paths.Find(p) == nil {
			t.Errorf("spec has no path %s", p)
		}
	}
	r := chi.NewRouter()
	if err := web.MountDocs(r, doc); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/docs/identity.json", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"bearerAuth"`) || !strings.Contains(rec.Body.String(), `"Problem"`) {
		t.Errorf("identity.json: %d, want the spec with bearerAuth and the shared Problem schema", rec.Code)
	}
}

// SeedAccount (chỉ cmd/seed dùng): tạo account có mật khẩu và role; email đã có
// thì trả id cũ, không lỗi; mật khẩu yếu thì lỗi
func TestModule_SeedAccount(t *testing.T) {
	m := newModule(t, identity.Config{})
	ctx := context.Background()
	email := "seed-" + uuid.NewString()[:8] + "@storeit.test"

	id, err := m.SeedAccount(ctx, email, "Phạm Thu Hà", "correct-horse-battery", []uuid.UUID{domain.EmployeeRoleID})
	if err != nil {
		t.Fatal(err)
	}
	again, err := m.SeedAccount(ctx, strings.ToUpper(email), "Khác", "correct-horse-battery", nil)
	if err != nil || again != id {
		t.Fatalf("second call = %s, %v; want the same id %s", again, err, id)
	}
	sess, err := m.Service().Login(ctx, email, "correct-horse-battery", service.Device{})
	if err != nil {
		t.Fatalf("login with the seeded password: %v", err)
	}
	if sess.Account.Name != "Phạm Thu Hà" {
		t.Errorf("name = %q", sess.Account.Name)
	}
	if _, err := m.SeedAccount(ctx, "weak-"+email, "X", "short", nil); err == nil {
		t.Error("weak password accepted")
	}
}
