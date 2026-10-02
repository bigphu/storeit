package service

import (
	"bytes"
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"storeit/internal/identity/domain"
	"storeit/internal/platform/jobs"
	"storeit/internal/platform/mail"
)

// issueCall là một lần phát token (kể cả lời mời lúc tạo account)
type issueCall struct {
	accountID uuid.UUID
	token     domain.IssuedToken
	ev        domain.TokenEvent
	minAge    time.Duration
}

type storedToken struct {
	domain.PasswordToken
	hash []byte
}

// fakePasswordTokens giữ token trong bộ nhớ, áp đúng luật của repository:
// một token mỗi (account, purpose), dùng một lần, kiểm Usable
type fakePasswordTokens struct {
	mu       sync.Mutex
	accounts *fakeAccounts
	now      func() time.Time
	tokens   map[uuid.UUID]storedToken // theo token id
	issued   []issueCall
	used     []domain.UsedToken
	pruned   time.Time
	// refuse != nil: Issue trả lỗi này (account bị khoá giữa chừng)
	refuse error
}

func newFakePasswordTokens(accounts *fakeAccounts, now func() time.Time) *fakePasswordTokens {
	f := &fakePasswordTokens{accounts: accounts, now: now, tokens: map[uuid.UUID]storedToken{}}
	accounts.invites = f
	return f
}

func (f *fakePasswordTokens) Issue(_ context.Context, accountID uuid.UUID, t domain.IssuedToken, ev domain.TokenEvent, minAge time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.refuse != nil {
		return false, f.refuse
	}
	for id, s := range f.tokens {
		if s.AccountID == accountID && s.Purpose == t.Purpose {
			if f.now().Sub(s.CreatedAt) < minAge {
				return false, nil
			}
			delete(f.tokens, id)
		}
	}
	f.tokens[t.ID] = storedToken{
		PasswordToken: domain.PasswordToken{ID: t.ID, AccountID: accountID, Purpose: t.Purpose, CreatedAt: f.now(), ExpiresAt: t.ExpiresAt},
		hash:          t.Hash,
	}
	f.issued = append(f.issued, issueCall{accountID: accountID, token: t, ev: ev, minAge: minAge})
	return true, nil
}

func (f *fakePasswordTokens) Use(ctx context.Context, hash []byte, now time.Time, passwordHash string) (domain.UsedToken, error) {
	f.mu.Lock()
	var found *storedToken
	for id, s := range f.tokens {
		if bytes.Equal(s.hash, hash) {
			delete(f.tokens, id)
			found = &s
			break
		}
	}
	f.mu.Unlock()
	if found == nil {
		return domain.UsedToken{}, domain.ErrInvalidPasswordToken
	}
	a, err := f.accounts.Get(ctx, found.AccountID)
	if err != nil {
		return domain.UsedToken{}, err
	}
	if err := found.Usable(a, now); err != nil {
		return domain.UsedToken{}, err
	}
	f.accounts.mu.Lock()
	a.PasswordHash = passwordHash
	f.accounts.accounts[a.ID] = a
	f.accounts.mu.Unlock()
	u := domain.UsedToken{AccountID: a.ID, Purpose: found.Purpose}
	f.mu.Lock()
	f.used = append(f.used, u)
	f.mu.Unlock()
	return u, nil
}

func (f *fakePasswordTokens) Get(_ context.Context, id uuid.UUID) (domain.PasswordToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.tokens[id]
	if !ok {
		return domain.PasswordToken{}, domain.ErrInvalidPasswordToken
	}
	return s.PasswordToken, nil
}

func (f *fakePasswordTokens) Prune(_ context.Context, cutoff time.Time) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pruned = cutoff
	return 3, nil
}

// lastIssued trả lần phát cuối
func (f *fakePasswordTokens) lastIssued() (issueCall, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.issued) == 0 {
		return issueCall{}, false
	}
	return f.issued[len(f.issued)-1], true
}

func (f *fakePasswordTokens) issuedCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.issued)
}

// fakeMail ghi lại thư đã gửi; err != nil thì trả lỗi đó
type fakeMail struct {
	mu   sync.Mutex
	sent []mail.Message
	err  error
}

func (f *fakeMail) Send(_ context.Context, m mail.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, m)
	return nil
}

// fakeJobs ghi lại job service xếp (ngoài transaction)
type fakeJobs struct {
	mu     sync.Mutex
	queued []jobs.Job
}

func (f *fakeJobs) Enqueue(_ context.Context, j jobs.Job, _ ...jobs.Option) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queued = append(f.queued, j)
	return nil
}
