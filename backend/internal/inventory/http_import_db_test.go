package inventory_test

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"storeit/internal/inventory/domain"
	"storeit/internal/inventory/spreadsheet"
)

// Import tài sản từ Excel (US-06) qua HTTP, trên Postgres thật: xem trước không ghi gì, xác nhận lưu từng
// dòng, tag tự sinh, cập nhật chỉ các cột trong file, lỗi cấp file, chặn nhập lại, quyền, file mẫu

// upload gửi file làm thân request (application/octet-stream), như frontend
func (a *app) upload(path, token string, data []byte) *httptest.ResponseRecorder {
	a.t.Helper()
	req := httptest.NewRequest("POST", path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/octet-stream")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	a.h.ServeHTTP(rec, req)
	return rec
}

func importBook(t *testing.T, sheet string, rows [][]any) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	_ = f.SetSheetName("Sheet1", sheet)
	for r, row := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, r+1)
		if err := f.SetSheetRow(sheet, cell, &row); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

type importReportDTO struct {
	FileName  string `json:"file_name"`
	Committed bool
	Sheets    []struct {
		Name     string
		TypeCode string `json:"type_code"`
		Rows     int
	}
	Counts struct{ Create, Update, Skip, Fail int }
	Rows   []struct {
		Sheet   string
		Row     int
		Tag     string
		Name    string
		Result  string
		Message string
		AssetID string `json:"asset_id"`
	}
}

func (r importReportDTO) row(n int) (res, msg, tag string) {
	for _, x := range r.Rows {
		if x.Row == n {
			return x.Result, x.Message, x.Tag
		}
	}
	return "", "ROW NOT FOUND", ""
}

func TestImportOverHTTP(t *testing.T) {
	a := newApp(t)
	tok := a.token(allPerms...)
	code := "IMP" + strings.ToUpper(uuid.NewString()[:6])
	typ := decode[typeDetail](t, a.do("POST", "/api/v1/asset-types", tok, map[string]any{
		"code": code, "name": "Importable " + code, "tag_prefix": code,
		"attributes": []map[string]any{
			{"key": "serial", "label": "Serial", "data_type": "text", "is_required": true},
			{"key": "ram_gb", "label": "RAM", "data_type": "number", "unit": "GB"},
			{"key": "os", "label": "OS", "data_type": "select", "options": []string{"Windows", "macOS"}},
		},
	}), 201)
	windows := typ.Attributes[2].Options[0].ID
	mk := func(tag, name string, ram float64) assetDetail {
		return decode[assetDetail](t, a.do("POST", "/api/v1/assets", tok, map[string]any{
			"tag": tag, "name": name, "asset_type_id": typ.ID,
			"attributes": map[string]any{"serial": "SN-" + tag, "ram_gb": ram, "os": windows},
		}), 201)
	}
	existing := mk(code+"-0001", "MacBook Air", 8)
	same := mk(code+"-0002", "Dell Latitude", 8)
	retired := mk(code+"-0003", "Old laptop", 8)
	decode[assetDetail](t, a.do("POST", "/api/v1/assets/"+retired.ID+"/retire", tok, map[string]any{"reason": "broken", "version": retired.Version}), 200)

	header := []any{"tag", "name", "status", "attr:serial", "attr:ram_gb", "attr:os"}
	book := importBook(t, code, [][]any{
		header,
		{existing.Tag, nil, nil, nil, 16, nil}, // 2 update: RAM 8 -> 16, the rest stays
		{strings.ToLower(same.Tag), "Dell Latitude", nil, nil, 8, "Windows"}, // 3 skip
		{nil, "ThinkPad T14", nil, "S9", 16, "macOS"},                        // 4 create, generated tag
		{code + "-0500", "Acer Swift", "Under repair", "S10", nil, nil},      // 5 create with its own tag
		{nil, "No serial", nil, nil, 8, nil},                                 // 6 fail
		{retired.Tag, "Old laptop", nil, nil, 16, nil},                       // 7 fail: retired
		{"DUP-1", "Twin", nil, "T", nil, nil},                                // 8 fail: twice
		{"dup-1", "Twin", nil, "T", nil, nil},                                // 9 fail: twice
	})

	count := func() int {
		var n int
		if err := a.pool.QueryRow(t.Context(), `SELECT count(*) FROM inventory.assets WHERE asset_type_id = $1`, typ.ID).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	before := count()

	t.Run("preview writes nothing", func(t *testing.T) {
		rep := decode[importReportDTO](t, a.upload("/api/v1/assets/import?mode=preview&file_name=assets.xlsx", tok, book), 200)
		if rep.Committed || rep.FileName != "assets.xlsx" || len(rep.Sheets) != 1 || rep.Sheets[0].TypeCode != code || rep.Sheets[0].Rows != 8 {
			t.Fatalf("report = %+v", rep)
		}
		if c := rep.Counts; c.Create != 2 || c.Update != 1 || c.Skip != 1 || c.Fail != 4 {
			t.Errorf("counts = %+v", c)
		}
		if res, msg, _ := rep.row(2); res != "update" || msg != "RAM 8 → 16 GB" {
			t.Errorf("row 2 = %s %q", res, msg)
		}
		if res, msg, _ := rep.row(7); res != "fail" || !strings.Contains(msg, "Retired") {
			t.Errorf("row 7 = %s %q", res, msg)
		}
		if count() != before {
			t.Error("a preview created assets")
		}
		var n int
		if err := a.pool.QueryRow(t.Context(), `SELECT count(*) FROM inventory.asset_imports`).Scan(&n); err != nil || n != 0 {
			t.Errorf("a preview wrote %d import history rows (err %v)", n, err)
		}
	})

	t.Run("commit saves good rows and reports the rest", func(t *testing.T) {
		rep := decode[importReportDTO](t, a.upload("/api/v1/assets/import?mode=commit&file_name=assets.xlsx", tok, book), 200)
		if !rep.Committed || rep.Counts.Create != 2 || rep.Counts.Update != 1 || rep.Counts.Fail != 4 {
			t.Fatalf("report = %+v", rep)
		}
		if got := count() - before; got != 2 {
			t.Fatalf("created %d assets, want 2", got)
		}
		// 0001..0003 exist and 0500 is typed, so the generated tag is one past the highest: 0501.
		// Row 4 comes before row 5 in the file but the typed tag is only created after it, so
		// the generated one is 0004 when the repository counts up from the highest existing tag.
		res, msg, genTag := rep.row(4)
		if res != "create" || msg != "Created" || !strings.HasPrefix(genTag, code+"-") || genTag == code+"-0500" {
			t.Errorf("row 4 = %s %q %q: want a created asset with a generated tag", res, msg, genTag)
		}
		if res, _, tag := rep.row(5); res != "create" || tag != code+"-0500" {
			t.Errorf("row 5 = %s %s", res, tag)
		}
		got := decode[assetDetail](t, a.do("GET", "/api/v1/assets/"+existing.ID, tok, nil), 200)
		if got.Name != "MacBook Air" || got.Version != existing.Version+1 {
			t.Errorf("updated asset = %q v%d: blank cells keep the name", got.Name, got.Version)
		}
		for _, at := range got.Attributes {
			if at.Key == "ram_gb" && at.Value != float64(16) {
				t.Errorf("ram = %v, want 16", at.Value)
			}
			if at.Key == "serial" && at.Value != "SN-"+existing.Tag {
				t.Errorf("serial = %v: its cell was blank, so it stays", at.Value)
			}
		}
		same2 := decode[assetDetail](t, a.do("GET", "/api/v1/assets/"+same.ID, tok, nil), 200)
		if same2.Version != same.Version {
			t.Error("a skipped row must not touch the asset")
		}
		var rows, created int
		if err := a.pool.QueryRow(t.Context(), `SELECT count(*), coalesce(sum(created_count), 0) FROM inventory.asset_imports WHERE file_name = 'assets.xlsx'`).Scan(&rows, &created); err != nil || rows != 1 || created != 2 {
			t.Errorf("history = %d rows, %d created (err %v)", rows, created, err)
		}
	})

	t.Run("the same file can't create its assets twice", func(t *testing.T) {
		n := count()
		wantProblem(t, "second commit", a.upload("/api/v1/assets/import?mode=commit&file_name=assets.xlsx", tok, book), 409, "/errors/import-already-done")
		if count() != n {
			t.Error("the refused import created assets anyway")
		}
	})
}

func TestImportFileErrorsOverHTTP(t *testing.T) {
	a := newApp(t)
	tok := a.token(allPerms...)
	mode := "/api/v1/assets/import?mode=preview"
	oneRow := importBook(t, "GENERAL", [][]any{{"tag", "name"}, {nil, "x"}})

	wantProblem(t, "csv", a.upload(mode, tok, []byte("tag,name\nA,B\n")), 422, "/errors/invalid-import-file")
	_, fields := problem(t, a.upload(mode, tok, importBook(t, "Sheet1", [][]any{{"tag", "name"}, {nil, "x"}})))
	if len(fields) != 1 || !strings.HasPrefix(fields[0], "Sheet1: isn't an asset type code") {
		t.Errorf("unknown sheet fields = %v", fields)
	}
	if rec := a.upload(mode, "", oneRow); rec.Code != 401 {
		t.Errorf("no token = %d, want 401", rec.Code)
	}
	if rec := a.upload(mode, a.token(domain.PermAssetRead, domain.PermAssetExport), oneRow); rec.Code != 403 {
		t.Errorf("reader = %d, want 403", rec.Code)
	}

	// mode thiếu hay sai: bộ kiểm OpenAPI chặn trước khi vào service
	if rec := a.upload("/api/v1/assets/import", tok, []byte("x")); rec.Code != 400 && rec.Code != 422 {
		t.Errorf("missing mode = %d", rec.Code)
	}
	if rec := a.upload("/api/v1/assets/import?mode=oops", tok, []byte("x")); rec.Code != 400 && rec.Code != 422 {
		t.Errorf("bad mode = %d", rec.Code)
	}
	// quá giới hạn thân request của riêng route này: bị từ chối, không đọc hết vào bộ nhớ
	big := bytes.Repeat([]byte{1}, domain.MaxImportBytes+8192)
	if rec := a.upload(mode, tok, big); rec.Code != 413 && rec.Code != 422 {
		t.Errorf("oversized body = %d, want 413 or 422", rec.Code)
	}
}

func TestImportTemplateOverHTTP(t *testing.T) {
	a := newApp(t)
	tok := a.token(allPerms...)
	rec := a.do("GET", "/api/v1/assets/import/template", tok, nil)
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Type"), "spreadsheetml") || !strings.Contains(rec.Header().Get("Content-Disposition"), "storeit-import-template.xlsx") {
		t.Fatalf("template = %d %q %q", rec.Code, rec.Header().Get("Content-Type"), rec.Header().Get("Content-Disposition"))
	}
	sheets, err := spreadsheet.ReadWorkbook(bytes.NewReader(rec.Body.Bytes()))
	if err != nil || len(sheets) == 0 {
		t.Fatalf("template unreadable: %v", err)
	}
	var general bool
	for _, s := range sheets {
		general = general || (s.Name == "GENERAL" && strings.HasPrefix(strings.Join(s.Header, ","), "tag,name,description,status,purchase_date"))
	}
	if !general {
		t.Errorf("no GENERAL sheet with the standard columns in %d sheets", len(sheets))
	}
	if rec := a.do("GET", "/api/v1/assets/import/template", a.token(domain.PermAssetRead), nil); rec.Code != 403 {
		t.Errorf("template without permission = %d, want 403", rec.Code)
	}
}
