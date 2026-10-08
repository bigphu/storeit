package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

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
	// xoá mềm: GetAny vẫn thấy, Restore đưa về
	got, err := r.profiles.GetAny(context.Background(), p.ID)
	if err != nil || got.DeletedAt == nil {
		t.Errorf("GetAny after delete = %+v, %v", got, err)
	}
	back, err := r.profiles.Restore(ctx, p.ID, nil)
	if err != nil || back.DeletedAt != nil {
		t.Fatalf("restore = %+v, %v", back, err)
	}
	if countEvents(t, r, contract.EventExportProfileRestored, p.ID) != 1 {
		t.Error("want one export_profile_restored event")
	}
	if err := r.profiles.RecordExport(ctx, domain.ExportRecord{Mode: "data", Rows: 3, Sheets: 1, Filters: map[string]any{"q": "x"}}); err != nil {
		t.Errorf("record export: %v", err)
	}
}

// Chủ làm profile riêng tư (chưa commit) đúng lúc người khác khôi phục nó: khôi phục phải đợi,
// kiểm tra trên hàng đã khoá thấy riêng tư, và không khôi phục
func TestExportProfiles_RestoreChecksTheLockedRow(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	p, err := r.profiles.Create(ctx, domain.NewExportProfile{OwnerID: uuid.New(), Name: "Shared " + uniq(), Shared: true, Layout: domain.DefaultReportLayout()})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.profiles.Delete(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if _, err := tx.Exec(ctx, `UPDATE inventory.export_profiles SET shared = false WHERE id = $1`, p.ID); err != nil {
		t.Fatal(err)
	}
	onlyShared := func(cur domain.ExportProfile) error {
		if !cur.Shared {
			return domain.ErrExportProfileNotFound
		}
		return nil
	}
	done := make(chan error, 1)
	go func() {
		_, err := r.profiles.Restore(ctx, p.ID, onlyShared)
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("restore did not wait for the uncommitted change (err %v)", err)
	case <-time.After(300 * time.Millisecond):
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, domain.ErrExportProfileNotFound) {
		t.Errorf("restore = %v, want ErrExportProfileNotFound", err)
	}
	if cur, err := r.profiles.GetAny(ctx, p.ID); err != nil || cur.DeletedAt == nil {
		t.Errorf("profile after refused restore: deleted_at %v, err %v; want still deleted", cur.DeletedAt, err)
	}
}
