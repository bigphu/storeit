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
	TagPrefix               string // "" là DefaultTagPrefix(Code)
	CanBorrow               bool
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
	// canBorrow nil: giữ nguyên
	Update(ctx context.Context, id uuid.UUID, name, description, tagPrefix *string, canBorrow *bool, version int32) (AssetType, error)
	SetArchived(ctx context.Context, id uuid.UUID, archived bool) (AssetType, error)
	AddAttribute(ctx context.Context, typeID uuid.UUID, in NewAttribute) (Attribute, error)
	// UpdateAttribute: đổi kiểu hay đơn vị khi đã có giá trị là ErrAttributeInUse;
	// bỏ kiểu select thì xoá option của nó
	UpdateAttribute(ctx context.Context, typeID, attrID uuid.UUID, ch AttributeChange) (Attribute, error)
	RemoveAttribute(ctx context.Context, typeID, attrID uuid.UUID) error
	// RestoreAttribute: hoàn tác bỏ thuộc tính (chưa bỏ thì trả nguyên); ErrAttributeNotFound,
	// ErrAttributeLabelTaken
	RestoreAttribute(ctx context.Context, typeID, attrID uuid.UUID) (Attribute, error)
	// ReorderAttributes: ids là mọi thuộc tính đang hoạt động theo thứ tự mới (vị trí
	// 1..n), không thì ErrInvalidOrder; một event cho cả lần sắp xếp
	ReorderAttributes(ctx context.Context, typeID uuid.UUID, ids []uuid.UUID) (AssetType, error)
	// ReorderOptions: như trên cho option đang hoạt động của một thuộc tính select
	ReorderOptions(ctx context.Context, typeID, attrID uuid.UUID, ids []uuid.UUID) (Attribute, error)
	// AddOption: thuộc tính không phải select là ErrNotSelectAttribute
	AddOption(ctx context.Context, typeID, attrID uuid.UUID, label string, position int32) (Option, error)
	UpdateOption(ctx context.Context, typeID, attrID, optID uuid.UUID, label *string, position *int32) (Option, error)
	RemoveOption(ctx context.Context, typeID, attrID, optID uuid.UUID) error
	// RestoreOption: hoàn tác bỏ option; thuộc tính phải còn dùng; ErrOptionNotFound,
	// ErrOptionLabelTaken
	RestoreOption(ctx context.Context, typeID, attrID, optID uuid.UUID) (Option, error)
	// AttributeLabels: nhãn thuộc tính đang dùng của mỗi loại theo thứ tự hiển thị;
	// loại không có thuộc tính thì không có trong map
	AttributeLabels(ctx context.Context) (map[uuid.UUID][]string, error)
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
	// Restore: status chưa archive thì trả nguyên; event status_restored
	Restore(ctx context.Context, id uuid.UUID) (Status, error)
	// Reorder: ids là mọi status đang dùng theo thứ tự mới (vị trí 1..n), không thì
	// ErrInvalidOrder; mỗi status đổi vị trí có một event status_updated
	Reorder(ctx context.Context, ids []uuid.UUID) ([]Status, error)
}

// TypeCounts: tài sản chưa retire của một loại; ByKind không có retired
type TypeCounts struct {
	Total  int64
	ByKind map[StatusKind]int64
}

// TypeSummary: TypeCounts và nhãn thuộc tính đang dùng của một loại (trang danh sách loại)
type TypeSummary struct {
	Counts          TypeCounts
	AttributeLabels []string
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
	// Borrowable: chỉ tài sản cho mượn được ngay (loại bật Can be borrowed, status kind available)
	Borrowable bool
	// IDs: chỉ các tài sản này ("Export selected"); nil là không lọc
	IDs           []uuid.UUID
	Sort          AssetSort
	Limit, Offset int32
	// Attrs: điều kiện "<key>:<op>:<value>" trên thuộc tính tuỳ chỉnh (AND), cần TypeID
	Attrs []string
	// Fields: điều kiện "<field>:<op>:<value>" trên trường có sẵn của mọi tài sản
	// (purchase_date, created_at, updated_at, description), không cần TypeID. TZ: múi
	// giờ IANA để so created_at/updated_at theo ngày; rỗng là UTC
	Fields []string
	TZ     string
	// Service điền từ Fields (ResolveFieldQuery); repository chỉ đọc trường này
	FieldFilters []FieldFilter
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
	// RecordReplacement: oldID được thay bằng newID, cũ retire (ErrAssetRetired, ErrAssetChanged,
	// ErrInvalidReplacement); event asset_replaced, asset_replaces
	RecordReplacement(ctx context.Context, oldID, newID uuid.UUID, replacedOn time.Time, reason string, retiredStatus uuid.UUID, version int32) (Asset, error)
	// UndoReplacement: bỏ liên kết, khôi phục tài sản cũ (ErrAssetNotReplaced); event
	// asset_replacement_undone ở cả hai
	UndoReplacement(ctx context.Context, oldID uuid.UUID, availableStatus uuid.UUID, version int32) (Asset, error)
	// Replacements: tài sản thay id (nil) và các tài sản id đã thay, mới nhất trước
	Replacements(ctx context.Context, id uuid.UUID) (*ReplacementLink, []ReplacementLink, error)
	List(ctx context.Context, f AssetFilter) ([]AssetListItem, int64, error)
	// Count: số dòng List sẽ trả với cùng bộ lọc (bỏ qua Limit, Offset)
	Count(ctx context.Context, f AssetFilter) (int64, error)
	// Stream đọc mọi dòng của bộ lọc theo trang pageSize, kèm giá trị thuộc tính, trong
	// một transaction chỉ đọc REPEATABLE READ (mọi trang cùng một ảnh dữ liệu)
	Stream(ctx context.Context, f AssetFilter, pageSize int32, fn func([]AssetListItem) error) error
	// CountByType: tài sản chưa retire của mỗi loại, tổng và theo kind; loại không có
	// tài sản thì không có trong map
	CountByType(ctx context.Context) (map[uuid.UUID]TypeCounts, error)
	// CountByStatus: số tài sản (kể cả đã retire) của mỗi status
	CountByStatus(ctx context.Context) (map[uuid.UUID]int64, error)
	// Replace thay toàn bộ trường và giá trị: ErrAssetRetired, ErrAssetChanged;
	// đổi loại thì xoá giá trị cũ trước; event asset_updated
	Replace(ctx context.Context, id uuid.UUID, f AssetFields, version int32) (Asset, error)
	Retire(ctx context.Context, id uuid.UUID, reason string, retiredStatus uuid.UUID, version int32) (Asset, error)
	Restore(ctx context.Context, id uuid.UUID, availableStatus uuid.UUID, version int32) (Asset, error)
}

// BorrowingRepository: phiếu mượn và khoản mượn (US-10). Mỗi thao tác ghi là một transaction:
// khoá phiếu rồi tới tài sản, kiểm tra, ghi, đổi status tài sản (không event asset_updated),
// thêm event và xếp thư trong cùng transaction.
type BorrowingRepository interface {
	CreateTicket(ctx context.Context, t NewBorrowTicket) (BorrowTicket, error)
	GetTicket(ctx context.Context, id uuid.UUID) (BorrowTicket, error)
	ListTickets(ctx context.Context, f TicketFilter) ([]BorrowTicket, int64, error)
	TicketCounts(ctx context.Context) (waiting, approved int64, err error)
	Approve(ctx context.Context, ticketID, assetID uuid.UUID, version int32) (BorrowTicket, error)
	Reject(ctx context.Context, ticketID uuid.UUID, reason string, version int32) (BorrowTicket, error)
	Cancel(ctx context.Context, ticketID uuid.UUID, reason string, byManager bool, version int32) (BorrowTicket, error)
	HandOver(ctx context.Context, ticketID uuid.UUID, startedOn time.Time, version int32) (BorrowTicket, Loan, error)
	Lend(ctx context.Context, assetID, borrowerID uuid.UUID, startedOn time.Time, returnBy *time.Time, version int32) (Loan, error)
	ActiveLoan(ctx context.Context, assetID uuid.UUID) (*Loan, error)
	// LoanForTicket: khoản mượn của phiếu đã giao; nil khi phiếu chưa giao
	LoanForTicket(ctx context.Context, ticketID uuid.UUID) (*Loan, error)
	ApprovedTicketFor(ctx context.Context, assetID uuid.UUID) (*BorrowTicket, error)
	GetLoan(ctx context.Context, id uuid.UUID) (Loan, error)
	ListLoans(ctx context.Context, today time.Time, f LoanFilter) ([]Loan, int64, error)
	// Return ghi nhận trả khoản mượn đang hiệu lực của tài sản: ErrNoActiveLoan, ErrAssetChanged
	Return(ctx context.Context, assetID uuid.UUID, r LoanReturn, version int32) (Loan, error)
	QueueOverdueEmails(ctx context.Context, today time.Time, limit int) (int, error)
}
