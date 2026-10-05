package service

import (
	"context"

	"github.com/google/uuid"

	"storeit/internal/inventory/domain"
	"storeit/internal/platform/auth"
)

func (s *Service) ListStatuses(ctx context.Context, includeArchived bool) ([]domain.Status, error) {
	if _, err := auth.Require(ctx, domain.PermAssetRead); err != nil {
		return nil, err
	}
	return s.statuses.List(ctx, includeArchived)
}

// StatusAssetCounts: số tài sản (kể cả đã retire) của mỗi status
func (s *Service) StatusAssetCounts(ctx context.Context) (map[uuid.UUID]int64, error) {
	if _, err := auth.Require(ctx, domain.PermAssetRead); err != nil {
		return nil, err
	}
	return s.assets.CountByStatus(ctx)
}

// ReorderStatuses: kéo thả trong trang status; ids là mọi status đang dùng
func (s *Service) ReorderStatuses(ctx context.Context, ids []uuid.UUID) ([]domain.Status, error) {
	if _, err := auth.Require(ctx, domain.PermStatusManage); err != nil {
		return nil, err
	}
	return s.statuses.Reorder(ctx, ids)
}

// RestoreStatus: cho chọn lại status đã archive
func (s *Service) RestoreStatus(ctx context.Context, id uuid.UUID) (domain.Status, error) {
	if _, err := auth.Require(ctx, domain.PermStatusManage); err != nil {
		return domain.Status{}, err
	}
	return s.statuses.Restore(ctx, id)
}

func (s *Service) CreateStatus(ctx context.Context, in domain.NewStatus) (domain.Status, error) {
	if _, err := auth.Require(ctx, domain.PermStatusManage); err != nil {
		return domain.Status{}, err
	}
	name, err := domain.CleanLabel(in.Name, maxNameLen)
	if err != nil {
		return domain.Status{}, err
	}
	if !in.Kind.Valid() {
		return domain.Status{}, domain.ErrInvalidStatusKind
	}
	in.Name = name
	return s.statuses.Create(ctx, in)
}

// UpdateStatus đổi tên, vị trí, hoặc đặt làm mặc định của kind (cờ cũ chuyển sang)
func (s *Service) UpdateStatus(ctx context.Context, id uuid.UUID, ch domain.StatusChange) (domain.Status, error) {
	if _, err := auth.Require(ctx, domain.PermStatusManage); err != nil {
		return domain.Status{}, err
	}
	if ch.Name != nil {
		n, err := domain.CleanLabel(*ch.Name, maxNameLen)
		if err != nil {
			return domain.Status{}, err
		}
		ch.Name = &n
	}
	return s.statuses.Update(ctx, id, ch)
}

// ArchiveStatus: status hệ thống và status đang là mặc định không archive được
func (s *Service) ArchiveStatus(ctx context.Context, id uuid.UUID) (domain.Status, error) {
	if _, err := auth.Require(ctx, domain.PermStatusManage); err != nil {
		return domain.Status{}, err
	}
	cur, err := s.statuses.Get(ctx, id)
	if err != nil {
		return domain.Status{}, err
	}
	switch {
	case cur.IsSystem:
		return domain.Status{}, domain.ErrSystemStatus
	case cur.IsDefault:
		return domain.Status{}, domain.ErrStatusIsDefault
	}
	return s.statuses.Archive(ctx, id)
}
