// Package identity lo đăng nhập và phân quyền: tài khoản, mật khẩu, role, phiên
// đăng nhập với refresh token xoay vòng.
//
// main dựng module một lần và dùng các phần nó cần:
//
//	m, err := identity.New(identity.Deps{Pool: pool, Tokens: tokens, Outbox: outbox, Config: cfg.Identity})
//	err = m.Bootstrap(ctx)                     // cmd/server: Administrator đầu tiên
//	err = m.Mount(srv.Router())                // cmd/server: route /api/v1/...
//	events.RegisterWorker(..., m.LoadActor)    // cmd/worker
//	m.RegisterWorkers(workers)                 // cmd/worker, cùng identity.PeriodicJobs()
//
// Module khác chỉ dùng identity qua identity/contract (m.AccountReader()).
package identity

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"storeit/internal/identity/contract"
	"storeit/internal/identity/handler"
	"storeit/internal/identity/job"
	"storeit/internal/identity/repository"
	"storeit/internal/identity/service"
	"storeit/internal/identity/worker"
	"storeit/internal/platform/auth"
	"storeit/internal/platform/events"
	"storeit/internal/platform/jobs"
	"storeit/internal/platform/jwt"
	"storeit/internal/platform/logger"
)

type Deps struct {
	Pool *pgxpool.Pool
	// Tokens chỉ cần cho API (đăng nhập, Mount); cmd/worker để nil
	Tokens *jwt.Provider
	Outbox *events.Outbox
	// Jobs xếp job gửi thư trong transaction phát link đặt mật khẩu
	Jobs   jobs.Enqueuer
	Config Config
}

type Module struct {
	cfg     Config
	tokens  *jwt.Provider
	svc     *service.Service
	handler *handler.Handler
}

func New(d Deps) (*Module, error) {
	if err := d.Config.Validate(); err != nil {
		return nil, err
	}
	if d.Pool == nil || d.Outbox == nil {
		return nil, fmt.Errorf("identity: Pool and Outbox are required")
	}
	cfg := d.Config.withDefaults()
	var tokens service.TokenIssuer
	if d.Tokens != nil { // interface nil thật, không phải *jwt.Provider nil
		tokens = d.Tokens
	}
	svc := service.New(service.Deps{
		Accounts: repository.NewAccountRepository(d.Pool, d.Outbox, d.Jobs),
		Roles:    repository.NewRoleRepository(d.Pool, d.Outbox),
		Sessions: repository.NewSessionRepository(d.Pool),
		Hasher:   service.NewBcrypt(0),
		Tokens:   tokens,
		Settings: service.Settings{
			SlidingTTL:  cfg.RefreshSlidingTTL,
			AbsoluteTTL: cfg.RefreshAbsoluteTTL,
			Grace:       cfg.RefreshGracePeriod,
			Retention:   cfg.RefreshRetention,
		},
	})
	return &Module{
		cfg:     cfg,
		tokens:  d.Tokens,
		svc:     svc,
		handler: handler.New(svc, handler.CookieSettings{Secure: cfg.CookieSecure, Domain: cfg.CookieDomain}),
	}, nil
}

// Mount gắn route /api/v1/... của identity lên router gốc
func (m *Module) Mount(r chi.Router) error {
	if m.tokens == nil {
		return fmt.Errorf("identity: Mount needs Deps.Tokens")
	}
	return m.handler.Mount(r, m.tokens)
}

// Bootstrap tạo Administrator đầu tiên từ ADMIN_EMAIL/ADMIN_PASSWORD nếu chưa
// có account nào. Không đặt hai biến đó thì bỏ qua.
func (m *Module) Bootstrap(ctx context.Context) error {
	if m.cfg.AdminEmail == "" {
		return nil
	}
	created, err := m.svc.Bootstrap(ctx, m.cfg.AdminEmail, m.cfg.AdminPassword)
	if err != nil {
		return fmt.Errorf("identity: bootstrap admin: %w", err)
	}
	if created {
		logger.FromContext(ctx).InfoContext(ctx, "created bootstrap administrator",
			slog.String("email", m.cfg.AdminEmail))
	}
	return nil
}

// LoadActor là jobs.ActorLoader cho events.RegisterWorker và job của module khác
func (m *Module) LoadActor(ctx context.Context, id uuid.UUID) (auth.Actor, error) {
	return m.svc.LoadActor(ctx, id)
}

// AccountReader cho module khác (qua contract)
func (m *Module) AccountReader() contract.AccountReader { return m.svc.AccountReader() }

// Service cho test và worker
func (m *Module) Service() *service.Service { return m.svc }

// RegisterWorkers thêm worker của identity vào workers của cmd/worker
func (m *Module) RegisterWorkers(workers *river.Workers) {
	river.AddWorker(workers, worker.NewPruneSessions(m.svc))
}

// PeriodicJobs: dọn phiên đã chết mỗi giờ
func PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{
		river.NewPeriodicJob(river.PeriodicInterval(time.Hour),
			func() (river.JobArgs, *river.InsertOpts) { return job.PruneSessionsArgs{}, nil },
			&river.PeriodicJobOpts{RunOnStart: true}),
	}
}
