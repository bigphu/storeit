package inventory_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	idcontract "storeit/internal/identity/contract"
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

func newApp(t *testing.T) *app { return newAppWith(t, 0) }

// newAppWith dựng app với trần số dòng export (0 là mặc định của service)
func newAppWith(t *testing.T, maxRows int) *app {
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
	assets := repository.NewAssetRepository(pool, outbox)
	svc := service.New(service.Deps{
		Types:    repository.NewTypeRepository(pool, outbox),
		Statuses: repository.NewStatusRepository(pool, outbox),
		Assets:   assets,
		Events:   assets,

		Profiles:      repository.NewExportProfileRepository(pool, outbox),
		Accounts:      stubAccounts{},
		ExportMaxRows: maxRows,
		Borrowing:     repository.NewBorrowingRepository(pool, outbox, jobs.NewRiver(client)),
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

// tokenFor: như token nhưng cho một account cụ thể (người gửi phiếu mượn)
func (a *app) tokenFor(id uuid.UUID, perms ...string) string {
	a.t.Helper()
	tok, err := a.tokens.Issue(id, perms)
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

var allPerms = []string{domain.PermAssetRead, domain.PermAssetManage, domain.PermTypeManage, domain.PermStatusManage,
	domain.PermAssetExport, domain.PermExportProfileManage, domain.PermBorrowRequest, domain.PermBorrowManage}

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
	typ := decode[typeDetail](t, a.do("POST", "/api/v1/asset-types", tok, map[string]any{
		"code": code, "name": "Counted " + code,
		"attributes": []map[string]any{
			{"key": "b", "label": "Second", "data_type": "text", "position": 2},
			{"key": "a", "label": "First", "data_type": "text", "position": 1},
		},
	}), 201)
	for _, status := range []string{"", domain.InUseStatusID.String()} {
		body := map[string]any{"tag": "CN-" + strings.ToUpper(uuid.NewString()[:8]), "name": "x", "asset_type_id": typ.ID}
		if status != "" {
			body["status_id"] = status
		}
		decode[assetDetail](t, a.do("POST", "/api/v1/assets", tok, body), 201)
	}
	type item struct {
		ID         string
		AssetCount *int64 `json:"asset_count"`
		KindCounts *struct {
			Available   int64 `json:"available"`
			InUse       int64 `json:"in_use"`
			Unavailable int64 `json:"unavailable"`
		} `json:"kind_counts"`
		AttributeCount  *int32   `json:"attribute_count"`
		AttributeLabels []string `json:"attribute_labels"`
	}
	type list struct{ Items []item }
	find := func(l list) item {
		for _, it := range l.Items {
			if it.ID == typ.ID {
				return it
			}
		}
		t.Fatalf("type %s not listed", typ.ID)
		return item{}
	}
	got := find(decode[list](t, a.do("GET", "/api/v1/asset-types?with_counts=true", tok, nil), 200))
	if got.AssetCount == nil || *got.AssetCount != 2 {
		t.Errorf("asset_count = %v, want 2", got.AssetCount)
	}
	if k := got.KindCounts; k == nil || k.Available != 1 || k.InUse != 1 || k.Unavailable != 0 {
		t.Errorf("kind_counts = %+v, want 1 available, 1 in use", k)
	}
	if got.AttributeCount == nil || *got.AttributeCount != 2 || !slices.Equal(got.AttributeLabels, []string{"First", "Second"}) {
		t.Errorf("attributes = %v %v, want 2 [First Second]", got.AttributeCount, got.AttributeLabels)
	}
	if got := find(decode[list](t, a.do("GET", "/api/v1/asset-types", tok, nil), 200)); got.AssetCount != nil || got.KindCounts != nil || got.AttributeLabels != nil {
		t.Errorf("summary without with_counts = %+v, want absent", got)
	}
}

func TestStatusesOverHTTP(t *testing.T) {
	a := newApp(t)
	tok := a.token(allPerms...)
	type status struct {
		ID         string
		Position   int32
		ArchivedAt *string `json:"archived_at"`
		AssetCount *int64  `json:"asset_count"`
	}
	type list struct{ Items []status }
	s := decode[status](t, a.do("POST", "/api/v1/asset-statuses", tok, map[string]any{"name": "Spare " + uuid.NewString()[:8], "kind": "available"}), 201)
	typ := decode[typeDetail](t, a.do("POST", "/api/v1/asset-types", tok, map[string]any{"code": "ST" + strings.ToUpper(uuid.NewString()[:6]), "name": "Status " + uuid.NewString()[:8]}), 201)
	decode[assetDetail](t, a.do("POST", "/api/v1/assets", tok, map[string]any{
		"tag": "ST-" + strings.ToUpper(uuid.NewString()[:8]), "name": "x", "asset_type_id": typ.ID, "status_id": s.ID,
	}), 201)

	counted := decode[list](t, a.do("GET", "/api/v1/asset-statuses?with_counts=true", tok, nil), 200)
	var ids []string
	for _, x := range counted.Items {
		ids = append(ids, x.ID)
		if x.ID == s.ID && (x.AssetCount == nil || *x.AssetCount != 1) {
			t.Errorf("asset_count = %v, want 1", x.AssetCount)
		}
	}
	slices.Reverse(ids)
	got := decode[list](t, a.do("PUT", "/api/v1/asset-statuses/order", tok, map[string]any{"ids": ids}), 200)
	if len(got.Items) != len(ids) || got.Items[0].ID != ids[0] || got.Items[0].Position != 1 {
		t.Errorf("reordered = %+v", got.Items)
	}
	rec := a.do("PUT", "/api/v1/asset-statuses/order", tok, map[string]any{"ids": ids[1:]})
	if typ, _ := problem(t, rec); rec.Code != 422 || typ != "/errors/invalid-order" {
		t.Errorf("missing status: %d %s", rec.Code, typ)
	}
	if rec := a.do("PUT", "/api/v1/asset-statuses/order", a.token(domain.PermAssetRead), map[string]any{"ids": ids}); rec.Code != 403 {
		t.Errorf("without status.manage: %d", rec.Code)
	}
	// route theo id vẫn chạy bên cạnh /order
	if rec := a.do("PATCH", "/api/v1/asset-statuses/"+s.ID, tok, map[string]any{"name": "Renamed " + uuid.NewString()[:8]}); rec.Code != 200 {
		t.Errorf("patch status: %d %s", rec.Code, rec.Body)
	}

	decode[status](t, a.do("POST", "/api/v1/asset-statuses/"+s.ID+"/archive", tok, nil), 200)
	back := decode[status](t, a.do("POST", "/api/v1/asset-statuses/"+s.ID+"/restore", tok, nil), 200)
	if back.ArchivedAt != nil {
		t.Errorf("restored status still archived: %+v", back)
	}
	if rec := a.do("POST", "/api/v1/asset-statuses/"+s.ID+"/restore", a.token(domain.PermAssetRead), nil); rec.Code != 403 {
		t.Errorf("restore without status.manage: %d", rec.Code)
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

// stubAccounts trả tên cố định, thay cho identity trong test
type stubAccounts struct{}

func (stubAccounts) GetAccount(_ context.Context, id uuid.UUID) (idcontract.Account, error) {
	return idcontract.Account{ID: id, Name: "Người dùng", Active: true}, nil
}

func (stubAccounts) GetAccounts(_ context.Context, ids []uuid.UUID) ([]idcontract.Account, error) {
	out := make([]idcontract.Account, len(ids))
	for i, id := range ids {
		out[i] = idcontract.Account{ID: id, Name: "Người dùng", Active: true}
	}
	return out, nil
}

func (stubAccounts) SearchAccounts(context.Context, string, int) ([]idcontract.Account, error) {
	return nil, nil
}

func TestExportOverHTTP(t *testing.T) {
	a := newApp(t)
	tok := a.token(allPerms...)
	exp := a.token(domain.PermAssetRead, domain.PermAssetExport)
	code := "EX" + strings.ToUpper(uuid.NewString()[:6])
	typ := decode[typeDetail](t, a.do("POST", "/api/v1/asset-types", tok, map[string]any{
		"code": code, "name": "Export " + code,
		"attributes": []map[string]any{{"key": "ram_gb", "label": "RAM", "data_type": "number", "unit": "GB", "position": 1}},
	}), 201)
	var ids []string
	for i := range 3 {
		x := decode[assetDetail](t, a.do("POST", "/api/v1/assets", tok, map[string]any{
			"tag": fmt.Sprintf("%s-%d", code, i), "name": "x", "asset_type_id": typ.ID, "attributes": map[string]any{"ram_gb": 16},
		}), 201)
		ids = append(ids, x.ID)
	}

	rec := a.do("POST", "/api/v1/assets/export", exp, map[string]any{"mode": "data", "filters": map[string]any{"type_id": typ.ID}})
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Type"), "spreadsheetml") {
		t.Fatalf("data export: %d %s", rec.Code, rec.Header())
	}
	x, err := excelize.OpenReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	rows, _ := x.GetRows(code)
	if len(rows) != 4 || rows[0][0] != "tag" || rows[0][len(rows[0])-1] != "attr:ram_gb" {
		t.Errorf("data sheet = %v", rows)
	}

	// selection + report with a non-ASCII profile name and a removed attribute
	layout := map[string]any{
		"columns": []map[string]any{{"field": "tag"}, {"field": "attr:gone"}}, "sheets": "single", "sheet_name": "Báo cáo",
		"title_row": true, "summary": false, "header": "bold", "freeze": true, "filter": true, "stripes": false,
		"date_format": "dd/mm/yyyy", "bool_style": "yes_no", "status_as": "name", "unit_in": "header",
	}
	p := decode[struct{ ID string }](t, a.do("POST", "/api/v1/export-profiles", exp, map[string]any{"name": "Kiểm kê quý 3", "layout": layout}), 201)
	rec = a.do("POST", "/api/v1/assets/export", exp, map[string]any{
		"mode": "report", "profile_id": p.ID, "filters": map[string]any{"ids": ids[:2]},
	})
	if rec.Code != 200 {
		t.Fatalf("report export: %d %s", rec.Code, rec.Body)
	}
	rep, err := excelize.OpenReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	repRows, _ := rep.GetRows("Báo cáo")
	if len(repRows) < 4 {
		t.Fatalf("report rows = %q", repRows)
	}
	hdr, data := repRows[len(repRows)-3], repRows[len(repRows)-2:]
	if len(hdr) != 1 || hdr[0] != "Tag" {
		t.Errorf("header = %q", hdr)
	}
	wantTags := []string{strings.ToUpper(fmt.Sprintf("%s-%d", code, 0)), strings.ToUpper(fmt.Sprintf("%s-%d", code, 1))}
	var gotTags []string
	for _, r := range data {
		gotTags = append(gotTags, r[0])
	}
	slices.Sort(gotTags)
	if !slices.Equal(gotTags, wantTags) {
		t.Errorf("selected tags = %v, want %v", gotTags, wantTags)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "filename*=utf-8''") {
		t.Errorf("content-disposition = %q", cd)
	}
	if got := rec.Header().Get("X-Export-Skipped-Columns"); got != "gone" {
		t.Errorf("skipped = %q", got)
	}

	if rec := a.do("POST", "/api/v1/assets/export", a.token(domain.PermAssetRead), map[string]any{"mode": "data"}); rec.Code != 403 {
		t.Errorf("without export permission: %d", rec.Code)
	}
	other := a.token(domain.PermAssetRead, domain.PermAssetExport)
	if rec := a.do("GET", "/api/v1/export-profiles/"+p.ID, other, nil); rec.Code != 404 {
		t.Errorf("someone else's private profile: %d", rec.Code)
	}
	if rec := a.do("PATCH", "/api/v1/export-profiles/"+p.ID, exp, map[string]any{"shared": true, "version": 1}); rec.Code != 200 {
		t.Errorf("share: %d %s", rec.Code, rec.Body)
	}
	if rec := a.do("DELETE", "/api/v1/export-profiles/"+p.ID, other, nil); rec.Code != 403 {
		t.Errorf("non-manager deletes shared profile: %d", rec.Code)
	}

	small := newAppWith(t, 1)
	stok := small.token(allPerms...)
	if rec := small.do("POST", "/api/v1/assets/export", stok, map[string]any{"mode": "data"}); rec.Code != 422 {
		t.Errorf("over the cap: %d", rec.Code)
	} else if typ, _ := problem(t, rec); typ != "/errors/export-too-large" {
		t.Errorf("problem type = %s", typ)
	}
}

// Xoá profile là xoá mềm: ẩn với mọi người (kể cả export theo profile_id), khôi phục được
// bởi người xoá được nó
func TestExportProfileRestoreOverHTTP(t *testing.T) {
	a := newApp(t)
	owner := a.token(domain.PermAssetRead, domain.PermAssetExport)
	other := a.token(domain.PermAssetRead, domain.PermAssetExport)
	layout := map[string]any{
		"columns": []map[string]any{{"field": "tag"}}, "sheets": "single", "sheet_name": "Assets",
		"title_row": false, "summary": false, "header": "bold", "freeze": true, "filter": true, "stripes": false,
		"date_format": "dd/mm/yyyy", "bool_style": "yes_no", "status_as": "name", "unit_in": "header",
	}
	name := "Kiểm kê " + uuid.NewString()[:6]
	p := decode[struct{ ID string }](t, a.do("POST", "/api/v1/export-profiles", owner, map[string]any{"name": name, "shared": true, "layout": layout}), 201)

	if rec := a.do("DELETE", "/api/v1/export-profiles/"+p.ID, owner, nil); rec.Code != 204 {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	if rec := a.do("GET", "/api/v1/export-profiles/"+p.ID, other, nil); rec.Code != 404 {
		t.Errorf("deleted shared profile visible to others: %d", rec.Code)
	}
	if rec := a.do("POST", "/api/v1/assets/export", owner, map[string]any{"mode": "report", "profile_id": p.ID}); rec.Code != 404 {
		t.Errorf("export with a deleted profile: %d", rec.Code)
	} else if typ, _ := problem(t, rec); typ != "/errors/export-profile-not-found" {
		t.Errorf("problem type = %s", typ)
	}
	// không phải chủ, không có quyền quản lý: không khôi phục được profile chia sẻ của người khác
	if rec := a.do("POST", "/api/v1/export-profiles/"+p.ID+"/restore", other, nil); rec.Code != 403 {
		t.Errorf("non-manager restores someone else's shared profile: %d", rec.Code)
	}
	if rec := a.do("POST", "/api/v1/export-profiles/"+p.ID+"/restore", owner, nil); rec.Code != 200 {
		t.Fatalf("restore: %d %s", rec.Code, rec.Body)
	}
	if rec := a.do("GET", "/api/v1/export-profiles/"+p.ID, other, nil); rec.Code != 200 {
		t.Errorf("restored shared profile: %d", rec.Code)
	}

	// profile riêng của người khác: 404 (không lộ là nó tồn tại)
	priv := decode[struct{ ID string }](t, a.do("POST", "/api/v1/export-profiles", owner, map[string]any{"name": "Riêng " + uuid.NewString()[:6], "layout": layout}), 201)
	a.do("DELETE", "/api/v1/export-profiles/"+priv.ID, owner, nil)
	if rec := a.do("POST", "/api/v1/export-profiles/"+priv.ID+"/restore", other, nil); rec.Code != 404 {
		t.Errorf("restore someone else's private profile: %d", rec.Code)
	}

	// tên dùng lại sau khi xoá: được; khôi phục bản cũ → 409
	a.do("DELETE", "/api/v1/export-profiles/"+p.ID, owner, nil)
	if rec := a.do("POST", "/api/v1/export-profiles", owner, map[string]any{"name": name, "layout": layout}); rec.Code != 201 {
		t.Fatalf("reuse name: %d %s", rec.Code, rec.Body)
	}
	rec := a.do("POST", "/api/v1/export-profiles/"+p.ID+"/restore", owner, nil)
	if rec.Code != 409 {
		t.Errorf("restore over reused name: %d", rec.Code)
	} else if typ, _ := problem(t, rec); typ != "/errors/export-profile-name-taken" {
		t.Errorf("problem type = %s", typ)
	}
}

// Bỏ thuộc tính / option đã là xoá mềm; khôi phục đưa lại, trùng nhãn với cái đang dùng thì 409
func TestRestoreAttributeAndOption(t *testing.T) {
	a := newApp(t)
	tok := a.token(allPerms...)
	code := "RS" + strings.ToUpper(uuid.NewString()[:6])
	typ := decode[typeDetail](t, a.do("POST", "/api/v1/asset-types", tok, map[string]any{
		"code": code, "name": "Restore " + code,
		"attributes": []map[string]any{
			{"key": "ram", "label": "RAM", "data_type": "number", "position": 1},
			{"key": "os", "label": "OS", "data_type": "select", "position": 2, "options": []string{"Windows", "macOS"}},
		},
	}), 201)
	var ramID, osID, winID string
	for _, at := range typ.Attributes {
		switch at.Key {
		case "ram":
			ramID = at.ID
		case "os":
			osID = at.ID
			for _, o := range at.Options {
				if o.Label == "Windows" {
					winID = o.ID
				}
			}
		}
	}
	if ramID == "" || osID == "" || winID == "" {
		t.Fatalf("attributes not found in %+v", typ)
	}
	base := "/api/v1/asset-types/" + typ.ID + "/attributes/"

	if rec := a.do("DELETE", base+ramID, tok, nil); rec.Code != 204 {
		t.Fatalf("remove attribute: %d %s", rec.Code, rec.Body)
	}
	rec := a.do("POST", base+ramID+"/restore", tok, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"key":"ram"`) {
		t.Fatalf("restore attribute: %d %s", rec.Code, rec.Body)
	}
	// khôi phục cái chưa bỏ: không đổi gì
	if rec := a.do("POST", base+ramID+"/restore", tok, nil); rec.Code != 200 {
		t.Errorf("restore active attribute: %d", rec.Code)
	}

	// bỏ, thêm thuộc tính mới cùng nhãn "RAM" (khoá khác), khôi phục cái cũ → 409
	a.do("DELETE", base+ramID, tok, nil)
	if rec := a.do("POST", "/api/v1/asset-types/"+typ.ID+"/attributes", tok, map[string]any{"key": "ram2", "label": "RAM", "data_type": "number", "position": 3}); rec.Code != 201 {
		t.Fatalf("add attribute with the removed label: %d %s", rec.Code, rec.Body)
	}
	rec = a.do("POST", base+ramID+"/restore", tok, nil)
	if rec.Code != 409 {
		t.Errorf("restore over a reused label: %d", rec.Code)
	} else if typ, _ := problem(t, rec); typ != "/errors/attribute-label-taken" {
		t.Errorf("problem type = %s", typ)
	}

	optBase := base + osID + "/options/"
	if rec := a.do("DELETE", optBase+winID, tok, nil); rec.Code != 204 {
		t.Fatalf("remove option: %d", rec.Code)
	}
	if rec := a.do("POST", optBase+winID+"/restore", tok, nil); rec.Code != 200 || !strings.Contains(rec.Body.String(), "Windows") {
		t.Errorf("restore option: %d %s", rec.Code, rec.Body)
	}
	if rec := a.do("POST", optBase+uuid.NewString()+"/restore", tok, nil); rec.Code != 404 {
		t.Errorf("restore unknown option: %d", rec.Code)
	}
	reader := a.token(domain.PermAssetRead)
	if rec := a.do("POST", base+ramID+"/restore", reader, nil); rec.Code != 403 {
		t.Errorf("restore without type manage: %d", rec.Code)
	}
}

func TestListByBuiltinFieldOverHTTP(t *testing.T) {
	a := newApp(t)
	tok := a.token(allPerms...)
	code := "BF" + strings.ToUpper(uuid.NewString()[:6])
	typ := decode[typeDetail](t, a.do("POST", "/api/v1/asset-types", tok, map[string]any{"code": code, "name": "Builtin " + code}), 201)
	for i, bought := range []string{"2026-01-10", "2026-03-01"} {
		decode[assetDetail](t, a.do("POST", "/api/v1/assets", tok, map[string]any{
			"tag": fmt.Sprintf("%s-%d", code, i), "name": "x", "asset_type_id": typ.ID,
			"purchase_date": bought, "description": fmt.Sprintf("dock %d", i),
		}), 201)
	}
	type page struct{ Total int64 }
	q := func(extra string) string { return "/api/v1/assets?q=" + code + extra }
	// không cần type_id
	if p := decode[page](t, a.do("GET", q("&field=purchase_date:gte:2026-02-01"), tok, nil), 200); p.Total != 1 {
		t.Errorf("purchase_date filter total = %d, want 1", p.Total)
	}
	if p := decode[page](t, a.do("GET", q("&field=description:contains:dock%201"), tok, nil), 200); p.Total != 1 {
		t.Errorf("description filter total = %d, want 1", p.Total)
	}
	if p := decode[page](t, a.do("GET", q("&field=created_at:eq:2999-01-01&tz=Asia/Ho_Chi_Minh"), tok, nil), 200); p.Total != 0 {
		t.Errorf("created_at in the future total = %d, want 0", p.Total)
	}
	// tz lạ: 422; field sai dạng: 400 (pattern của OpenAPI)
	if rec := a.do("GET", q("&tz=Mars/Base"), tok, nil); rec.Code != 422 {
		t.Errorf("bad tz = %d, want 422", rec.Code)
	}
	if rec := a.do("GET", q("&field=location:eq:x"), tok, nil); rec.Code != 400 {
		t.Errorf("unknown field = %d, want 400", rec.Code)
	}
	if rec := a.do("GET", q("&field=description:eq:dock"), tok, nil); rec.Code != 422 {
		t.Errorf("bad operator = %d, want 422", rec.Code)
	}

	// export: field thu hẹp số dòng
	exp := a.token(domain.PermAssetRead, domain.PermAssetExport)
	rec := a.do("POST", "/api/v1/assets/export", exp, map[string]any{
		"mode": "data", "tz": "Asia/Ho_Chi_Minh",
		"filters": map[string]any{"q": code, "field": []string{"purchase_date:lt:2026-02-01"}},
	})
	if rec.Code != 200 {
		t.Fatalf("export: %d %s", rec.Code, rec.Body)
	}
	x, err := excelize.OpenReader(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	rows, _ := x.GetRows(code)
	if len(rows) != 2 { // dòng tên cột + 1 tài sản
		t.Errorf("exported rows = %d, want 2: %v", len(rows), rows)
	}
}

func TestAssetActivityOverHTTP(t *testing.T) {
	a := newApp(t)
	tok := a.token(allPerms...)
	code := "AC" + strings.ToUpper(uuid.NewString()[:6])
	typ := decode[typeDetail](t, a.do("POST", "/api/v1/asset-types", tok, map[string]any{
		"code": code, "name": "Activity " + code,
		"attributes": []map[string]any{{"key": "ram_gb", "label": "RAM", "data_type": "number", "unit": "GB"}},
	}), 201)
	created := decode[assetDetail](t, a.do("POST", "/api/v1/assets", tok, map[string]any{
		"tag": code + "-1", "name": "Laptop", "asset_type_id": typ.ID, "attributes": map[string]any{"ram_gb": 8},
	}), 201)
	upd := decode[assetDetail](t, a.do("PUT", "/api/v1/assets/"+created.ID, tok, map[string]any{
		"name": "Laptop", "asset_type_id": typ.ID, "status_id": domain.InUseStatusID.String(),
		"attributes": map[string]any{"ram_gb": 16}, "version": created.Version,
	}), 200)
	ret := decode[assetDetail](t, a.do("POST", "/api/v1/assets/"+created.ID+"/retire", tok, map[string]any{"reason": "Broken", "version": upd.Version}), 200)
	rest := decode[assetDetail](t, a.do("POST", "/api/v1/assets/"+created.ID+"/restore", tok, map[string]any{"version": ret.Version}), 200)

	type change struct {
		Field, Label string
		From, To     *string
	}
	type entry struct {
		ID      string
		Kind    string
		Actor   *struct{ ID, Name string }
		Changes []change
		Reason  string
	}
	type page struct {
		Items []entry
		Next  *string
	}
	path := "/api/v1/assets/" + created.ID + "/activity"
	p := decode[page](t, a.do("GET", path, tok, nil), 200)
	var kinds []string
	for _, e := range p.Items {
		kinds = append(kinds, e.Kind)
	}
	if strings.Join(kinds, ",") != "restored,retired,updated,created" || p.Next != nil {
		t.Fatalf("kinds = %v next = %v", kinds, p.Next)
	}
	if p.Items[1].Reason != "Broken" {
		t.Errorf("retired reason = %q", p.Items[1].Reason)
	}
	if p.Items[0].Actor == nil || p.Items[0].Actor.Name != "Người dùng" {
		t.Errorf("actor = %+v", p.Items[0].Actor)
	}
	got := map[string]change{}
	for _, c := range p.Items[2].Changes {
		got[c.Label] = c
	}
	if c := got["RAM"]; c.From == nil || *c.From != "8 GB" || c.To == nil || *c.To != "16 GB" {
		t.Errorf("RAM change = %+v", c)
	}
	if c := got["Status"]; c.From == nil || *c.From != created.Status.Name || c.To == nil || *c.To != upd.Status.Name {
		t.Errorf("Status change = %+v, want %q → %q", c, created.Status.Name, upd.Status.Name)
	}

	// Phân trang: 2 dòng mỗi trang, next dẫn tới phần còn lại
	p1 := decode[page](t, a.do("GET", path+"?limit=2", tok, nil), 200)
	if len(p1.Items) != 2 || p1.Next == nil {
		t.Fatalf("page 1 = %+v", p1)
	}
	p2 := decode[page](t, a.do("GET", path+"?limit=2&before="+*p1.Next, tok, nil), 200)
	if len(p2.Items) != 2 || p2.Next != nil || p2.Items[1].Kind != "created" {
		t.Fatalf("page 2 = %+v", p2)
	}

	// Đổi chỗ để (trường ẩn) ba lần: không hiện, nhưng next vẫn đi tiếp tới "created"
	cur := rest
	for range 3 {
		cur = decode[assetDetail](t, a.do("PUT", "/api/v1/assets/"+created.ID, tok, map[string]any{
			"name": "Laptop", "asset_type_id": typ.ID, "location_id": uuid.NewString(),
			"attributes": map[string]any{"ram_gb": 16}, "version": cur.Version,
		}), 200)
	}
	var walked []string
	next := ""
	for range 10 {
		q := path + "?limit=2"
		if next != "" {
			q += "&before=" + next
		}
		pg := decode[page](t, a.do("GET", q, tok, nil), 200)
		for _, e := range pg.Items {
			walked = append(walked, e.Kind)
		}
		if pg.Next == nil {
			break
		}
		next = *pg.Next
	}
	if strings.Join(walked, ",") != "restored,retired,updated,created" {
		t.Errorf("walked = %v, want the four visible entries after skipping hidden ones", walked)
	}

	// Tài sản đã retire vẫn đọc được lịch sử
	decode[assetDetail](t, a.do("POST", "/api/v1/assets/"+created.ID+"/retire", tok, map[string]any{"version": cur.Version}), 200)
	decode[page](t, a.do("GET", path, tok, nil), 200)

	if rec := a.do("GET", "/api/v1/assets/"+uuid.NewString()+"/activity", tok, nil); rec.Code != 404 {
		t.Errorf("unknown asset = %d, want 404", rec.Code)
	}
	if rec := a.do("GET", path, a.token(domain.PermTypeManage), nil); rec.Code != 403 {
		t.Errorf("no asset.read = %d, want 403", rec.Code)
	}
}

func TestReplacementOverHTTP(t *testing.T) {
	a := newApp(t)
	tok := a.token(allPerms...)
	mk := func(name string) assetDetail {
		t.Helper()
		return decode[assetDetail](t, a.do("POST", "/api/v1/assets", tok, map[string]any{
			"tag": "RH-" + strings.ToUpper(uuid.NewString()[:8]), "name": name, "asset_type_id": domain.GeneralTypeID.String(),
		}), 201)
	}
	type link struct {
		ID, Tag, Name, Reason string
		ReplacedOn            string `json:"replaced_on"`
	}
	type detail struct {
		ID         string
		Version    int32
		RetiredAt  *string `json:"retired_at"`
		ReplacedBy *link   `json:"replaced_by"`
		Replaces   []link
	}
	o, n := mk("Old laptop"), mk("New laptop")
	today := time.Now().UTC().Format(time.DateOnly)
	path := "/api/v1/assets/" + o.ID + "/replacement"

	rec := decode[detail](t, a.do("POST", path, tok, map[string]any{
		"replacement_asset_id": n.ID, "replaced_on": today, "reason": "Broken screen", "version": o.Version,
	}), 200)
	if rec.RetiredAt == nil || rec.ReplacedBy == nil || rec.ReplacedBy.Tag != n.Tag || rec.ReplacedBy.Reason != "Broken screen" {
		t.Fatalf("old after replace = %+v", rec)
	}
	nd := decode[detail](t, a.do("GET", "/api/v1/assets/"+n.ID, tok, nil), 200)
	if len(nd.Replaces) != 1 || nd.Replaces[0].Tag != o.Tag {
		t.Fatalf("new replaces = %+v", nd.Replaces)
	}

	// Tài sản mới bị retire sau đó: vẫn hoàn tác được, tài sản mới giữ nguyên retire
	decode[detail](t, a.do("POST", "/api/v1/assets/"+n.ID+"/retire", tok, map[string]any{"version": nd.Version}), 200)
	back := decode[detail](t, a.do("POST", path+"/undo", tok, map[string]any{"version": rec.Version}), 200)
	if back.RetiredAt != nil || back.ReplacedBy != nil {
		t.Errorf("old after undo = %+v", back)
	}
	if nd2 := decode[detail](t, a.do("GET", "/api/v1/assets/"+n.ID, tok, nil), 200); nd2.RetiredAt == nil || len(nd2.Replaces) != 0 {
		t.Errorf("new after undo = %+v", nd2)
	}

	// Lịch sử của hai tài sản có các mục thay thế, kèm tag của tài sản kia
	type entry struct {
		Kind    string
		Related *struct{ ID, Tag, Role string }
	}
	type page struct{ Items []entry }
	op := decode[page](t, a.do("GET", "/api/v1/assets/"+o.ID+"/activity", tok, nil), 200)
	if len(op.Items) < 2 || op.Items[0].Kind != "replacement_undone" || op.Items[1].Kind != "replaced" ||
		op.Items[0].Related == nil || op.Items[0].Related.Tag != n.Tag || op.Items[0].Related.Role != "old" ||
		op.Items[1].Related == nil || op.Items[1].Related.Tag != n.Tag {
		t.Errorf("old activity = %+v", op.Items)
	}
	np := decode[page](t, a.do("GET", "/api/v1/assets/"+n.ID+"/activity", tok, nil), 200)
	var nk []string
	for _, e := range np.Items {
		nk = append(nk, e.Kind)
	}
	if len(np.Items) == 0 || np.Items[0].Kind != "replacement_undone" || np.Items[0].Related == nil || np.Items[0].Related.Tag != o.Tag ||
		np.Items[0].Related.Role != "new" || !slices.Contains(nk, "replaces") {
		t.Errorf("new activity = %v %+v", nk, np.Items)
	}

	o2, n2 := mk("Old 2"), mk("New 2")
	p2 := "/api/v1/assets/" + o2.ID + "/replacement"
	future := time.Now().UTC().AddDate(0, 0, 3).Format(time.DateOnly)
	rec2 := a.do("POST", p2, tok, map[string]any{"replacement_asset_id": n2.ID, "replaced_on": future, "reason": "x", "version": o2.Version})
	if typ, fields := problem(t, rec2); rec2.Code != 422 || typ != "/errors/invalid-replacement" || len(fields) != 1 || !strings.HasPrefix(fields[0], "replaced_on") {
		t.Errorf("future date = %d %s %v", rec2.Code, typ, fields)
	}
	if rec := a.do("POST", p2, tok, map[string]any{"replacement_asset_id": n2.ID, "replaced_on": today, "reason": "  ", "version": o2.Version}); rec.Code != 422 {
		t.Errorf("blank reason = %d, want 422", rec.Code)
	}
	if rec := a.do("POST", p2, a.token(domain.PermAssetRead), map[string]any{"replacement_asset_id": n2.ID, "replaced_on": today, "reason": "x", "version": o2.Version}); rec.Code != 403 {
		t.Errorf("no manage = %d, want 403", rec.Code)
	}
	if rec := a.do("POST", "/api/v1/assets/"+uuid.NewString()+"/replacement", tok, map[string]any{"replacement_asset_id": n2.ID, "replaced_on": today, "reason": "x", "version": 1}); rec.Code != 404 {
		t.Errorf("unknown asset = %d, want 404", rec.Code)
	}
	rec3 := a.do("POST", p2+"/undo", tok, map[string]any{"version": o2.Version})
	if typ, _ := problem(t, rec3); rec3.Code != 409 || typ != "/errors/asset-not-replaced" {
		t.Errorf("undo without link = %d %s", rec3.Code, typ)
	}
	if rec := a.do("POST", p2+"/undo", a.token(domain.PermAssetRead), map[string]any{"version": o2.Version}); rec.Code != 403 {
		t.Errorf("undo without manage = %d, want 403", rec.Code)
	}
	if rec := a.do("POST", "/api/v1/assets/"+uuid.NewString()+"/replacement/undo", tok, map[string]any{"version": 1}); rec.Code != 404 {
		t.Errorf("undo unknown asset = %d, want 404", rec.Code)
	}
	if rec := a.do("POST", p2, tok, map[string]any{"replacement_asset_id": n2.ID, "replaced_on": today, "reason": "x", "version": o2.Version + 7}); rec.Code != 409 {
		t.Errorf("stale version = %d, want 409", rec.Code)
	}
}

// Reserved và On loan chỉ việc mượn đặt (US-10): chọn tay khi tạo hay đổi hàng loạt là 422
func TestBorrowingStatusesNotPickable(t *testing.T) {
	a := newApp(t)
	tok := a.token(allPerms...)
	rec := a.do("POST", "/api/v1/assets", tok, map[string]any{
		"tag": "BS-" + strings.ToUpper(uuid.NewString()[:8]), "name": "Projector",
		"asset_type_id": domain.GeneralTypeID.String(), "status_id": domain.ReservedStatusID.String(),
	})
	if typ, _ := problem(t, rec); rec.Code != 422 || typ != "/errors/status-set-by-borrowing" {
		t.Errorf("create with Reserved = %d %s", rec.Code, typ)
	}
	x := decode[assetDetail](t, a.do("POST", "/api/v1/assets", tok, map[string]any{
		"tag": "BS-" + strings.ToUpper(uuid.NewString()[:8]), "name": "Projector", "asset_type_id": domain.GeneralTypeID.String(),
	}), 201)
	rec = a.do("POST", "/api/v1/assets/bulk-status", tok, map[string]any{
		"status_id": domain.OnLoanStatusID.String(), "items": []map[string]any{{"id": x.ID, "version": x.Version}},
	})
	if typ, _ := problem(t, rec); rec.Code != 422 || typ != "/errors/status-set-by-borrowing" {
		t.Errorf("bulk status On loan = %d %s", rec.Code, typ)
	}
}

// "Can be borrowed" trên loại tài sản: mặc định tắt, sửa được
func TestTypeCanBorrow(t *testing.T) {
	a := newApp(t)
	tok := a.token(allPerms...)
	type typ struct {
		ID        string
		Version   int32
		CanBorrow bool `json:"can_borrow"`
	}
	code := "CB" + strings.ToUpper(uuid.NewString()[:6])
	created := decode[typ](t, a.do("POST", "/api/v1/asset-types", tok, map[string]any{"code": code, "name": "Projector " + code}), 201)
	if created.CanBorrow {
		t.Fatal("new type can be borrowed, want off by default")
	}
	upd := decode[typ](t, a.do("PATCH", "/api/v1/asset-types/"+created.ID, tok, map[string]any{"can_borrow": true, "version": created.Version}), 200)
	if got := decode[typ](t, a.do("GET", "/api/v1/asset-types/"+created.ID, tok, nil), 200); !upd.CanBorrow || !got.CanBorrow {
		t.Errorf("after turning it on: update %v, get %v", upd.CanBorrow, got.CanBorrow)
	}
}

// borrowable=true: loại cho mượn được và status thuộc kind available
func TestAssetsBorrowableFilter(t *testing.T) {
	a := newApp(t)
	tok := a.token(allPerms...)
	mkType := func(borrow bool) string {
		code := "BF" + strings.ToUpper(uuid.NewString()[:6])
		ty := decode[typeDetail](t, a.do("POST", "/api/v1/asset-types", tok, map[string]any{"code": code, "name": "Type " + code}), 201)
		if borrow {
			decode[typeDetail](t, a.do("PATCH", "/api/v1/asset-types/"+ty.ID, tok, map[string]any{"can_borrow": true, "version": ty.Version}), 200)
		}
		return ty.ID
	}
	prefix := "BF" + strings.ToUpper(uuid.NewString()[:5])
	mk := func(typeID, suffix string, status uuid.UUID) assetDetail {
		return decode[assetDetail](t, a.do("POST", "/api/v1/assets", tok, map[string]any{
			"tag": prefix + "-" + suffix, "name": "Asset " + suffix, "asset_type_id": typeID, "status_id": status.String(),
		}), 201)
	}
	yes, no := mkType(true), mkType(false)
	free := mk(yes, "1", domain.AvailableStatusID)
	mk(yes, "2", domain.InUseStatusID)
	mk(no, "3", domain.AvailableStatusID)

	type page struct{ Items []struct{ ID string } }
	got := decode[page](t, a.do("GET", "/api/v1/assets?borrowable=true&q="+prefix, tok, nil), 200)
	if len(got.Items) != 1 || got.Items[0].ID != free.ID {
		t.Errorf("borrowable = %+v, want only %s", got.Items, free.ID)
	}
}

// Mượn qua HTTP (US-10): nhân viên gửi phiếu có yêu cầu thuộc tính, người quản lý duyệt (giữ
// chỗ), giao, cho mượn trực tiếp; tài sản hiện trạng thái mượn và lịch sử
func TestBorrowingOverHTTP(t *testing.T) {
	a := newApp(t)
	admin := a.token(allPerms...)
	emp := uuid.New()
	empTok := a.tokenFor(emp, domain.PermAssetRead, domain.PermBorrowRequest)
	officer := a.token(domain.PermAssetRead, domain.PermBorrowRequest, domain.PermBorrowManage)
	code := "HB" + strings.ToUpper(uuid.NewString()[:6])
	ty := decode[typeDetail](t, a.do("POST", "/api/v1/asset-types", admin, map[string]any{
		"code": code, "name": "Laptop " + code, "can_borrow": true,
		"attributes": []map[string]any{{"key": "ram", "label": "RAM", "data_type": "number"}},
	}), 201)
	mk := func() assetDetail {
		return decode[assetDetail](t, a.do("POST", "/api/v1/assets", admin, map[string]any{
			"tag": code + "-" + strings.ToUpper(uuid.NewString()[:4]), "name": "Dell", "asset_type_id": ty.ID,
			"attributes": map[string]any{"ram": 16},
		}), 201)
	}
	x := mk()
	today := time.Now().UTC().Format(time.DateOnly)
	type ticket struct {
		ID, State string
		Number    int64
		Version   int32
		Requester struct{ ID, Name string }
		Asset     *struct{ ID, Tag string }
	}

	tk := decode[ticket](t, a.do("POST", "/api/v1/borrow-tickets", empTok, map[string]any{
		"asset_type_id": ty.ID, "requirements": []string{"ram:gte:16"}, "needed_from": today, "note": "Client demo",
	}), 201)
	if tk.State != "waiting" || tk.Number == 0 || tk.Requester.ID != emp.String() {
		t.Fatalf("new ticket = %+v", tk)
	}
	rec := a.do("POST", "/api/v1/borrow-tickets", empTok, map[string]any{
		"asset_type_id": ty.ID, "requirements": []string{"nope:eq:1"}, "needed_from": today,
	})
	if typ, fields := problem(t, rec); rec.Code != 422 || typ != "/errors/invalid-borrow-ticket" || len(fields) == 0 || !strings.HasPrefix(fields[0], "requirements[0]") {
		t.Errorf("bad requirement = %d %s %v", rec.Code, typ, fields)
	}
	if rec := a.do("POST", "/api/v1/borrow-tickets/"+tk.ID+"/approve", empTok, map[string]any{"asset_id": x.ID, "version": tk.Version}); rec.Code != 403 {
		t.Errorf("employee approves = %d, want 403", rec.Code)
	}
	if rec := a.do("GET", "/api/v1/borrow-tickets/"+tk.ID, a.token(domain.PermBorrowRequest), nil); rec.Code != 404 {
		t.Errorf("another employee reads the ticket = %d, want 404", rec.Code)
	}

	ap := decode[ticket](t, a.do("POST", "/api/v1/borrow-tickets/"+tk.ID+"/approve", officer, map[string]any{"asset_id": x.ID, "version": tk.Version}), 200)
	if ap.State != "approved" || ap.Asset == nil || ap.Asset.Tag != x.Tag {
		t.Fatalf("approved = %+v", ap)
	}
	type withBorrowing struct {
		Status    struct{ ID string }
		Borrowing *struct {
			State   string
			Account struct{ ID string }
			Ticket  *struct{ Number int64 }
		}
	}
	got := decode[withBorrowing](t, a.do("GET", "/api/v1/assets/"+x.ID, admin, nil), 200)
	if got.Borrowing == nil || got.Borrowing.State != "reserved" || got.Borrowing.Account.ID != emp.String() ||
		got.Borrowing.Ticket == nil || got.Borrowing.Ticket.Number != tk.Number || got.Status.ID != domain.ReservedStatusID.String() {
		t.Fatalf("asset after approve = %+v", got)
	}

	ho := decode[ticket](t, a.do("POST", "/api/v1/borrow-tickets/"+tk.ID+"/hand-over", officer, map[string]any{"version": ap.Version}), 200)
	if ho.State != "handed_over" {
		t.Errorf("hand over = %+v", ho)
	}
	if got := decode[withBorrowing](t, a.do("GET", "/api/v1/assets/"+x.ID, admin, nil), 200); got.Borrowing == nil || got.Borrowing.State != "on_loan" {
		t.Errorf("asset after hand over = %+v", got.Borrowing)
	}

	y := mk()
	lent := decode[withBorrowing](t, a.do("POST", "/api/v1/assets/"+y.ID+"/lend", officer, map[string]any{"borrower_id": uuid.NewString(), "version": y.Version}), 200)
	if lent.Borrowing == nil || lent.Borrowing.State != "on_loan" || lent.Borrowing.Ticket != nil {
		t.Errorf("direct lend = %+v", lent.Borrowing)
	}
	type loanPage struct {
		Items []struct{ Asset struct{ ID string } }
		Total int64
	}
	loans := decode[loanPage](t, a.do("GET", "/api/v1/loans?page_size=100", officer, nil), 200)
	seen := map[string]bool{}
	for _, l := range loans.Items {
		seen[l.Asset.ID] = true
	}
	if !seen[x.ID] || !seen[y.ID] {
		t.Errorf("loans = %+v, want both assets", loans)
	}
	type summary struct{ Waiting, Approved int64 }
	decode[summary](t, a.do("GET", "/api/v1/borrow-tickets/summary", officer, nil), 200)
	if rec := a.do("GET", "/api/v1/borrow-tickets/summary", empTok, nil); rec.Code != 403 {
		t.Errorf("employee summary = %d, want 403", rec.Code)
	}
	type ticketPage struct {
		Items []ticket
		Total int64
	}
	if mine := decode[ticketPage](t, a.do("GET", "/api/v1/borrow-tickets", empTok, nil), 200); mine.Total != 1 || mine.Items[0].ID != tk.ID {
		t.Errorf("employee's tickets = %+v, want only theirs", mine)
	}
	type activity struct{ Items []struct{ Kind string } }
	var kinds []string
	for _, e := range decode[activity](t, a.do("GET", "/api/v1/assets/"+x.ID+"/activity", admin, nil), 200).Items {
		kinds = append(kinds, e.Kind)
	}
	if !slices.Contains(kinds, "reserved") || !slices.Contains(kinds, "lent") {
		t.Errorf("activity kinds = %v, want reserved and lent", kinds)
	}
	// Trả tài sản (US-11): ngày trả là "hôm nay" của ứng dụng
	type businessDay struct{ Today string }
	day := decode[businessDay](t, a.do("GET", "/api/v1/borrowing/today", empTok, nil), 200).Today
	if mine := decode[loanPage](t, a.do("GET", "/api/v1/loans?mine=true", empTok, nil), 200); mine.Total != 1 || mine.Items[0].Asset.ID != x.ID {
		t.Errorf("employee's loans = %+v, want only x", mine)
	}
	if rec := a.do("GET", "/api/v1/loans", empTok, nil); rec.Code != 403 {
		t.Errorf("employee lists everyone's loans = %d, want 403", rec.Code)
	}
	cur := decode[assetDetail](t, a.do("GET", "/api/v1/assets/"+x.ID, admin, nil), 200)
	if rec := a.do("POST", "/api/v1/assets/"+x.ID+"/return", empTok, map[string]any{"returned_on": day, "version": cur.Version}); rec.Code != 403 {
		t.Errorf("employee records a return = %d, want 403", rec.Code)
	}
	back := decode[withBorrowing](t, a.do("POST", "/api/v1/assets/"+x.ID+"/return", officer, map[string]any{
		"returned_on": day, "status_id": domain.RepairStatusID.String(), "note": "loose hinge", "version": cur.Version,
	}), 200)
	if back.Borrowing != nil || back.Status.ID != domain.RepairStatusID.String() {
		t.Errorf("after return = %+v, want no borrowing and Under repair", back)
	}
	rec = a.do("POST", "/api/v1/assets/"+x.ID+"/return", officer, map[string]any{"returned_on": day, "version": cur.Version + 1})
	if typ, _ := problem(t, rec); rec.Code != 409 || typ != "/errors/no-active-loan" {
		t.Errorf("second return = %d %s, want 409 no-active-loan", rec.Code, typ)
	}
	type ticketLoan struct {
		Loan *struct {
			StartedOn  string `json:"started_on"`
			ReturnedOn string `json:"returned_on"`
		} `json:"loan"`
	}
	if v := decode[ticketLoan](t, a.do("GET", "/api/v1/borrow-tickets/"+tk.ID, empTok, nil), 200); v.Loan == nil || v.Loan.ReturnedOn != day {
		t.Errorf("ticket loan = %+v, want returned on %s", v.Loan, day)
	}
	type returnedEntry struct {
		Items []struct {
			Kind       string
			StatusName string `json:"status_name"`
		}
	}
	if es := decode[returnedEntry](t, a.do("GET", "/api/v1/assets/"+x.ID+"/activity", admin, nil), 200).Items; len(es) == 0 || es[0].Kind != "returned" || es[0].StatusName != "Under repair" {
		t.Errorf("latest activity = %+v, want returned · Under repair", es)
	}
	for _, p := range []string{"/api/v1/borrow-tickets", "/api/v1/loans", "/api/v1/borrowers", "/api/v1/borrow-tickets/summary"} {
		if rec := a.do("GET", p, "", nil); rec.Code != 401 {
			t.Errorf("GET %s without token = %d, want 401", p, rec.Code)
		}
	}
}
