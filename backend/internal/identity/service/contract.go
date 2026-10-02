package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"storeit/internal/identity/contract"
	"storeit/internal/identity/domain"
	"storeit/internal/platform/auth"
	"storeit/internal/platform/jobs"
)

// Bootstrap tạo Administrator đầu tiên khi chưa có account nào. Có account rồi
// thì không làm gì (trả false), nên gọi mỗi lần khởi động là an toàn.
func (s *Service) Bootstrap(ctx context.Context, email, password string) (bool, error) {
	n, err := s.accounts.Count(ctx)
	if err != nil || n > 0 {
		return false, err
	}
	e, err := domain.NormalizeEmail(email)
	if err != nil {
		return false, err
	}
	if err := domain.ValidatePassword(password); err != nil {
		return false, err
	}
	h, err := s.hasher.Hash(password)
	if err != nil {
		return false, err
	}
	// Không do ai yêu cầu: event ghi actor là hệ thống
	ctx = auth.WithActor(ctx, auth.SystemActor)
	_, err = s.accounts.Create(ctx, domain.NewAccount{
		Email: e, Name: "Administrator", PasswordHash: h, RoleIDs: []uuid.UUID{domain.AdministratorRoleID},
	})
	// Hai server khởi động cùng lúc trên DB trống: bên kia vừa tạo xong, không phải lỗi
	if errors.Is(err, domain.ErrEmailTaken) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// LoadActor nạp quyền hiện tại của account cho job nền (jobs.ActorLoader).
// Account không còn: lỗi bọc jobs.ErrActorNotFound để job bị huỷ. Account bị
// khoá: actor không có quyền nào, việc nó yêu cầu sẽ bị từ chối khi chạy.
func (s *Service) LoadActor(ctx context.Context, id uuid.UUID) (auth.Actor, error) {
	a, err := s.accounts.Get(ctx, id)
	if errors.Is(err, domain.ErrAccountNotFound) {
		return auth.Actor{}, fmt.Errorf("identity: account %s: %w", id, jobs.ErrActorNotFound)
	}
	if err != nil {
		return auth.Actor{}, err
	}
	if !a.Active {
		return auth.Actor{AccountID: id}, nil
	}
	perms, err := s.accounts.Permissions(ctx, id)
	if err != nil {
		return auth.Actor{}, err
	}
	return auth.Actor{AccountID: id, Permissions: perms}, nil
}

// AccountReader trả cài đặt contract.AccountReader cho module khác
func (s *Service) AccountReader() contract.AccountReader { return accountReader{s} }

type accountReader struct{ s *Service }

func (r accountReader) GetAccount(ctx context.Context, id uuid.UUID) (contract.Account, error) {
	a, err := r.s.accounts.Get(ctx, id)
	if errors.Is(err, domain.ErrAccountNotFound) {
		return contract.Account{}, contract.ErrAccountNotFound
	}
	if err != nil {
		return contract.Account{}, err
	}
	return toContract(a), nil
}

func (r accountReader) GetAccounts(ctx context.Context, ids []uuid.UUID) ([]contract.Account, error) {
	as, err := r.s.accounts.GetMany(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]contract.Account, len(as))
	for i, a := range as {
		out[i] = toContract(a)
	}
	return out, nil
}

func toContract(a domain.Account) contract.Account {
	return contract.Account{ID: a.ID, Name: a.Name, Email: a.Email, Active: a.Active}
}
