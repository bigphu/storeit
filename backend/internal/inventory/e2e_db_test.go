package inventory_test

import (
	"encoding/base64"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"storeit/internal/inventory"
	"storeit/internal/inventory/domain"
	"storeit/internal/platform/database/dbtest"
	"storeit/internal/platform/events"
	"storeit/internal/platform/jobs"
	"storeit/internal/platform/jwt"
	"storeit/internal/platform/server"
)

// newModuleApp dựng app qua inventory.New như cmd/server
func newModuleApp(t *testing.T) (*app, *inventory.Module) {
	t.Helper()
	pool := dbtest.Pool(t)
	client, err := jobs.NewInsertClient(pool, nil)
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := jwt.New(jwt.Config{
		Keys:      "v1:" + base64.StdEncoding.EncodeToString([]byte(strings.Repeat("m", 32))),
		ActiveKID: "v1", Issuer: "storeit", Audience: "storeit-api", AccessTokenTTL: 15 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	m, err := inventory.New(inventory.Deps{Pool: pool, Outbox: events.NewOutbox(events.NewRegistry(), client), Accounts: stubAccounts{}})
	if err != nil {
		t.Fatal(err)
	}
	srv := server.New(server.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := m.Mount(srv.Router(), tokens); err != nil {
		t.Fatal(err)
	}
	return &app{t: t, h: srv.Handler(), tokens: tokens}, m
}

// Luồng của Sprint 2: quản lý tạo loại Laptop có thuộc tính số (đơn vị GB), chọn
// một (OS) và ngày → nhân viên kho tạo tài sản, xem lại → thêm option Linux, đổi OS
// → đổi loại sang GENERAL (giá trị bỏ) → retire (ẩn khỏi danh sách) → khôi phục.
func TestEndToEnd(t *testing.T) {
	a, _ := newModuleApp(t)
	mgr := a.token(domain.PermAssetRead, domain.PermTypeManage, domain.PermStatusManage)
	officer := a.token(domain.PermAssetRead, domain.PermAssetManage)
	code := "E2E" + strings.ToUpper(uuid.NewString()[:6])

	typ := decode[typeDetail](t, a.do("POST", "/api/v1/asset-types", mgr, map[string]any{
		"code": code, "name": "Laptop " + code,
		"attributes": []map[string]any{
			{"key": "ram_gb", "label": "RAM", "data_type": "number", "unit": "GB", "is_required": true},
			{"key": "os", "label": "OS", "data_type": "select", "options": []string{"Windows", "macOS"}},
			{"key": "warranty_end", "label": "Warranty end", "data_type": "date"},
		},
	}), 201)
	osID := typ.Attributes[1].ID
	// Quản lý không tạo được tài sản; nhân viên kho không sửa được loại
	if rec := a.do("POST", "/api/v1/assets", mgr, map[string]any{"tag": "X", "name": "X", "asset_type_id": typ.ID}); rec.Code != 403 {
		t.Errorf("manager creates asset: %d", rec.Code)
	}
	if rec := a.do("POST", "/api/v1/asset-types/"+typ.ID+"/attributes", officer, map[string]any{"key": "k", "label": "K", "data_type": "text"}); rec.Code != 403 {
		t.Errorf("officer adds attribute: %d", rec.Code)
	}

	tag := "LAP-" + strings.ToUpper(uuid.NewString()[:6])
	created := decode[assetDetail](t, a.do("POST", "/api/v1/assets", officer, map[string]any{
		"tag": tag, "name": "ThinkPad", "asset_type_id": typ.ID,
		"attributes": map[string]any{"ram_gb": 32, "os": typ.Attributes[1].Options[0].ID, "warranty_end": "2027-06-30"},
	}), 201)
	got := decode[assetDetail](t, a.do("GET", "/api/v1/assets/"+created.ID, officer, nil), 200)
	if got.Attributes[0].Key != "ram_gb" || got.Attributes[0].Value != 32.0 || got.Attributes[0].Unit != "GB" ||
		got.Attributes[1].OptionLabel != "Windows" || got.Attributes[2].Value != "2027-06-30" {
		t.Fatalf("read back = %+v", got.Attributes)
	}

	linux := decode[struct{ ID string }](t, a.do("POST", "/api/v1/asset-types/"+typ.ID+"/attributes/"+osID+"/options", mgr, map[string]any{"label": "Linux"}), 201)
	upd := decode[assetDetail](t, a.do("PUT", "/api/v1/assets/"+created.ID, officer, map[string]any{
		"name": "ThinkPad", "asset_type_id": typ.ID, "version": got.Version,
		"attributes": map[string]any{"ram_gb": 32, "os": linux.ID},
	}), 200)
	if upd.Attributes[1].OptionLabel != "Linux" || upd.Attributes[2].Value != nil {
		t.Errorf("after update = %+v", upd.Attributes)
	}

	gen := decode[assetDetail](t, a.do("PUT", "/api/v1/assets/"+created.ID, officer, map[string]any{
		"name": "ThinkPad", "asset_type_id": domain.GeneralTypeID.String(), "version": upd.Version,
	}), 200)
	if gen.AssetType.Code != "GENERAL" || len(gen.Attributes) != 0 {
		t.Errorf("after type change = %+v", gen)
	}

	ret := decode[assetDetail](t, a.do("POST", "/api/v1/assets/"+created.ID+"/retire", officer, map[string]any{"version": gen.Version}), 200)
	type list struct{ Total int64 }
	if l := decode[list](t, a.do("GET", "/api/v1/assets?q="+tag, officer, nil), 200); l.Total != 0 {
		t.Errorf("retired asset listed: %+v", l)
	}
	rest := decode[assetDetail](t, a.do("POST", "/api/v1/assets/"+created.ID+"/restore", officer, map[string]any{"version": ret.Version}), 200)
	if l := decode[list](t, a.do("GET", "/api/v1/assets?q="+tag, officer, nil), 200); l.Total != 1 || rest.Status.Kind != "available" {
		t.Errorf("after restore: list %+v, status %s", l, rest.Status.Kind)
	}
}

func TestModule_APIDoc(t *testing.T) {
	_, m := newModuleApp(t)
	doc, err := m.APIDoc()
	if err != nil {
		t.Fatal(err)
	}
	if doc.Name != "inventory" {
		t.Errorf("name = %q", doc.Name)
	}
	for _, p := range []string{"/assets", "/assets/{assetID}", "/asset-types", "/asset-statuses"} {
		if doc.Spec.Paths.Find(p) == nil {
			t.Errorf("spec has no path %s", p)
		}
	}
	if _, err := inventory.New(inventory.Deps{}); err == nil {
		t.Error("New without Pool and Outbox = nil error")
	}
}
