package repository_test

import (
	"errors"
	"testing"

	"storeit/internal/inventory/domain"
	"storeit/internal/inventory/repository/db"
)

// Tài sản đang Reserved hay On loan (US-10): không retire, không đổi status hay loại bằng tay,
// không bị thay thế; sửa trường khác vẫn được và status giữ nguyên
func TestAssetGuards_BorrowingAsset(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	a := activeAsset(t, r, "Reserved laptop")
	row, err := db.New(r.pool).SetAssetStatusQuiet(ctx, db.SetAssetStatusQuietParams{ID: a.ID, StatusID: domain.ReservedStatusID})
	if err != nil {
		t.Fatal(err)
	}
	ver := row.Version
	fields := domain.AssetFields{Name: a.Name, TypeID: a.TypeID, StatusID: domain.ReservedStatusID}

	if _, err := r.assets.Retire(ctx, a.ID, "gone", domain.RetiredStatusID, ver); !errors.Is(err, domain.ErrAssetInBorrowing) {
		t.Errorf("retire: %v, want ErrAssetInBorrowing", err)
	}
	f := fields
	f.StatusID = domain.AvailableStatusID
	if _, err := r.assets.Replace(ctx, a.ID, f, ver); !errors.Is(err, domain.ErrAssetInBorrowing) {
		t.Errorf("status change: %v, want ErrAssetInBorrowing", err)
	}
	f = fields
	f.TypeID = laptop(t, r).ID
	if _, err := r.assets.Replace(ctx, a.ID, f, ver); !errors.Is(err, domain.ErrAssetInBorrowing) {
		t.Errorf("type change: %v, want ErrAssetInBorrowing", err)
	}
	n := activeAsset(t, r, "New laptop")
	if _, err := r.assets.RecordReplacement(ctx, a.ID, n.ID, replacedDay, "x", domain.RetiredStatusID, ver); !errors.Is(err, domain.ErrAssetInBorrowing) {
		t.Errorf("replace as old asset: %v, want ErrAssetInBorrowing", err)
	}
	f = fields
	f.Name = "Renamed laptop"
	got, err := r.assets.Replace(ctx, a.ID, f, ver)
	if err != nil || got.Name != "Renamed laptop" || got.StatusID != domain.ReservedStatusID {
		t.Errorf("rename = %+v, %v; want renamed, still Reserved", got, err)
	}
}
