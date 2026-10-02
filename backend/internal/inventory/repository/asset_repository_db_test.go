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
