// Package inventory quản lý tài sản: loại tài sản và thuộc tính riêng, status,
// tài sản (tạo, xem, sửa, retire, tìm kiếm). Sau này: mượn/trả, thay thế,
// cha/con, import/export Excel.
//
// main dựng module một lần:
//
//	m, err := inventory.New(inventory.Deps{Pool: pool, Outbox: outbox, Jobs: jobs.NewRiver(insertClient), Accounts: identityMod.AccountReader(), Config: cfg.Inventory})
//	err = m.Mount(srv.Router(), tokens)   // cmd/server: route /api/v1/...
//	doc, err := m.APIDoc()                // cmd/server, HTTP_API_DOCS: web.MountDocs
//	err = m.RegisterWorkers(workers)      // cmd/worker (cần Deps.Mail), cùng inventory.PeriodicJobs()
//
// Module khác chỉ dùng inventory qua inventory/contract (event).
package inventory

import (
	"fmt"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	idcontract "storeit/internal/identity/contract"
	"storeit/internal/inventory/handler"
	"storeit/internal/inventory/job"
	"storeit/internal/inventory/repository"
	"storeit/internal/inventory/service"
	"storeit/internal/inventory/worker"
	"storeit/internal/platform/events"
	"storeit/internal/platform/jobs"
	"storeit/internal/platform/jwt"
	"storeit/internal/platform/mail"
	"storeit/internal/platform/web"
)

type Deps struct {
	Pool   *pgxpool.Pool
	Outbox *events.Outbox
	// Jobs xếp thư của việc mượn trong cùng transaction (US-10)
	Jobs     jobs.Enqueuer
	Accounts idcontract.AccountReader // tên chủ profile export, người mượn
	// Mail gửi thư của việc mượn; chỉ cmd/worker cần (RegisterWorkers)
	Mail   mail.Sender
	Config Config
}

type Module struct {
	svc     *service.Service
	handler *handler.Handler
	mail    mail.Sender
}

func New(d Deps) (*Module, error) {
	if d.Pool == nil || d.Outbox == nil || d.Jobs == nil || d.Accounts == nil {
		return nil, fmt.Errorf("inventory: Pool, Outbox, Jobs and Accounts are required")
	}
	assets := repository.NewAssetRepository(d.Pool, d.Outbox)
	svc := service.New(service.Deps{
		Types:    repository.NewTypeRepository(d.Pool, d.Outbox),
		Statuses: repository.NewStatusRepository(d.Pool, d.Outbox),
		Assets:   assets,
		Events:   assets,

		Profiles:      repository.NewExportProfileRepository(d.Pool, d.Outbox),
		Accounts:      d.Accounts,
		ExportMaxRows: d.Config.ExportMaxRows,

		Borrowing: repository.NewBorrowingRepository(d.Pool, d.Outbox, d.Jobs),
		Location:  d.Config.Location(),
		Mail:      d.Mail,
		AppURL:    d.Config.appURL(),
	})
	return &Module{svc: svc, handler: handler.New(svc), mail: d.Mail}, nil
}

// RegisterWorkers gắn worker của inventory (thư mượn tài sản, quét quá hạn); cần Deps.Mail
func (m *Module) RegisterWorkers(workers *river.Workers) error {
	if m.mail == nil {
		return fmt.Errorf("inventory: RegisterWorkers needs Deps.Mail")
	}
	river.AddWorker(workers, worker.NewBorrowingEmail(m.svc))
	river.AddWorker(workers, worker.NewOverdueScan(m.svc))
	return nil
}

// PeriodicJobs: quét khoản mượn quá hạn mỗi giờ (chỉ xếp thư từ 08:00 giờ ứng dụng)
func PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{
		river.NewPeriodicJob(river.PeriodicInterval(time.Hour),
			func() (river.JobArgs, *river.InsertOpts) { return job.OverdueScanArgs{}, nil },
			&river.PeriodicJobOpts{RunOnStart: true}),
	}
}

// Mount gắn route /api/v1/... của inventory lên router gốc; mọi route cần access token
func (m *Module) Mount(r chi.Router, tokens *jwt.Provider) error {
	if tokens == nil {
		return fmt.Errorf("inventory: Mount needs a token provider")
	}
	return m.handler.Mount(r, tokens)
}

// APIDoc là spec của inventory cho trang tài liệu API (web.MountDocs)
func (m *Module) APIDoc() (web.APIDoc, error) {
	spec, err := handler.Spec()
	if err != nil {
		return web.APIDoc{}, fmt.Errorf("inventory: load openapi spec: %w", err)
	}
	return web.APIDoc{Name: "inventory", Spec: spec}, nil
}

// Service cho test và cmd/seed (dữ liệu demo)
func (m *Module) Service() *service.Service { return m.svc }
