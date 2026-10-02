package inventory_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"storeit/internal/inventory/domain"
	"storeit/internal/inventory/handler"
	"storeit/internal/inventory/repository"
	"storeit/internal/inventory/service"
	"storeit/internal/platform/database/dbtest"
	"storeit/internal/platform/events"
	"storeit/internal/platform/jobs"
	"storeit/internal/platform/jwt"
	"storeit/internal/platform/server"
)

type app struct {
	t      *testing.T
	h      http.Handler
	tokens *jwt.Provider
}

func newApp(t *testing.T) *app {
	t.Helper()
	pool := dbtest.Pool(t)
	client, err := jobs.NewInsertClient(pool, nil)
	if err != nil {
		t.Fatal(err)
	}
	outbox := events.NewOutbox(events.NewRegistry(), client)
	tokens, err := jwt.New(jwt.Config{
		Keys:      "v1:" + base64.StdEncoding.EncodeToString([]byte(strings.Repeat("i", 32))),
		ActiveKID: "v1", Issuer: "storeit", Audience: "storeit-api", AccessTokenTTL: 15 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	svc := service.New(service.Deps{
		Types:    repository.NewTypeRepository(pool, outbox),
		Statuses: repository.NewStatusRepository(pool, outbox),
		Assets:   repository.NewAssetRepository(pool, outbox),
	})
	srv := server.New(server.Config{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := handler.New(svc).Mount(srv.Router(), tokens); err != nil {
		t.Fatal(err)
	}
	return &app{t: t, h: srv.Handler(), tokens: tokens}
}

// token phát access token mang đúng các quyền cho trước (không cần identity)
func (a *app) token(perms ...string) string {
	a.t.Helper()
	tok, err := a.tokens.Issue(uuid.New(), perms)
	if err != nil {
		a.t.Fatal(err)
	}
	return tok.Value
}

func (a *app) do(method, path, token string, body any) *httptest.ResponseRecorder {
	a.t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			a.t.Fatal(err)
		}
		r = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	a.h.ServeHTTP(rec, req)
	return rec
}

// decode đọc body JSON; sai mã trạng thái thì dừng test
func decode[T any](t *testing.T, rec *httptest.ResponseRecorder, want int) T {
	t.Helper()
	var v T
	if rec.Code != want {
		t.Fatalf("status %d, want %d: %s", rec.Code, want, rec.Body)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode: %v: %s", err, rec.Body)
	}
	return v
}

func problem(t *testing.T, rec *httptest.ResponseRecorder) (typ string, fields []string) {
	t.Helper()
	var p struct {
		Type   string
		Errors []struct{ Field, Detail string }
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &p)
	for _, e := range p.Errors {
		fields = append(fields, e.Field+": "+e.Detail)
	}
	return p.Type, fields
}

var allPerms = []string{domain.PermAssetRead, domain.PermAssetManage, domain.PermTypeManage, domain.PermStatusManage}

type typeDetail struct {
	ID         string
	Code       string
	Version    int32
	Attributes []struct {
		ID, Key, Label, Unit string
		DataType             string `json:"data_type"`
		Options              []struct{ ID, Label string }
	}
}

type assetDetail struct {
	ID, Tag, Name string
	Version       int32
	AssetType     struct{ ID, Code string } `json:"asset_type"`
	Status        struct{ ID, Name, Kind string }
	RetiredAt     *string `json:"retired_at"`
	Attributes    []struct {
		Key, Label, Unit string
		DataType         string `json:"data_type"`
		Value            any
		OptionLabel      string `json:"option_label"`
	}
}

func TestAssetLifecycleOverHTTP(t *testing.T) {
	a := newApp(t)
	tok := a.token(allPerms...)
	code := "LAP" + strings.ToUpper(uuid.NewString()[:6])

	typ := decode[typeDetail](t, a.do("POST", "/api/v1/asset-types", tok, map[string]any{
		"code": code, "name": "Laptop " + code,
		"attributes": []map[string]any{
			{"key": "serial", "label": "Serial", "data_type": "text", "is_required": true},
			{"key": "ram_gb", "label": "RAM", "data_type": "number", "unit": "GB"},
			{"key": "os", "label": "OS", "data_type": "select", "options": []string{"Windows", "macOS"}},
		},
	}), 201)
	if typ.Code != code || len(typ.Attributes) != 3 || typ.Attributes[1].Unit != "GB" || len(typ.Attributes[2].Options) != 2 {
		t.Fatalf("type = %+v", typ)
	}
	osID, windows := typ.Attributes[2].ID, typ.Attributes[2].Options[0].ID

	tag := "L-" + strings.ToUpper(uuid.NewString()[:8])
	created := decode[assetDetail](t, a.do("POST", "/api/v1/assets", tok, map[string]any{
		"tag": strings.ToLower(tag), "name": "Dell", "asset_type_id": typ.ID, "purchase_date": "2025-01-15",
		"attributes": map[string]any{"serial": "SN-1", "ram_gb": 15.6, "os": windows},
	}), 201)
	if created.Tag != tag || created.Status.Kind != "available" || created.AssetType.Code != code || len(created.Attributes) != 3 {
		t.Fatalf("created = %+v", created)
	}
	got := decode[assetDetail](t, a.do("GET", "/api/v1/assets/"+created.ID, tok, nil), 200)
	byKey := map[string]int{}
	for i, at := range got.Attributes {
		byKey[at.Key] = i
	}
	ram, os := got.Attributes[byKey["ram_gb"]], got.Attributes[byKey["os"]]
	if ram.Value != 15.6 || ram.Unit != "GB" || os.Value != windows || os.OptionLabel != "Windows" {
		t.Errorf("attributes = %+v", got.Attributes)
	}

	// Trùng tag (khác hoa thường): 409
	rec := a.do("POST", "/api/v1/assets", tok, map[string]any{"tag": tag, "name": "Dup", "asset_type_id": typ.ID, "attributes": map[string]any{"serial": "x"}})
	if typ, _ := problem(t, rec); rec.Code != 409 || typ != "/errors/tag-taken" {
		t.Errorf("duplicate tag: %d %s", rec.Code, rec.Body)
	}
	// Giá trị sai: 422 với từng field
	rec = a.do("POST", "/api/v1/assets", tok, map[string]any{"tag": "X-" + tag, "name": "Bad", "asset_type_id": typ.ID, "attributes": map[string]any{"ram_gb": "16"}})
	if pt, fields := problem(t, rec); rec.Code != 422 || pt != "/errors/invalid-attribute-values" || len(fields) != 2 {
		t.Errorf("invalid values: %d %s", rec.Code, rec.Body)
	}

	// PUT với version cũ: 409
	rec = a.do("PUT", "/api/v1/assets/"+created.ID, tok, map[string]any{"name": "x", "asset_type_id": typ.ID, "version": created.Version + 9, "attributes": map[string]any{"serial": "S"}})
	if pt, _ := problem(t, rec); rec.Code != 409 || pt != "/errors/asset-changed" {
		t.Errorf("stale version: %d %s", rec.Code, rec.Body)
	}
	// Đổi sang GENERAL: giá trị cũ bỏ
	upd := decode[assetDetail](t, a.do("PUT", "/api/v1/assets/"+created.ID, tok, map[string]any{
		"name": "Dell renamed", "asset_type_id": domain.GeneralTypeID.String(), "version": created.Version,
	}), 200)
	if upd.AssetType.Code != "GENERAL" || len(upd.Attributes) != 0 || upd.Version != created.Version+1 {
		t.Errorf("change type = %+v", upd)
	}

	// Retire, rồi sửa: 409; danh sách mặc định ẩn nó
	ret := decode[assetDetail](t, a.do("POST", "/api/v1/assets/"+created.ID+"/retire", tok, map[string]any{"reason": "Broken", "version": upd.Version}), 200)
	if ret.RetiredAt == nil || ret.Status.Kind != "retired" {
		t.Errorf("retired = %+v", ret)
	}
	rec = a.do("PUT", "/api/v1/assets/"+created.ID, tok, map[string]any{"name": "x", "asset_type_id": domain.GeneralTypeID.String(), "version": ret.Version})
	if pt, _ := problem(t, rec); rec.Code != 409 || pt != "/errors/asset-retired" {
		t.Errorf("edit retired: %d %s", rec.Code, rec.Body)
	}
	type list struct {
		Items []struct{ ID, Tag string }
		Total int64
	}
	if l := decode[list](t, a.do("GET", "/api/v1/assets?q="+tag, tok, nil), 200); l.Total != 0 {
		t.Errorf("retired asset in default list: %+v", l)
	}
	if l := decode[list](t, a.do("GET", "/api/v1/assets?q="+tag+"&include_retired=true&status_kind=retired", tok, nil), 200); l.Total != 1 {
		t.Errorf("retired asset with include_retired: %+v", l)
	}
	rest := decode[assetDetail](t, a.do("POST", "/api/v1/assets/"+created.ID+"/restore", tok, map[string]any{"version": ret.Version}), 200)
	if rest.RetiredAt != nil || rest.Status.Kind != "available" {
		t.Errorf("restored = %+v", rest)
	}

	// Thuộc tính và option: thêm option, gỡ thuộc tính
	opt := decode[struct{ ID, Label string }](t, a.do("POST", "/api/v1/asset-types/"+typ.ID+"/attributes/"+osID+"/options", tok, map[string]any{"label": "Linux"}), 201)
	if opt.Label != "Linux" {
		t.Errorf("option = %+v", opt)
	}
	if rec := a.do("DELETE", "/api/v1/asset-types/"+typ.ID+"/attributes/"+typ.Attributes[1].ID, tok, nil); rec.Code != 204 {
		t.Errorf("remove attribute: %d %s", rec.Code, rec.Body)
	}
	if rec := a.do("GET", "/api/v1/assets/"+uuid.NewString(), tok, nil); rec.Code != 404 {
		t.Errorf("unknown asset: %d", rec.Code)
	}
	if rec := a.do("GET", "/api/v1/assets?sort=color", tok, nil); rec.Code != 400 {
		t.Errorf("invalid sort: %d", rec.Code)
	}
}

func TestPermissionsOverHTTP(t *testing.T) {
	a := newApp(t)
	reader := a.token(domain.PermAssetRead)
	if rec := a.do("GET", "/api/v1/assets", "", nil); rec.Code != 401 {
		t.Errorf("no token: %d", rec.Code)
	}
	if rec := a.do("GET", "/api/v1/assets", reader, nil); rec.Code != 200 {
		t.Errorf("reader lists: %d", rec.Code)
	}
	for _, c := range []struct{ method, path string }{
		{"POST", "/api/v1/assets"},
		{"POST", "/api/v1/asset-types"},
		{"POST", "/api/v1/asset-statuses"},
	} {
		body := map[string]any{"tag": "P-1", "name": "N", "asset_type_id": domain.GeneralTypeID.String(), "code": "P1", "kind": "available"}
		if rec := a.do(c.method, c.path, reader, body); rec.Code != 403 {
			t.Errorf("%s %s as reader: %d", c.method, c.path, rec.Code)
		}
	}
	// Status và loại GENERAL seed sẵn
	type items struct {
		Items []struct{ ID, Name, Kind, Code string }
	}
	if s := decode[items](t, a.do("GET", "/api/v1/asset-statuses", reader, nil), 200); len(s.Items) < 4 {
		t.Errorf("statuses = %+v", s)
	}
	if ty := decode[items](t, a.do("GET", "/api/v1/asset-types", reader, nil), 200); len(ty.Items) == 0 || ty.Items[0].Code != "GENERAL" {
		t.Errorf("types = %+v", ty)
	}
}

// Danh sách lọc theo loại kèm thuộc tính từng dòng (bảng theo loại); không lọc thì không kèm
func TestListByTypeIncludesAttributes(t *testing.T) {
	a := newApp(t)
	tok := a.token(allPerms...)
	code := "LT" + strings.ToUpper(uuid.NewString()[:6])
	typ := decode[typeDetail](t, a.do("POST", "/api/v1/asset-types", tok, map[string]any{
		"code": code, "name": "Monitor " + code,
		"attributes": []map[string]any{
			{"key": "size_in", "label": "Size", "data_type": "number", "unit": "inch"},
			{"key": "panel", "label": "Panel", "data_type": "select", "options": []string{"IPS", "VA"}},
			{"key": "note", "label": "Note", "data_type": "text"},
		},
	}), 201)
	ips := typ.Attributes[1].Options[0].ID
	tag := "M-" + strings.ToUpper(uuid.NewString()[:8])
	decode[assetDetail](t, a.do("POST", "/api/v1/assets", tok, map[string]any{
		"tag": tag, "name": "Dell U2723", "asset_type_id": typ.ID,
		"attributes": map[string]any{"size_in": 27, "panel": ips},
	}), 201)

	type row struct {
		Tag        string
		Attributes *[]struct {
			Key, Unit   string
			Value       any
			OptionLabel string `json:"option_label"`
		}
	}
	type page struct {
		Items []row
		Total int64
	}
	p := decode[page](t, a.do("GET", "/api/v1/assets?type_id="+typ.ID, tok, nil), 200)
	if p.Total != 1 || p.Items[0].Attributes == nil {
		t.Fatalf("typed list = %+v", p)
	}
	attrs := *p.Items[0].Attributes
	if len(attrs) != 3 || attrs[0].Key != "size_in" || attrs[0].Value != 27.0 || attrs[0].Unit != "inch" ||
		attrs[1].OptionLabel != "IPS" || attrs[2].Value != nil {
		t.Errorf("row attributes = %+v", attrs)
	}

	// Không có type_id: dòng không kèm attributes
	p = decode[page](t, a.do("GET", "/api/v1/assets?q="+tag, tok, nil), 200)
	if p.Total != 1 || p.Items[0].Attributes != nil {
		t.Errorf("untyped list carries attributes: %+v", p.Items)
	}
}
