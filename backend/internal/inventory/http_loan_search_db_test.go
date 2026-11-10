package inventory_test

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"storeit/internal/inventory/domain"
)

// Tìm kiếm và lọc khoản mượn (US-20) qua HTTP: trạng thái, loại, từ khoá, kỳ, sắp xếp, phân trang,
// trường hiển thị, và quyền (người mượn chỉ thấy khoản của mình dù lọc thế nào)

type loanItem struct {
	ID            string
	Asset         struct{ ID, Tag, Name string }
	AssetTypeName string `json:"asset_type_name"`
	Borrower      struct{ ID, Name string }
	TicketNumber  int64   `json:"ticket_number"`
	StartedOn     string  `json:"started_on"`
	ReturnBy      *string `json:"return_by"`
	OverdueDays   int     `json:"overdue_days"`
	ReturnedOn    *string `json:"returned_on"`
}

type loanResult struct {
	Items []loanItem
	Total int64
}

func TestLoanSearchOverHTTP(t *testing.T) {
	// đồng hồ đóng băng: "hôm nay" không đổi suốt test, kể cả khi chạy qua nửa đêm
	frozen := time.Now()
	a := newAppClock(t, 0, func() time.Time { return frozen })
	admin := a.token(allPerms...)
	officer := a.token(domain.PermAssetRead, domain.PermBorrowRequest, domain.PermBorrowManage)
	e1, e2 := uuid.New(), uuid.New()
	a.names[e1], a.names[e2] = "Nguyễn Văn An", "Trần Thị Bình"
	emp1 := a.tokenFor(e1, domain.PermAssetRead, domain.PermBorrowRequest)
	emp2 := a.tokenFor(e2, domain.PermAssetRead, domain.PermBorrowRequest)

	day := decode[struct{ Today string }](t, a.do("GET", "/api/v1/borrowing/today", officer, nil), 200).Today
	base, err := time.Parse(time.DateOnly, day)
	if err != nil {
		t.Fatal(err)
	}
	d := func(offset int) string { return base.AddDate(0, 0, offset).Format(time.DateOnly) }

	// Hai loại, mỗi loại vài tài sản; DB dùng chung với test khác nên mọi kết quả cấp quản lý đều khoanh theo loại
	mkType := func(prefix string) typeDetail {
		code := prefix + strings.ToUpper(uuid.NewString()[:6])
		return decode[typeDetail](t, a.do("POST", "/api/v1/asset-types", admin, map[string]any{"code": code, "name": "Loại " + code, "can_borrow": true}), 201)
	}
	typeA, typeB := mkType("LA"), mkType("LB")
	mkAsset := func(ty typeDetail, n int, name string) assetDetail {
		return decode[assetDetail](t, a.do("POST", "/api/v1/assets", admin, map[string]any{
			"tag": fmt.Sprintf("%s-%d", ty.Code, n), "name": name, "asset_type_id": ty.ID,
		}), 201)
	}
	a1, a2, a3 := mkAsset(typeA, 1, "Zebra Printer"), mkAsset(typeA, 2, "Epson Projector"), mkAsset(typeA, 3, "Dell Laptop")
	b1, b2 := mkAsset(typeB, 1, "Epson Scanner"), mkAsset(typeB, 2, "Canon Camera")

	lend := func(x assetDetail, who uuid.UUID, started, returnBy int, hasDue bool) {
		t.Helper()
		decode[assetDetail](t, a.do("POST", "/api/v1/assets/"+x.ID+"/lend", officer, map[string]any{"borrower_id": who.String(), "version": x.Version}), 200)
		var due any
		if hasDue {
			due = d(returnBy)
		}
		// lùi ngày: API chỉ cho mượn từ hôm nay
		if _, err := a.pool.Exec(context.Background(), `UPDATE inventory.loans SET started_on = $2::date, return_by = $3::date WHERE asset_id = $1 AND returned_on IS NULL`, x.ID, d(started), due); err != nil {
			t.Fatal(err)
		}
	}
	giveBack := func(x assetDetail) {
		t.Helper()
		cur := decode[assetDetail](t, a.do("GET", "/api/v1/assets/"+x.ID, admin, nil), 200)
		decode[assetDetail](t, a.do("POST", "/api/v1/assets/"+x.ID+"/return", officer, map[string]any{"returned_on": day, "version": cur.Version}), 200)
	}
	lend(a1, e1, -10, -5, true) // quá hạn 5 ngày
	lend(a2, e1, -3, 0, true)   // hạn trả là hôm nay: chưa quá hạn
	lend(a3, e2, -2, 0, false)  // không hạn
	lend(b1, e1, -8, -4, true)
	giveBack(b1) // trả hôm nay, hạn là 4 ngày trước: trả muộn
	lend(b2, e2, -6, 0, true)
	giveBack(b2) // trả hôm nay đúng hạn: không muộn

	get := func(t *testing.T, tok, query string) loanResult {
		t.Helper()
		return decode[loanResult](t, a.do("GET", "/api/v1/loans?"+query, tok, nil), 200)
	}
	assetsOf := func(r loanResult) []string {
		var out []string
		for _, l := range r.Items {
			out = append(out, l.Asset.ID)
		}
		return out
	}
	// want: đúng tập tài sản (không xét thứ tự) và total khớp
	wantSet := func(t *testing.T, what string, r loanResult, want ...assetDetail) {
		t.Helper()
		got := assetsOf(r)
		var ids []string
		for _, x := range want {
			ids = append(ids, x.ID)
		}
		slices.Sort(got)
		slices.Sort(ids)
		if !slices.Equal(got, ids) || r.Total != int64(len(want)) {
			t.Errorf("%s: got %d loans (total %d), want %d", what, len(got), r.Total, len(want))
		}
	}
	wantOrder := func(t *testing.T, what string, r loanResult, want ...assetDetail) {
		t.Helper()
		var ids []string
		for _, x := range want {
			ids = append(ids, x.ID)
		}
		if got := assetsOf(r); !slices.Equal(got, ids) {
			t.Errorf("%s: order = %v, want %v", what, got, ids)
		}
	}
	// contains: dùng khi không khoanh được theo loại (kết quả có thể kèm dữ liệu của test khác)
	wantHas := func(t *testing.T, what string, r loanResult, in []assetDetail, out []assetDetail) {
		t.Helper()
		got := assetsOf(r)
		for _, x := range in {
			if !slices.Contains(got, x.ID) {
				t.Errorf("%s: missing %s", what, x.Tag)
			}
		}
		for _, x := range out {
			if slices.Contains(got, x.ID) {
				t.Errorf("%s: should not list %s", what, x.Tag)
			}
		}
	}
	A, B := "&type_id="+typeA.ID, "&type_id="+typeB.ID

	t.Run("status", func(t *testing.T) {
		wantSet(t, "overdue", get(t, officer, "state=overdue"+A), a1)
		wantSet(t, "active", get(t, officer, "state=active"+A), a1, a2, a3)
		wantSet(t, "returned", get(t, officer, "state=returned"+B), b1, b2)
		wantSet(t, "returned late", get(t, officer, "state=returned_late"+B), b1)
		wantSet(t, "all of type A", get(t, officer, "state=all"+A), a1, a2, a3)
		wantSet(t, "all of type B", get(t, officer, "state=all"+B), b1, b2)
		wantSet(t, "default state is active, not overdue", get(t, officer, "page_size=200"+A), a1, a2, a3)
		wantHas(t, "returned late across types", get(t, officer, "state=returned_late&page_size=200"), []assetDetail{b1}, []assetDetail{a1, a2, a3, b2})
		if r := get(t, officer, "state=all&type_id="+uuid.NewString()); len(r.Items) != 0 || r.Total != 0 {
			t.Errorf("unknown type = %d loans, want an empty page", len(r.Items))
		}
	})

	t.Run("search", func(t *testing.T) {
		tagA2 := strings.ToLower(a2.Tag)
		wantSet(t, "tag, lower case", get(t, officer, "state=all&q="+url.QueryEscape(tagA2)+A), a2)
		wantSet(t, "name part", get(t, officer, "state=all&q="+url.QueryEscape("EBRA pri")+A), a1)
		wantSet(t, "trimmed", get(t, officer, "state=all&q="+url.QueryEscape("  epson ")+A), a2)
		wantSet(t, "text AND type", get(t, officer, "state=all&q=epson"+B), b1)
		wantHas(t, "name across types", get(t, officer, "state=all&q=epson&page_size=200"), []assetDetail{a2, b1}, []assetDetail{a1, a3, b2})
		wantSet(t, "borrower name", get(t, officer, "state=all&q="+url.QueryEscape("văn an")+A), a1, a2)
		wantSet(t, "borrower name, upper case", get(t, officer, "state=all&q="+url.QueryEscape("BÌNH")+B), b2)
		wantSet(t, "borrower AND status", get(t, officer, "state=overdue&q="+url.QueryEscape("văn an")+A), a1)
		wantSet(t, "no match", get(t, officer, "state=all&q=zzz-no-such-thing"+A))
		// % và _ là chữ thường, không phải ký tự đại diện của LIKE
		wantSet(t, "percent sign", get(t, officer, "state=all&q=%25"+A))
		wantSet(t, "underscore", get(t, officer, "state=all&q=_"+A))
	})

	t.Run("period, sort and paging", func(t *testing.T) {
		wantSet(t, "out before the 9th day back", get(t, officer, "state=all&to="+d(-9)+A), a1)
		wantSet(t, "period starting today keeps open and just-returned loans", get(t, officer, "state=all&from="+day+A), a1, a2, a3)
		wantSet(t, "period starting today, type B (returned today)", get(t, officer, "state=all&from="+day+B), b1, b2)
		wantSet(t, "period that ended before any loan", get(t, officer, "state=all&to="+d(-30)+A))
		wantSet(t, "all four filters", get(t, officer, "state=returned_late&q=epson&from="+d(-9)+"&to="+day+B), b1)
		wantSet(t, "all four filters, other asset", get(t, officer, "state=returned_late&q=canon&from="+d(-9)+"&to="+day+B))

		wantOrder(t, "overdue first by default", get(t, officer, "state=active"+A), a1, a2, a3)
		wantOrder(t, "history newest first by default", get(t, officer, "state=returned"+B), b2, b1)
		wantOrder(t, "started_on descending", get(t, officer, "state=active&sort=-started_on"+A), a3, a2, a1)
		wantOrder(t, "return_by ascending, no due date last", get(t, officer, "state=active&sort=return_by"+A), a1, a2, a3)
		wantOrder(t, "return_by descending, no due date last", get(t, officer, "state=active&sort=-return_by"+A), a2, a1, a3)
		wantOrder(t, "started_on ascending", get(t, officer, "state=all&sort=started_on"+A), a1, a2, a3)

		p1, p2 := get(t, officer, "state=all&page_size=2&page=1"+A), get(t, officer, "state=all&page_size=2&page=2"+A)
		if len(p1.Items) != 2 || len(p2.Items) != 1 || p1.Total != 3 || p2.Total != 3 {
			t.Fatalf("paging = %d + %d items, totals %d/%d; want 2 + 1 of 3", len(p1.Items), len(p2.Items), p1.Total, p2.Total)
		}
		if slices.Contains(assetsOf(p1), p2.Items[0].Asset.ID) {
			t.Error("the second page repeats a loan of the first")
		}
	})

	// Dòng kết quả mang đủ thứ cần để đọc mà không mở thêm: tài sản, loại, người mượn, ngày, số ngày quá hạn, ngày trả
	t.Run("result fields", func(t *testing.T) {
		byAsset := map[string]loanItem{}
		for _, l := range get(t, officer, "state=all&page_size=200"+A).Items {
			byAsset[l.Asset.ID] = l
		}
		for _, l := range get(t, officer, "state=all&page_size=200"+B).Items {
			byAsset[l.Asset.ID] = l
		}
		l := byAsset[a1.ID]
		if l.Asset.Tag != a1.Tag || l.Asset.Name != "Zebra Printer" || l.AssetTypeName != "Loại "+typeA.Code || l.Borrower.ID != e1.String() ||
			l.Borrower.Name != "Nguyễn Văn An" || l.StartedOn != d(-10) || l.ReturnBy == nil || *l.ReturnBy != d(-5) || l.OverdueDays != 5 || l.ReturnedOn != nil || l.TicketNumber != 0 {
			t.Errorf("overdue loan = %+v", l)
		}
		if l := byAsset[a2.ID]; l.OverdueDays != 0 || l.ReturnBy == nil || *l.ReturnBy != day {
			t.Errorf("loan due today = %+v, want 0 days overdue", l)
		}
		if l := byAsset[a3.ID]; l.ReturnBy != nil || l.OverdueDays != 0 {
			t.Errorf("loan without a due date = %+v", l)
		}
		if l := byAsset[b1.ID]; l.ReturnedOn == nil || *l.ReturnedOn != day || l.OverdueDays != 0 {
			t.Errorf("returned loan = %+v, want returned today and no overdue days", l)
		}
	})

	t.Run("invalid input", func(t *testing.T) {
		for what, q := range map[string]string{
			"unknown state":   "state=late",
			"type not a uuid": "type_id=nope",
			"bad date":        "from=2026-13-45",
			"long search":     "q=" + strings.Repeat("a", 101),
			"page size 0":     "page_size=0",
			"page size 201":   "page_size=201",
			"unknown sort":    "sort=borrower",
		} {
			rec := a.do("GET", "/api/v1/loans?"+q, officer, nil)
			if typ, _ := problem(t, rec); rec.Code != 400 || typ != "/errors/invalid-request" {
				t.Errorf("%s: %d %s, want 400 invalid-request", what, rec.Code, typ)
			}
		}
		rec := a.do("GET", "/api/v1/loans?from="+d(1)+"&to="+d(-1), officer, nil)
		if typ, fields := problem(t, rec); rec.Code != 422 || typ != "/errors/invalid-loan" || len(fields) == 0 || !strings.HasPrefix(fields[0], "from:") {
			t.Errorf("from after to = %d %s %v, want 422 invalid-loan on from", rec.Code, typ, fields)
		}
	})

	// Quyền: người mượn chỉ thấy khoản của mình, bất kể lọc/tìm gì; người không có quyền mượn không thấy gì
	t.Run("access control", func(t *testing.T) {
		wantSet(t, "employee 1, all", get(t, emp1, "mine=true&state=all"), a1, a2, b1)
		wantSet(t, "employee 2, all", get(t, emp2, "mine=true&state=all"), a3, b2)
		wantSet(t, "employee 1, overdue", get(t, emp1, "mine=true&state=overdue"), a1)
		wantSet(t, "employee 1, returned late", get(t, emp1, "mine=true&state=returned_late"), b1)
		wantSet(t, "employee 1, type A", get(t, emp1, "mine=true&state=all"+A), a1, a2)
		wantSet(t, "employee 1, type A, own search", get(t, emp1, "mine=true&state=all&q=epson"+A), a2)
		wantSet(t, "employee 1, own borrower name", get(t, emp1, "mine=true&state=all&q="+url.QueryEscape("văn an")), a1, a2, b1)
		wantSet(t, "employee 1, the other's asset tag", get(t, emp1, "mine=true&state=all&q="+url.QueryEscape(a3.Tag)))
		wantSet(t, "employee 1, the other's name", get(t, emp1, "mine=true&state=all&q="+url.QueryEscape("bình")))
		wantSet(t, "employee 1, the other's type and status", get(t, emp1, "mine=true&state=returned"+B+"&q=canon"))
		wantSet(t, "employee 1, a period before any loan", get(t, emp1, "mine=true&state=all&type_id="+typeB.ID+"&to="+d(-30)))
		// tham số lạ không đổi người xem
		smuggled := a.do("GET", "/api/v1/loans?mine=true&state=all&borrower_id="+e2.String(), emp1, nil)
		wantSet(t, "employee 1, borrower_id smuggled in", decode[loanResult](t, smuggled, 200), a1, a2, b1)

		for _, q := range []string{"state=all", "state=all" + A, "state=overdue&q=zebra", "mine=true&asset_id=" + a1.ID} {
			if rec := a.do("GET", "/api/v1/loans?"+q, emp1, nil); rec.Code != 403 {
				t.Errorf("employee asks for %q = %d, want 403", q, rec.Code)
			}
		}
		none := a.token(domain.PermAssetRead)
		for _, q := range []string{"mine=true", "state=all", "mine=true&state=all&q=epson"} {
			if rec := a.do("GET", "/api/v1/loans?"+q, none, nil); rec.Code != 403 {
				t.Errorf("no borrow permission, %q = %d, want 403", q, rec.Code)
			}
		}
		if rec := a.do("GET", "/api/v1/loans?mine=true", "", nil); rec.Code != 401 {
			t.Errorf("no token = %d, want 401", rec.Code)
		}
		// người quản lý: thấy mọi người; "của tôi" là của chính họ, không phải của cả kho
		wantSet(t, "officer sees everyone's loans, type A", get(t, officer, "state=all"+A), a1, a2, a3)
		wantSet(t, "officer sees everyone's loans, type B", get(t, officer, "state=all"+B), b1, b2)
		if r := get(t, officer, "mine=true&state=all"); r.Total != 0 {
			t.Errorf("officer's own loans = %d, want 0", r.Total)
		}
	})
}
