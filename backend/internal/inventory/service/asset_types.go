package service

import (
	"context"

	"github.com/google/uuid"

	"storeit/internal/inventory/domain"
	"storeit/internal/platform/auth"
)

func (s *Service) ListAssetTypes(ctx context.Context, includeArchived bool) ([]domain.AssetType, error) {
	if _, err := auth.Require(ctx, domain.PermAssetRead); err != nil {
		return nil, err
	}
	return s.types.List(ctx, includeArchived)
}

// TypeSummaries: số tài sản chưa retire (tổng, theo kind) và nhãn thuộc tính đang dùng
// của mỗi loại; loại không có cả hai thì không có trong map
func (s *Service) TypeSummaries(ctx context.Context) (map[uuid.UUID]domain.TypeSummary, error) {
	if _, err := auth.Require(ctx, domain.PermAssetRead); err != nil {
		return nil, err
	}
	counts, err := s.assets.CountByType(ctx)
	if err != nil {
		return nil, err
	}
	labels, err := s.types.AttributeLabels(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]domain.TypeSummary, len(counts)+len(labels))
	for id, c := range counts {
		out[id] = domain.TypeSummary{Counts: c}
	}
	for id, l := range labels {
		sum := out[id]
		sum.AttributeLabels = l
		out[id] = sum
	}
	return out, nil
}

func (s *Service) GetAssetType(ctx context.Context, id uuid.UUID) (domain.AssetType, error) {
	if _, err := auth.Require(ctx, domain.PermAssetRead); err != nil {
		return domain.AssetType{}, err
	}
	return s.types.Get(ctx, id)
}

// CreateAssetType tạo loại cùng thuộc tính khởi tạo (US17-AC4)
func (s *Service) CreateAssetType(ctx context.Context, in domain.NewAssetType) (domain.AssetType, error) {
	if _, err := auth.Require(ctx, domain.PermTypeManage); err != nil {
		return domain.AssetType{}, err
	}
	code, err := domain.NormalizeTypeCode(in.Code)
	if err != nil {
		return domain.AssetType{}, err
	}
	name, err := domain.CleanLabel(in.Name, maxNameLen)
	if err != nil {
		return domain.AssetType{}, err
	}
	desc, err := cleanText(in.Description, maxTypeDescLen)
	if err != nil {
		return domain.AssetType{}, err
	}
	attrs := make([]domain.NewAttribute, len(in.Attributes))
	for i, a := range in.Attributes {
		if attrs[i], err = cleanNewAttribute(a); err != nil {
			return domain.AssetType{}, err
		}
		if attrs[i].Position == 0 {
			attrs[i].Position = int32(i + 1)
		}
	}
	return s.types.Create(ctx, domain.NewAssetType{Code: code, Name: name, Description: desc, Attributes: attrs})
}

func (s *Service) UpdateAssetType(ctx context.Context, id uuid.UUID, name, description *string, version int32) (domain.AssetType, error) {
	if _, err := auth.Require(ctx, domain.PermTypeManage); err != nil {
		return domain.AssetType{}, err
	}
	if name != nil {
		n, err := domain.CleanLabel(*name, maxNameLen)
		if err != nil {
			return domain.AssetType{}, err
		}
		name = &n
	}
	if description != nil {
		d, err := cleanText(*description, maxTypeDescLen)
		if err != nil {
			return domain.AssetType{}, err
		}
		description = &d
	}
	return s.types.Update(ctx, id, name, description, version)
}

// ArchiveAssetType: loại đã archive không chọn được cho tài sản mới; tài sản
// đang dùng nó giữ nguyên. Loại hệ thống (GENERAL) không archive được.
func (s *Service) ArchiveAssetType(ctx context.Context, id uuid.UUID) (domain.AssetType, error) {
	if _, err := auth.Require(ctx, domain.PermTypeManage); err != nil {
		return domain.AssetType{}, err
	}
	t, err := s.types.Get(ctx, id)
	if err != nil {
		return domain.AssetType{}, err
	}
	if t.IsSystem {
		return domain.AssetType{}, domain.ErrSystemType
	}
	return s.types.SetArchived(ctx, id, true)
}

func (s *Service) RestoreAssetType(ctx context.Context, id uuid.UUID) (domain.AssetType, error) {
	if _, err := auth.Require(ctx, domain.PermTypeManage); err != nil {
		return domain.AssetType{}, err
	}
	return s.types.SetArchived(ctx, id, false)
}

func (s *Service) AddAttribute(ctx context.Context, typeID uuid.UUID, in domain.NewAttribute) (domain.Attribute, error) {
	if _, err := auth.Require(ctx, domain.PermTypeManage); err != nil {
		return domain.Attribute{}, err
	}
	a, err := cleanNewAttribute(in)
	if err != nil {
		return domain.Attribute{}, err
	}
	return s.types.AddAttribute(ctx, typeID, a)
}

// UpdateAttribute: đổi kiểu hay đơn vị chỉ được khi chưa có giá trị
// (repository trả ErrAttributeInUse). Unit "" là bỏ đơn vị.
func (s *Service) UpdateAttribute(ctx context.Context, typeID, attrID uuid.UUID, ch domain.AttributeChange) (domain.Attribute, error) {
	if _, err := auth.Require(ctx, domain.PermTypeManage); err != nil {
		return domain.Attribute{}, err
	}
	if ch.Label != nil {
		l, err := domain.CleanLabel(*ch.Label, maxNameLen)
		if err != nil {
			return domain.Attribute{}, err
		}
		ch.Label = &l
	}
	if ch.DataType != nil && !ch.DataType.Valid() {
		return domain.Attribute{}, domain.ErrInvalidDataType
	}
	if ch.Unit != nil {
		// Đơn vị hợp lệ hay không tuỳ kiểu sau khi đổi
		dt := domain.DataType("")
		if ch.DataType != nil {
			dt = *ch.DataType
		} else {
			cur, err := s.attribute(ctx, typeID, attrID)
			if err != nil {
				return domain.Attribute{}, err
			}
			dt = cur.DataType
		}
		u, err := domain.CheckUnit(dt, *ch.Unit)
		if err != nil {
			return domain.Attribute{}, err
		}
		ch.Unit = &u
	}
	return s.types.UpdateAttribute(ctx, typeID, attrID, ch)
}

// RemoveAttribute ẩn thuộc tính khỏi form và kiểm tra; giá trị đã lưu giữ lại
func (s *Service) RemoveAttribute(ctx context.Context, typeID, attrID uuid.UUID) error {
	if _, err := auth.Require(ctx, domain.PermTypeManage); err != nil {
		return err
	}
	return s.types.RemoveAttribute(ctx, typeID, attrID)
}

// ReorderAttributes, ReorderOptions: kéo thả trong trang cài đặt loại
func (s *Service) ReorderAttributes(ctx context.Context, typeID uuid.UUID, ids []uuid.UUID) (domain.AssetType, error) {
	if _, err := auth.Require(ctx, domain.PermTypeManage); err != nil {
		return domain.AssetType{}, err
	}
	return s.types.ReorderAttributes(ctx, typeID, ids)
}

func (s *Service) ReorderOptions(ctx context.Context, typeID, attrID uuid.UUID, ids []uuid.UUID) (domain.Attribute, error) {
	if _, err := auth.Require(ctx, domain.PermTypeManage); err != nil {
		return domain.Attribute{}, err
	}
	return s.types.ReorderOptions(ctx, typeID, attrID, ids)
}

func (s *Service) AddOption(ctx context.Context, typeID, attrID uuid.UUID, label string, position int32) (domain.Option, error) {
	if _, err := auth.Require(ctx, domain.PermTypeManage); err != nil {
		return domain.Option{}, err
	}
	l, err := domain.CleanLabel(label, maxNameLen)
	if err != nil {
		return domain.Option{}, err
	}
	return s.types.AddOption(ctx, typeID, attrID, l, position)
}

func (s *Service) UpdateOption(ctx context.Context, typeID, attrID, optID uuid.UUID, label *string, position *int32) (domain.Option, error) {
	if _, err := auth.Require(ctx, domain.PermTypeManage); err != nil {
		return domain.Option{}, err
	}
	if label != nil {
		l, err := domain.CleanLabel(*label, maxNameLen)
		if err != nil {
			return domain.Option{}, err
		}
		label = &l
	}
	return s.types.UpdateOption(ctx, typeID, attrID, optID, label, position)
}

// RemoveOption: không chọn được cho giá trị mới; tài sản đang dùng nó vẫn hiện
func (s *Service) RemoveOption(ctx context.Context, typeID, attrID, optID uuid.UUID) error {
	if _, err := auth.Require(ctx, domain.PermTypeManage); err != nil {
		return err
	}
	return s.types.RemoveOption(ctx, typeID, attrID, optID)
}

func (s *Service) attribute(ctx context.Context, typeID, attrID uuid.UUID) (domain.Attribute, error) {
	t, err := s.types.Get(ctx, typeID)
	if err != nil {
		return domain.Attribute{}, err
	}
	for _, a := range t.Attributes {
		if a.ID == attrID && a.RemovedAt == nil {
			return a, nil
		}
	}
	return domain.Attribute{}, domain.ErrAttributeNotFound
}

// cleanNewAttribute kiểm tra và chuẩn hoá một thuộc tính mới
func cleanNewAttribute(a domain.NewAttribute) (domain.NewAttribute, error) {
	if err := domain.ValidateAttributeKey(a.Key); err != nil {
		return a, err
	}
	l, err := domain.CleanLabel(a.Label, maxNameLen)
	if err != nil {
		return a, err
	}
	a.Label = l
	if !a.DataType.Valid() {
		return a, domain.ErrInvalidDataType
	}
	if a.Unit, err = domain.CheckUnit(a.DataType, a.Unit); err != nil {
		return a, err
	}
	if len(a.Options) > 0 && a.DataType != domain.TypeSelect {
		return a, domain.ErrNotSelectAttribute
	}
	for i, o := range a.Options {
		if a.Options[i], err = domain.CleanLabel(o, maxNameLen); err != nil {
			return a, err
		}
	}
	return a, nil
}
