package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"storeit/internal/inventory/domain"
)

// Fake repository trong bộ nhớ: đủ để kiểm tra luật của service. SQL, khoá
// ngoại và event được kiểm ở repository (test với Postgres).

type fakeTypes struct {
	mu    sync.Mutex
	types map[uuid.UUID]domain.AssetType
}

func newFakeTypes() *fakeTypes {
	return &fakeTypes{types: map[uuid.UUID]domain.AssetType{
		domain.GeneralTypeID: {ID: domain.GeneralTypeID, Code: "GENERAL", Name: "General", IsSystem: true, Version: 1},
	}}
}

func (f *fakeTypes) List(_ context.Context, includeArchived bool) ([]domain.AssetType, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.AssetType
	for _, t := range f.types {
		if includeArchived || t.ArchivedAt == nil {
			out = append(out, t)
		}
	}
	return out, nil
}

func (f *fakeTypes) Get(_ context.Context, id uuid.UUID) (domain.AssetType, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.types[id]
	if !ok {
		return domain.AssetType{}, domain.ErrTypeNotFound
	}
	return t, nil
}

func (f *fakeTypes) Create(_ context.Context, in domain.NewAssetType) (domain.AssetType, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t := domain.AssetType{ID: uuid.New(), Code: in.Code, Name: in.Name, Description: in.Description, Version: 1}
	for _, na := range in.Attributes {
		t.Attributes = append(t.Attributes, newAttr(t.ID, na))
	}
	f.types[t.ID] = t
	return t, nil
}

func newAttr(typeID uuid.UUID, na domain.NewAttribute) domain.Attribute {
	a := domain.Attribute{ID: uuid.New(), TypeID: typeID, Key: na.Key, Label: na.Label, DataType: na.DataType,
		Unit: na.Unit, Required: na.Required, Position: na.Position}
	for i, l := range na.Options {
		a.Options = append(a.Options, domain.Option{ID: uuid.New(), AttributeID: a.ID, Label: l, Position: int32(i + 1)})
	}
	return a
}

func (f *fakeTypes) Update(_ context.Context, id uuid.UUID, name, description *string, version int32) (domain.AssetType, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t := f.types[id]
	if t.Version != version {
		return domain.AssetType{}, domain.ErrAssetTypeChanged
	}
	if name != nil {
		t.Name = *name
	}
	if description != nil {
		t.Description = *description
	}
	t.Version++
	f.types[id] = t
	return t, nil
}

func (f *fakeTypes) SetArchived(_ context.Context, id uuid.UUID, archived bool) (domain.AssetType, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t := f.types[id]
	if archived {
		now := time.Now()
		t.ArchivedAt = &now
	} else {
		t.ArchivedAt = nil
	}
	f.types[id] = t
	return t, nil
}

func (f *fakeTypes) AddAttribute(_ context.Context, typeID uuid.UUID, in domain.NewAttribute) (domain.Attribute, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t := f.types[typeID]
	a := newAttr(typeID, in)
	t.Attributes = append(t.Attributes, a)
	f.types[typeID] = t
	return a, nil
}

func (f *fakeTypes) UpdateAttribute(_ context.Context, typeID, attrID uuid.UUID, ch domain.AttributeChange) (domain.Attribute, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t := f.types[typeID]
	for i, a := range t.Attributes {
		if a.ID != attrID {
			continue
		}
		if ch.Label != nil {
			a.Label = *ch.Label
		}
		if ch.DataType != nil {
			a.DataType = *ch.DataType
		}
		if ch.Unit != nil {
			a.Unit = *ch.Unit
		}
		if ch.Required != nil {
			a.Required = *ch.Required
		}
		t.Attributes[i] = a
		f.types[typeID] = t
		return a, nil
	}
	return domain.Attribute{}, domain.ErrAttributeNotFound
}

func (f *fakeTypes) RemoveAttribute(_ context.Context, typeID, attrID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	t := f.types[typeID]
	for i, a := range t.Attributes {
		if a.ID == attrID {
			now := time.Now()
			t.Attributes[i].RemovedAt = &now
			return nil
		}
	}
	return domain.ErrAttributeNotFound
}

func (f *fakeTypes) AddOption(_ context.Context, typeID, attrID uuid.UUID, label string, position int32) (domain.Option, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t := f.types[typeID]
	for i, a := range t.Attributes {
		if a.ID == attrID {
			if a.DataType != domain.TypeSelect {
				return domain.Option{}, domain.ErrNotSelectAttribute
			}
			o := domain.Option{ID: uuid.New(), AttributeID: attrID, Label: label, Position: position}
			t.Attributes[i].Options = append(t.Attributes[i].Options, o)
			return o, nil
		}
	}
	return domain.Option{}, domain.ErrAttributeNotFound
}

func (f *fakeTypes) UpdateOption(_ context.Context, _, _, optID uuid.UUID, label *string, _ *int32) (domain.Option, error) {
	return domain.Option{ID: optID, Label: deref(label)}, nil
}

func (f *fakeTypes) RemoveOption(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error { return nil }

type fakeStatuses struct {
	mu       sync.Mutex
	statuses map[uuid.UUID]domain.Status
}

func newFakeStatuses() *fakeStatuses {
	mk := func(id uuid.UUID, name string, kind domain.StatusKind) domain.Status {
		return domain.Status{ID: id, Name: name, Kind: kind, IsDefault: true, IsSystem: true}
	}
	return &fakeStatuses{statuses: map[uuid.UUID]domain.Status{
		domain.AvailableStatusID: mk(domain.AvailableStatusID, "Available", domain.KindAvailable),
		domain.InUseStatusID:     mk(domain.InUseStatusID, "In use", domain.KindInUse),
		domain.RepairStatusID:    mk(domain.RepairStatusID, "Under repair", domain.KindUnavailable),
		domain.RetiredStatusID:   mk(domain.RetiredStatusID, "Retired", domain.KindRetired),
	}}
}

func (f *fakeStatuses) List(context.Context, bool) ([]domain.Status, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.Status
	for _, s := range f.statuses {
		out = append(out, s)
	}
	return out, nil
}

func (f *fakeStatuses) Get(_ context.Context, id uuid.UUID) (domain.Status, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.statuses[id]
	if !ok {
		return domain.Status{}, domain.ErrStatusNotFound
	}
	return s, nil
}

func (f *fakeStatuses) Default(_ context.Context, kind domain.StatusKind) (domain.Status, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.statuses {
		if s.Kind == kind && s.IsDefault {
			return s, nil
		}
	}
	return domain.Status{}, domain.ErrStatusNotFound
}

func (f *fakeStatuses) Create(_ context.Context, in domain.NewStatus) (domain.Status, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := domain.Status{ID: uuid.New(), Name: in.Name, Kind: in.Kind, Position: in.Position}
	f.statuses[s.ID] = s
	return s, nil
}

func (f *fakeStatuses) Update(_ context.Context, id uuid.UUID, ch domain.StatusChange) (domain.Status, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := f.statuses[id]
	if ch.Name != nil {
		s.Name = *ch.Name
	}
	f.statuses[id] = s
	return s, nil
}

func (f *fakeStatuses) Archive(_ context.Context, id uuid.UUID) (domain.Status, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := f.statuses[id]
	now := time.Now()
	s.ArchivedAt = &now
	f.statuses[id] = s
	return s, nil
}

// archive đánh dấu archived trực tiếp (để test luật "đã archive")
// byKind: status mặc định của kind (cho test)
func (f *fakeStatuses) byKind(t *testing.T, k domain.StatusKind) domain.Status {
	t.Helper()
	s, err := f.Default(context.Background(), k)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func (f *fakeStatuses) archive(id uuid.UUID) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := f.statuses[id]
	now := time.Now()
	s.ArchivedAt = &now
	f.statuses[id] = s
}

type fakeAssets struct {
	mu         sync.Mutex
	assets     map[uuid.UUID]domain.Asset
	lastFilter domain.AssetFilter // bộ lọc List nhận gần nhất
}

func newFakeAssets() *fakeAssets { return &fakeAssets{assets: map[uuid.UUID]domain.Asset{}} }

func (f *fakeAssets) Create(_ context.Context, tag string, in domain.AssetFields) (domain.Asset, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, a := range f.assets {
		if a.Tag == tag {
			return domain.Asset{}, domain.ErrTagTaken
		}
	}
	a := apply(domain.Asset{ID: uuid.New(), Tag: tag, Version: 1}, in)
	f.assets[a.ID] = a
	return a, nil
}

func apply(a domain.Asset, in domain.AssetFields) domain.Asset {
	a.Name, a.Description, a.TypeID, a.StatusID = in.Name, in.Description, in.TypeID, in.StatusID
	a.LocationID, a.HolderMemberID, a.PurchaseDate, a.Values = in.LocationID, in.HolderMemberID, in.PurchaseDate, in.Values
	return a
}

func (f *fakeAssets) Get(_ context.Context, id uuid.UUID) (domain.Asset, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.assets[id]
	if !ok {
		return domain.Asset{}, domain.ErrAssetNotFound
	}
	return a, nil
}

func (f *fakeAssets) CountByType(context.Context) (map[uuid.UUID]int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[uuid.UUID]int64{}
	for _, a := range f.assets {
		if a.RetiredAt == nil {
			out[a.TypeID]++
		}
	}
	return out, nil
}

func (f *fakeAssets) List(_ context.Context, filter domain.AssetFilter) ([]domain.AssetListItem, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastFilter = filter
	return nil, 0, nil
}

func (f *fakeAssets) Replace(_ context.Context, id uuid.UUID, in domain.AssetFields, version int32) (domain.Asset, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.assets[id]
	if !ok {
		return domain.Asset{}, domain.ErrAssetNotFound
	}
	if a.Retired() {
		return domain.Asset{}, domain.ErrAssetRetired
	}
	if a.Version != version {
		return domain.Asset{}, domain.ErrAssetChanged
	}
	a = apply(a, in)
	a.Version++
	f.assets[id] = a
	return a, nil
}

func (f *fakeAssets) Retire(_ context.Context, id uuid.UUID, reason string, status uuid.UUID, version int32) (domain.Asset, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.assets[id]
	switch {
	case !ok:
		return domain.Asset{}, domain.ErrAssetNotFound
	case a.Retired():
		return domain.Asset{}, domain.ErrAssetRetired
	case a.Version != version:
		return domain.Asset{}, domain.ErrAssetChanged
	}
	now := time.Now()
	a.RetiredAt, a.RetiredReason, a.StatusID = &now, reason, status
	a.Version++
	f.assets[id] = a
	return a, nil
}

func (f *fakeAssets) Restore(_ context.Context, id uuid.UUID, status uuid.UUID, version int32) (domain.Asset, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a := f.assets[id]
	a.RetiredAt, a.RetiredReason, a.StatusID = nil, "", status
	a.Version++
	f.assets[id] = a
	return a, nil
}

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}
