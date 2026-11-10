package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"storeit/internal/inventory/domain"
	"storeit/internal/platform/auth"
)

func actorWith(id uuid.UUID, perms ...string) context.Context {
	return auth.WithActor(context.Background(), auth.Actor{AccountID: id, Permissions: perms})
}

func TestExportProfiles_OwnershipAndSharing(t *testing.T) {
	e := newEnv()
	ownerID, otherID, managerID := uuid.New(), uuid.New(), uuid.New()
	owner := actorWith(ownerID, domain.PermAssetExport)
	other := actorWith(otherID, domain.PermAssetExport)
	manager := actorWith(managerID, domain.PermAssetExport, domain.PermExportProfileManage)

	p, err := e.svc.CreateExportProfile(owner, ExportProfileInput{Name: "  Monthly  ", Layout: domain.DefaultReportLayout()})
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Monthly" || !p.CanEdit || p.OwnerName == "" {
		t.Errorf("created = %+v", p)
	}
	if _, err := e.svc.GetExportProfile(other, p.ID); !errors.Is(err, domain.ErrExportProfileNotFound) {
		t.Errorf("private profile seen by another user: %v", err)
	}
	if _, err := e.svc.GetExportProfile(manager, p.ID); !errors.Is(err, domain.ErrExportProfileNotFound) {
		t.Errorf("private profile seen by a manager: %v", err)
	}

	shared := true
	p, err = e.svc.UpdateExportProfile(owner, p.ID, domain.ExportProfileChange{Shared: &shared, Version: p.Version})
	if err != nil {
		t.Fatal(err)
	}
	seen, err := e.svc.GetExportProfile(other, p.ID)
	if err != nil || seen.CanEdit {
		t.Errorf("shared profile for another user = %+v, %v (want visible, not editable)", seen, err)
	}
	name := "Hijacked"
	if _, err := e.svc.UpdateExportProfile(other, p.ID, domain.ExportProfileChange{Name: &name, Version: p.Version}); !errors.Is(err, domain.ErrExportProfileForbidden) {
		t.Errorf("non-owner edits shared profile: %v", err)
	}
	if err := e.svc.DeleteExportProfile(other, p.ID); !errors.Is(err, domain.ErrExportProfileForbidden) {
		t.Errorf("non-owner deletes shared profile: %v", err)
	}
	if v, _ := e.svc.GetExportProfile(manager, p.ID); !v.CanEdit {
		t.Error("manager can edit a shared profile")
	}
	if _, err := e.svc.UpdateExportProfile(manager, p.ID, domain.ExportProfileChange{Name: &name, Version: p.Version}); err != nil {
		t.Errorf("manager edits shared profile: %v", err)
	}
	list, _ := e.svc.ListExportProfiles(other)
	if len(list) != 1 {
		t.Errorf("other user lists %d profiles, want the shared one", len(list))
	}
}

func TestExportProfiles_ValidatesInput(t *testing.T) {
	e := newEnv()
	ctx := actorWith(uuid.New(), domain.PermAssetExport)
	bad := domain.DefaultReportLayout()
	bad.Columns = []domain.ExportColumn{{Field: "nope"}}
	if _, err := e.svc.CreateExportProfile(ctx, ExportProfileInput{Name: "x", Layout: bad}); !errors.Is(err, domain.ErrInvalidExportLayout) {
		t.Errorf("bad layout: %v", err)
	}
	if _, err := e.svc.CreateExportProfile(ctx, ExportProfileInput{Name: "  ", Layout: domain.DefaultReportLayout()}); !errors.Is(err, domain.ErrInvalidLabel) {
		t.Errorf("blank name: %v", err)
	}
	// tên: tối đa 100 ký tự (không phải byte), không ký tự điều khiển
	for _, name := range []string{strings.Repeat("a", 101), "a\tb", "a\u0000b"} {
		if _, err := e.svc.CreateExportProfile(ctx, ExportProfileInput{Name: name, Layout: domain.DefaultReportLayout()}); !errors.Is(err, domain.ErrInvalidLabel) {
			t.Errorf("name %q: %v, want ErrInvalidLabel", name, err)
		}
	}
	long := strings.Repeat("ạ", 100)
	if p, err := e.svc.CreateExportProfile(ctx, ExportProfileInput{Name: long, Layout: domain.DefaultReportLayout()}); err != nil || p.Name != long {
		t.Errorf("100 accented characters: %v", err)
	}
	// sửa: tên và layout sai bị từ chối trước khi chạm kho; không có quyền export thì 403
	cur, _ := e.svc.CreateExportProfile(ctx, ExportProfileInput{Name: "edit", Layout: domain.DefaultReportLayout()})
	blank := " "
	if _, err := e.svc.UpdateExportProfile(ctx, cur.ID, domain.ExportProfileChange{Name: &blank, Version: cur.Version}); !errors.Is(err, domain.ErrInvalidLabel) {
		t.Errorf("rename to blank: %v", err)
	}
	if _, err := e.svc.UpdateExportProfile(ctx, cur.ID, domain.ExportProfileChange{Layout: &bad, Version: cur.Version}); !errors.Is(err, domain.ErrInvalidExportLayout) {
		t.Errorf("bad layout on update: %v", err)
	}
	noExport := actorWith(uuid.New(), domain.PermAssetRead)
	if _, err := e.svc.CreateExportProfile(noExport, ExportProfileInput{Name: "x", Layout: domain.DefaultReportLayout()}); status(err) != 403 {
		t.Errorf("create without asset.export: %v", err)
	}

	p, err := e.svc.CreateExportProfile(ctx, ExportProfileInput{Name: "x", Layout: domain.ExportLayout{Columns: []domain.ExportColumn{{Field: "tag"}}}})
	if err != nil || p.Layout.Header != domain.HeaderBold {
		t.Errorf("defaults not applied: %+v, %v", p.Layout, err)
	}
}
