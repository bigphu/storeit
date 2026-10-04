package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Interface repository do service dùng, repository/ cài đặt. Mọi method ghi tự
// mở transaction và ghi event vào outbox trong transaction đó.

type NewAttribute struct {
	Key, Label string
	DataType   DataType
	Unit       string
	Required   bool
	Position   int32
	Options    []string // nhãn option, chỉ cho select
}

type NewAssetType struct {
	Code, Name, Description string
	Attributes              []NewAttribute
}

// AttributeChange: field nil là giữ nguyên. Unit "" là bỏ đơn vị.
type AttributeChange struct {
	Label    *string
	Unit     *string
	DataType *DataType
	Required *bool
	Position *int32
}

type TypeRepository interface {
	List(ctx context.Context, includeArchived bool) ([]AssetType, error)
	// Get: kèm mọi thuộc tính (kể cả đã gỡ) và option; ErrTypeNotFound
	Get(ctx context.Context, id uuid.UUID) (AssetType, error)
	// Create: ErrTypeCodeTaken, ErrTypeNameTaken, ErrAttributeKeyTaken...; event asset_type_created
	Create(ctx context.Context, in NewAssetType) (AssetType, error)
	// Update: ErrAssetTypeChanged khi version cũ; event asset_type_updated
	Update(ctx context.Context, id uuid.UUID, name, description *string, version int32) (AssetType, error)
	SetArchived(ctx context.Context, id uuid.UUID, archived bool) (AssetType, error)
	AddAttribute(ctx context.Context, typeID uuid.UUID, in NewAttribute) (Attribute, error)
	// UpdateAttribute: đổi kiểu hay đơn vị khi đã có giá trị là ErrAttributeInUse;
	// bỏ kiểu select thì xoá option của nó
	UpdateAttribute(ctx context.Context, typeID, attrID uuid.UUID, ch AttributeChange) (Attribute, error)
	RemoveAttribute(ctx context.Context, typeID, attrID uuid.UUID) error
	// AddOption: thuộc tính không phải select là ErrNotSelectAttribute
	AddOption(ctx context.Context, typeID, attrID uuid.UUID, label string, position int32) (Option, error)
	UpdateOption(ctx context.Context, typeID, attrID, optID uuid.UUID, label *string, position *int32) (Option, error)
	RemoveOption(ctx context.Context, typeID, attrID, optID uuid.UUID) error
}

type NewStatus struct {
	Name     string
	Kind     StatusKind
	Position int32
}

// StatusChange: field nil là giữ nguyên; MakeDefault chuyển cờ mặc định của kind sang status này
type StatusChange struct {
	Name        *string
	Position    *int32
	MakeDefault bool
}

type StatusRepository interface {
	List(ctx context.Context, includeArchived bool) ([]Status, error)
	Get(ctx context.Context, id uuid.UUID) (Status, error)
	// Default: status mặc định của kind
	Default(ctx context.Context, kind StatusKind) (Status, error)
	Create(ctx context.Context, in NewStatus) (Status, error)
	Update(ctx context.Context, id uuid.UUID, ch StatusChange) (Status, error)
	// Archive: ErrSystemStatus, ErrStatusIsDefault
	Archive(ctx context.Context, id uuid.UUID) (Status, error)
}

// AssetFields là mọi trường sửa được của tài sản (PUT thay toàn bộ)
type AssetFields struct {
	Name, Description string
	TypeID, StatusID  uuid.UUID
	LocationID        *uuid.UUID
	HolderMemberID    *uuid.UUID
	PurchaseDate      *time.Time
	Values            []Value
}

// AssetSort: tên cột hoặc "attributes.<key>" (cần lọc theo loại), tiền tố "-" là giảm dần
type AssetSort string

const (
	SortTag              AssetSort = "tag"
	SortTagDesc          AssetSort = "-tag"
	SortName             AssetSort = "name"
	SortNameDesc         AssetSort = "-name"
	SortPurchaseDate     AssetSort = "purchase_date"
	SortPurchaseDateDesc AssetSort = "-purchase_date"
	SortUpdatedAt        AssetSort = "updated_at"
	SortUpdatedAtDesc    AssetSort = "-updated_at"
	SortAssetType        AssetSort = "asset_type" // theo tên loại
	SortAssetTypeDesc    AssetSort = "-asset_type"
	SortStatus           AssetSort = "status" // theo thứ tự status (position, rồi tên) như danh sách status
	SortStatusDesc       AssetSort = "-status"
)

type AssetFilter struct {
	Query          string // tìm trong tag và tên, không phân biệt hoa thường
	TypeID         *uuid.UUID
	StatusID       *uuid.UUID
	StatusKind     *StatusKind
	LocationID     *uuid.UUID
	HolderMemberID *uuid.UUID
	IncludeRetired bool
	Sort           AssetSort
	Limit, Offset  int32
	// Attrs: điều kiện "<key>:<op>:<value>" trên thuộc tính tuỳ chỉnh (AND), cần TypeID
	Attrs []string
	// Service điền từ Attrs và Sort "attributes.<key>" (ResolveAttrQuery);
	// repository chỉ đọc hai trường này
	AttrFilters []AttrFilter
	AttrOrder   *AttrOrder
	// IncludeValues: nạp giá trị thuộc tính cho các dòng của trang (một truy vấn
	// thêm). Service bật khi lọc theo một loại: các dòng cùng cột, dựng được bảng.
	IncludeValues bool
}

// AssetListItem là một dòng của danh sách; Asset.Values chỉ có khi IncludeValues
type AssetListItem struct {
	Asset
	TypeName   string
	StatusName string
	StatusKind StatusKind
}

type AssetRepository interface {
	// Create: ErrTagTaken; event asset_created
	Create(ctx context.Context, tag string, f AssetFields) (Asset, error)
	// Get: kèm giá trị thuộc tính; ErrAssetNotFound
	Get(ctx context.Context, id uuid.UUID) (Asset, error)
	List(ctx context.Context, f AssetFilter) ([]AssetListItem, int64, error)
	// CountByType: số tài sản chưa retire của mỗi loại; loại không có tài sản thì không có trong map
	CountByType(ctx context.Context) (map[uuid.UUID]int64, error)
	// Replace thay toàn bộ trường và giá trị: ErrAssetRetired, ErrAssetChanged;
	// đổi loại thì xoá giá trị cũ trước; event asset_updated
	Replace(ctx context.Context, id uuid.UUID, f AssetFields, version int32) (Asset, error)
	Retire(ctx context.Context, id uuid.UUID, reason string, retiredStatus uuid.UUID, version int32) (Asset, error)
	Restore(ctx context.Context, id uuid.UUID, availableStatus uuid.UUID, version int32) (Asset, error)
}
