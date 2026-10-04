package inventory_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func TestListFilterAndSortByAttribute(t *testing.T) {
	a := newApp(t)
	tok := a.token(allPerms...)
	code := "FS" + strings.ToUpper(uuid.NewString()[:6])
	typ := decode[typeDetail](t, a.do("POST", "/api/v1/asset-types", tok, map[string]any{
		"code": code, "name": "Monitor " + code,
		"attributes": []map[string]any{
			{"key": "size_in", "label": "Size", "data_type": "number", "unit": "inch"},
			{"key": "panel", "label": "Panel", "data_type": "select", "options": []string{"IPS", "VA"}},
		},
	}), 201)
	ips, va := typ.Attributes[1].Options[0].ID, typ.Attributes[1].Options[1].ID
	prefix := "FS-" + strings.ToUpper(uuid.NewString()[:8]) + "-"
	for name, attrs := range map[string]map[string]any{
		"24": {"size_in": 24, "panel": va}, "27": {"size_in": 27, "panel": ips},
		"32": {"size_in": 32, "panel": ips}, "NONE": {},
	} {
		decode[assetDetail](t, a.do("POST", "/api/v1/assets", tok, map[string]any{
			"tag": prefix + name, "name": name, "asset_type_id": typ.ID, "attributes": attrs,
		}), 201)
	}
	list := func(q url.Values, want int) *httptest.ResponseRecorder {
		t.Helper()
		rec := a.do("GET", "/api/v1/assets?"+q.Encode(), tok, nil)
		if rec.Code != want {
			t.Fatalf("GET ?%s = %d %s", q.Encode(), rec.Code, rec.Body)
		}
		return rec
	}
	names := func(rec *httptest.ResponseRecorder) string {
		p := decode[struct {
			Items []struct{ Name string }
			Total int64
		}](t, rec, 200)
		var out []string
		for _, it := range p.Items {
			out = append(out, it.Name)
		}
		return strings.Join(out, ",")
	}

	got := names(list(url.Values{"type_id": {typ.ID}, "attr": {"size_in:gte:25", "panel:eq:" + ips},
		"sort": {"-attributes.size_in"}}, 200))
	if got != "32,27" {
		t.Errorf("filtered and sorted = %q", got)
	}
	if got := names(list(url.Values{"type_id": {typ.ID}, "sort": {"attributes.size_in"}}, 200)); got != "24,27,32,NONE" {
		t.Errorf("sorted = %q", got)
	}

	// Cột thường cũng sắp được, kể cả loại và status
	for _, s := range []string{"tag", "-name", "purchase_date", "-updated_at", "asset_type", "-status"} {
		list(url.Values{"type_id": {typ.ID}, "sort": {s}}, 200)
	}

	// Lỗi: thiếu type_id, key lạ, toán tử sai kiểu
	for q, field := range map[string]string{
		"attr=size_in:gte:25":                                  "type_id",
		"type_id=" + typ.ID + "&sort=attributes.nope":          "sort",
		"type_id=" + typ.ID + "&attr=panel:gt:" + ips:          "attr[0]",
		"type_id=" + typ.ID + "&attr=size_in:eq:1&attr=x:eq:1": "attr[1]",
	} {
		rec := a.do("GET", "/api/v1/assets?"+q, tok, nil)
		typ, fields := problem(t, rec)
		if rec.Code != 422 || typ != "/errors/invalid-attribute-query" || len(fields) != 1 || !strings.HasPrefix(fields[0], field+":") {
			t.Errorf("?%s = %d %s %v", q, rec.Code, typ, fields)
		}
	}
	// Sai cú pháp bị chặn ở bước kiểm request
	for _, q := range []string{"sort=bogus", "attr=no-colons", "attr=Size:eq:1"} {
		if rec := a.do("GET", "/api/v1/assets?"+q, tok, nil); rec.Code/100 != 4 {
			t.Errorf("?%s = %d", q, rec.Code)
		}
	}
}

func TestListTypesWithCounts(t *testing.T) {
	a := newApp(t)
	tok := a.token(allPerms...)
	code := "CN" + strings.ToUpper(uuid.NewString()[:6])
	typ := decode[typeDetail](t, a.do("POST", "/api/v1/asset-types", tok, map[string]any{"code": code, "name": "Counted " + code}), 201)
	for i := 0; i < 2; i++ {
		decode[assetDetail](t, a.do("POST", "/api/v1/assets", tok, map[string]any{
			"tag": "CN-" + strings.ToUpper(uuid.NewString()[:8]), "name": "x", "asset_type_id": typ.ID,
		}), 201)
	}
	type list struct {
		Items []struct {
			ID         string
			AssetCount *int64 `json:"asset_count"`
		}
	}
	find := func(l list) *int64 {
		for _, it := range l.Items {
			if it.ID == typ.ID {
				return it.AssetCount
			}
		}
		t.Fatalf("type %s not listed", typ.ID)
		return nil
	}
	if n := find(decode[list](t, a.do("GET", "/api/v1/asset-types?with_counts=true", tok, nil), 200)); n == nil || *n != 2 {
		t.Errorf("asset_count = %v, want 2", n)
	}
	if n := find(decode[list](t, a.do("GET", "/api/v1/asset-types", tok, nil), 200)); n != nil {
		t.Errorf("asset_count without with_counts = %d, want absent", *n)
	}
}

func TestBulkActionsOverHTTP(t *testing.T) {
	a := newApp(t)
	tok := a.token(allPerms...)
	mk := func() assetDetail {
		return decode[assetDetail](t, a.do("POST", "/api/v1/assets", tok, map[string]any{
			"tag": "BK-" + strings.ToUpper(uuid.NewString()[:8]), "name": "Bulk", "asset_type_id": domain.GeneralTypeID.String(),
		}), 201)
	}
	x, y := mk(), mk()
	type result struct {
		Succeeded []string
		Failed    []struct {
			ID      string
			Problem struct{ Type string }
		}
	}

	// y gửi version cũ: x đổi được, y báo asset-changed; cả lô vẫn 200
	r := decode[result](t, a.do("POST", "/api/v1/assets/bulk-status", tok, map[string]any{
		"status_id": domain.RepairStatusID.String(),
		"items":     []map[string]any{{"id": x.ID, "version": x.Version}, {"id": y.ID, "version": y.Version - 1}},
	}), 200)
	if len(r.Succeeded) != 1 || r.Succeeded[0] != x.ID || len(r.Failed) != 1 || r.Failed[0].ID != y.ID ||
		r.Failed[0].Problem.Type != "/errors/asset-changed" {
		t.Fatalf("bulk status = %+v", r)
	}
	if got := decode[assetDetail](t, a.do("GET", "/api/v1/assets/"+x.ID, tok, nil), 200); got.Status.ID != domain.RepairStatusID.String() {
		t.Errorf("x status = %+v", got.Status)
	}

	r = decode[result](t, a.do("POST", "/api/v1/assets/bulk-retire", tok, map[string]any{
		"reason": "Disposal batch",
		"items":  []map[string]any{{"id": y.ID, "version": y.Version}},
	}), 200)
	if len(r.Succeeded) != 1 || len(r.Failed) != 0 {
		t.Fatalf("bulk retire = %+v", r)
	}
	if got := decode[assetDetail](t, a.do("GET", "/api/v1/assets/"+y.ID, tok, nil), 200); got.RetiredAt == nil {
		t.Error("y not retired")
	}

	// lỗi của cả yêu cầu
	if rec := a.do("POST", "/api/v1/assets/bulk-retire", tok, map[string]any{"items": []any{}}); rec.Code != 422 {
		t.Errorf("empty items: %d %s", rec.Code, rec.Body)
	}
	if rec := a.do("POST", "/api/v1/assets/bulk-status", tok, map[string]any{
		"status_id": domain.RetiredStatusID.String(), "items": []map[string]any{{"id": x.ID, "version": 1}},
	}); rec.Code != 422 {
		t.Errorf("retired-kind status: %d %s", rec.Code, rec.Body)
	}
	if rec := a.do("POST", "/api/v1/assets/bulk-retire", a.token(domain.PermAssetRead), map[string]any{
		"items": []map[string]any{{"id": x.ID, "version": 1}},
	}); rec.Code != 403 {
		t.Errorf("without manage: %d", rec.Code)
	}
	// đường dẫn tài sản đơn vẫn chạy bên cạnh
	if rec := a.do("GET", "/api/v1/assets/"+x.ID, tok, nil); rec.Code != 200 {
		t.Errorf("single asset route: %d", rec.Code)
	}
}

func TestReorderOverHTTP(t *testing.T) {
	a := newApp(t)
	tok := a.token(allPerms...)
	code := "RO" + strings.ToUpper(uuid.NewString()[:6])
	typ := decode[typeDetail](t, a.do("POST", "/api/v1/asset-types", tok, map[string]any{
		"code": code, "name": "Reorder " + code,
		"attributes": []map[string]any{
			{"key": "a1", "label": "First", "data_type": "text", "position": 1},
			{"key": "a2", "label": "Second", "data_type": "select", "position": 2, "options": []string{"X", "Y", "Z"}},
		},
	}), 201)
	first, second := typ.Attributes[0].ID, typ.Attributes[1].ID

	got := decode[typeDetail](t, a.do("PUT", "/api/v1/asset-types/"+typ.ID+"/attributes/order", tok, map[string]any{
		"ids": []string{second, first},
	}), 200)
	if got.Attributes[0].ID != second {
		t.Errorf("attribute order = %+v", got.Attributes)
	}

	opts := typ.Attributes[1].Options
	type attrOut struct {
		Options []struct{ ID, Label string }
	}
	at := decode[attrOut](t, a.do("PUT", "/api/v1/asset-types/"+typ.ID+"/attributes/"+second+"/options/order", tok, map[string]any{
		"ids": []string{opts[2].ID, opts[0].ID, opts[1].ID},
	}), 200)
	if at.Options[0].Label != "Z" || at.Options[2].Label != "Y" {
		t.Errorf("option order = %+v", at.Options)
	}

	rec := a.do("PUT", "/api/v1/asset-types/"+typ.ID+"/attributes/order", tok, map[string]any{"ids": []string{first}})
	if typ, _ := problem(t, rec); rec.Code != 422 || typ != "/errors/invalid-order" {
		t.Errorf("missing attribute: %d %s", rec.Code, typ)
	}
	if rec := a.do("PUT", "/api/v1/asset-types/"+typ.ID+"/attributes/order", a.token(domain.PermAssetRead), map[string]any{
		"ids": []string{first, second},
	}); rec.Code != 403 {
		t.Errorf("without type.manage: %d", rec.Code)
	}
	// các route của từng thuộc tính vẫn chạy bên cạnh
	if rec := a.do("PATCH", "/api/v1/asset-types/"+typ.ID+"/attributes/"+first, tok, map[string]any{"label": "Renamed"}); rec.Code != 200 {
		t.Errorf("patch attribute: %d %s", rec.Code, rec.Body)
	}
}
