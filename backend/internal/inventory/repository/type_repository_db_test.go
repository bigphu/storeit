package repository_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"storeit/internal/inventory/contract"
	"storeit/internal/inventory/domain"
)

func TestTypes_CreateWithAttributesAndOptions(t *testing.T) {
	r := newRepos(t)
	typ := laptop(t, r)

	if typ.IsSystem || typ.Archived() || typ.Version != 1 || len(typ.Attributes) != 5 {
		t.Fatalf("type = %+v", typ)
	}
	keys := []string{}
	for _, a := range typ.Attributes {
		keys = append(keys, a.Key)
	}
	if strings.Join(keys, ",") != "serial,ram_gb,warranty_end,has_dock,os" {
		t.Errorf("attribute order = %v", keys)
	}
	ram, os := attr(t, typ, "ram_gb"), attr(t, typ, "os")
	if ram.Unit != "GB" || ram.DataType != domain.TypeNumber || ram.TypeID != typ.ID {
		t.Errorf("ram_gb = %+v", ram)
	}
	if len(os.Options) != 2 || os.Options[0].Label != "Windows" || os.Options[1].Label != "macOS" {
		t.Errorf("os options = %+v", os.Options)
	}
	if !attr(t, typ, "serial").Required {
		t.Error("serial not required")
	}
	if n := countEvents(t, r, contract.EventAssetTypeCreated, typ.ID); n != 1 {
		t.Errorf("asset_type_created events = %d", n)
	}

	list, err := r.types.List(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	found, general := false, false
	for _, l := range list {
		found = found || l.ID == typ.ID
		general = general || l.ID == domain.GeneralTypeID
	}
	if !found || !general {
		t.Errorf("List misses the new type or GENERAL")
	}
}

func TestTypes_DuplicatesAndRemoval(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	typ := laptop(t, r)

	if _, err := r.types.Create(ctx, domain.NewAssetType{Code: typ.Code, Name: "Other " + uniq()}); !errors.Is(err, domain.ErrTypeCodeTaken) {
		t.Errorf("duplicate code: %v", err)
	}
	if _, err := r.types.Create(ctx, domain.NewAssetType{Code: "X" + uniq(), Name: strings.ToUpper(typ.Name)}); !errors.Is(err, domain.ErrTypeNameTaken) {
		t.Errorf("duplicate name any case: %v", err)
	}
	if _, err := r.types.AddAttribute(ctx, typ.ID, domain.NewAttribute{Key: "serial", Label: "Serial 2", DataType: domain.TypeText}); !errors.Is(err, domain.ErrAttributeKeyTaken) {
		t.Errorf("duplicate key: %v", err)
	}
	if _, err := r.types.AddAttribute(ctx, typ.ID, domain.NewAttribute{Key: "serial2", Label: "SERIAL", DataType: domain.TypeText}); !errors.Is(err, domain.ErrAttributeLabelTaken) {
		t.Errorf("duplicate label any case: %v", err)
	}
	os := attr(t, typ, "os")
	if _, err := r.types.AddOption(ctx, typ.ID, os.ID, "windows", 9); !errors.Is(err, domain.ErrOptionLabelTaken) {
		t.Errorf("duplicate option label: %v", err)
	}
	if _, err := r.types.AddOption(ctx, typ.ID, attr(t, typ, "serial").ID, "X", 1); !errors.Is(err, domain.ErrNotSelectAttribute) {
		t.Errorf("option on text attribute: %v", err)
	}

	// Gỡ thuộc tính: nhãn dùng lại được, key thì không
	serial := attr(t, typ, "serial")
	if err := r.types.RemoveAttribute(ctx, typ.ID, serial.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.types.AddAttribute(ctx, typ.ID, domain.NewAttribute{Key: "serial_no", Label: "Serial", DataType: domain.TypeText}); err != nil {
		t.Errorf("reuse removed label: %v", err)
	}
	if _, err := r.types.AddAttribute(ctx, typ.ID, domain.NewAttribute{Key: "serial", Label: "Serial old", DataType: domain.TypeText}); !errors.Is(err, domain.ErrAttributeKeyTaken) {
		t.Errorf("reuse removed key: %v", err)
	}
	if err := r.types.RemoveAttribute(ctx, typ.ID, serial.ID); !errors.Is(err, domain.ErrAttributeNotFound) {
		t.Errorf("remove twice: %v", err)
	}

	// Gỡ option: nhãn dùng lại được
	if err := r.types.RemoveOption(ctx, typ.ID, os.ID, os.Options[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.types.AddOption(ctx, typ.ID, os.ID, "Windows", 3); err != nil {
		t.Errorf("reuse removed option label: %v", err)
	}
	if n := countEvents(t, r, contract.EventAssetTypeUpdated, typ.ID); n < 4 {
		t.Errorf("asset_type_updated events = %d, want one per change", n)
	}
}

func TestTypes_UpdateArchiveAndVersion(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	typ := laptop(t, r)

	upd, err := r.types.Update(ctx, typ.ID, ptr("Notebook "+uniq()), ptr("Renamed"), typ.Version)
	if err != nil || upd.Version != typ.Version+1 || upd.Description != "Renamed" {
		t.Fatalf("update = %+v, %v", upd, err)
	}
	if _, err := r.types.Update(ctx, typ.ID, ptr("Stale"), nil, typ.Version); !errors.Is(err, domain.ErrAssetTypeChanged) {
		t.Errorf("stale version: %v", err)
	}
	arch, err := r.types.SetArchived(ctx, typ.ID, true)
	if err != nil || !arch.Archived() {
		t.Fatalf("archive: %+v, %v", arch, err)
	}
	if list, _ := r.types.List(context.Background(), false); containsType(list, typ.ID) {
		t.Error("archived type listed without include_archived")
	}
	if list, _ := r.types.List(context.Background(), true); !containsType(list, typ.ID) {
		t.Error("archived type missing with include_archived")
	}
	if rest, err := r.types.SetArchived(ctx, typ.ID, false); err != nil || rest.Archived() {
		t.Errorf("restore: %+v, %v", rest, err)
	}
	if _, err := r.types.Get(context.Background(), domain.GeneralTypeID); err != nil {
		t.Errorf("GENERAL: %v", err)
	}
}

func containsType(list []domain.AssetType, id interface{ String() string }) bool {
	for _, l := range list {
		if l.ID.String() == id.String() {
			return true
		}
	}
	return false
}

// Review Focus 2: đổi kiểu dữ liệu khi chưa / đã có giá trị
func TestTypes_ChangeDataType(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	typ := laptop(t, r)
	ram, os := attr(t, typ, "ram_gb"), attr(t, typ, "os")

	// number -> text, chưa có giá trị: được, đơn vị phải bỏ cùng lúc
	if a, err := r.types.UpdateAttribute(ctx, typ.ID, ram.ID, domain.AttributeChange{DataType: ptr(domain.TypeText), Unit: ptr("")}); err != nil || a.DataType != domain.TypeText || a.Unit != "" {
		t.Errorf("number->text without values: %+v, %v", a, err)
	}
	// select có option nhưng chưa có giá trị -> text: được, option bị xoá
	if a, err := r.types.UpdateAttribute(ctx, typ.ID, os.ID, domain.AttributeChange{DataType: ptr(domain.TypeText)}); err != nil || a.DataType != domain.TypeText || len(a.Options) != 0 {
		t.Errorf("select->text without values: %+v, %v", a, err)
	}
	var n int
	_ = r.pool.QueryRow(context.Background(), `SELECT count(*) FROM inventory.asset_attribute_options WHERE attribute_id = $1`, os.ID).Scan(&n)
	if n != 0 {
		t.Errorf("%d options left after leaving select", n)
	}

	// Có giá trị: đổi kiểu hay đơn vị đều bị chặn
	typ2 := laptop(t, r)
	ram2 := attr(t, typ2, "ram_gb")
	if _, err := r.assets.Create(ctx, "T-"+uniq(), domain.AssetFields{
		Name: "With RAM", TypeID: typ2.ID, StatusID: domain.AvailableStatusID,
		Values: []domain.Value{
			{AttributeID: attr(t, typ2, "serial").ID, DataType: domain.TypeText, Text: ptr("SN")},
			{AttributeID: ram2.ID, DataType: domain.TypeNumber, Number: ptr("16")},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.types.UpdateAttribute(ctx, typ2.ID, ram2.ID, domain.AttributeChange{DataType: ptr(domain.TypeText), Unit: ptr("")}); !errors.Is(err, domain.ErrAttributeInUse) {
		t.Errorf("data type change with values: %v", err)
	}
	if _, err := r.types.UpdateAttribute(ctx, typ2.ID, ram2.ID, domain.AttributeChange{Unit: ptr("TB")}); !errors.Is(err, domain.ErrAttributeInUse) {
		t.Errorf("unit change with values: %v", err)
	}
	// Nhãn, bắt buộc, vị trí vẫn đổi được
	if a, err := r.types.UpdateAttribute(ctx, typ2.ID, ram2.ID, domain.AttributeChange{Label: ptr("Memory"), Required: ptr(true), Position: ptr(int32(9))}); err != nil || a.Label != "Memory" || !a.Required || a.Position != 9 {
		t.Errorf("label/required/position: %+v, %v", a, err)
	}
	if _, err := r.types.UpdateAttribute(ctx, typ.ID, ram2.ID, domain.AttributeChange{Label: ptr("x")}); !errors.Is(err, domain.ErrAttributeNotFound) {
		t.Errorf("attribute of another type: %v", err)
	}
}

func TestTypes_Reorder(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	typ := laptop(t, r)
	ids := func(t domain.AssetType) []uuid.UUID {
		var out []uuid.UUID
		for _, a := range t.ActiveAttributes() {
			out = append(out, a.ID)
		}
		return out
	}
	before := ids(typ)
	reversed := slices.Clone(before)
	slices.Reverse(reversed)
	events := countEvents(t, r, contract.EventAssetTypeUpdated, typ.ID)

	got, err := r.types.ReorderAttributes(ctx, typ.ID, reversed)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ids(got), reversed) {
		t.Errorf("order = %v, want %v", ids(got), reversed)
	}
	for i, a := range got.ActiveAttributes() {
		if a.Position != int32(i+1) {
			t.Errorf("%s position = %d, want %d", a.Key, a.Position, i+1)
		}
	}
	// một event cho cả lần sắp xếp
	if n := countEvents(t, r, contract.EventAssetTypeUpdated, typ.ID); n != events+1 {
		t.Errorf("events = %d, want %d", n, events+1)
	}
	// cùng thứ tự: không ghi, không event
	if _, err := r.types.ReorderAttributes(ctx, typ.ID, reversed); err != nil {
		t.Fatal(err)
	}
	if n := countEvents(t, r, contract.EventAssetTypeUpdated, typ.ID); n != events+1 {
		t.Errorf("unchanged order wrote an event")
	}
	// danh sách phải đúng các thuộc tính đang hoạt động, mỗi cái một lần
	for name, bad := range map[string][]uuid.UUID{
		"missing one": reversed[1:],
		"duplicate":   append(slices.Clone(reversed[1:]), reversed[1]),
		"unknown":     append(slices.Clone(reversed[1:]), uuid.New()),
	} {
		if _, err := r.types.ReorderAttributes(ctx, typ.ID, bad); !errors.Is(err, domain.ErrInvalidOrder) {
			t.Errorf("%s: %v, want ErrInvalidOrder", name, err)
		}
	}

	// option của thuộc tính select
	os := attr(t, typ, "os")
	opts := []uuid.UUID{os.Options[1].ID, os.Options[0].ID}
	a, err := r.types.ReorderOptions(ctx, typ.ID, os.ID, opts)
	if err != nil {
		t.Fatal(err)
	}
	if a.Options[0].ID != opts[0] || a.Options[0].Position != 1 || a.Options[1].Position != 2 {
		t.Errorf("options = %+v", a.Options)
	}
	if _, err := r.types.ReorderOptions(ctx, typ.ID, os.ID, opts[:1]); !errors.Is(err, domain.ErrInvalidOrder) {
		t.Errorf("missing option: %v", err)
	}
	if _, err := r.types.ReorderOptions(ctx, typ.ID, attr(t, typ, "serial").ID, nil); !errors.Is(err, domain.ErrNotSelectAttribute) {
		t.Errorf("options of a text attribute: %v", err)
	}
}

func TestTypes_AttributeLabels(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	typ := laptop(t, r)
	active := typ.ActiveAttributes()
	if err := r.types.RemoveAttribute(ctx, typ.ID, active[0].ID); err != nil {
		t.Fatal(err)
	}
	labels, err := r.types.AttributeLabels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// đã gỡ thì không có; còn lại theo thứ tự hiển thị
	var want []string
	for _, a := range active[1:] {
		want = append(want, a.Label)
	}
	if !slices.Equal(labels[typ.ID], want) {
		t.Errorf("labels = %v, want %v", labels[typ.ID], want)
	}
}
