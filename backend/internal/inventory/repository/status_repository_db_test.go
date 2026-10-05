package repository_test

import (
	"context"
	"errors"
	"testing"

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
