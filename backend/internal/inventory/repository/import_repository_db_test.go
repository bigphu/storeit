package repository_test

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"storeit/internal/inventory/domain"
	"storeit/internal/inventory/repository"
)

// ByTags đọc tài sản theo tag kèm giá trị thuộc tính, kể cả tài sản đã retire; tag không có thì vắng
func TestAssets_ByTags(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	typ := laptop(t, r)
	u := uniq()
	mk := func(tag string, withValues bool) domain.Asset {
		f := domain.AssetFields{Name: "By tag " + tag, TypeID: typ.ID, StatusID: domain.AvailableStatusID}
		if withValues {
			f.Values = []domain.Value{{AttributeID: attr(t, typ, "serial").ID, DataType: domain.TypeText, Text: ptr("SN-" + tag)}}
		}
		a, err := r.assets.Create(ctx, tag, f)
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	a := mk("BT1-"+u, true)
	b := mk("BT2-"+u, false)
	if _, err := r.assets.Retire(ctx, b.ID, "gone", domain.RetiredStatusID, b.Version); err != nil {
		t.Fatal(err)
	}

	got, err := r.assets.ByTags(ctx, []string{a.Tag, b.Tag, "NOPE-" + u})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d assets, want 2 (the unknown tag is simply absent)", len(got))
	}
	for _, x := range got {
		switch x.Tag {
		case a.Tag:
			if len(x.Values) != 1 || x.Values[0].Text == nil || *x.Values[0].Text != "SN-"+a.Tag {
				t.Errorf("values of %s = %+v", a.Tag, x.Values)
			}
		case b.Tag:
			if !x.Retired() || len(x.Values) != 0 {
				t.Errorf("%s: retired=%v values=%d", b.Tag, x.Retired(), len(x.Values))
			}
		default:
			t.Errorf("unexpected tag %s", x.Tag)
		}
	}
	if none, err := r.assets.ByTags(ctx, nil); err != nil || len(none) != 0 {
		t.Errorf("no tags = %v, %v", none, err)
	}
}

// Claim: một file có dòng tạo mới chỉ được giữ chỗ một lần, kể cả hai yêu cầu cùng lúc; file không tạo gì thì không bị chặn
func TestImports_ClaimBlocksTheSameFileWithCreates(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	imports := repository.NewImportRepository(r.pool)
	sha := strings.ReplaceAll(uuid.NewString()+uuid.NewString(), "-", "")[:64]
	who := uuid.New()
	claim := func(sha string, creates bool) error {
		_, err := imports.Claim(ctx, domain.ImportRecord{AccountID: who, FileName: "a.xlsx", SHA256: sha, HasCreates: creates, Rows: 3})
		return err
	}

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = claim(sha, true)
		}()
	}
	wg.Wait()
	ok, refused := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, domain.ErrImportAlreadyDone):
			refused++
		default:
			t.Errorf("unexpected: %v", err)
		}
	}
	if ok != 1 || refused != 1 {
		t.Fatalf("ok=%d refused=%d, want 1 and 1", ok, refused)
	}

	err := claim(sha, true)
	if !errors.Is(err, domain.ErrImportAlreadyDone) || !strings.Contains(err.Error(), "a.xlsx was imported on") {
		t.Errorf("third claim = %v, want a message that names the file and the earlier import", err)
	}
	// file không tạo gì thì chạy lại được; mỗi lần đều nằm trong lịch sử
	other := strings.ReplaceAll(uuid.NewString()+uuid.NewString(), "-", "")[:64]
	for i := 0; i < 2; i++ {
		id, err := imports.Claim(ctx, domain.ImportRecord{AccountID: who, FileName: "u.xlsx", SHA256: other, HasCreates: false, Rows: 1})
		if err != nil {
			t.Fatalf("update-only claim %d: %v", i+1, err)
		}
		if err := imports.Finish(ctx, id, domain.ImportCounts{Update: 1}); err != nil {
			t.Fatal(err)
		}
	}
	var n, updated int
	if err := r.pool.QueryRow(ctx, `SELECT count(*), coalesce(sum(updated_count), 0) FROM inventory.asset_imports WHERE file_sha256 = $1 AND finished_at IS NOT NULL`, other).Scan(&n, &updated); err != nil || n != 2 || updated != 2 {
		t.Errorf("history = %d finished rows, %d updated (err %v)", n, updated, err)
	}
}
