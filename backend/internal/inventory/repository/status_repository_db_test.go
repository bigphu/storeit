package repository_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"

	"storeit/internal/inventory/contract"
	"storeit/internal/inventory/domain"
)

func TestStatuses_CreateDefaultArchive(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()

	def, err := r.statuses.Default(context.Background(), domain.KindAvailable)
	if err != nil || def.ID != domain.AvailableStatusID || !def.IsSystem {
		t.Fatalf("default available = %+v, %v", def, err)
	}
	s, err := r.statuses.Create(ctx, domain.NewStatus{Name: "Ready " + uniq(), Kind: domain.KindAvailable, Position: 5})
	if err != nil || s.IsDefault || s.IsSystem {
		t.Fatalf("create = %+v, %v", s, err)
	}
	if _, err := r.statuses.Create(ctx, domain.NewStatus{Name: "available", Kind: domain.KindInUse}); !errors.Is(err, domain.ErrStatusNameTaken) {
		t.Errorf("duplicate name any case: %v", err)
	}

	// Chuyển mặc định: cái cũ mất cờ trong cùng transaction
	s, err = r.statuses.Update(ctx, s.ID, domain.StatusChange{MakeDefault: true})
	if err != nil || !s.IsDefault {
		t.Fatalf("make default: %+v, %v", s, err)
	}
	if old, _ := r.statuses.Get(context.Background(), domain.AvailableStatusID); old.IsDefault {
		t.Error("previous default kept its flag")
	}
	if _, err := r.statuses.Archive(ctx, s.ID); !errors.Is(err, domain.ErrStatusIsDefault) {
		t.Errorf("archive the default: %v", err)
	}
	if _, err := r.statuses.Archive(ctx, domain.InUseStatusID); !errors.Is(err, domain.ErrSystemStatus) {
		t.Errorf("archive a system status: %v", err)
	}
	// Trả mặc định về cho Available rồi archive status vừa tạo
	if _, err := r.statuses.Update(ctx, domain.AvailableStatusID, domain.StatusChange{MakeDefault: true}); err != nil {
		t.Fatal(err)
	}
	arch, err := r.statuses.Archive(ctx, s.ID)
	if err != nil || !arch.Archived() {
		t.Fatalf("archive: %+v, %v", arch, err)
	}
	list, _ := r.statuses.List(context.Background(), false)
	for _, l := range list {
		if l.ID == s.ID {
			t.Error("archived status listed")
		}
	}
	for _, typ := range []string{contract.EventStatusCreated, contract.EventStatusUpdated, contract.EventStatusArchived} {
		if countEvents(t, r, typ, s.ID) == 0 {
			t.Errorf("no %s event", typ)
		}
	}
	if renamed, err := r.statuses.Update(ctx, domain.RepairStatusID, domain.StatusChange{Name: ptr("Repair " + uniq()), Position: ptr(int32(7))}); err != nil || renamed.Position != 7 {
		t.Errorf("rename system status: %+v, %v", renamed, err)
	}
}

func TestStatuses_ReorderAndRestore(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	s, err := r.statuses.Create(ctx, domain.NewStatus{Name: "Spare " + uniq(), Kind: domain.KindAvailable})
	if err != nil {
		t.Fatal(err)
	}
	active := func() []uuid.UUID {
		list, err := r.statuses.List(context.Background(), false)
		if err != nil {
			t.Fatal(err)
		}
		var out []uuid.UUID
		for _, x := range list {
			out = append(out, x.ID)
		}
		return out
	}
	reversed := active()
	slices.Reverse(reversed)
	events := countEvents(t, r, contract.EventStatusUpdated, s.ID)

	got, err := r.statuses.Reorder(ctx, reversed)
	if err != nil {
		t.Fatal(err)
	}
	for i, x := range got {
		if x.ID != reversed[i] || x.Position != int32(i+1) {
			t.Errorf("item %d = %s at %d, want %s at %d", i, x.ID, x.Position, reversed[i], i+1)
		}
	}
	if !slices.Equal(active(), reversed) {
		t.Errorf("list order = %v, want %v", active(), reversed)
	}
	if n := countEvents(t, r, contract.EventStatusUpdated, s.ID); n != events+1 {
		t.Errorf("status_updated events = %d, want %d", n, events+1)
	}
	for name, bad := range map[string][]uuid.UUID{
		"missing one": reversed[1:],
		"duplicate":   append(slices.Clone(reversed[1:]), reversed[1]),
		"unknown":     append(slices.Clone(reversed[1:]), uuid.New()),
	} {
		if _, err := r.statuses.Reorder(ctx, bad); !errors.Is(err, domain.ErrInvalidOrder) {
			t.Errorf("%s: %v, want ErrInvalidOrder", name, err)
		}
	}

	// archive rồi restore: quay lại danh sách, có event; restore lần nữa không ghi gì
	if _, err := r.statuses.Archive(ctx, s.ID); err != nil {
		t.Fatal(err)
	}
	back, err := r.statuses.Restore(ctx, s.ID)
	if err != nil || back.Archived() {
		t.Fatalf("restore = %+v, %v", back, err)
	}
	if !slices.Contains(active(), s.ID) {
		t.Error("restored status not listed")
	}
	if n := countEvents(t, r, contract.EventStatusRestored, s.ID); n != 1 {
		t.Errorf("status_restored events = %d, want 1", n)
	}
	if _, err := r.statuses.Restore(ctx, s.ID); err != nil {
		t.Fatal(err)
	}
	if n := countEvents(t, r, contract.EventStatusRestored, s.ID); n != 1 {
		t.Errorf("restoring an active status wrote an event")
	}
	if _, err := r.statuses.Restore(ctx, uuid.New()); !errors.Is(err, domain.ErrStatusNotFound) {
		t.Errorf("unknown status: %v", err)
	}
}

func TestAssets_CountByStatus(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	typ := laptop(t, r)
	s, err := r.statuses.Create(ctx, domain.NewStatus{Name: "Counted " + uniq(), Kind: domain.KindInUse})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := r.assets.Create(ctx, "CS-"+uniq(), domain.AssetFields{Name: "x", TypeID: typ.ID, StatusID: s.ID, Values: fullValues(t, typ)[:1]}); err != nil {
			t.Fatal(err)
		}
	}
	counts, err := r.assets.CountByStatus(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if counts[s.ID] != 2 {
		t.Errorf("count = %d, want 2", counts[s.ID])
	}
}
