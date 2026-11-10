package inventory_test

import (
	"fmt"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"storeit/internal/inventory/domain"
)

// Hồ sơ export (US-09) qua HTTP: lưu và đọc lại, kiểm đầu vào, luật tên, quyền sở hữu/chia sẻ, dùng lại để export

type profileDTO struct {
	ID      string
	Name    string
	Shared  bool
	Owner   struct{ ID, Name string }
	CanEdit bool `json:"can_edit"`
	Version int32
	Layout  struct {
		Columns []struct {
			Field  string
			Header string
			Width  float64
		}
		SheetName string `json:"sheet_name"`
		Header    string
		TitleRow  bool `json:"title_row"`
	}
}

type profileList struct{ Items []profileDTO }

// reportLayout dựng layout đủ trường bắt buộc: cột theo thứ tự cho trước, sheet đơn, không dòng tiêu đề
func reportLayout(sheet string, cols ...map[string]any) map[string]any {
	return map[string]any{
		"columns": append([]map[string]any{}, cols...), "sheets": "single", "sheet_name": sheet,
		"title_row": false, "summary": false, "header": "bold", "freeze": true, "filter": true, "stripes": false,
		"date_format": "dd/mm/yyyy", "bool_style": "yes_no", "status_as": "name", "unit_in": "header",
	}
}

func col(field string, extra ...any) map[string]any {
	c := map[string]any{"field": field}
	if len(extra) > 0 {
		c["header"] = extra[0]
	}
	if len(extra) > 1 {
		c["width"] = extra[1]
	}
	return c
}

func wantProblem(t *testing.T, what string, rec *httptest.ResponseRecorder, status int, typ string) {
	t.Helper()
	got, _ := problem(t, rec)
	if rec.Code != status || got != typ {
		body := rec.Body.String()
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "json") { // export thành công trả file xlsx, không in nhị phân
			body = fmt.Sprintf("<%s, %d bytes>", ct, rec.Body.Len())
		}
		t.Errorf("%s = %d %q, want %d %q: %s", what, rec.Code, got, status, typ, body)
	}
}

// wantInvalid: bộ kiểm schema của OpenAPI từ chối trước khi tới service (422 validation-failed, nêu field)
func wantInvalid(t *testing.T, what string, rec *httptest.ResponseRecorder, field string) {
	t.Helper()
	typ, fields := problem(t, rec)
	if rec.Code != 422 || typ != "/errors/validation-failed" || len(fields) != 1 || !strings.HasPrefix(fields[0], field+":") {
		t.Errorf("%s = %d %q %v, want 422 validation-failed on %s", what, rec.Code, typ, fields, field)
	}
}

func TestExportProfilesOverHTTP(t *testing.T) {
	a := newApp(t)
	perms := []string{domain.PermAssetRead, domain.PermAssetExport}
	uniq := func(prefix string) string { return prefix + " " + uuid.NewString()[:8] }

	// Lưu rồi đọc lại đúng thứ tự cột, tên có dấu; sửa layout thì phiên bản tăng và lần đọc sau thấy bản mới
	t.Run("persistence", func(t *testing.T) {
		owner := uuid.New()
		a.names[owner] = "Lan"
		tok := a.tokenFor(owner, perms...)
		name := uniq("Kiểm kê quý 3")
		lay := reportLayout("Báo cáo", col("name", "Tên máy"), col("tag", "Mã", 18), col("attr:ram_gb"))
		lay["title_row"] = true
		p := decode[profileDTO](t, a.do("POST", "/api/v1/export-profiles", tok, map[string]any{"name": name, "layout": lay}), 201)
		if p.Version != 1 || p.Shared || !p.CanEdit || p.Owner.ID != owner.String() || p.Owner.Name != "Lan" {
			t.Errorf("created = %+v, want version 1, private, editable, owned by Lan", p)
		}
		got := decode[profileDTO](t, a.do("GET", "/api/v1/export-profiles/"+p.ID, tok, nil), 200)
		fields := []string{}
		for _, c := range got.Layout.Columns {
			fields = append(fields, c.Field)
		}
		if got.Name != name || !slices.Equal(fields, []string{"name", "tag", "attr:ram_gb"}) {
			t.Errorf("read back = %q %v, want the saved name and column order", got.Name, fields)
		}
		if c := got.Layout.Columns[0]; c.Header != "Tên máy" || got.Layout.Columns[1].Width != 18 || got.Layout.SheetName != "Báo cáo" || !got.Layout.TitleRow {
			t.Errorf("read back layout = %+v", got.Layout)
		}

		// chỉ đổi tên: layout giữ nguyên
		renamed := name + " (cũ)"
		up := decode[profileDTO](t, a.do("PATCH", "/api/v1/export-profiles/"+p.ID, tok, map[string]any{"name": renamed, "version": p.Version}), 200)
		if up.Version != 2 || up.Name != renamed || len(up.Layout.Columns) != 3 {
			t.Errorf("rename = %+v, want version 2 with the layout untouched", up)
		}
		// đổi layout
		lay2 := reportLayout("Quý 4", col("tag"))
		up = decode[profileDTO](t, a.do("PATCH", "/api/v1/export-profiles/"+p.ID, tok, map[string]any{"layout": lay2, "version": up.Version}), 200)
		got = decode[profileDTO](t, a.do("GET", "/api/v1/export-profiles/"+p.ID, tok, nil), 200)
		if up.Version != 3 || got.Version != 3 || got.Name != renamed || len(got.Layout.Columns) != 1 || got.Layout.SheetName != "Quý 4" {
			t.Errorf("after layout change = %+v, want version 3, new layout, same name", got)
		}
		// danh sách có profile, kèm quyền sửa của người xem
		found := false
		for _, x := range decode[profileList](t, a.do("GET", "/api/v1/export-profiles", tok, nil), 200).Items {
			found = found || (x.ID == p.ID && x.CanEdit)
		}
		if !found {
			t.Error("the owner's list lacks the profile")
		}
	})

	// Đầu vào sai bị từ chối và không lưu gì; tên 100 ký tự có dấu vẫn được (đếm ký tự, không đếm byte)
	t.Run("validation", func(t *testing.T) {
		tok := a.token(perms...)
		good := reportLayout("S", col("tag"))
		post := func(body map[string]any) *httptest.ResponseRecorder {
			return a.do("POST", "/api/v1/export-profiles", tok, body)
		}
		wantProblem(t, "blank name", post(map[string]any{"name": "   ", "layout": good}), 422, "/errors/invalid-label")
		wantProblem(t, "control character", post(map[string]any{"name": "a\u0007b", "layout": good}), 422, "/errors/invalid-label")
		wantInvalid(t, "empty name", post(map[string]any{"name": "", "layout": good}), "name")
		wantInvalid(t, "101 characters", post(map[string]any{"name": strings.Repeat("a", 101), "layout": good}), "name")
		wantInvalid(t, "no name", post(map[string]any{"layout": good}), "name")
		wantInvalid(t, "no layout", post(map[string]any{"name": uniq("x")}), "layout")

		bad := func(what string, lay map[string]any, field string) {
			t.Helper()
			rec := post(map[string]any{"name": uniq("bad"), "layout": lay})
			typ, fields := problem(t, rec)
			if rec.Code != 422 || typ != "/errors/invalid-export-layout" || len(fields) == 0 || !strings.HasPrefix(fields[0], field) {
				t.Errorf("%s = %d %s %v, want 422 invalid-export-layout on %s", what, rec.Code, typ, fields, field)
			}
		}
		bad("unknown column", reportLayout("S", col("nope")), "layout.columns[0].field")
		bad("repeated column", reportLayout("S", col("tag"), col("tag")), "layout.columns[1].field")
		bad("no columns", reportLayout("S"), "layout.columns")
		bad("sheet name with a slash", reportLayout("a/b", col("tag")), "layout.sheet_name")
		bad("header with a control character", reportLayout("S", col("tag", "x\u0001")), "layout.columns[0].header")
		bad("width out of range", reportLayout("S", col("tag", "H", 3)), "layout.columns[0].width")
		fancy := reportLayout("S", col("tag"))
		fancy["header"] = "fancy"
		wantInvalid(t, "unknown header style", post(map[string]any{"name": uniq("x"), "layout": fancy}), "layout.header")

		long := strings.Repeat("ạ", 100) // 100 ký tự, 300 byte
		ok := decode[profileDTO](t, post(map[string]any{"name": long, "layout": good}), 201)
		if ok.Name != long {
			t.Errorf("100-character name stored as %q", ok.Name)
		}

		// sửa: không có version là sai; layout không hợp lệ không đổi bản đang có
		wantInvalid(t, "patch without version", a.do("PATCH", "/api/v1/export-profiles/"+ok.ID, tok, map[string]any{"name": "y"}), "version")
		wantProblem(t, "patch with a bad layout", a.do("PATCH", "/api/v1/export-profiles/"+ok.ID, tok, map[string]any{
			"layout": reportLayout("S", col("nope")), "version": ok.Version,
		}), 422, "/errors/invalid-export-layout")
		wantProblem(t, "patch with a blank name", a.do("PATCH", "/api/v1/export-profiles/"+ok.ID, tok, map[string]any{"name": " ", "version": ok.Version}), 422, "/errors/invalid-label")
		if cur := decode[profileDTO](t, a.do("GET", "/api/v1/export-profiles/"+ok.ID, tok, nil), 200); cur.Version != ok.Version || cur.Name != long {
			t.Errorf("profile after refused edits = %+v, want it untouched", cur)
		}
		wantProblem(t, "unknown profile", a.do("GET", "/api/v1/export-profiles/"+uuid.NewString(), tok, nil), 404, "/errors/export-profile-not-found")
		if rec := a.do("GET", "/api/v1/export-profiles/nope", tok, nil); rec.Code != 422 && rec.Code != 400 {
			t.Errorf("profile id that is not a uuid = %d, want a 4xx", rec.Code)
		}
	})

	// Tên: duy nhất theo chủ, không phân biệt hoa thường và khoảng trắng hai đầu; hai chủ trùng tên thì được
	t.Run("names", func(t *testing.T) {
		tok, other := a.token(perms...), a.token(perms...)
		lay := reportLayout("S", col("tag"))
		name := uniq("Monthly")
		first := decode[profileDTO](t, a.do("POST", "/api/v1/export-profiles", tok, map[string]any{"name": name, "layout": lay}), 201)
		for _, dup := range []string{name, strings.ToUpper(name), "  " + strings.ToLower(name) + "  "} {
			wantProblem(t, "duplicate "+dup, a.do("POST", "/api/v1/export-profiles", tok, map[string]any{"name": dup, "layout": lay}), 409, "/errors/export-profile-name-taken")
		}
		// chữ có dấu: hoa/thường cũng trùng (lower() của Postgres trong image của compose)
		vi := uniq("Kiểm kê")
		decode[profileDTO](t, a.do("POST", "/api/v1/export-profiles", tok, map[string]any{"name": vi, "layout": lay}), 201)
		wantProblem(t, "duplicate with other case in Vietnamese", a.do("POST", "/api/v1/export-profiles", tok, map[string]any{"name": strings.ToUpper(vi), "layout": lay}), 409, "/errors/export-profile-name-taken")
		if rec := a.do("POST", "/api/v1/export-profiles", other, map[string]any{"name": name, "layout": lay}); rec.Code != 201 {
			t.Errorf("same name for another owner = %d %s, want 201", rec.Code, rec.Body)
		}
		second := decode[profileDTO](t, a.do("POST", "/api/v1/export-profiles", tok, map[string]any{"name": uniq("Weekly"), "layout": lay}), 201)
		wantProblem(t, "rename into a taken name", a.do("PATCH", "/api/v1/export-profiles/"+second.ID, tok, map[string]any{"name": strings.ToUpper(name), "version": second.Version}), 409, "/errors/export-profile-name-taken")
		// đổi lại đúng tên của chính nó (khác hoa thường) thì được
		if rec := a.do("PATCH", "/api/v1/export-profiles/"+first.ID, tok, map[string]any{"name": strings.ToUpper(name), "version": first.Version}); rec.Code != 200 {
			t.Errorf("recase its own name = %d %s, want 200", rec.Code, rec.Body)
		}
		// phiên bản cũ: 409, không ghi đè
		wantProblem(t, "stale version", a.do("PATCH", "/api/v1/export-profiles/"+second.ID, tok, map[string]any{"name": uniq("Late"), "version": second.Version + 5}), 409, "/errors/export-profile-changed")
	})

	// Quyền: riêng tư thì chỉ chủ thấy; chia sẻ thì ai export được cũng thấy và dùng được nhưng không sửa/xoá;
	// người có quyền quản lý sửa được profile chia sẻ, không thấy profile riêng của người khác
	t.Run("ownership and sharing", func(t *testing.T) {
		ownerID := uuid.New()
		a.names[ownerID] = "Hà"
		owner := a.tokenFor(ownerID, perms...)
		other := a.token(perms...)
		manager := a.token(append(slices.Clone(perms), domain.PermExportProfileManage)...)
		lay := reportLayout("S", col("tag"))
		p := decode[profileDTO](t, a.do("POST", "/api/v1/export-profiles", owner, map[string]any{"name": uniq("Riêng"), "layout": lay}), 201)
		path := "/api/v1/export-profiles/" + p.ID

		inList := func(tok string) bool {
			for _, x := range decode[profileList](t, a.do("GET", "/api/v1/export-profiles", tok, nil), 200).Items {
				if x.ID == p.ID {
					return true
				}
			}
			return false
		}
		invisible := func(who, tok string) {
			t.Helper()
			wantProblem(t, who+" reads a private profile", a.do("GET", path, tok, nil), 404, "/errors/export-profile-not-found")
			wantProblem(t, who+" edits a private profile", a.do("PATCH", path, tok, map[string]any{"name": uniq("x"), "version": p.Version}), 404, "/errors/export-profile-not-found")
			wantProblem(t, who+" deletes a private profile", a.do("DELETE", path, tok, nil), 404, "/errors/export-profile-not-found")
			wantProblem(t, who+" restores a private profile", a.do("POST", path+"/restore", tok, nil), 404, "/errors/export-profile-not-found")
			wantProblem(t, who+" exports with a private profile", a.do("POST", "/api/v1/assets/export", tok, map[string]any{"mode": "report", "profile_id": p.ID}), 404, "/errors/export-profile-not-found")
			if inList(tok) {
				t.Errorf("%s lists someone else's private profile", who)
			}
		}
		invisible("another user", other)
		invisible("a manager", manager)
		if !inList(owner) {
			t.Error("the owner's list lacks the private profile")
		}

		// chia sẻ: thấy được, không sửa được
		shared := decode[profileDTO](t, a.do("PATCH", path, owner, map[string]any{"shared": true, "version": p.Version}), 200)
		if !shared.Shared || shared.Version != 2 {
			t.Fatalf("shared = %+v", shared)
		}
		seen := decode[profileDTO](t, a.do("GET", path, other, nil), 200)
		if seen.CanEdit || seen.Owner.ID != ownerID.String() || seen.Owner.Name != "Hà" || !inList(other) {
			t.Errorf("shared profile for another user = %+v, listed %v; want visible, not editable, owned by Hà", seen, inList(other))
		}
		wantProblem(t, "non-owner edits a shared profile", a.do("PATCH", path, other, map[string]any{"name": uniq("Hijack"), "version": shared.Version}), 403, "/errors/export-profile-forbidden")
		wantProblem(t, "non-owner deletes a shared profile", a.do("DELETE", path, other, nil), 403, "/errors/export-profile-forbidden")
		if m := decode[profileDTO](t, a.do("GET", path, manager, nil), 200); !m.CanEdit {
			t.Error("a manager should be able to edit a shared profile")
		}
		if rec := a.do("PATCH", path, manager, map[string]any{"name": uniq("Chuẩn hoá"), "version": shared.Version}); rec.Code != 200 {
			t.Errorf("manager edits a shared profile = %d %s, want 200", rec.Code, rec.Body)
		}

		// bỏ chia sẻ: người khác lại không thấy
		cur := decode[profileDTO](t, a.do("GET", path, owner, nil), 200)
		decode[profileDTO](t, a.do("PATCH", path, owner, map[string]any{"shared": false, "version": cur.Version}), 200)
		invisible("another user after unsharing", other)

		// không có quyền export: mọi thao tác bị chặn; chưa đăng nhập: 401
		noExport := a.token(domain.PermAssetRead)
		for _, c := range []struct{ method, path string }{
			{"GET", "/api/v1/export-profiles"}, {"POST", "/api/v1/export-profiles"}, {"GET", path},
			{"PATCH", path}, {"DELETE", path}, {"POST", path + "/restore"},
		} {
			var body any
			if c.method == "POST" && c.path == "/api/v1/export-profiles" {
				body = map[string]any{"name": "x", "layout": lay}
			} else if c.method == "PATCH" {
				body = map[string]any{"name": "x", "version": 1}
			}
			if rec := a.do(c.method, c.path, noExport, body); rec.Code != 403 {
				t.Errorf("%s %s without asset.export = %d, want 403", c.method, c.path, rec.Code)
			}
			if rec := a.do(c.method, c.path, "", body); rec.Code != 401 {
				t.Errorf("%s %s without a token = %d, want 401", c.method, c.path, rec.Code)
			}
		}
	})

	// Dùng lại: người khác export bằng profile chia sẻ ra file theo layout của chủ; chủ đổi layout thì lần export sau theo bản mới
	t.Run("reuse", func(t *testing.T) {
		admin := a.token(allPerms...)
		code := "PR" + strings.ToUpper(uuid.NewString()[:6])
		typ := decode[typeDetail](t, a.do("POST", "/api/v1/asset-types", admin, map[string]any{"code": code, "name": "Profile " + code}), 201)
		tag := code + "-1"
		decode[assetDetail](t, a.do("POST", "/api/v1/assets", admin, map[string]any{"tag": tag, "name": "Máy chiếu", "asset_type_id": typ.ID}), 201)

		owner, other := a.token(perms...), a.token(perms...)
		p := decode[profileDTO](t, a.do("POST", "/api/v1/export-profiles", owner, map[string]any{
			"name": uniq("Dùng chung"), "shared": true, "layout": reportLayout("Bản một", col("tag", "Mã tài sản"), col("name")),
		}), 201)
		exportRows := func(tok, sheet string) [][]string {
			t.Helper()
			rec := a.do("POST", "/api/v1/assets/export", tok, map[string]any{"mode": "report", "profile_id": p.ID, "filters": map[string]any{"type_id": typ.ID}})
			if rec.Code != 200 {
				t.Fatalf("export with the profile = %d %s", rec.Code, rec.Body)
			}
			x, err := excelize.OpenReader(rec.Body)
			if err != nil {
				t.Fatal(err)
			}
			rows, err := x.GetRows(sheet)
			if err != nil {
				t.Fatalf("sheet %q: %v (sheets %v)", sheet, err, x.GetSheetList())
			}
			return rows
		}
		rows := exportRows(other, "Bản một")
		if len(rows) != 2 || !slices.Equal(rows[0], []string{"Mã tài sản", "Name"}) || rows[1][0] != strings.ToUpper(tag) || rows[1][1] != "Máy chiếu" {
			t.Errorf("another user's export = %q, want the owner's header, columns and the asset", rows)
		}

		decode[profileDTO](t, a.do("PATCH", "/api/v1/export-profiles/"+p.ID, owner, map[string]any{
			"layout": reportLayout("Bản hai", col("name"), col("tag")), "version": p.Version,
		}), 200)
		rows = exportRows(other, "Bản hai")
		if len(rows) != 2 || !slices.Equal(rows[0], []string{"Name", "Tag"}) || rows[1][0] != "Máy chiếu" {
			t.Errorf("export after the owner changed the layout = %q, want the new sheet and column order", rows)
		}
	})
}
