package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"storeit/internal/inventory/domain"
	"storeit/internal/platform/auth"
	"storeit/internal/platform/errs"
)

// ReplacementInput: tài sản thay thế, ngày (chỉ ngày) và lý do
type ReplacementInput struct {
	ReplacementID uuid.UUID
	ReplacedOn    time.Time
	Reason        string
}

// RecordReplacement: ghi tài sản oldID được thay bằng in.ReplacementID; tài sản cũ retire
func (s *Service) RecordReplacement(ctx context.Context, oldID uuid.UUID, in ReplacementInput, version int32) (AssetView, error) {
	if _, err := auth.Require(ctx, domain.PermAssetManage); err != nil {
		return AssetView{}, err
	}
	reason, err := cleanText(in.Reason, domain.MaxReplacementReason)
	if err != nil {
		return AssetView{}, replacementField("reason", "must be text up to 500 characters")
	}
	if reason == "" {
		return AssetView{}, replacementField("reason", "is required")
	}
	if err := domain.CheckReplacedOn(in.ReplacedOn, time.Now()); err != nil {
		return AssetView{}, err
	}
	st, err := s.statuses.Default(ctx, domain.KindRetired)
	if err != nil {
		return AssetView{}, err
	}
	a, err := s.assets.RecordReplacement(ctx, oldID, in.ReplacementID, in.ReplacedOn, reason, st.ID, version)
	if err != nil {
		return AssetView{}, err
	}
	return s.view(ctx, a)
}

// UndoReplacement: bỏ liên kết thay thế, khôi phục tài sản cũ về status mặc định available
func (s *Service) UndoReplacement(ctx context.Context, oldID uuid.UUID, version int32) (AssetView, error) {
	if _, err := auth.Require(ctx, domain.PermAssetManage); err != nil {
		return AssetView{}, err
	}
	st, err := s.statuses.Default(ctx, domain.KindAvailable)
	if err != nil {
		return AssetView{}, err
	}
	a, err := s.assets.UndoReplacement(ctx, oldID, st.ID, version)
	if err != nil {
		return AssetView{}, err
	}
	return s.view(ctx, a)
}

func replacementField(field, detail string) error {
	return domain.ErrInvalidReplacement.With(errs.WithFields(errs.FieldError{Field: field, Detail: detail}))
}
