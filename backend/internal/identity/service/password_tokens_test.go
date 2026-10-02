package service

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"storeit/internal/identity/domain"
	"storeit/internal/identity/job"
	"storeit/internal/platform/mail"
)

func adminCtx(t *testing.T, e *env) context.Context {
	t.Helper()
	admin := e.seed(t, "admin-"+uuid.NewString()[:6]+"@storeit.test", true, domain.AdministratorRoleID)
	return as(admin.ID, domain.PermAccountManage, domain.PermAccountRead)
}

// invited tạo account được mời qua service (như admin bấm "tạo")
func (e *env) invited(t *testing.T, ctx context.Context, email string) domain.Account {
	t.Helper()
	v, err := e.svc.CreateAccount(ctx, CreateAccountInput{Email: email, Name: "Invitee"})
	if err != nil {
		t.Fatal(err)
	}
	return v.Account
}

func TestCreateAccountIssuesInvite(t *testing.T) {
	e := newEnv(t)
	a := e.invited(t, adminCtx(t, e), "minh@storeit.test")

	call, ok := e.pwTokens.lastIssued()
	if !ok || call.accountID != a.ID {
		t.Fatalf("no invite issued for the new account: %+v", call)
	}
	tok := call.token
	if tok.Purpose != domain.PurposeInvite || !tok.ExpiresAt.Equal(e.now.Add(72*time.Hour)) {
		t.Errorf("invite = %s expiring %v, want invite expiring in 72h", tok.Purpose, tok.ExpiresAt)
	}
	if sum := sha256.Sum256([]byte(tok.Raw)); string(sum[:]) != string(tok.Hash) || len(tok.Raw) < 40 {
		t.Errorf("token hash is not SHA-256 of a 32-byte token (raw length %d)", len(tok.Raw))
	}
	if tok.ID == uuid.Nil {
		t.Error("token has no id")
	}
}

func TestResendInvitation(t *testing.T) {
	e := newEnv(t)
	ctx := adminCtx(t, e)
	inv := e.invited(t, ctx, "inv@storeit.test")
	active := e.seed(t, "act@storeit.test", true)
	off := e.invited(t, ctx, "off@storeit.test")
	if _, err := e.accounts.SetActive(ctx, off.ID, false); err != nil {
		t.Fatal(err)
	}

	if err := e.svc.ResendInvitation(ctx, inv.ID); err != nil {
		t.Fatal(err)
	}
	call, _ := e.pwTokens.lastIssued()
	if call.accountID != inv.ID || call.token.Purpose != domain.PurposeInvite || call.ev != domain.TokenEventInvitationResent {
		t.Errorf("resend issued %+v", call)
	}
	if err := e.svc.ResendInvitation(ctx, active.ID); !errors.Is(err, domain.ErrNotInvited) {
		t.Errorf("active account: %v, want ErrNotInvited", err)
	}
	if err := e.svc.ResendInvitation(ctx, off.ID); !errors.Is(err, domain.ErrAccountInactive) {
		t.Errorf("disabled account: %v, want ErrAccountInactive", err)
	}
	if err := e.svc.ResendInvitation(ctx, uuid.New()); !errors.Is(err, domain.ErrAccountNotFound) {
		t.Errorf("unknown account: %v, want ErrAccountNotFound", err)
	}
}

func TestSendPasswordReset(t *testing.T) {
	e := newEnv(t)
	ctx := adminCtx(t, e)
	active := e.seed(t, "act@storeit.test", true)
	inv := e.invited(t, ctx, "inv@storeit.test")
	off := e.seed(t, "off@storeit.test", false)

	if err := e.svc.SendPasswordReset(ctx, active.ID); err != nil {
		t.Fatal(err)
	}
	call, _ := e.pwTokens.lastIssued()
	if call.accountID != active.ID || call.token.Purpose != domain.PurposeReset || call.ev != domain.TokenEventResetSent ||
		!call.token.ExpiresAt.Equal(e.now.Add(time.Hour)) {
		t.Errorf("reset issued %+v", call)
	}
	// Chưa nhận lời mời: gửi lại lời mời thay vì link đặt lại
	if err := e.svc.SendPasswordReset(ctx, inv.ID); err != nil {
		t.Fatal(err)
	}
	call, _ = e.pwTokens.lastIssued()
	if call.token.Purpose != domain.PurposeInvite || call.ev != domain.TokenEventInvitationResent {
		t.Errorf("reset on invited account issued %s with event %d, want invite + invitation_resent", call.token.Purpose, call.ev)
	}
	if err := e.svc.SendPasswordReset(ctx, off.ID); !errors.Is(err, domain.ErrAccountInactive) {
		t.Errorf("disabled: %v, want ErrAccountInactive", err)
	}
	// Admin không bị cooldown: bấm lại ngay vẫn gửi
	before := e.pwTokens.issuedCount()
	if err := e.svc.SendPasswordReset(ctx, active.ID); err != nil || e.pwTokens.issuedCount() != before+1 {
		t.Errorf("second admin reset: %v, issued %d", err, e.pwTokens.issuedCount()-before)
	}
	if call, _ := e.pwTokens.lastIssued(); call.minAge != 0 {
		t.Errorf("admin reset passed cooldown %v, want 0", call.minAge)
	}
}

// Request công khai chỉ kiểm dạng email rồi xếp job: email có tài khoản hay
// không thì request cũng làm đúng một việc như nhau, không lộ qua thời gian
func TestForgotPasswordQueuesJob(t *testing.T) {
	e := newEnv(t)
	e.seed(t, "lan@storeit.test", true)
	anon := context.Background()

	for _, email := range []string{" LAN@StoreIT.test ", "ghost@storeit.test"} {
		before := len(e.jobs.queued)
		if err := e.svc.ForgotPassword(anon, email); err != nil {
			t.Fatalf("%s: %v", email, err)
		}
		if len(e.jobs.queued) != before+1 {
			t.Fatalf("%s: queued %d jobs, want 1", email, len(e.jobs.queued)-before)
		}
		args, ok := e.jobs.queued[len(e.jobs.queued)-1].(job.ForgotPasswordArgs)
		if !ok || args.Email != strings.ToLower(strings.TrimSpace(email)) {
			t.Errorf("%s: queued %#v, want ForgotPasswordArgs with the normalized email", email, e.jobs.queued[len(e.jobs.queued)-1])
		}
	}
	if e.pwTokens.issuedCount() != 0 {
		t.Error("the request itself issued a token; the job should")
	}
	// Sai dạng: không có gì để tra, không xếp job
	before := len(e.jobs.queued)
	if err := e.svc.ForgotPassword(anon, "not-an-email"); err != nil || len(e.jobs.queued) != before {
		t.Errorf("malformed: %v, queued %d", err, len(e.jobs.queued)-before)
	}
}

func TestProcessForgotPassword(t *testing.T) {
	e := newEnv(t)
	ctx := adminCtx(t, e)
	active := e.seed(t, "lan@storeit.test", true)
	e.seed(t, "off@storeit.test", false)
	inv := e.invited(t, ctx, "inv@storeit.test")
	jobCtx := context.Background()
	process := func(email string) error {
		return e.svc.ProcessForgotPassword(jobCtx, job.ForgotPasswordArgs{Email: email})
	}

	// Email lạ, account khoá: không phát gì, không lỗi (job xong)
	for _, email := range []string{"ghost@storeit.test", "off@storeit.test"} {
		before := e.pwTokens.issuedCount()
		if err := process(email); err != nil || e.pwTokens.issuedCount() != before {
			t.Errorf("%s: %v, issued %d", email, err, e.pwTokens.issuedCount()-before)
		}
	}

	if err := process("lan@storeit.test"); err != nil {
		t.Fatal(err)
	}
	call, _ := e.pwTokens.lastIssued()
	if call.accountID != active.ID || call.token.Purpose != domain.PurposeReset || call.ev != domain.TokenEventNone {
		t.Errorf("forgot issued %+v, want reset without event", call)
	}
	if call.minAge != 60*time.Second {
		t.Errorf("cooldown passed to the repository = %v, want 60s", call.minAge)
	}

	// Cooldown 60 giây mỗi account (repository quyết định, nguyên tử)
	before := e.pwTokens.issuedCount()
	e.now = e.now.Add(59 * time.Second)
	if err := process("lan@storeit.test"); err != nil || e.pwTokens.issuedCount() != before {
		t.Errorf("within cooldown: %v, issued %d", err, e.pwTokens.issuedCount()-before)
	}
	e.now = e.now.Add(time.Second)
	if err := process("lan@storeit.test"); err != nil || e.pwTokens.issuedCount() != before+1 {
		t.Errorf("after cooldown: %v, issued %d", err, e.pwTokens.issuedCount()-before)
	}

	// Account chưa nhận lời mời: gửi lời mời mới (lời mời cũ có thể đã hết hạn)
	e.now = e.now.Add(time.Hour)
	if err := process("inv@storeit.test"); err != nil {
		t.Fatal(err)
	}
	call, _ = e.pwTokens.lastIssued()
	if call.accountID != inv.ID || call.token.Purpose != domain.PurposeInvite || call.ev != domain.TokenEventNone {
		t.Errorf("forgot on invited account issued %+v, want invite without event", call)
	}

	// Account bị khoá giữa lúc xếp job và lúc chạy: repository từ chối, job vẫn xong
	e.now = e.now.Add(time.Hour)
	e.pwTokens.refuse = domain.ErrAccountInactive
	if err := process("lan@storeit.test"); err != nil {
		t.Errorf("account disabled meanwhile: %v, want nil", err)
	}
}

func TestSetPassword(t *testing.T) {
	e := newEnv(t)
	ctx := adminCtx(t, e)
	a := e.invited(t, ctx, "minh@storeit.test")
	call, _ := e.pwTokens.lastIssued()
	anon := context.Background()

	if err := e.svc.SetPassword(anon, call.token.Raw, "short"); !errors.Is(err, domain.ErrWeakPassword) {
		t.Errorf("weak: %v, want ErrWeakPassword", err)
	}
	for _, bad := range []string{"", "not-a-token"} {
		if err := e.svc.SetPassword(anon, bad, goodPassword); !errors.Is(err, domain.ErrInvalidPasswordToken) {
			t.Errorf("token %q: %v, want ErrInvalidPasswordToken", bad, err)
		}
	}
	if err := e.svc.SetPassword(anon, call.token.Raw, goodPassword); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Login(anon, "minh@storeit.test", goodPassword, Device{}); err != nil {
		t.Errorf("login after accepting the invite: %v", err)
	}
	got, _ := e.accounts.Get(anon, a.ID)
	if got.PasswordHash == goodPassword || !e.hasher.Compare(got.PasswordHash, goodPassword) {
		t.Error("password not stored as a bcrypt hash")
	}
	if err := e.svc.SetPassword(anon, call.token.Raw, goodPassword); !errors.Is(err, domain.ErrInvalidPasswordToken) {
		t.Errorf("reuse: %v, want ErrInvalidPasswordToken", err)
	}

	// Link hết hạn
	if err := e.svc.SendPasswordReset(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	reset, _ := e.pwTokens.lastIssued()
	e.now = e.now.Add(time.Hour)
	if err := e.svc.SetPassword(anon, reset.token.Raw, "another-good-password"); !errors.Is(err, domain.ErrInvalidPasswordToken) {
		t.Errorf("expired: %v, want ErrInvalidPasswordToken", err)
	}
}

func TestSendAccountEmail(t *testing.T) {
	e := newEnv(t)
	ctx := adminCtx(t, e)
	a := e.invited(t, ctx, "minh@storeit.test")
	call, _ := e.pwTokens.lastIssued()
	args := job.SendAccountEmailArgs{TokenID: call.token.ID, Purpose: string(call.token.Purpose), Token: call.token.Raw}
	jobCtx := context.Background()

	if err := e.svc.SendAccountEmail(jobCtx, args); err != nil {
		t.Fatal(err)
	}
	if len(e.mail.sent) != 1 {
		t.Fatalf("sent %d messages, want 1", len(e.mail.sent))
	}
	m := e.mail.sent[0]
	link := "http://app.test/accept-invite#token=" + call.token.Raw
	if m.To.Email != a.Email || m.To.Name != a.Name {
		t.Errorf("to = %+v", m.To)
	}
	if !strings.Contains(m.Text, link) || !strings.Contains(m.HTML, link) {
		t.Errorf("link %q missing from text or html:\n%s\n%s", link, m.Text, m.HTML)
	}
	if !strings.Contains(m.Text, "72 hours") {
		t.Errorf("text does not say how long the link lasts:\n%s", m.Text)
	}
	if m.IdempotencyKey != "identity/invite/"+call.token.ID.String() {
		t.Errorf("idempotency key = %q", m.IdempotencyKey)
	}
	if m.Subject == "" {
		t.Error("empty subject")
	}

	// Reset dùng trang khác và nói đúng thời hạn
	e.seed(t, "lan@storeit.test", true)
	if err := e.svc.ProcessForgotPassword(jobCtx, job.ForgotPasswordArgs{Email: "lan@storeit.test"}); err != nil {
		t.Fatal(err)
	}
	reset, _ := e.pwTokens.lastIssued()
	if err := e.svc.SendAccountEmail(jobCtx, job.SendAccountEmailArgs{TokenID: reset.token.ID, Purpose: "reset", Token: reset.token.Raw}); err != nil {
		t.Fatal(err)
	}
	m = e.mail.sent[1]
	if !strings.Contains(m.Text, "http://app.test/reset-password#token="+reset.token.Raw) || !strings.Contains(m.Text, "1 hour") {
		t.Errorf("reset email text:\n%s", m.Text)
	}
}

// Job chạy trễ hoặc chạy lại: link đã chết thì không gửi thư nào
func TestSendAccountEmailSkipsDeadLinks(t *testing.T) {
	e := newEnv(t)
	ctx := adminCtx(t, e)
	a := e.invited(t, ctx, "minh@storeit.test")
	first, _ := e.pwTokens.lastIssued()
	argsOf := func(c issueCall) job.SendAccountEmailArgs {
		return job.SendAccountEmailArgs{TokenID: c.token.ID, Purpose: string(c.token.Purpose), Token: c.token.Raw}
	}

	// Bị thay bởi lời mời mới
	if err := e.svc.ResendInvitation(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	second, _ := e.pwTokens.lastIssued()
	if err := e.svc.SendAccountEmail(context.Background(), argsOf(first)); err != nil {
		t.Errorf("superseded: %v, want nil", err)
	}
	// Hết hạn
	e.now = e.now.Add(73 * time.Hour)
	if err := e.svc.SendAccountEmail(context.Background(), argsOf(second)); err != nil {
		t.Errorf("expired: %v, want nil", err)
	}
	// Account bị khoá sau khi phát
	e.now = e.now.Add(-73 * time.Hour)
	if _, err := e.accounts.SetActive(ctx, a.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.SendAccountEmail(context.Background(), argsOf(second)); err != nil {
		t.Errorf("disabled: %v, want nil", err)
	}
	if len(e.mail.sent) != 0 {
		t.Errorf("sent %d messages for dead links", len(e.mail.sent))
	}
}

// Lỗi gửi đi nguyên vẹn qua lớp bọc, để worker phân biệt vĩnh viễn hay tạm thời
func TestSendAccountEmailPropagatesSenderError(t *testing.T) {
	e := newEnv(t)
	e.invited(t, adminCtx(t, e), "minh@storeit.test")
	call, _ := e.pwTokens.lastIssued()
	e.mail.err = &mail.StatusError{Status: 422, Reason: "domain not verified"}
	err := e.svc.SendAccountEmail(context.Background(), job.SendAccountEmailArgs{
		TokenID: call.token.ID, Purpose: "invite", Token: call.token.Raw,
	})
	if !mail.IsPermanent(err) {
		t.Errorf("err = %v, want a permanent mail error", err)
	}
	e.mail.err = fmt.Errorf("connection reset")
	if err := e.svc.SendAccountEmail(context.Background(), job.SendAccountEmailArgs{
		TokenID: call.token.ID, Purpose: "invite", Token: call.token.Raw,
	}); err == nil || mail.IsPermanent(err) {
		t.Errorf("err = %v, want a temporary error", err)
	}
}
