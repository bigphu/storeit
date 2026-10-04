// Package service là use case của identity: đăng nhập, phiên, account, role.
// Mọi use case quản trị bắt đầu bằng auth.Require; service không thấy HTTP,
// SQL hay transaction.
package service

import (
	"context"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/google/uuid"

	"storeit/internal/identity/domain"
	"storeit/internal/platform/auth"
	"storeit/internal/platform/jobs"
	"storeit/internal/platform/jwt"
	"storeit/internal/platform/mail"
)

// TokenIssuer phát access token; *jwt.Provider thoả interface này
type TokenIssuer interface {
	Issue(accountID uuid.UUID, permissions []string) (jwt.Token, error)
}

// JobQueue xếp job ngoài transaction; *jobs.River thoả interface này. Job đi
// kèm thay đổi dữ liệu thì repository xếp bằng EnqueueTx, không qua đây.
type JobQueue interface {
	Enqueue(ctx context.Context, job jobs.Job, opts ...jobs.Option) error
}

// Settings là thời hạn phiên (lấy từ identity.Config)
type Settings struct {
	SlidingTTL  time.Duration
	AbsoluteTTL time.Duration
	Grace       time.Duration
	// Giữ hàng phiên đã chết bao lâu trước khi PruneSessions xoá
	Retention time.Duration
	// Hạn của link mời và link đặt lại mật khẩu
	InviteTTL time.Duration
	ResetTTL  time.Duration
}

type Deps struct {
	Accounts domain.AccountRepository
	Roles    domain.RoleRepository
	Sessions domain.SessionRepository
	Hasher   Hasher
	Tokens   TokenIssuer
	// PasswordTokens lưu link mời / đặt lại mật khẩu
	PasswordTokens domain.TokenRepository
	// Mail chỉ cần cho SendAccountEmail (worker); API để nil
	Mail mail.Sender
	// Jobs xếp job quên mật khẩu
	Jobs JobQueue
	// AppURL là gốc của frontend, link trong thư trỏ về đây
	AppURL   string
	Settings Settings
	// Now cho test điều khiển thời gian; nil là time.Now
	Now func() time.Time
}

type Service struct {
	accounts domain.AccountRepository
	roles    domain.RoleRepository
	sessions domain.SessionRepository
	hasher   Hasher
	tokens   TokenIssuer
	pwTokens domain.TokenRepository
	mail     mail.Sender
	jobs     JobQueue
	appURL   string
	settings Settings
	now      func() time.Time

	// Hash giả để email lạ vẫn tốn đúng một lần bcrypt (chống dò email theo thời gian)
	dummyOnce sync.Once
	dummyHash string
}

func New(d Deps) *Service {
	now := d.Now
	if now == nil {
		now = time.Now
	}
	return &Service{
		accounts: d.Accounts, roles: d.Roles, sessions: d.Sessions,
		hasher: d.Hasher, tokens: d.Tokens, pwTokens: d.PasswordTokens, mail: d.Mail, jobs: d.Jobs,
		appURL: strings.TrimRight(d.AppURL, "/"), settings: d.Settings, now: now,
	}
}

// Device là ngữ cảnh thiết bị ghi vào phiên đăng nhập
type Device struct {
	UserAgent string
	IP        string
}

// AccountView là account kèm role của nó
type AccountView struct {
	domain.Account
	Roles []domain.Role
	// chỉ GetAccount điền: số phiên còn sống, hạn link mời đang chờ
	Sessions        *int64
	InviteExpiresAt *time.Time
}

// currentActor trả actor của request; không có (hoặc là SystemActor, không
// phải người) thì 401
func currentActor(ctx context.Context) (auth.Actor, error) {
	a, ok := auth.FromContext(ctx)
	if !ok || a.IsSystem() {
		return auth.Actor{}, auth.ErrUnauthenticated
	}
	return a, nil
}

func (s *Service) view(ctx context.Context, a domain.Account) (AccountView, error) {
	roles, err := s.accounts.Roles(ctx, a.ID)
	if err != nil {
		return AccountView{}, err
	}
	return AccountView{Account: a, Roles: roles}, nil
}

// cleanName bỏ khoảng trắng hai đầu; rỗng hay có ký tự điều khiển (xuống
// dòng, tab, NUL) là lỗi: tên đi vào thư, log và giao diện
func cleanName(name string) (string, error) {
	n := strings.TrimSpace(name)
	if n == "" || strings.ContainsFunc(n, unicode.IsControl) {
		return "", domain.ErrInvalidName
	}
	return n, nil
}

// requireHolds: actor phải có mọi quyền trong perms. Không ai trao, gỡ hay
// tác động lên quyền mình không có; SystemActor có mọi quyền.
func requireHolds(actor auth.Actor, perms []string) error {
	for _, p := range perms {
		if !actor.Can(p) {
			return domain.ErrExceedsOwnPermissions
		}
	}
	return nil
}

// symmetricDiff trả phần tử chỉ có ở một trong hai tập: thứ bị thêm hoặc bị gỡ
func symmetricDiff[T comparable](a, b []T) []T {
	inA := make(map[T]bool, len(a))
	for _, x := range a {
		inA[x] = true
	}
	inB := make(map[T]bool, len(b))
	var out []T
	for _, x := range b {
		inB[x] = true
		if !inA[x] {
			out = append(out, x)
		}
	}
	for _, x := range a {
		if !inB[x] {
			out = append(out, x)
		}
	}
	return out
}
