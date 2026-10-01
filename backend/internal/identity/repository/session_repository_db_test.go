package repository_test

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"storeit/internal/identity/domain"
)

const (
	grace   = 30 * time.Second
	sliding = time.Hour
)

// startSession mở family với token gốc "root"; trả hash của root và family
func startSession(t *testing.T, r repos, accountID uuid.UUID, now time.Time, absolute time.Duration) ([]byte, uuid.UUID) {
	t.Helper()
	root := hash("root-" + uuid.NewString())
	fam, err := r.sessions.Start(context.Background(), domain.NewSession{
		AccountID: accountID, TokenHash: root, UserAgent: "test", IP: "203.0.113.5",
		ExpiresAt: now.Add(sliding), AbsoluteExpiresAt: now.Add(absolute),
	})
	if err != nil {
		t.Fatal(err)
	}
	return root, fam
}

func refresh(t *testing.T, r repos, token []byte, now time.Time) (domain.RefreshResult, []byte) {
	t.Helper()
	next := hash("next-" + uuid.NewString())
	res, err := r.sessions.Refresh(context.Background(), domain.RefreshInput{
		TokenHash: token, NextHash: next, Now: now, Grace: grace, Sliding: sliding,
	})
	if err != nil {
		t.Fatal(err)
	}
	return res, next
}

func familyState(t *testing.T, r repos, fam uuid.UUID) (revokedReason *string, liveTokens int) {
	t.Helper()
	ctx := context.Background()
	if err := r.pool.QueryRow(ctx, `SELECT revoked_reason FROM identity.refresh_families WHERE id = $1`, fam).
		Scan(&revokedReason); err != nil {
		t.Fatal(err)
	}
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM identity.refresh_tokens WHERE family_id = $1 AND used_at IS NULL`, fam).
		Scan(&liveTokens); err != nil {
		t.Fatal(err)
	}
	return revokedReason, liveTokens
}

func TestRefresh_RotateGraceReuse(t *testing.T) {
	r := newRepos(t)
	a := newAccount(t, r)
	now := time.Now()
	root, fam := startSession(t, r, a.ID, now, 24*time.Hour)

	res, child := refresh(t, r, root, now)
	if res.Decision != domain.RefreshRotate || res.AccountID != a.ID || res.FamilyID != fam {
		t.Fatalf("first refresh = %+v", res)
	}
	if !res.ExpiresAt.Equal(now.Add(sliding).Truncate(time.Microsecond)) && res.ExpiresAt.Sub(now.Add(sliding)).Abs() > time.Millisecond {
		t.Errorf("child expiry = %v, want now+sliding", res.ExpiresAt)
	}

	// Gửi lại root trong cửa sổ ân hạn: xoay ngọn (child), không thu hồi
	res, _ = refresh(t, r, root, now.Add(5*time.Second))
	if res.Decision != domain.RefreshGrace {
		t.Fatalf("retry within grace = %v", res.Decision)
	}
	if reason, live := familyState(t, r, fam); reason != nil || live != 1 {
		t.Errorf("after grace: revoked %v, live tokens %d", reason, live)
	}

	// child đã bị xoay bởi lần ân hạn; dùng nó sau cửa sổ là dùng lại → thu hồi cả family
	res, _ = refresh(t, r, child, now.Add(time.Minute))
	if res.Decision != domain.RefreshReuse {
		t.Fatalf("reuse after grace = %v", res.Decision)
	}
	if reason, _ := familyState(t, r, fam); reason == nil || *reason != string(domain.RevokeReuseDetected) {
		t.Errorf("family reason = %v, want reuse_detected", reason)
	}
}

func TestRefresh_UnknownExpiredDisabled(t *testing.T) {
	r := newRepos(t)
	now := time.Now()

	if res, _ := refresh(t, r, hash("never-issued"), now); res.Decision != domain.RefreshReject {
		t.Errorf("unknown token = %v, want reject", res.Decision)
	}

	a := newAccount(t, r)
	root, fam := startSession(t, r, a.ID, now, 24*time.Hour)
	res, _ := refresh(t, r, root, now.Add(25*time.Hour))
	if res.Decision != domain.RefreshExpired {
		t.Errorf("past absolute = %v", res.Decision)
	}
	if reason, _ := familyState(t, r, fam); reason == nil || *reason != string(domain.RevokeExpired) {
		t.Errorf("reason = %v, want expired", reason)
	}

	b := newAccount(t, r)
	root, fam = startSession(t, r, b.ID, now, 24*time.Hour)
	if _, err := r.pool.Exec(context.Background(), `UPDATE identity.accounts SET active = false WHERE id = $1`, b.ID); err != nil {
		t.Fatal(err)
	}
	if res, _ := refresh(t, r, root, now); res.Decision != domain.RefreshAccountDisabled {
		t.Errorf("disabled account = %v", res.Decision)
	}
	if reason, _ := familyState(t, r, fam); reason == nil || *reason != string(domain.RevokeAdmin) {
		t.Errorf("reason = %v, want admin", reason)
	}
}

// Hai tab refresh cùng lúc bằng cùng cookie: một lần xoay, một lần ân hạn,
// và family không bao giờ có hai token sống
func TestRefresh_ConcurrentSameToken(t *testing.T) {
	r := newRepos(t)
	a := newAccount(t, r)
	now := time.Now()
	root, fam := startSession(t, r, a.ID, now, 24*time.Hour)

	var wg sync.WaitGroup
	decisions := make([]domain.RefreshDecision, 2)
	errs := make([]error, 2)
	for i := range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := r.sessions.Refresh(context.Background(), domain.RefreshInput{
				TokenHash: root, NextHash: hash("c-" + uuid.NewString()), Now: now, Grace: grace, Sliding: sliding,
			})
			decisions[i], errs[i] = res.Decision, err
		}()
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	slices.Sort(decisions)
	if !slices.Equal(decisions, []domain.RefreshDecision{domain.RefreshGrace, domain.RefreshRotate}) {
		t.Errorf("decisions = %v, want one rotate and one grace", decisions)
	}
	if reason, live := familyState(t, r, fam); reason != nil || live != 1 {
		t.Errorf("revoked %v, live tokens %d, want live family with exactly one token", reason, live)
	}
}

func TestSession_RevokeAndPrune(t *testing.T) {
	r := newRepos(t)
	a := newAccount(t, r)
	ctx := context.Background()
	now := time.Now()

	// Đăng xuất: token lạ không lỗi; token thật thu hồi đúng family
	if fam, err := r.sessions.Revoke(ctx, hash("unknown"), domain.RevokeLogout); err != nil || fam != nil {
		t.Errorf("revoke unknown = %v, %v", fam, err)
	}
	root, fam := startSession(t, r, a.ID, now, 24*time.Hour)
	got, err := r.sessions.Revoke(ctx, root, domain.RevokeLogout)
	if err != nil || got == nil || *got != fam {
		t.Fatalf("revoke = %v, %v", got, err)
	}
	if reason, _ := familyState(t, r, fam); reason == nil || *reason != string(domain.RevokeLogout) {
		t.Errorf("reason = %v, want logout", reason)
	}

	// Family còn sống với một token đã dùng đã hết hạn lâu, cộng family đã thu hồi ở trên
	liveRoot, liveFam := startSession(t, r, a.ID, now, 24*time.Hour)
	refresh(t, r, liveRoot, now)
	if _, err := r.pool.Exec(ctx, `UPDATE identity.refresh_tokens SET expires_at = now() - interval '60 days'
		WHERE family_id = $1 AND used_at IS NOT NULL`, liveFam); err != nil {
		t.Fatal(err)
	}
	if _, err := r.pool.Exec(ctx, `UPDATE identity.refresh_families SET revoked_at = now() - interval '60 days',
		absolute_expires_at = now() - interval '60 days' WHERE id = $1`, fam); err != nil {
		t.Fatal(err)
	}

	families, tokens, err := r.sessions.Prune(ctx, now.Add(-30*24*time.Hour))
	if err != nil || families < 1 || tokens < 1 {
		t.Errorf("prune = %d families, %d tokens, %v", families, tokens, err)
	}
	// Ngọn của family còn sống không bị xoá
	if _, live := familyState(t, r, liveFam); live != 1 {
		t.Errorf("live tokens in surviving family = %d, want 1", live)
	}
}
