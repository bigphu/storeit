package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"storeit/internal/inventory/contract"
	"storeit/internal/inventory/domain"
)

func TestExportProfiles_CRUD(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	owner, other := uuid.New(), uuid.New()
	layout := domain.DefaultReportLayout()
	layout.TitleRow = true

	p, err := r.profiles.Create(ctx, domain.NewExportProfile{OwnerID: owner, Name: "Monthly " + uniq(), Layout: layout})
	if err != nil {
		t.Fatal(err)
	}
	if !p.Layout.TitleRow || p.Version != 1 {
		t.Errorf("created = %+v", p)
	}
	if _, err := r.profiles.Create(ctx, domain.NewExportProfile{OwnerID: owner, Name: p.Name + "", Layout: layout}); !errors.Is(err, domain.ErrExportProfileNameTaken) {
		t.Errorf("same name same owner: %v", err)
	}
	if _, err := r.profiles.Create(ctx, domain.NewExportProfile{OwnerID: other, Name: p.Name, Layout: layout}); err != nil {
		t.Errorf("same name other owner: %v", err)
	}
	shared, err := r.profiles.Create(ctx, domain.NewExportProfile{OwnerID: other, Name: "Shared " + uniq(), Shared: true, Layout: layout})
	if err != nil {
		t.Fatal(err)
	}

	list, err := r.profiles.List(context.Background(), owner)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[uuid.UUID]bool{}
	for _, x := range list {
		ids[x.ID] = true
	}
	if !ids[p.ID] || !ids[shared.ID] {
		t.Errorf("list misses own or shared profile")
	}

	name, sh := "Renamed "+uniq(), true
	up, err := r.profiles.Update(ctx, p.ID, domain.ExportProfileChange{Name: &name, Shared: &sh, Version: p.Version})
	if err != nil || up.Name != name || !up.Shared || up.Version != 2 {
		t.Fatalf("update = %+v, %v", up, err)
	}
	if _, err := r.profiles.Update(ctx, p.ID, domain.ExportProfileChange{Name: &name, Version: 1}); !errors.Is(err, domain.ErrExportProfileChanged) {
		t.Errorf("stale version: %v", err)
	}
	if err := r.profiles.Delete(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.profiles.Get(context.Background(), p.ID); !errors.Is(err, domain.ErrExportProfileNotFound) {
		t.Errorf("get deleted: %v", err)
	}
	for _, typ := range []string{contract.EventExportProfileCreated, contract.EventExportProfileUpdated, contract.EventExportProfileDeleted} {
		if countEvents(t, r, typ, p.ID) != 1 {
			t.Errorf("want one %s event", typ)
		}
	}
	if err := r.profiles.RecordExport(ctx, domain.ExportRecord{Mode: "data", Rows: 3, Sheets: 1, Filters: map[string]any{"q": "x"}}); err != nil {
		t.Errorf("record export: %v", err)
	}
}
