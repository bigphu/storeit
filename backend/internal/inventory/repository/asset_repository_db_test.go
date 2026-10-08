package repository_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"storeit/internal/inventory/contract"
	"storeit/internal/inventory/domain"
)

// fullValues: giá trị cho đủ năm thuộc tính của laptop
func fullValues(t *testing.T, typ domain.AssetType) []domain.Value {
	t.Helper()
	date := time.Date(2027, 6, 30, 0, 0, 0, 0, time.UTC)
	os := attr(t, typ, "os")
	return []domain.Value{
		{AttributeID: attr(t, typ, "serial").ID, DataType: domain.TypeText, Text: ptr("SN-1")},
		{AttributeID: attr(t, typ, "ram_gb").ID, DataType: domain.TypeNumber, Number: ptr("15.6")},
		{AttributeID: attr(t, typ, "warranty_end").ID, DataType: domain.TypeDate, Date: &date},
		{AttributeID: attr(t, typ, "has_dock").ID, DataType: domain.TypeBoolean, Bool: ptr(false)},
		{AttributeID: os.ID, DataType: domain.TypeSelect, OptionID: &os.Options[0].ID},
	}
}

func valueOf(a domain.Asset, attrID uuid.UUID) *domain.Value {
	for _, v := range a.Values {
		if v.AttributeID == attrID {
			return &v
		}
	}
	return nil
}

func TestAssets_CreateGetValues(t *testing.T) {
	r := newRepos(t)
	typ := laptop(t, r)
	loc, holder := uuid.New(), uuid.New()
	bought := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	tag := "LAP-" + uniq()

	a, err := r.assets.Create(actorCtx(), tag, domain.AssetFields{
		Name: "Dell 7440", Description: "Sales team", TypeID: typ.ID, StatusID: domain.AvailableStatusID,
		LocationID: &loc, HolderMemberID: &holder, PurchaseDate: &bought, Values: fullValues(t, typ),
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.assets.Get(context.Background(), a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Tag != tag || got.Version != 1 || *got.LocationID != loc || *got.HolderMemberID != holder ||
		got.PurchaseDate.Format(time.DateOnly) != "2025-01-15" || len(got.Values) != 5 {
		t.Fatalf("asset = %+v", got)
	}
	if v := valueOf(got, attr(t, typ, "ram_gb").ID); v == nil || *v.Number != "15.6" {
		t.Errorf("number value = %+v", v)
	}
	if v := valueOf(got, attr(t, typ, "warranty_end").ID); v == nil || v.Date.Format(time.DateOnly) != "2027-06-30" {
		t.Errorf("date value = %+v", v)
	}
	if v := valueOf(got, attr(t, typ, "has_dock").ID); v == nil || *v.Bool {
		t.Errorf("bool value = %+v", v)
	}
	if v := valueOf(got, attr(t, typ, "os").ID); v == nil || *v.OptionID != attr(t, typ, "os").Options[0].ID {
		t.Errorf("select value = %+v", v)
	}
	if countEvents(t, r, contract.EventAssetCreated, a.ID) != 1 {
		t.Error("no asset_created event")
	}
	if _, err := r.assets.Get(context.Background(), uuid.New()); !errors.Is(err, domain.ErrAssetNotFound) {
		t.Errorf("unknown asset: %v", err)
	}
}

// Review Focus 3: hai lần tạo cùng tag cùng lúc
func TestAssets_TagUniqueAnyCase(t *testing.T) {
	r := newRepos(t)
	tag := "DUP-" + uniq()
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = r.assets.Create(actorCtx(), tag, domain.AssetFields{
				Name: "Dup", TypeID: domain.GeneralTypeID, StatusID: domain.AvailableStatusID,
			})
		}()
	}
	wg.Wait()
	ok, taken := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, domain.ErrTagTaken):
			taken++
		default:
			t.Errorf("unexpected: %v", err)
		}
	}
	if ok != 1 || taken != 1 {
		t.Errorf("ok=%d taken=%d, want 1 and 1", ok, taken)
	}
}

func TestAssets_ReplaceChangesTypeAndValues(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	typ := laptop(t, r)
	a, err := r.assets.Create(ctx, "REP-"+uniq(), domain.AssetFields{
		Name: "Old", TypeID: typ.ID, StatusID: domain.AvailableStatusID, Values: fullValues(t, typ),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Cùng loại, đổi một giá trị và bỏ một giá trị tuỳ chọn
	vals := fullValues(t, typ)
	vals[1].Number = ptr("32")
	vals = vals[:4]
	upd, err := r.assets.Replace(ctx, a.ID, domain.AssetFields{
		Name: "New", TypeID: typ.ID, StatusID: domain.RepairStatusID, Values: vals,
	}, a.Version)
	if err != nil || upd.Version != a.Version+1 || upd.Name != "New" || upd.StatusID != domain.RepairStatusID || len(upd.Values) != 4 {
		t.Fatalf("replace = %+v, %v", upd, err)
	}
	p := lastPayload(t, r, contract.EventAssetUpdated, a.ID)
	for _, want := range []string{`"field": "name"`, `"field": "status_id"`, `"field": "attributes.ram_gb"`, `"field": "attributes.os"`} {
		if !strings.Contains(p, want) {
			t.Errorf("asset_updated payload %s lacks %s", p, want)
		}
	}

	// Đổi sang GENERAL: giá trị cũ bị xoá
	upd2, err := r.assets.Replace(ctx, a.ID, domain.AssetFields{
		Name: "New", TypeID: domain.GeneralTypeID, StatusID: domain.RepairStatusID,
	}, upd.Version)
	if err != nil || upd2.TypeID != domain.GeneralTypeID || len(upd2.Values) != 0 {
		t.Fatalf("change type = %+v, %v", upd2, err)
	}
	if !strings.Contains(lastPayload(t, r, contract.EventAssetUpdated, a.ID), `"field": "asset_type_id"`) {
		t.Error("type change not in event")
	}
}

func TestAssets_StaleVersionRetireRestore(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	a, err := r.assets.Create(ctx, "RET-"+uniq(), domain.AssetFields{
		Name: "To retire", TypeID: domain.GeneralTypeID, StatusID: domain.AvailableStatusID,
	})
	if err != nil {
		t.Fatal(err)
	}
	f := domain.AssetFields{Name: "x", TypeID: domain.GeneralTypeID, StatusID: domain.AvailableStatusID}
	if _, err := r.assets.Replace(ctx, a.ID, f, a.Version+5); !errors.Is(err, domain.ErrAssetChanged) {
		t.Errorf("stale version: %v", err)
	}
	ret, err := r.assets.Retire(ctx, a.ID, "Broken screen", domain.RetiredStatusID, a.Version)
	if err != nil || !ret.Retired() || ret.RetiredReason != "Broken screen" || ret.StatusID != domain.RetiredStatusID {
		t.Fatalf("retire = %+v, %v", ret, err)
	}
	if _, err := r.assets.Replace(ctx, a.ID, f, ret.Version); !errors.Is(err, domain.ErrAssetRetired) {
		t.Errorf("edit retired: %v", err)
	}
	if _, err := r.assets.Retire(ctx, a.ID, "", domain.RetiredStatusID, ret.Version); !errors.Is(err, domain.ErrAssetRetired) {
		t.Errorf("retire twice: %v", err)
	}
	rest, err := r.assets.Restore(ctx, a.ID, domain.AvailableStatusID, ret.Version)
	if err != nil || rest.Retired() || rest.RetiredReason != "" || rest.StatusID != domain.AvailableStatusID {
		t.Fatalf("restore = %+v, %v", rest, err)
	}
	if countEvents(t, r, contract.EventAssetRetired, a.ID) != 1 || countEvents(t, r, contract.EventAssetRestored, a.ID) != 1 {
		t.Error("retire/restore events missing")
	}
}

func TestAssets_ListFiltersSortSearch(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	typ := laptop(t, r)
	u := uniq()
	mk := func(tag, name string, typeID, status uuid.UUID, vals []domain.Value) domain.Asset {
		t.Helper()
		a, err := r.assets.Create(ctx, tag+"-"+u, domain.AssetFields{Name: name + " " + u, TypeID: typeID, StatusID: status, Values: vals})
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	a1 := mk("B1", "Sale 50%", typ.ID, domain.AvailableStatusID, fullValues(t, typ))
	a2 := mk("A2", "Sale 500", domain.GeneralTypeID, domain.RepairStatusID, nil)
	a3 := mk("C3", "Spare", domain.GeneralTypeID, domain.AvailableStatusID, nil)
	if _, err := r.assets.Retire(ctx, a3.ID, "", domain.RetiredStatusID, a3.Version); err != nil {
		t.Fatal(err)
	}

	list := func(f domain.AssetFilter) []string {
		t.Helper()
		if f.Query == "" {
			f.Query = u
		}
		f.Limit = 50
		items, total, err := r.assets.List(context.Background(), f)
		if err != nil {
			t.Fatal(err)
		}
		if int(total) != len(items) {
			t.Errorf("total %d != %d items", total, len(items))
		}
		var tags []string
		for _, it := range items {
			tags = append(tags, strings.TrimSuffix(it.Tag, "-"+u))
		}
		return tags
	}
	check := func(name string, got []string, want ...string) {
		t.Helper()
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
	check("default (tag asc, no retired)", list(domain.AssetFilter{}), "A2", "B1")
	check("include retired, -tag", list(domain.AssetFilter{IncludeRetired: true, Sort: domain.SortTagDesc}), "C3", "B1", "A2")
	check("by type", list(domain.AssetFilter{TypeID: &typ.ID}), "B1")
	check("by status", list(domain.AssetFilter{StatusID: ptr(domain.RepairStatusID)}), "A2")
	check("by kind", list(domain.AssetFilter{StatusKind: ptr(domain.KindRetired), IncludeRetired: true}), "C3")
	check("search literal %", list(domain.AssetFilter{Query: "50% " + u}), "B1")
	check("search tag lowercase", list(domain.AssetFilter{Query: strings.ToLower("a2-" + u)}), "A2")
	check("sort by name desc", list(domain.AssetFilter{Sort: domain.SortNameDesc}), "A2", "B1")

	items, _, _ := r.assets.List(context.Background(), domain.AssetFilter{Query: u, Limit: 50})
	for _, it := range items {
		if it.ID == a1.ID && (it.TypeName != typ.Name || it.StatusName != "Available" || it.StatusKind != domain.KindAvailable) {
			t.Errorf("list item = %+v", it)
		}
	}
	_ = a2
}

// Danh sách theo một loại: mỗi dòng kèm giá trị thuộc tính khi được yêu cầu
func TestAssets_ListWithValues(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	typ := laptop(t, r)
	a, err := r.assets.Create(ctx, "LV-"+uniq(), domain.AssetFields{
		Name: "With values", TypeID: typ.ID, StatusID: domain.AvailableStatusID, Values: fullValues(t, typ),
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.assets.Create(ctx, "LV-"+uniq(), domain.AssetFields{
		Name: "Only serial", TypeID: typ.ID, StatusID: domain.AvailableStatusID, Values: fullValues(t, typ)[:1],
	})
	if err != nil {
		t.Fatal(err)
	}

	items, _, err := r.assets.List(context.Background(), domain.AssetFilter{TypeID: &typ.ID, IncludeValues: true, Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	got := map[uuid.UUID]int{}
	for _, it := range items {
		got[it.ID] = len(it.Values)
	}
	if got[a.ID] != 5 || got[b.ID] != 1 {
		t.Errorf("values per asset = %v, want 5 and 1", got)
	}
	for _, it := range items {
		if it.ID == a.ID {
			if v := valueOf(it.Asset, attr(t, typ, "ram_gb").ID); v == nil || *v.Number != "15.6" {
				t.Errorf("ram_gb in list = %+v", v)
			}
		}
	}

	// Không yêu cầu thì không tải giá trị
	items, _, _ = r.assets.List(context.Background(), domain.AssetFilter{TypeID: &typ.ID, Limit: 50})
	for _, it := range items {
		if it.Values != nil {
			t.Errorf("values loaded without IncludeValues: %+v", it.Values)
		}
	}
}

func TestAssets_ListByAttribute(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	typ := laptop(t, r)
	serial, ram, warranty, dock, osAttr :=
		attr(t, typ, "serial"), attr(t, typ, "ram_gb"), attr(t, typ, "warranty_end"), attr(t, typ, "has_dock"), attr(t, typ, "os")
	windows, macos := osAttr.Options[0].ID, osAttr.Options[1].ID
	day := func(s string) *time.Time { d, _ := time.Parse(time.DateOnly, s); return &d }
	prefix := "AQ-" + uniq() + "-"
	create := func(name string, vals ...domain.Value) {
		t.Helper()
		if _, err := r.assets.Create(ctx, prefix+name, domain.AssetFields{
			Name: name, TypeID: typ.ID, StatusID: domain.AvailableStatusID, Values: vals,
		}); err != nil {
			t.Fatal(err)
		}
	}
	create("A",
		domain.Value{AttributeID: serial.ID, DataType: domain.TypeText, Text: ptr("SN-Alpha")},
		domain.Value{AttributeID: ram.ID, DataType: domain.TypeNumber, Number: ptr("8")},
		domain.Value{AttributeID: warranty.ID, DataType: domain.TypeDate, Date: day("2027-01-01")},
		domain.Value{AttributeID: dock.ID, DataType: domain.TypeBoolean, Bool: ptr(true)},
		domain.Value{AttributeID: osAttr.ID, DataType: domain.TypeSelect, OptionID: &windows})
	create("B",
		domain.Value{AttributeID: serial.ID, DataType: domain.TypeText, Text: ptr("sn-beta")},
		domain.Value{AttributeID: ram.ID, DataType: domain.TypeNumber, Number: ptr("16")},
		domain.Value{AttributeID: warranty.ID, DataType: domain.TypeDate, Date: day("2028-01-01")},
		domain.Value{AttributeID: dock.ID, DataType: domain.TypeBoolean, Bool: ptr(false)},
		domain.Value{AttributeID: osAttr.ID, DataType: domain.TypeSelect, OptionID: &macos})
	create("C",
		domain.Value{AttributeID: serial.ID, DataType: domain.TypeText, Text: ptr("X-100%")},
		domain.Value{AttributeID: ram.ID, DataType: domain.TypeNumber, Number: ptr("32")})
	create("D", domain.Value{AttributeID: serial.ID, DataType: domain.TypeText, Text: ptr("100x")})

	// list trả tên tài sản theo thứ tự; tổng phải khớp số dòng
	list := func(sort domain.AssetSort, conds ...string) string {
		t.Helper()
		filters, order, err := domain.ResolveAttrQuery(typ, conds, sort)
		if err != nil {
			t.Fatal(err)
		}
		items, total, err := r.assets.List(context.Background(), domain.AssetFilter{
			TypeID: &typ.ID, Sort: sort, AttrFilters: filters, AttrOrder: order, Limit: 50,
		})
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, it := range items {
			names = append(names, it.Name)
		}
		if int(total) != len(names) {
			t.Errorf("%v: total %d, rows %d", conds, total, len(names))
		}
		return strings.Join(names, ",")
	}

	for _, c := range []struct {
		sort  domain.AssetSort
		conds []string
		want  string
	}{
		{"", []string{"ram_gb:gte:16"}, "B,C"},
		{"", []string{"ram_gb:gte:8", "ram_gb:lt:32"}, "A,B"},
		{"", []string{"ram_gb:eq:16.0"}, "B"},
		{"", []string{"serial:contains:sn-"}, "A,B"},
		{"", []string{"serial:contains:100%"}, "C"}, // % là chữ, không khớp "100x"
		{"", []string{"serial:eq:SN-ALPHA"}, "A"},
		{"", []string{"warranty_end:lt:2028-01-01"}, "A"},
		{"", []string{"warranty_end:gte:2027-01-01"}, "A,B"},
		{"", []string{"has_dock:eq:false"}, "B"},
		{"", []string{"os:eq:" + macos.String()}, "B"},
		{"", []string{"os:in:" + windows.String() + "," + macos.String()}, "A,B"},
		{"", []string{"ram_gb:gt:100"}, ""},

		// Không có giá trị luôn ở cuối, hai chiều; hoà thì theo tag
		{"attributes.ram_gb", nil, "A,B,C,D"},
		{"-attributes.ram_gb", nil, "C,B,A,D"},
		{"attributes.serial", nil, "D,A,B,C"},
		{"-attributes.serial", nil, "C,B,A,D"},
		{"attributes.warranty_end", nil, "A,B,C,D"},
		{"-attributes.warranty_end", nil, "B,A,C,D"},
		{"attributes.has_dock", nil, "B,A,C,D"},
		{"attributes.os", nil, "A,B,C,D"}, // theo thứ tự option: Windows rồi macOS
		{"-attributes.os", nil, "B,A,C,D"},
		{"-attributes.ram_gb", []string{"ram_gb:lte:16"}, "B,A"},
	} {
		if got := list(c.sort, c.conds...); got != c.want {
			t.Errorf("sort %q %v = %q, want %q", c.sort, c.conds, got, c.want)
		}
	}
}

func TestAssets_SortByTypeAndStatus(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	u := uniq()
	lap := laptop(t, r) // tên "Laptop …"
	aard, err := r.types.Create(ctx, domain.NewAssetType{Code: "AAR" + u, Name: "Aardvark " + u})
	if err != nil {
		t.Fatal(err)
	}
	// thứ tự status theo position trước, tên sau: Zeta (900) đứng trước Alpha (901)
	zeta, err := r.statuses.Create(ctx, domain.NewStatus{Name: "Zeta " + u, Kind: domain.KindUnavailable, Position: 900})
	if err != nil {
		t.Fatal(err)
	}
	alpha, err := r.statuses.Create(ctx, domain.NewStatus{Name: "Alpha " + u, Kind: domain.KindUnavailable, Position: 901})
	if err != nil {
		t.Fatal(err)
	}
	prefix := "SO-" + u + "-"
	for _, c := range []struct {
		name          string
		typ, statusID uuid.UUID
		vals          []domain.Value
	}{
		{"P1", lap.ID, zeta.ID, fullValues(t, lap)[:1]},
		{"P2", aard.ID, alpha.ID, nil},
		{"P3", lap.ID, alpha.ID, fullValues(t, lap)[:1]},
	} {
		if _, err := r.assets.Create(ctx, prefix+c.name, domain.AssetFields{
			Name: c.name, TypeID: c.typ, StatusID: c.statusID, Values: c.vals,
		}); err != nil {
			t.Fatal(err)
		}
	}
	for sort, want := range map[domain.AssetSort]string{
		domain.SortAssetType:     "P2,P1,P3", // hoà thì theo tag
		domain.SortAssetTypeDesc: "P1,P3,P2",
		domain.SortStatus:        "P1,P2,P3",
		domain.SortStatusDesc:    "P2,P3,P1",
	} {
		items, _, err := r.assets.List(context.Background(), domain.AssetFilter{Query: prefix, Sort: sort, Limit: 50})
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, it := range items {
			names = append(names, it.Name)
		}
		if got := strings.Join(names, ","); got != want {
			t.Errorf("sort %q = %q, want %q", sort, got, want)
		}
	}
}

func TestAssets_CountByType(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	a, b := laptop(t, r), laptop(t, r)
	mk := func(typ domain.AssetType, vals []domain.Value) domain.Asset {
		t.Helper()
		x, err := r.assets.Create(ctx, "CT-"+uniq(), domain.AssetFields{Name: "x", TypeID: typ.ID, StatusID: domain.AvailableStatusID, Values: vals})
		if err != nil {
			t.Fatal(err)
		}
		return x
	}
	mk(a, fullValues(t, a)[:1])
	inUse := mk(a, fullValues(t, a)[:1])
	if _, err := r.assets.Replace(ctx, inUse.ID, domain.AssetFields{Name: "x", TypeID: a.ID, StatusID: domain.InUseStatusID, Values: fullValues(t, a)[:1]}, inUse.Version); err != nil {
		t.Fatal(err)
	}
	gone := mk(b, fullValues(t, b)[:1])
	if _, err := r.assets.Retire(ctx, gone.ID, "lost", domain.RetiredStatusID, gone.Version); err != nil {
		t.Fatal(err)
	}

	counts, err := r.assets.CountByType(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// tài sản đã retire không tính; loại không còn tài sản nào không có trong map
	ca := counts[a.ID]
	if ca.Total != 2 || ca.ByKind[domain.KindAvailable] != 1 || ca.ByKind[domain.KindInUse] != 1 {
		t.Errorf("counts[a] = %+v, want total 2: 1 available, 1 in use", ca)
	}
	if _, ok := counts[b.ID]; ok {
		t.Errorf("type with only retired assets is in the map")
	}
}

func TestAssets_StreamMatchesListAndIDs(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	typ := laptop(t, r)
	var made []domain.Asset
	for range 7 {
		a, err := r.assets.Create(ctx, "ST-"+uniq(), domain.AssetFields{Name: "x", TypeID: typ.ID, StatusID: domain.AvailableStatusID, Values: fullValues(t, typ)})
		if err != nil {
			t.Fatal(err)
		}
		made = append(made, a)
	}
	f := domain.AssetFilter{TypeID: &typ.ID, Sort: domain.SortTag}
	want, total, err := r.assets.List(context.Background(), domain.AssetFilter{TypeID: &typ.ID, Sort: domain.SortTag, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	var got []domain.AssetListItem
	pages := 0
	err = r.assets.Stream(context.Background(), f, 3, func(items []domain.AssetListItem) error {
		pages++
		got = append(got, items...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if int64(len(got)) != total || pages != 3 {
		t.Fatalf("stream = %d rows in %d pages, want %d rows in 3 pages", len(got), pages, total)
	}
	for i := range got {
		if got[i].ID != want[i].ID {
			t.Fatalf("row %d = %s, want %s", i, got[i].Tag, want[i].Tag)
		}
		if len(got[i].Values) == 0 {
			t.Fatalf("row %d has no attribute values", i)
		}
	}
	if n, err := r.assets.Count(context.Background(), f); err != nil || n != total {
		t.Errorf("count = %d, %v, want %d", n, err, total)
	}
	sel := domain.AssetFilter{TypeID: &typ.ID, IDs: []uuid.UUID{made[0].ID, made[3].ID}}
	if n, _ := r.assets.Count(context.Background(), sel); n != 2 {
		t.Errorf("count by ids = %d, want 2", n)
	}
}

func TestAssets_ListByBuiltinField(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	typ := laptop(t, r)
	day := func(s string) *time.Time { d, _ := time.Parse(time.DateOnly, s); return &d }
	prefix := "BF-" + uniq() + "-"
	create := func(name, desc string, bought *time.Time) {
		t.Helper()
		if _, err := r.assets.Create(ctx, prefix+name, domain.AssetFields{
			Name: name, Description: desc, TypeID: typ.ID, StatusID: domain.AvailableStatusID, PurchaseDate: bought,
		}); err != nil {
			t.Fatal(err)
		}
	}
	create("A", "Dock 100% included", day("2026-01-10"))
	create("B", "no dock", day("2026-03-01"))
	create("C", "", nil) // không có ngày mua: không bao giờ khớp điều kiện purchase_date

	list := func(conds ...domain.FieldFilter) string {
		t.Helper()
		items, total, err := r.assets.List(context.Background(), domain.AssetFilter{Query: prefix, FieldFilters: conds, Limit: 50})
		if err != nil {
			t.Fatal(err)
		}
		if int(total) != len(items) {
			t.Errorf("total %d != %d rows", total, len(items))
		}
		var names []string
		for _, it := range items {
			names = append(names, it.Name)
		}
		return strings.Join(names, ",")
	}
	ff := func(f domain.BuiltinField, op domain.AttrOp, v string) domain.FieldFilter {
		return domain.FieldFilter{Field: f, Op: op, Value: v}
	}
	hourAgo := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	inHour := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	cases := []struct {
		name  string
		conds []domain.FieldFilter
		want  string
	}{
		{"purchase gte", []domain.FieldFilter{ff(domain.FieldPurchaseDate, domain.OpGte, "2026-02-01")}, "B"},
		{"purchase lt skips no date", []domain.FieldFilter{ff(domain.FieldPurchaseDate, domain.OpLt, "2026-02-01")}, "A"},
		{"purchase eq", []domain.FieldFilter{ff(domain.FieldPurchaseDate, domain.OpEq, "2026-01-10")}, "A"},
		{"purchase gt", []domain.FieldFilter{ff(domain.FieldPurchaseDate, domain.OpGt, "2026-01-10")}, "B"},
		{"purchase lte", []domain.FieldFilter{ff(domain.FieldPurchaseDate, domain.OpLte, "2026-03-01")}, "A,B"},
		{"description contains, case-insensitive", []domain.FieldFilter{ff(domain.FieldDescription, domain.OpContains, "DOCK")}, "A,B"},
		{"description % is literal", []domain.FieldFilter{ff(domain.FieldDescription, domain.OpContains, "100%")}, "A"},
		{"created since an hour ago", []domain.FieldFilter{ff(domain.FieldCreatedAt, domain.OpGte, hourAgo)}, "A,B,C"},
		{"created before an hour ago", []domain.FieldFilter{ff(domain.FieldCreatedAt, domain.OpLt, hourAgo)}, ""},
		{"updated within the hour", []domain.FieldFilter{ff(domain.FieldUpdatedAt, domain.OpGte, hourAgo), ff(domain.FieldUpdatedAt, domain.OpLt, inHour)}, "A,B,C"},
		{"combined AND", []domain.FieldFilter{ff(domain.FieldDescription, domain.OpContains, "dock"), ff(domain.FieldPurchaseDate, domain.OpGte, "2026-02-01")}, "B"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := list(c.conds...); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}
