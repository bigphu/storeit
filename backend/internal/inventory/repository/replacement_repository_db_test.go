package repository_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"storeit/internal/inventory/contract"
	"storeit/internal/inventory/domain"
)

// activeAsset tạo một tài sản đang dùng (loại General) để thay thế
func activeAsset(t *testing.T, r repos, name string) domain.Asset {
	t.Helper()
	a, err := r.assets.Create(actorCtx(), "RP-"+uniq(), domain.AssetFields{
		Name: name, TypeID: domain.GeneralTypeID, StatusID: domain.AvailableStatusID,
	})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

var replacedDay = time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)

func TestReplacement_RecordAndRead(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	o, n := activeAsset(t, r, "Old"), activeAsset(t, r, "New")

	got, err := r.assets.RecordReplacement(ctx, o.ID, n.ID, replacedDay, "broken", domain.RetiredStatusID, o.Version)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Retired() || got.RetiredReason != "Replaced by "+n.Tag+": broken" || got.StatusID != domain.RetiredStatusID {
		t.Errorf("old after replace = retired %v reason %q status %v", got.Retired(), got.RetiredReason, got.StatusID)
	}
	by, _, err := r.assets.Replacements(ctx, o.ID)
	if err != nil || by == nil || by.AssetID != n.ID || by.Tag != n.Tag || by.Reason != "broken" || !by.ReplacedOn.Equal(replacedDay) {
		t.Fatalf("replaced_by of old = %+v, %v", by, err)
	}
	_, replaces, err := r.assets.Replacements(ctx, n.ID)
	if err != nil || len(replaces) != 1 || replaces[0].AssetID != o.ID || replaces[0].Tag != o.Tag {
		t.Fatalf("replaces of new = %+v, %v", replaces, err)
	}
	if c := countEvents(t, r, contract.EventAssetReplaced, o.ID); c != 1 {
		t.Errorf("asset_replaced on old = %d, want 1", c)
	}
	if c := countEvents(t, r, contract.EventAssetReplaces, n.ID); c != 1 {
		t.Errorf("asset_replaces on new = %d, want 1", c)
	}
	if c := countEvents(t, r, contract.EventAssetRetired, o.ID); c != 0 {
		t.Errorf("asset_retired on old = %d, want 0 (replaced says it)", c)
	}
}

func TestReplacement_Rules(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	o, n, n2 := activeAsset(t, r, "Old"), activeAsset(t, r, "New"), activeAsset(t, r, "Other")
	retired := activeAsset(t, r, "Retired")
	retired, err := r.assets.Retire(ctx, retired.ID, "gone", domain.RetiredStatusID, retired.Version)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := r.assets.RecordReplacement(ctx, o.ID, n.ID, replacedDay, "x", domain.RetiredStatusID, o.Version+5); !errors.Is(err, domain.ErrAssetChanged) {
		t.Errorf("stale version: %v, want ErrAssetChanged", err)
	}
	for name, newID := range map[string]uuid.UUID{"retired new": retired.ID, "itself": o.ID, "unknown": uuid.New()} {
		if _, err := r.assets.RecordReplacement(ctx, o.ID, newID, replacedDay, "x", domain.RetiredStatusID, o.Version); !errors.Is(err, domain.ErrInvalidReplacement) {
			t.Errorf("%s: %v, want ErrInvalidReplacement", name, err)
		}
	}
	old, err := r.assets.RecordReplacement(ctx, o.ID, n.ID, replacedDay, "x", domain.RetiredStatusID, o.Version)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.assets.RecordReplacement(ctx, o.ID, n2.ID, replacedDay, "x", domain.RetiredStatusID, old.Version); !errors.Is(err, domain.ErrAssetRetired) {
		t.Errorf("replace again: %v, want ErrAssetRetired", err)
	}
}

func TestReplacement_Undo(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	o, n := activeAsset(t, r, "Old"), activeAsset(t, r, "New")
	old, err := r.assets.RecordReplacement(ctx, o.ID, n.ID, replacedDay, "broken", domain.RetiredStatusID, o.Version)
	if err != nil {
		t.Fatal(err)
	}

	back, err := r.assets.UndoReplacement(ctx, o.ID, domain.AvailableStatusID, old.Version)
	if err != nil {
		t.Fatal(err)
	}
	if back.Retired() || back.StatusID != domain.AvailableStatusID {
		t.Errorf("old after undo = retired %v status %v", back.Retired(), back.StatusID)
	}
	if by, _, _ := r.assets.Replacements(ctx, o.ID); by != nil {
		t.Errorf("link still there: %+v", by)
	}
	for _, id := range []uuid.UUID{o.ID, n.ID} {
		if c := countEvents(t, r, contract.EventAssetReplacementUndone, id); c != 1 {
			t.Errorf("replacement_undone on %v = %d, want 1", id, c)
		}
	}
	if c := countEvents(t, r, contract.EventAssetRestored, o.ID); c != 0 {
		t.Errorf("asset_restored on old = %d, want 0", c)
	}
	if _, err := r.assets.UndoReplacement(ctx, o.ID, domain.AvailableStatusID, back.Version); !errors.Is(err, domain.ErrAssetNotReplaced) {
		t.Errorf("undo again: %v, want ErrAssetNotReplaced", err)
	}
}

// Restore thường trên tài sản đã bị thay = hoàn tác: bỏ liên kết, event replacement_undone
func TestReplacement_RestoreUndoes(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	o, n := activeAsset(t, r, "Old"), activeAsset(t, r, "New")
	old, err := r.assets.RecordReplacement(ctx, o.ID, n.ID, replacedDay, "broken", domain.RetiredStatusID, o.Version)
	if err != nil {
		t.Fatal(err)
	}
	back, err := r.assets.Restore(ctx, o.ID, domain.AvailableStatusID, old.Version)
	if err != nil {
		t.Fatal(err)
	}
	if back.Retired() {
		t.Error("still retired after restore")
	}
	if by, _, _ := r.assets.Replacements(ctx, o.ID); by != nil {
		t.Errorf("link still there: %+v", by)
	}
	if c := countEvents(t, r, contract.EventAssetReplacementUndone, o.ID); c != 1 {
		t.Errorf("replacement_undone on old = %d, want 1", c)
	}
	if c := countEvents(t, r, contract.EventAssetRestored, o.ID); c != 0 {
		t.Errorf("asset_restored on old = %d, want 0", c)
	}
}

// Hai người cùng thay một tài sản: đúng một người thành công
func TestReplacement_Concurrent(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	o, n1, n2 := activeAsset(t, r, "Old"), activeAsset(t, r, "New 1"), activeAsset(t, r, "New 2")
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i, n := range []domain.Asset{n1, n2} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = r.assets.RecordReplacement(ctx, o.ID, n.ID, replacedDay, "x", domain.RetiredStatusID, o.Version)
		}()
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, domain.ErrAssetChanged), errors.Is(err, domain.ErrAssetRetired):
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}
	if ok != 1 {
		t.Errorf("successes = %d, want exactly 1 (%v)", ok, errs)
	}
}

// A thay bằng B và B thay bằng A cùng lúc: khoá theo thứ tự ID nên không deadlock, đúng một
// lần thành công. 100 cặp chạy cùng lúc để các transaction thật sự chen nhau (một cặp lẻ thường
// xong trước khi cặp kia bắt đầu)
func TestReplacement_OppositeOrderNoDeadlock(t *testing.T) {
	r := newRepos(t)
	const pairs = 100
	ctx, cancel := context.WithTimeout(actorCtx(), 30*time.Second)
	defer cancel()
	type pair struct{ a, b domain.Asset }
	ps := make([]pair, pairs)
	for i := range ps {
		ps[i] = pair{activeAsset(t, r, "A"), activeAsset(t, r, "B")}
	}
	errs := make([][2]error, pairs)
	var wg sync.WaitGroup
	for i, p := range ps {
		for k, o := range [][2]domain.Asset{{p.a, p.b}, {p.b, p.a}} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, errs[i][k] = r.assets.RecordReplacement(ctx, o[0].ID, o[1].ID, replacedDay, "x", domain.RetiredStatusID, o[0].Version)
			}()
		}
	}
	wg.Wait()
	if ctx.Err() != nil {
		t.Fatal("timed out: possible deadlock")
	}
	// Postgres tự gỡ deadlock (huỷ một transaction, lỗi 40P01) nên vẫn có đúng một lần thành
	// công; phải kiểm lỗi của lần thua: khoá đúng thứ tự thì nó chờ rồi thấy tài sản thay đã
	// bị retire
	for i, e := range errs {
		ok := 0
		for _, err := range e {
			switch {
			case err == nil:
				ok++
			case !errors.Is(err, domain.ErrInvalidReplacement):
				t.Errorf("pair %d: loser error = %v, want ErrInvalidReplacement (replacement retired), not a deadlock", i, err)
			}
		}
		if ok != 1 {
			t.Errorf("pair %d: successes = %d, want exactly 1 (%v)", i, ok, e)
		}
	}
}

// Ngày thay trước ngày mua của một trong hai tài sản: 422 ở replaced_on, không ghi gì
func TestReplacement_NotBeforePurchase(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	bought := func(name, on string) domain.Asset {
		t.Helper()
		d, _ := time.Parse(time.DateOnly, on)
		a, err := r.assets.Create(ctx, "RP-"+uniq(), domain.AssetFields{
			Name: name, TypeID: domain.GeneralTypeID, StatusID: domain.AvailableStatusID, PurchaseDate: &d,
		})
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	o, n := bought("Old", "2026-10-10"), bought("New", "2026-10-15")
	early := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	if _, err := r.assets.RecordReplacement(ctx, o.ID, n.ID, early, "x", domain.RetiredStatusID, o.Version); !errors.Is(err, domain.ErrInvalidReplacement) {
		t.Fatalf("before the new asset was bought: %v, want ErrInvalidReplacement", err)
	}
	if by, _, _ := r.assets.Replacements(ctx, o.ID); by != nil {
		t.Errorf("link written despite the error: %+v", by)
	}
	if _, err := r.assets.RecordReplacement(ctx, o.ID, n.ID, early.AddDate(0, 0, 1), "x", domain.RetiredStatusID, o.Version); err != nil {
		t.Errorf("on the purchase day: %v, want ok", err)
	}
}
