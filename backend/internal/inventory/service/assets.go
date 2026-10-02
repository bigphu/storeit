package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"storeit/internal/inventory/domain"
	"storeit/internal/platform/auth"
)

// AssetInput là mọi trường người dùng gửi khi tạo hoặc sửa (PUT thay toàn bộ).
// Attributes: key thuộc tính -> giá trị JSON đã decode; key thiếu là không có giá trị.
type AssetInput struct {
	Name           string
	Description    string
	TypeID         uuid.UUID
	StatusID       *uuid.UUID // nil: tạo mới lấy status mặc định available, sửa thì giữ nguyên
	LocationID     *uuid.UUID
	HolderMemberID *uuid.UUID
	PurchaseDate   *time.Time
	Attributes     map[string]any
}

// AssetView là tài sản kèm loại (có thuộc tính, để hiện nhãn và đơn vị) và status
type AssetView struct {
	domain.Asset
	Type   domain.AssetType
	Status domain.Status
}

// ListAssets trả danh sách (không kèm giá trị thuộc tính); chi tiết dùng GetAsset
func (s *Service) ListAssets(ctx context.Context, f domain.AssetFilter) ([]domain.AssetListItem, int64, error) {
	if _, err := auth.Require(ctx, domain.PermAssetRead); err != nil {
		return nil, 0, err
	}
	// lọc theo một loại thì các dòng cùng bộ thuộc tính: kèm giá trị để hiện dạng bảng
	f.IncludeValues = f.TypeID != nil
	return s.assets.List(ctx, f)
}

func (s *Service) GetAsset(ctx context.Context, id uuid.UUID) (AssetView, error) {
	if _, err := auth.Require(ctx, domain.PermAssetRead); err != nil {
		return AssetView{}, err
	}
	a, err := s.assets.Get(ctx, id)
	if err != nil {
		return AssetView{}, err
	}
	return s.view(ctx, a)
}

// CreateAsset: tag chuẩn hoá và không đổi sau này; loại và status phải đang
// hoạt động; giá trị thuộc tính kiểm tra theo loại (US19-AC4)
func (s *Service) CreateAsset(ctx context.Context, tag string, in AssetInput) (AssetView, error) {
	if _, err := auth.Require(ctx, domain.PermAssetManage); err != nil {
		return AssetView{}, err
	}
	t, err := domain.NormalizeTag(tag)
	if err != nil {
		return AssetView{}, err
	}
	f, err := s.fields(ctx, in, nil)
	if err != nil {
		return AssetView{}, err
	}
	a, err := s.assets.Create(ctx, t, f)
	if err != nil {
		return AssetView{}, err
	}
	return s.view(ctx, a)
}

// UpdateAsset thay toàn bộ trường và giá trị thuộc tính. Loại hay status đã
// archive vẫn giữ được nếu không đổi; đổi loại thì giá trị cũ bị bỏ.
func (s *Service) UpdateAsset(ctx context.Context, id uuid.UUID, in AssetInput, version int32) (AssetView, error) {
	if _, err := auth.Require(ctx, domain.PermAssetManage); err != nil {
		return AssetView{}, err
	}
	cur, err := s.assets.Get(ctx, id)
	if err != nil {
		return AssetView{}, err
	}
	if cur.Retired() {
		return AssetView{}, domain.ErrAssetRetired
	}
	f, err := s.fields(ctx, in, &cur)
	if err != nil {
		return AssetView{}, err
	}
	a, err := s.assets.Replace(ctx, id, f, version)
	if err != nil {
		return AssetView{}, err
	}
	return s.view(ctx, a)
}

// RetireAsset (US-04): ẩn khỏi danh sách mặc định, giữ tag mãi mãi, không sửa
// được cho tới khi khôi phục
func (s *Service) RetireAsset(ctx context.Context, id uuid.UUID, reason string, version int32) (AssetView, error) {
	if _, err := auth.Require(ctx, domain.PermAssetManage); err != nil {
		return AssetView{}, err
	}
	r, err := cleanText(reason, maxRetireReasonLn)
	if err != nil {
		return AssetView{}, err
	}
	st, err := s.statuses.Default(ctx, domain.KindRetired)
	if err != nil {
		return AssetView{}, err
	}
	a, err := s.assets.Retire(ctx, id, r, st.ID, version)
	if err != nil {
		return AssetView{}, err
	}
	return s.view(ctx, a)
}

// RestoreAsset đưa tài sản đã retire về status mặc định available
func (s *Service) RestoreAsset(ctx context.Context, id uuid.UUID, version int32) (AssetView, error) {
	if _, err := auth.Require(ctx, domain.PermAssetManage); err != nil {
		return AssetView{}, err
	}
	st, err := s.statuses.Default(ctx, domain.KindAvailable)
	if err != nil {
		return AssetView{}, err
	}
	a, err := s.assets.Restore(ctx, id, st.ID, version)
	if err != nil {
		return AssetView{}, err
	}
	return s.view(ctx, a)
}

// fields kiểm tra input thành AssetFields. cur là tài sản đang sửa (nil khi tạo):
// loại/status đã archive chỉ được giữ khi không đổi.
func (s *Service) fields(ctx context.Context, in AssetInput, cur *domain.Asset) (domain.AssetFields, error) {
	name, err := domain.CleanLabel(in.Name, maxAssetNameLen)
	if err != nil {
		return domain.AssetFields{}, err
	}
	desc, err := cleanText(in.Description, maxAssetDescLen)
	if err != nil {
		return domain.AssetFields{}, err
	}
	t, err := s.types.Get(ctx, in.TypeID)
	if err != nil {
		return domain.AssetFields{}, err
	}
	if t.Archived() && (cur == nil || cur.TypeID != t.ID) {
		return domain.AssetFields{}, domain.ErrTypeArchived
	}
	st, err := s.status(ctx, in.StatusID, cur)
	if err != nil {
		return domain.AssetFields{}, err
	}
	values, err := domain.ValidateValues(t, in.Attributes)
	if err != nil {
		return domain.AssetFields{}, err
	}
	var purchase *time.Time
	if in.PurchaseDate != nil {
		d := time.Date(in.PurchaseDate.Year(), in.PurchaseDate.Month(), in.PurchaseDate.Day(), 0, 0, 0, 0, time.UTC)
		purchase = &d
	}
	return domain.AssetFields{
		Name: name, Description: desc, TypeID: t.ID, StatusID: st.ID,
		LocationID: in.LocationID, HolderMemberID: in.HolderMemberID, PurchaseDate: purchase, Values: values,
	}, nil
}

// status chọn status cho tài sản: nil thì mặc định (tạo) hoặc giữ nguyên (sửa).
// Status kind retired chỉ đặt bằng retire.
func (s *Service) status(ctx context.Context, id *uuid.UUID, cur *domain.Asset) (domain.Status, error) {
	if id == nil {
		if cur != nil {
			return s.statuses.Get(ctx, cur.StatusID)
		}
		return s.statuses.Default(ctx, domain.KindAvailable)
	}
	st, err := s.statuses.Get(ctx, *id)
	if err != nil {
		return domain.Status{}, err
	}
	if st.Kind == domain.KindRetired {
		return domain.Status{}, domain.ErrRetiredStatus
	}
	if st.Archived() && (cur == nil || cur.StatusID != st.ID) {
		return domain.Status{}, domain.ErrStatusArchived
	}
	return st, nil
}

func (s *Service) view(ctx context.Context, a domain.Asset) (AssetView, error) {
	t, err := s.types.Get(ctx, a.TypeID)
	if err != nil {
		return AssetView{}, err
	}
	st, err := s.statuses.Get(ctx, a.StatusID)
	if err != nil {
		return AssetView{}, err
	}
	return AssetView{Asset: a, Type: t, Status: st}, nil
}
