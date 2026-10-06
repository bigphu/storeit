// Package inventory quản lý tài sản: loại tài sản và thuộc tính riêng, status,
// tài sản (tạo, xem, sửa, retire, tìm kiếm). Sau này: mượn/trả, thay thế,
// cha/con, import/export Excel.
//
// main dựng module một lần:
//
//	m, err := inventory.New(inventory.Deps{Pool: pool, Outbox: outbox, Accounts: identityMod.AccountReader(), Config: cfg.Inventory})
//	err = m.Mount(srv.Router(), tokens)   // cmd/server: route /api/v1/...
//	doc, err := m.APIDoc()                // cmd/server, HTTP_API_DOCS: web.MountDocs
//
// Module khác chỉ dùng inventory qua inventory/contract (event).
package inventory

import (
	"fmt"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	idcontract "storeit/internal/identity/contract"
	"storeit/internal/inventory/handler"
	"storeit/internal/inventory/repository"
	"storeit/internal/inventory/service"
	"storeit/internal/platform/events"
	"storeit/internal/platform/jwt"
	"storeit/internal/platform/web"
)

type Deps struct {
	Pool     *pgxpool.Pool
	Outbox   *events.Outbox
	Accounts idcontract.AccountReader // tên chủ profile export
	Config   Config
}

type Module struct {
	svc     *service.Service
	handler *handler.Handler
}

func New(d Deps) (*Module, error) {
	if d.Pool == nil || d.Outbox == nil || d.Accounts == nil {
		return nil, fmt.Errorf("inventory: Pool, Outbox and Accounts are required")
	}
	svc := service.New(service.Deps{
		Types:    repository.NewTypeRepository(d.Pool, d.Outbox),
		Statuses: repository.NewStatusRepository(d.Pool, d.Outbox),
		Assets:   repository.NewAssetRepository(d.Pool, d.Outbox),

		Profiles:      repository.NewExportProfileRepository(d.Pool, d.Outbox),
		Accounts:      d.Accounts,
		ExportMaxRows: d.Config.ExportMaxRows,
	})
	return &Module{svc: svc, handler: handler.New(svc)}, nil
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
