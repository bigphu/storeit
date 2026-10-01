package repository_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"storeit/internal/identity/contract"
	"storeit/internal/identity/domain"
)

// issued tạo token mới cho test; hash là SHA-256 của raw như service làm
func issued(p domain.TokenPurpose, expires time.Time) domain.IssuedToken {
	raw := "raw-" + uuid.NewString()
	return domain.IssuedToken{ID: uuid.New(), Purpose: p, Raw: raw, Hash: hash(raw), ExpiresAt: expires}
}

// emailJobs đếm job gửi thư của token id, và trả token thô trong args
func emailJobs(t *testing.T, r repos, tokenID uuid.UUID) (n int, raw string) {
	t.Helper()
	err := r.pool.QueryRow(context.Background(), `
		SELECT count(*), coalesce(max(args->>'token'), '') FROM river_job
		WHERE kind = 'identity.send_account_email' AND args->>'token_id' = $1`, tokenID.String()).Scan(&n, &raw)
	if err != nil {
		t.Fatal(err)
	}
	return n, raw
}

func newInvited(t *testing.T, r repos, admin uuid.UUID) (domain.Account, domain.IssuedToken) {
	t.Helper()
	inv := issued(domain.PurposeInvite, time.Now().Add(72*time.Hour))
	a, err := r.accounts.Create(actorCtx(admin), domain.NewAccount{
		Email: "inv-" + uuid.NewString()[:8] + "@storeit.test", Name: "Invited",
		RoleIDs: []uuid.UUID{domain.EmployeeRoleID}, Invite: &inv,
	})
	if err != nil {
		t.Fatal(err)
	}
	return a, inv
}

func TestToken_CreateWithInvite(t *testing.T) {
	r := newRepos(t)
	a, inv := newInvited(t, r, uuid.New())

	if a.Status() != domain.StatusInvited || a.PasswordHash != "" {
		t.Errorf("account = %+v, want invited without password", a)
	}
	got, err := r.tokens.Latest(context.Background(), a.ID, domain.PurposeInvite)
	if err != nil || got == nil || got.ID != inv.ID {
		t.Fatalf("Latest = %+v, %v, want token %v", got, err, inv.ID)
	}
	if n, raw := emailJobs(t, r, inv.ID); n != 1 || raw != inv.Raw {
		t.Errorf("email jobs = %d (token %q), want 1 carrying the raw token", n, raw)
	}
}

func TestToken_IssueSupersedes(t *testing.T) {
	r := newRepos(t)
	a := newAccount(t, r)
	ctx := context.Background()
	first := issued(domain.PurposeReset, time.Now().Add(time.Hour))
	second := issued(domain.PurposeReset, time.Now().Add(time.Hour))
	for _, tok := range []domain.IssuedToken{first, second} {
		if err := r.tokens.Issue(ctx, a.ID, tok, domain.TokenEventNone); err != nil {
			t.Fatal(err)
		}
	}
	if got, _ := r.tokens.Latest(ctx, a.ID, domain.PurposeReset); got == nil || got.ID != second.ID {
		t.Errorf("Latest = %+v, want %v", got, second.ID)
	}
	if _, err := r.tokens.Get(ctx, first.ID); !errors.Is(err, domain.ErrInvalidPasswordToken) {
		t.Errorf("Get(first) = %v, want ErrInvalidPasswordToken", err)
	}
	if _, err := r.tokens.Use(ctx, first.Hash, time.Now(), "h2"); !errors.Is(err, domain.ErrInvalidPasswordToken) {
		t.Errorf("Use(first) = %v, want ErrInvalidPasswordToken", err)
	}
	if _, err := r.tokens.Use(ctx, second.Hash, time.Now(), "h2"); err != nil {
		t.Errorf("Use(second) = %v", err)
	}
	if n, _ := emailJobs(t, r, first.ID); n != 1 {
		t.Errorf("jobs for first = %d, want 1", n)
	}
}

func TestToken_UseInviteSetsPassword(t *testing.T) {
	r := newRepos(t)
	a, inv := newInvited(t, r, uuid.New())
	ctx := context.Background()

	used, err := r.tokens.Use(ctx, inv.Hash, time.Now(), "$2a$04$new")
	if err != nil {
		t.Fatal(err)
	}
	if used.AccountID != a.ID || used.Purpose != domain.PurposeInvite {
		t.Errorf("Use = %+v", used)
	}
	got, _ := r.accounts.Get(ctx, a.ID)
	if got.PasswordHash != "$2a$04$new" || got.Status() != domain.StatusActive {
		t.Errorf("account after accept = %+v", got)
	}
	// Actor của event là chính account vừa nhận lời
	if n, actor := countEvents(t, r, contract.EventInvitationAccepted, a.ID); n != 1 || actor == nil || *actor != a.ID {
		t.Errorf("invitation_accepted = %d by %v, want 1 by the account", n, actor)
	}
	if _, err := r.tokens.Use(ctx, inv.Hash, time.Now(), "$2a$04$again"); !errors.Is(err, domain.ErrInvalidPasswordToken) {
		t.Errorf("second Use = %v, want ErrInvalidPasswordToken", err)
	}
	if _, err := r.tokens.Get(ctx, inv.ID); !errors.Is(err, domain.ErrInvalidPasswordToken) {
		t.Errorf("Get after use = %v, want ErrInvalidPasswordToken", err)
	}
}

func TestToken_UseResetRevokesSessions(t *testing.T) {
	r := newRepos(t)
	a := newAccount(t, r)
	ctx := context.Background()
	fam, err := r.sessions.Start(ctx, domain.NewSession{
		AccountID: a.ID, TokenHash: hash(uuid.NewString()),
		ExpiresAt: time.Now().Add(time.Hour), AbsoluteExpiresAt: time.Now().Add(2 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	tok := issued(domain.PurposeReset, time.Now().Add(time.Hour))
	if err := r.tokens.Issue(ctx, a.ID, tok, domain.TokenEventNone); err != nil {
		t.Fatal(err)
	}
	if _, err := r.tokens.Use(ctx, tok.Hash, time.Now(), "$2a$04$reset"); err != nil {
		t.Fatal(err)
	}
	var reason *string
	if err := r.pool.QueryRow(ctx, `SELECT revoked_reason FROM identity.refresh_families WHERE id = $1`, fam).Scan(&reason); err != nil {
		t.Fatal(err)
	}
	if reason == nil || *reason != string(domain.RevokeAdmin) {
		t.Errorf("family revoked_reason = %v, want admin", reason)
	}
	if n, _ := countEvents(t, r, contract.EventInvitationAccepted, a.ID); n != 0 {
		t.Errorf("reset wrote %d invitation_accepted events", n)
	}
}

// Hai tab gửi cùng một link một lúc: đúng một bên thành công
func TestToken_UseConcurrent(t *testing.T) {
	r := newRepos(t)
	_, inv := newInvited(t, r, uuid.New())

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = r.tokens.Use(context.Background(), inv.Hash, time.Now(), "$2a$04$x")
		}()
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case !errors.Is(err, domain.ErrInvalidPasswordToken):
			t.Errorf("unexpected error: %v", err)
		}
	}
	if ok != 1 {
		t.Errorf("%d of 2 concurrent uses succeeded, want 1", ok)
	}
}

func TestToken_UseExpiredOrDisabled(t *testing.T) {
	r := newRepos(t)
	ctx := context.Background()

	a := newAccount(t, r)
	expired := issued(domain.PurposeReset, time.Now().Add(-time.Minute))
	if err := r.tokens.Issue(ctx, a.ID, expired, domain.TokenEventNone); err != nil {
		t.Fatal(err)
	}
	if _, err := r.tokens.Use(ctx, expired.Hash, time.Now(), "h"); !errors.Is(err, domain.ErrInvalidPasswordToken) {
		t.Errorf("expired: %v, want ErrInvalidPasswordToken", err)
	}
	if got, _ := r.accounts.Get(ctx, a.ID); got.PasswordHash != "$2a$04$hash" {
		t.Error("expired token changed the password")
	}

	// Khoá account xoá link đang chờ
	b, inv := newInvited(t, r, uuid.New())
	if _, err := r.accounts.SetActive(ctx, b.ID, false); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.tokens.Latest(ctx, b.ID, domain.PurposeInvite); got != nil {
		t.Errorf("token survived disable: %+v", got)
	}
	if _, err := r.tokens.Use(ctx, inv.Hash, time.Now(), "h"); !errors.Is(err, domain.ErrInvalidPasswordToken) {
		t.Errorf("after disable: %v, want ErrInvalidPasswordToken", err)
	}

	// Token phát cho account đã khoá (đường vòng) cũng không dùng được
	late := issued(domain.PurposeReset, time.Now().Add(time.Hour))
	if err := r.tokens.Issue(ctx, b.ID, late, domain.TokenEventNone); err != nil {
		t.Fatal(err)
	}
	if _, err := r.tokens.Use(ctx, late.Hash, time.Now(), "h"); !errors.Is(err, domain.ErrInvalidPasswordToken) {
		t.Errorf("disabled account: %v, want ErrInvalidPasswordToken", err)
	}
}

func TestToken_IssueEvents(t *testing.T) {
	r := newRepos(t)
	admin := uuid.New()
	a := newAccount(t, r)
	cases := []struct {
		ev   domain.TokenEvent
		p    domain.TokenPurpose
		typ  string
		want int
	}{
		{domain.TokenEventResetSent, domain.PurposeReset, contract.EventPasswordResetSent, 1},
		{domain.TokenEventInvitationResent, domain.PurposeInvite, contract.EventInvitationResent, 1},
		{domain.TokenEventNone, domain.PurposeReset, contract.EventPasswordResetSent, 1}, // không thêm
	}
	for _, tc := range cases {
		if err := r.tokens.Issue(actorCtx(admin), a.ID, issued(tc.p, time.Now().Add(time.Hour)), tc.ev); err != nil {
			t.Fatal(err)
		}
		if n, actor := countEvents(t, r, tc.typ, a.ID); n != tc.want || actor == nil || *actor != admin {
			t.Errorf("%s: %d events by %v, want %d by admin", tc.typ, n, actor, tc.want)
		}
	}
}

func TestToken_PruneAndGetUnknown(t *testing.T) {
	r := newRepos(t)
	ctx := context.Background()
	a := newAccount(t, r)
	old := issued(domain.PurposeReset, time.Now().Add(-2*time.Hour))
	if err := r.tokens.Issue(ctx, a.ID, old, domain.TokenEventNone); err != nil {
		t.Fatal(err)
	}
	n, err := r.tokens.Prune(ctx, time.Now().Add(-time.Hour))
	if err != nil || n < 1 {
		t.Errorf("Prune = %d, %v, want at least 1", n, err)
	}
	if got, _ := r.tokens.Latest(ctx, a.ID, domain.PurposeReset); got != nil {
		t.Errorf("expired token not pruned: %+v", got)
	}
	if _, err := r.tokens.Get(ctx, uuid.New()); !errors.Is(err, domain.ErrInvalidPasswordToken) {
		t.Errorf("Get(unknown) = %v, want ErrInvalidPasswordToken", err)
	}
}
