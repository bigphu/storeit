package service

import (
	"context"

	"github.com/google/uuid"

	"storeit/internal/inventory/domain"
	"storeit/internal/platform/auth"
)

// Số dòng tối đa một lần export khi cấu hình để 0
const defaultExportMaxRows = 50_000

// ExportProfileView: profile kèm tên chủ và quyền sửa của người đang xem
type ExportProfileView struct {
	domain.ExportProfile
	OwnerName string
	CanEdit   bool
}

type ExportProfileInput struct {
	Name   string
	Shared bool
	Layout domain.ExportLayout
}

// ListExportProfiles: của mình và mọi profile được chia sẻ
func (s *Service) ListExportProfiles(ctx context.Context) ([]ExportProfileView, error) {
	actor, err := auth.Require(ctx, domain.PermAssetExport)
	if err != nil {
		return nil, err
	}
	ps, err := s.profiles.List(ctx, actor.AccountID)
	if err != nil {
		return nil, err
	}
	return s.profileViews(ctx, actor, ps)
}

func (s *Service) GetExportProfile(ctx context.Context, id uuid.UUID) (ExportProfileView, error) {
	actor, err := auth.Require(ctx, domain.PermAssetExport)
	if err != nil {
		return ExportProfileView{}, err
	}
	p, err := s.visibleProfile(ctx, actor, id)
	if err != nil {
		return ExportProfileView{}, err
	}
	return s.profileView(ctx, actor, p)
}

func (s *Service) CreateExportProfile(ctx context.Context, in ExportProfileInput) (ExportProfileView, error) {
	actor, err := auth.Require(ctx, domain.PermAssetExport)
	if err != nil {
		return ExportProfileView{}, err
	}
	name, err := domain.CleanLabel(in.Name, maxNameLen)
	if err != nil {
		return ExportProfileView{}, err
	}
	layout := in.Layout.WithDefaults()
	if err := layout.Validate(); err != nil {
		return ExportProfileView{}, err
	}
	p, err := s.profiles.Create(ctx, domain.NewExportProfile{OwnerID: actor.AccountID, Name: name, Shared: in.Shared, Layout: layout})
	if err != nil {
		return ExportProfileView{}, err
	}
	return s.profileView(ctx, actor, p)
}

// UpdateExportProfile: chủ sửa được; profile chia sẻ thì người có quyền quản lý cũng sửa được
func (s *Service) UpdateExportProfile(ctx context.Context, id uuid.UUID, ch domain.ExportProfileChange) (ExportProfileView, error) {
	actor, err := auth.Require(ctx, domain.PermAssetExport)
	if err != nil {
		return ExportProfileView{}, err
	}
	cur, err := s.visibleProfile(ctx, actor, id)
	if err != nil {
		return ExportProfileView{}, err
	}
	if !canEditProfile(actor, cur) {
		return ExportProfileView{}, domain.ErrExportProfileForbidden
	}
	if ch.Name != nil {
		n, err := domain.CleanLabel(*ch.Name, maxNameLen)
		if err != nil {
			return ExportProfileView{}, err
		}
		ch.Name = &n
	}
	if ch.Layout != nil {
		l := ch.Layout.WithDefaults()
		if err := l.Validate(); err != nil {
			return ExportProfileView{}, err
		}
		ch.Layout = &l
	}
	p, err := s.profiles.Update(ctx, id, ch)
	if err != nil {
		return ExportProfileView{}, err
	}
	return s.profileView(ctx, actor, p)
}

func (s *Service) DeleteExportProfile(ctx context.Context, id uuid.UUID) error {
	actor, err := auth.Require(ctx, domain.PermAssetExport)
	if err != nil {
		return err
	}
	cur, err := s.visibleProfile(ctx, actor, id)
	if err != nil {
		return err
	}
	if !canEditProfile(actor, cur) {
		return domain.ErrExportProfileForbidden
	}
	return s.profiles.Delete(ctx, id)
}

// RestoreExportProfile: hoàn tác xoá. Ai xoá được thì khôi phục được; profile riêng của
// người khác (kể cả đã xoá) như không tồn tại
func (s *Service) RestoreExportProfile(ctx context.Context, id uuid.UUID) (ExportProfileView, error) {
	actor, err := auth.Require(ctx, domain.PermAssetExport)
	if err != nil {
		return ExportProfileView{}, err
	}
	cur, err := s.profiles.GetAny(ctx, id)
	if err != nil {
		return ExportProfileView{}, err
	}
	if cur.OwnerID != actor.AccountID && !cur.Shared {
		return ExportProfileView{}, domain.ErrExportProfileNotFound
	}
	if !canEditProfile(actor, cur) {
		return ExportProfileView{}, domain.ErrExportProfileForbidden
	}
	p, err := s.profiles.Restore(ctx, id)
	if err != nil {
		return ExportProfileView{}, err
	}
	return s.profileView(ctx, actor, p)
}

// visibleProfile: profile của người khác mà không chia sẻ thì như không tồn tại
func (s *Service) visibleProfile(ctx context.Context, actor auth.Actor, id uuid.UUID) (domain.ExportProfile, error) {
	p, err := s.profiles.Get(ctx, id)
	if err != nil {
		return domain.ExportProfile{}, err
	}
	if p.OwnerID != actor.AccountID && !p.Shared {
		return domain.ExportProfile{}, domain.ErrExportProfileNotFound
	}
	return p, nil
}

func canEditProfile(actor auth.Actor, p domain.ExportProfile) bool {
	return p.OwnerID == actor.AccountID || (p.Shared && actor.Can(domain.PermExportProfileManage))
}

func (s *Service) profileView(ctx context.Context, actor auth.Actor, p domain.ExportProfile) (ExportProfileView, error) {
	vs, err := s.profileViews(ctx, actor, []domain.ExportProfile{p})
	if err != nil {
		return ExportProfileView{}, err
	}
	return vs[0], nil
}

func (s *Service) profileViews(ctx context.Context, actor auth.Actor, ps []domain.ExportProfile) ([]ExportProfileView, error) {
	ids := make([]uuid.UUID, 0, len(ps))
	for _, p := range ps {
		ids = append(ids, p.OwnerID)
	}
	accounts, err := s.accounts.GetAccounts(ctx, ids)
	if err != nil {
		return nil, err
	}
	names := make(map[uuid.UUID]string, len(accounts))
	for _, a := range accounts {
		names[a.ID] = a.Name
	}
	out := make([]ExportProfileView, len(ps))
	for i, p := range ps {
		name := names[p.OwnerID]
		if name == "" {
			name = "Unknown account"
		}
		out[i] = ExportProfileView{ExportProfile: p, OwnerName: name, CanEdit: canEditProfile(actor, p)}
	}
	return out, nil
}
