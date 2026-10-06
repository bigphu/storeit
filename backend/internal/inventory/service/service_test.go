package service

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"storeit/internal/inventory/domain"
	"storeit/internal/platform/auth"
	"storeit/internal/platform/errs"
)

type env struct {
	svc      *Service
	types    *fakeTypes
	statuses *fakeStatuses
	assets   *fakeAssets
	profiles *fakeProfiles
}

func newEnv() *env {
	e := &env{types: newFakeTypes(), statuses: newFakeStatuses(), assets: newFakeAssets(), profiles: newFakeProfiles()}
	e.svc = New(Deps{Types: e.types, Statuses: e.statuses, Assets: e.assets, Profiles: e.profiles, Accounts: fakeAccounts{}})
	return e
}

func as(perms ...string) context.Context {
	return auth.WithActor(context.Background(), auth.Actor{AccountID: uuid.New(), Permissions: perms})
}

var manager = as(domain.PermAssetRead, domain.PermAssetManage, domain.PermTypeManage, domain.PermStatusManage)

func status(err error) int {
	var e *errs.Error
	if errors.As(err, &e) {
		return e.Status()
	}
	return 0
}

// laptop tạo qua service một loại có thuộc tính bắt buộc serial và select os
func (e *env) laptop(t *testing.T) domain.AssetType {
	t.Helper()
	typ, err := e.svc.CreateAssetType(manager, domain.NewAssetType{
		Code: "laptop", Name: "Laptop", Attributes: []domain.NewAttribute{
			{Key: "serial", Label: "Serial", DataType: domain.TypeText, Required: true},
			{Key: "ram_gb", Label: "RAM", DataType: domain.TypeNumber, Unit: "GB"},
			{Key: "os", Label: "OS", DataType: domain.TypeSelect, Options: []string{"Windows", "macOS"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return typ
}

func TestPermissionChecks(t *testing.T) {
	e := newEnv()
	id := uuid.New()
	calls := map[string]struct {
		perm string
		call func(context.Context) error
	}{
		"ListAssetTypes":   {domain.PermAssetRead, func(c context.Context) error { _, err := e.svc.ListAssetTypes(c, false); return err }},
		"GetAssetType":     {domain.PermAssetRead, func(c context.Context) error { _, err := e.svc.GetAssetType(c, id); return err }},
		"CreateAssetType":  {domain.PermTypeManage, func(c context.Context) error { _, err := e.svc.CreateAssetType(c, domain.NewAssetType{}); return err }},
		"UpdateAssetType":  {domain.PermTypeManage, func(c context.Context) error { _, err := e.svc.UpdateAssetType(c, id, nil, nil, 1); return err }},
		"ArchiveAssetType": {domain.PermTypeManage, func(c context.Context) error { _, err := e.svc.ArchiveAssetType(c, id); return err }},
		"RestoreAssetType": {domain.PermTypeManage, func(c context.Context) error { _, err := e.svc.RestoreAssetType(c, id); return err }},
		"AddAttribute":     {domain.PermTypeManage, func(c context.Context) error { _, err := e.svc.AddAttribute(c, id, domain.NewAttribute{}); return err }},
		"UpdateAttribute": {domain.PermTypeManage, func(c context.Context) error {
			_, err := e.svc.UpdateAttribute(c, id, id, domain.AttributeChange{})
			return err
		}},
		"RemoveAttribute":   {domain.PermTypeManage, func(c context.Context) error { return e.svc.RemoveAttribute(c, id, id) }},
		"AddOption":         {domain.PermTypeManage, func(c context.Context) error { _, err := e.svc.AddOption(c, id, id, "x", 1); return err }},
		"UpdateOption":      {domain.PermTypeManage, func(c context.Context) error { _, err := e.svc.UpdateOption(c, id, id, id, nil, nil); return err }},
		"RemoveOption":      {domain.PermTypeManage, func(c context.Context) error { return e.svc.RemoveOption(c, id, id, id) }},
		"ListStatuses":      {domain.PermAssetRead, func(c context.Context) error { _, err := e.svc.ListStatuses(c, false); return err }},
		"CreateStatus":      {domain.PermStatusManage, func(c context.Context) error { _, err := e.svc.CreateStatus(c, domain.NewStatus{}); return err }},
		"UpdateStatus":      {domain.PermStatusManage, func(c context.Context) error { _, err := e.svc.UpdateStatus(c, id, domain.StatusChange{}); return err }},
		"ArchiveStatus":     {domain.PermStatusManage, func(c context.Context) error { _, err := e.svc.ArchiveStatus(c, id); return err }},
		"RestoreStatus":     {domain.PermStatusManage, func(c context.Context) error { _, err := e.svc.RestoreStatus(c, id); return err }},
		"ReorderStatuses":   {domain.PermStatusManage, func(c context.Context) error { _, err := e.svc.ReorderStatuses(c, nil); return err }},
		"StatusAssetCounts": {domain.PermAssetRead, func(c context.Context) error { _, err := e.svc.StatusAssetCounts(c); return err }},
		"TypeSummaries":     {domain.PermAssetRead, func(c context.Context) error { _, err := e.svc.TypeSummaries(c); return err }},
		"ListAssets":        {domain.PermAssetRead, func(c context.Context) error { _, _, err := e.svc.ListAssets(c, domain.AssetFilter{}); return err }},
		"GetAsset":          {domain.PermAssetRead, func(c context.Context) error { _, err := e.svc.GetAsset(c, id); return err }},
		"CreateAsset":       {domain.PermAssetManage, func(c context.Context) error { _, err := e.svc.CreateAsset(c, "X", AssetInput{}); return err }},
		"UpdateAsset":       {domain.PermAssetManage, func(c context.Context) error { _, err := e.svc.UpdateAsset(c, id, AssetInput{}, 1); return err }},
		"RetireAsset":       {domain.PermAssetManage, func(c context.Context) error { _, err := e.svc.RetireAsset(c, id, "", 1); return err }},
		"RestoreAsset":      {domain.PermAssetManage, func(c context.Context) error { _, err := e.svc.RestoreAsset(c, id, 1); return err }},

		"ListExportProfiles": {domain.PermAssetExport, func(c context.Context) error { _, err := e.svc.ListExportProfiles(c); return err }},
		"CreateExportProfile": {domain.PermAssetExport, func(c context.Context) error {
			_, err := e.svc.CreateExportProfile(c, ExportProfileInput{})
			return err
		}},
		"GetExportProfile":    {domain.PermAssetExport, func(c context.Context) error { _, err := e.svc.GetExportProfile(c, id); return err }},
		"DeleteExportProfile": {domain.PermAssetExport, func(c context.Context) error { return e.svc.DeleteExportProfile(c, id) }},
	}
	for name, c := range calls {
		if err := c.call(context.Background()); status(err) != 401 {
			t.Errorf("%s without actor: %v, want 401", name, err)
		}
		// Có mọi quyền khác trừ quyền cần: 403
		var others []string
		for _, p := range []string{domain.PermAssetRead, domain.PermAssetManage, domain.PermTypeManage, domain.PermStatusManage, domain.PermAssetExport, domain.PermExportProfileManage} {
			if p != c.perm {
				others = append(others, p)
			}
		}
		if err := c.call(as(others...)); status(err) != 403 {
			t.Errorf("%s without %s: %v, want 403", name, c.perm, err)
		}
	}
}

func TestAssetTypeRules(t *testing.T) {
	e := newEnv()
	typ := e.laptop(t)
	if typ.Code != "LAPTOP" {
		t.Errorf("code not normalized: %q", typ.Code)
	}
	bad := map[string]struct {
		in   domain.NewAssetType
		want error
	}{
		"code":          {domain.NewAssetType{Code: "a b", Name: "X"}, domain.ErrInvalidTypeCode},
		"name":          {domain.NewAssetType{Code: "X", Name: "  "}, domain.ErrInvalidLabel},
		"key":           {domain.NewAssetType{Code: "X", Name: "X", Attributes: []domain.NewAttribute{{Key: "Bad", Label: "L", DataType: domain.TypeText}}}, domain.ErrInvalidAttributeKey},
		"data type":     {domain.NewAssetType{Code: "X", Name: "X", Attributes: []domain.NewAttribute{{Key: "a", Label: "L", DataType: "json"}}}, domain.ErrInvalidDataType},
		"unit on text":  {domain.NewAssetType{Code: "X", Name: "X", Attributes: []domain.NewAttribute{{Key: "a", Label: "L", DataType: domain.TypeText, Unit: "GB"}}}, domain.ErrInvalidUnit},
		"options text":  {domain.NewAssetType{Code: "X", Name: "X", Attributes: []domain.NewAttribute{{Key: "a", Label: "L", DataType: domain.TypeText, Options: []string{"x"}}}}, domain.ErrNotSelectAttribute},
		"blank option":  {domain.NewAssetType{Code: "X", Name: "X", Attributes: []domain.NewAttribute{{Key: "a", Label: "L", DataType: domain.TypeSelect, Options: []string{" "}}}}, domain.ErrInvalidLabel},
		"label control": {domain.NewAssetType{Code: "X", Name: "X", Attributes: []domain.NewAttribute{{Key: "a", Label: "L\nx", DataType: domain.TypeText}}}, domain.ErrInvalidLabel},
	}
	for name, tc := range bad {
		if _, err := e.svc.CreateAssetType(manager, tc.in); !errors.Is(err, tc.want) {
			t.Errorf("%s: %v, want %v", name, err, tc.want)
		}
	}
	if _, err := e.svc.ArchiveAssetType(manager, domain.GeneralTypeID); !errors.Is(err, domain.ErrSystemType) {
		t.Errorf("archive GENERAL: %v", err)
	}
	serial := typ.Attributes[0]
	if _, err := e.svc.UpdateAttribute(manager, typ.ID, serial.ID, domain.AttributeChange{Unit: ptr("GB")}); !errors.Is(err, domain.ErrInvalidUnit) {
		t.Errorf("unit on text attribute: %v", err)
	}
	ram := typ.Attributes[1]
	if a, err := e.svc.UpdateAttribute(manager, typ.ID, ram.ID, domain.AttributeChange{Unit: ptr("  ")}); err != nil || a.Unit != "" {
		t.Errorf("clear unit with blank: %+v, %v", a, err)
	}
	if _, err := e.svc.UpdateAttribute(manager, typ.ID, ram.ID, domain.AttributeChange{DataType: ptr(domain.DataType("xml"))}); !errors.Is(err, domain.ErrInvalidDataType) {
		t.Errorf("invalid data type change: %v", err)
	}
	if _, err := e.svc.AddOption(manager, typ.ID, serial.ID, "x", 1); !errors.Is(err, domain.ErrNotSelectAttribute) {
		t.Errorf("option on text: %v", err)
	}
	if _, err := e.svc.AddOption(manager, typ.ID, typ.Attributes[2].ID, " ", 1); !errors.Is(err, domain.ErrInvalidLabel) {
		t.Errorf("blank option label: %v", err)
	}
}

func TestStatusRules(t *testing.T) {
	e := newEnv()
	if _, err := e.svc.CreateStatus(manager, domain.NewStatus{Name: "Lost", Kind: "gone"}); !errors.Is(err, domain.ErrInvalidStatusKind) {
		t.Errorf("invalid kind: %v", err)
	}
	if _, err := e.svc.CreateStatus(manager, domain.NewStatus{Name: " ", Kind: domain.KindUnavailable}); !errors.Is(err, domain.ErrInvalidLabel) {
		t.Errorf("blank name: %v", err)
	}
	if _, err := e.svc.ArchiveStatus(manager, domain.InUseStatusID); !errors.Is(err, domain.ErrSystemStatus) {
		t.Errorf("archive system status: %v", err)
	}
	if s, err := e.svc.CreateStatus(manager, domain.NewStatus{Name: " Lost ", Kind: domain.KindUnavailable}); err != nil || s.Name != "Lost" {
		t.Errorf("create: %+v, %v", s, err)
	}
}

func TestCreateAsset_DefaultsAndValidation(t *testing.T) {
	e := newEnv()
	typ := e.laptop(t)
	v, err := e.svc.CreateAsset(manager, " lap-1 ", AssetInput{
		Name: " Dell ", TypeID: typ.ID, Attributes: map[string]any{"serial": "SN-1", "ram_gb": 16.0},
	})
	if err != nil {
		t.Fatal(err)
	}
	if v.Tag != "LAP-1" || v.Name != "Dell" || v.StatusID != domain.AvailableStatusID || v.Status.Name != "Available" ||
		v.Type.ID != typ.ID || len(v.Values) != 2 {
		t.Errorf("created = %+v", v)
	}

	_, err = e.svc.CreateAsset(manager, "LAP-2", AssetInput{Name: "X", TypeID: typ.ID, Attributes: map[string]any{"ram_gb": "x"}})
	var pe *errs.Error
	if !errors.As(err, &pe) || !errors.Is(err, domain.ErrInvalidAttributeValues) || len(pe.Fields()) != 2 {
		t.Errorf("invalid attributes: %v", err)
	}
	for name, tc := range map[string]struct {
		tag  string
		in   AssetInput
		want error
	}{
		"bad tag":          {"a b", AssetInput{Name: "X", TypeID: domain.GeneralTypeID}, domain.ErrInvalidTag},
		"blank name":       {"T-1", AssetInput{Name: " ", TypeID: domain.GeneralTypeID}, domain.ErrInvalidLabel},
		"unknown type":     {"T-1", AssetInput{Name: "X", TypeID: uuid.New()}, domain.ErrTypeNotFound},
		"unknown status":   {"T-1", AssetInput{Name: "X", TypeID: domain.GeneralTypeID, StatusID: ptr(uuid.New())}, domain.ErrStatusNotFound},
		"long description": {"T-1", AssetInput{Name: "X", TypeID: domain.GeneralTypeID, Description: strings.Repeat("x", 2001)}, domain.ErrInvalidLabel},
	} {
		if _, err := e.svc.CreateAsset(manager, tc.tag, tc.in); !errors.Is(err, tc.want) {
			t.Errorf("%s: %v, want %v", name, err, tc.want)
		}
	}
}

// Review Focus 4: loại/status đã archive
func TestAssetRules_Archived(t *testing.T) {
	e := newEnv()
	typ := e.laptop(t)
	custom, _ := e.svc.CreateStatus(manager, domain.NewStatus{Name: "Lost", Kind: domain.KindUnavailable})
	a, err := e.svc.CreateAsset(manager, "ARC-1", AssetInput{Name: "X", TypeID: typ.ID, StatusID: &custom.ID, Attributes: map[string]any{"serial": "S"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.ArchiveAssetType(manager, typ.ID); err != nil {
		t.Fatal(err)
	}
	e.statuses.archive(custom.ID)

	if _, err := e.svc.CreateAsset(manager, "ARC-2", AssetInput{Name: "X", TypeID: typ.ID, Attributes: map[string]any{"serial": "S"}}); !errors.Is(err, domain.ErrTypeArchived) {
		t.Errorf("new asset with archived type: %v", err)
	}
	if _, err := e.svc.CreateAsset(manager, "ARC-3", AssetInput{Name: "X", TypeID: domain.GeneralTypeID, StatusID: &custom.ID}); !errors.Is(err, domain.ErrStatusArchived) {
		t.Errorf("new asset with archived status: %v", err)
	}
	// Tài sản đang dùng loại/status đã archive vẫn sửa được khi giữ nguyên chúng
	upd, err := e.svc.UpdateAsset(manager, a.ID, AssetInput{Name: "Renamed", TypeID: typ.ID, StatusID: &custom.ID, Attributes: map[string]any{"serial": "S"}}, a.Version)
	if err != nil || upd.Name != "Renamed" {
		t.Errorf("edit keeping archived type/status: %+v, %v", upd, err)
	}
	// Đổi sang status khác rồi muốn quay lại status đã archive: không được
	upd, err = e.svc.UpdateAsset(manager, a.ID, AssetInput{Name: "Renamed", TypeID: typ.ID, StatusID: ptr(domain.AvailableStatusID), Attributes: map[string]any{"serial": "S"}}, upd.Version)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.UpdateAsset(manager, a.ID, AssetInput{Name: "Renamed", TypeID: typ.ID, StatusID: &custom.ID, Attributes: map[string]any{"serial": "S"}}, upd.Version); !errors.Is(err, domain.ErrStatusArchived) {
		t.Errorf("choose archived status: %v", err)
	}
}

func TestAssetRules_RetiredStatus(t *testing.T) {
	e := newEnv()
	if _, err := e.svc.CreateAsset(manager, "R-1", AssetInput{Name: "X", TypeID: domain.GeneralTypeID, StatusID: ptr(domain.RetiredStatusID)}); !errors.Is(err, domain.ErrRetiredStatus) {
		t.Errorf("create retired: %v", err)
	}
	a, _ := e.svc.CreateAsset(manager, "R-2", AssetInput{Name: "X", TypeID: domain.GeneralTypeID})
	if _, err := e.svc.UpdateAsset(manager, a.ID, AssetInput{Name: "X", TypeID: domain.GeneralTypeID, StatusID: ptr(domain.RetiredStatusID)}, a.Version); !errors.Is(err, domain.ErrRetiredStatus) {
		t.Errorf("edit to retired: %v", err)
	}
	ret, err := e.svc.RetireAsset(manager, a.ID, "  Broken  ", a.Version)
	if err != nil || !ret.Retired() || ret.RetiredReason != "Broken" || ret.StatusID != domain.RetiredStatusID {
		t.Fatalf("retire: %+v, %v", ret, err)
	}
	rest, err := e.svc.RestoreAsset(manager, a.ID, ret.Version)
	if err != nil || rest.Retired() || rest.StatusID != domain.AvailableStatusID {
		t.Errorf("restore: %+v, %v", rest, err)
	}
	if _, err := e.svc.RetireAsset(manager, a.ID, strings.Repeat("x", 501), rest.Version); !errors.Is(err, domain.ErrInvalidLabel) {
		t.Errorf("long reason: %v", err)
	}
}

// Review Focus 5: PUT thay toàn bộ, kể cả khi không gửi attributes
func TestUpdateAsset_FullReplacement(t *testing.T) {
	e := newEnv()
	typ := e.laptop(t)
	a, err := e.svc.CreateAsset(manager, "FR-1", AssetInput{Name: "X", TypeID: typ.ID, Attributes: map[string]any{"serial": "S", "ram_gb": 8.0}})
	if err != nil {
		t.Fatal(err)
	}
	// Bỏ attributes: thuộc tính bắt buộc thiếu
	_, err = e.svc.UpdateAsset(manager, a.ID, AssetInput{Name: "X", TypeID: typ.ID}, a.Version)
	var pe *errs.Error
	if !errors.As(err, &pe) || len(pe.Fields()) != 1 || pe.Fields()[0].Field != "attributes.serial" {
		t.Errorf("PUT without attributes: %v", err)
	}
	// Chỉ gửi serial: ram_gb bị xoá
	upd, err := e.svc.UpdateAsset(manager, a.ID, AssetInput{Name: "X", TypeID: typ.ID, Attributes: map[string]any{"serial": "S"}}, a.Version)
	if err != nil || len(upd.Values) != 1 {
		t.Errorf("PUT replaces values: %+v, %v", upd.Values, err)
	}
	// Không gửi status: giữ status hiện tại
	if upd.StatusID != a.StatusID {
		t.Errorf("status changed without status_id: %v", upd.StatusID)
	}
	// Đổi sang GENERAL không thuộc tính: hợp lệ, không còn giá trị
	gen, err := e.svc.UpdateAsset(manager, a.ID, AssetInput{Name: "X", TypeID: domain.GeneralTypeID}, upd.Version)
	if err != nil || len(gen.Values) != 0 || gen.Type.ID != domain.GeneralTypeID {
		t.Errorf("change to GENERAL: %+v, %v", gen, err)
	}
}

func TestGetAssetView(t *testing.T) {
	e := newEnv()
	read := as(domain.PermAssetRead)
	a, _ := e.svc.CreateAsset(manager, "V-1", AssetInput{Name: "X", TypeID: domain.GeneralTypeID})
	v, err := e.svc.GetAsset(read, a.ID)
	if err != nil || v.Type.Code != "GENERAL" || v.Status.Kind != domain.KindAvailable {
		t.Errorf("view = %+v, %v", v, err)
	}
	if _, err := e.svc.GetAsset(read, uuid.New()); !errors.Is(err, domain.ErrAssetNotFound) {
		t.Errorf("unknown: %v", err)
	}
}

func ptr[T any](v T) *T { return &v }

func TestListAssets_AttributeQuery(t *testing.T) {
	e := newEnv()
	typ := e.laptop(t)
	reader := as(domain.PermAssetRead)
	fieldsOf := func(err error) string {
		t.Helper()
		if !errors.Is(err, domain.ErrInvalidAttributeQuery) {
			t.Fatalf("err = %v, want ErrInvalidAttributeQuery", err)
		}
		var pe *errs.Error
		errors.As(err, &pe)
		var out []string
		for _, f := range pe.Fields() {
			out = append(out, f.Field)
		}
		return strings.Join(out, ",")
	}

	// Lọc/sắp theo thuộc tính cần type_id; loại lạ cũng là lỗi của type_id
	_, _, err := e.svc.ListAssets(reader, domain.AssetFilter{Attrs: []string{"ram_gb:gte:8"}})
	if got := fieldsOf(err); got != "type_id" {
		t.Errorf("no type_id: fields %s", got)
	}
	_, _, err = e.svc.ListAssets(reader, domain.AssetFilter{Sort: "attributes.ram_gb"})
	if got := fieldsOf(err); got != "type_id" {
		t.Errorf("sort without type_id: fields %s", got)
	}
	unknown := uuid.New()
	_, _, err = e.svc.ListAssets(reader, domain.AssetFilter{TypeID: &unknown, Attrs: []string{"ram_gb:gte:8"}})
	if got := fieldsOf(err); got != "type_id" {
		t.Errorf("unknown type: fields %s", got)
	}
	_, _, err = e.svc.ListAssets(reader, domain.AssetFilter{TypeID: &typ.ID, Attrs: []string{"ram_gb:gte:x"}, Sort: "attributes.nope"})
	if got := fieldsOf(err); got != "attr[0],sort" {
		t.Errorf("bad query: fields %s", got)
	}

	// Hợp lệ: repository nhận điều kiện đã kiểm
	if _, _, err := e.svc.ListAssets(reader, domain.AssetFilter{
		TypeID: &typ.ID, Attrs: []string{"ram_gb:gte:8"}, Sort: "-attributes.ram_gb",
	}); err != nil {
		t.Fatal(err)
	}
	f := e.assets.lastFilter
	if len(f.AttrFilters) != 1 || f.AttrFilters[0].Op != domain.OpGte || f.AttrFilters[0].Value != "8" ||
		f.AttrOrder == nil || !f.AttrOrder.Desc || f.AttrOrder.DataType != domain.TypeNumber || !f.IncludeValues {
		t.Errorf("filter passed to repository = %+v", f)
	}

	// Không có điều kiện thuộc tính thì không cần type_id, không tra loại
	if _, _, err := e.svc.ListAssets(reader, domain.AssetFilter{Sort: "name"}); err != nil {
		t.Errorf("plain list: %v", err)
	}
}

// Thao tác hàng loạt: mỗi tài sản thành công hay thất bại riêng
func TestBulkActions(t *testing.T) {
	e := newEnv()
	typ := e.laptop(t)
	mk := func(tag string) AssetView {
		t.Helper()
		v, err := e.svc.CreateAsset(manager, tag, AssetInput{Name: tag, TypeID: typ.ID, Attributes: map[string]any{"serial": "SN-" + tag}})
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	a, b, c := mk("B-1"), mk("B-2"), mk("B-3")
	gone := uuid.New()

	// đổi status: b gửi version cũ, gone không tồn tại; a và c đổi được
	repair := e.statuses.byKind(t, domain.KindUnavailable)
	res, err := e.svc.SetAssetsStatus(manager, []BulkItem{
		{ID: a.ID, Version: a.Version}, {ID: b.ID, Version: b.Version - 1}, {ID: gone, Version: 1}, {ID: c.ID, Version: c.Version},
	}, repair.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(res.Succeeded, []uuid.UUID{a.ID, c.ID}) || len(res.Failed) != 2 ||
		!errors.Is(res.Failed[0].Err, domain.ErrAssetChanged) || res.Failed[0].ID != b.ID ||
		!errors.Is(res.Failed[1].Err, domain.ErrAssetNotFound) {
		t.Errorf("status result = %+v", res)
	}
	if got, _ := e.svc.GetAsset(manager, a.ID); got.Status.ID != repair.ID {
		t.Errorf("a status = %s", got.Status.Name)
	}

	// status đã có sẵn: coi như thành công, không ghi (version giữ nguyên)
	cur, _ := e.svc.GetAsset(manager, c.ID)
	res, _ = e.svc.SetAssetsStatus(manager, []BulkItem{{ID: c.ID, Version: cur.Version}}, repair.ID)
	if after, _ := e.svc.GetAsset(manager, c.ID); len(res.Succeeded) != 1 || after.Version != cur.Version {
		t.Errorf("same status rewrote the asset: %+v, version %d -> %d", res, cur.Version, after.Version)
	}

	// retire: a thành công; b vẫn version cũ
	cur, _ = e.svc.GetAsset(manager, a.ID)
	res, err = e.svc.RetireAssets(manager, []BulkItem{{ID: a.ID, Version: cur.Version}, {ID: b.ID, Version: b.Version - 1}}, " disposal batch ")
	if err != nil || !slices.Equal(res.Succeeded, []uuid.UUID{a.ID}) || len(res.Failed) != 1 {
		t.Fatalf("retire = %+v, %v", res, err)
	}
	if got, _ := e.svc.GetAsset(manager, a.ID); got.RetiredReason != "disposal batch" {
		t.Errorf("reason = %q", got.RetiredReason)
	}
	// tài sản đã retire không đổi status được
	cur, _ = e.svc.GetAsset(manager, a.ID)
	res, _ = e.svc.SetAssetsStatus(manager, []BulkItem{{ID: a.ID, Version: cur.Version}}, repair.ID)
	if len(res.Failed) != 1 || !errors.Is(res.Failed[0].Err, domain.ErrAssetRetired) {
		t.Errorf("retired asset status change = %+v", res)
	}

	// Lỗi của cả yêu cầu: không có tài sản, quá nhiều, status retired, thiếu quyền
	if _, err := e.svc.RetireAssets(manager, nil, ""); !errors.Is(err, domain.ErrInvalidBulk) {
		t.Errorf("empty: %v", err)
	}
	if _, err := e.svc.RetireAssets(manager, make([]BulkItem, 201), ""); !errors.Is(err, domain.ErrInvalidBulk) {
		t.Errorf("too many: %v", err)
	}
	retired := e.statuses.byKind(t, domain.KindRetired)
	if _, err := e.svc.SetAssetsStatus(manager, []BulkItem{{ID: b.ID, Version: b.Version}}, retired.ID); !errors.Is(err, domain.ErrRetiredStatus) {
		t.Errorf("retired-kind status: %v", err)
	}
	if _, err := e.svc.RetireAssets(as(domain.PermAssetRead), []BulkItem{{ID: b.ID, Version: b.Version}}, ""); status(err) != 403 {
		t.Errorf("without manage: %v", err)
	}
}

func TestReorderNeedsTypeManage(t *testing.T) {
	e := newEnv()
	typ := e.laptop(t)
	if _, err := e.svc.ReorderAttributes(as(domain.PermAssetRead, domain.PermAssetManage), typ.ID, nil); status(err) != 403 {
		t.Errorf("reorder attributes without type.manage: %v", err)
	}
	if _, err := e.svc.ReorderOptions(as(domain.PermAssetRead), typ.ID, uuid.New(), nil); status(err) != 403 {
		t.Errorf("reorder options without type.manage: %v", err)
	}
	if _, err := e.svc.ReorderAttributes(manager, typ.ID, nil); err != nil {
		t.Errorf("manager: %v", err)
	}
}
