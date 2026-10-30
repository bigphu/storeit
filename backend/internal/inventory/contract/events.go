package contract

import "github.com/google/uuid"

// Event công khai của inventory. Module khác (activity) nghe theo các tên này
// và đọc payload bằng events.Decode[T]. Payload chỉ mang những gì API cũng trả.
const (
	EventAssetCreated  = "inventory.asset_created"
	EventAssetUpdated  = "inventory.asset_updated"
	EventAssetRetired  = "inventory.asset_retired"
	EventAssetRestored = "inventory.asset_restored"
	// Thay thế (US-13): asset_replaced trên tài sản cũ, asset_replaces trên tài sản mới,
	// asset_replacement_undone trên cả hai khi hoàn tác
	EventAssetReplaced          = "inventory.asset_replaced"
	EventAssetReplaces          = "inventory.asset_replaces"
	EventAssetReplacementUndone = "inventory.asset_replacement_undone"
	// Mượn (US-10), trên tài sản: giữ chỗ khi duyệt phiếu, huỷ giữ chỗ, bắt đầu khoản mượn (giao
	// theo phiếu hay cho mượn trực tiếp)
	EventAssetReserved             = "inventory.asset_reserved"
	EventAssetReservationCancelled = "inventory.asset_reservation_cancelled"
	EventAssetLent                 = "inventory.asset_lent"
	// Trên phiếu mượn (aggregate borrow_ticket)
	EventBorrowTicketCreated    = "inventory.borrow_ticket_created"
	EventBorrowTicketApproved   = "inventory.borrow_ticket_approved"
	EventBorrowTicketRejected   = "inventory.borrow_ticket_rejected"
	EventBorrowTicketCancelled  = "inventory.borrow_ticket_cancelled"
	EventBorrowTicketHandedOver = "inventory.borrow_ticket_handed_over"

	EventAssetTypeCreated  = "inventory.asset_type_created"
	EventAssetTypeUpdated  = "inventory.asset_type_updated"
	EventAssetTypeArchived = "inventory.asset_type_archived"
	EventAssetTypeRestored = "inventory.asset_type_restored"

	EventStatusCreated  = "inventory.status_created"
	EventStatusUpdated  = "inventory.status_updated"
	EventStatusArchived = "inventory.status_archived"
	EventStatusRestored = "inventory.status_restored"

	EventExportProfileCreated  = "inventory.export_profile_created"
	EventExportProfileUpdated  = "inventory.export_profile_updated"
	EventExportProfileDeleted  = "inventory.export_profile_deleted"
	EventExportProfileRestored = "inventory.export_profile_restored"
	EventAssetsExported        = "inventory.assets_exported"
)

// Loại aggregate trong platform.events
const (
	AggregateAsset     = "asset"
	AggregateAssetType = "asset_type"
	AggregateStatus    = "asset_status"

	AggregateExportProfile = "export_profile"
	AggregateAssetExport   = "asset_export"

	AggregateBorrowTicket = "borrow_ticket"
)

// FieldChange là giá trị trước và sau của một field. Thuộc tính riêng dùng
// field "attributes.<key>"; thuộc tính và option của loại dùng
// "attributes.<key>.<field>" và "attributes.<key>.options.<label>".
type FieldChange struct {
	Field string `json:"field"`
	From  any    `json:"from"`
	To    any    `json:"to"`
}

type AssetCreated struct {
	AssetID  uuid.UUID `json:"asset_id"`
	Tag      string    `json:"tag"`
	Name     string    `json:"name"`
	TypeID   uuid.UUID `json:"asset_type_id"`
	StatusID uuid.UUID `json:"status_id"`
}

type AssetUpdated struct {
	AssetID uuid.UUID     `json:"asset_id"`
	Changes []FieldChange `json:"changes"`
}

type AssetRetired struct {
	AssetID uuid.UUID `json:"asset_id"`
	Reason  string    `json:"reason,omitempty"`
}

type AssetRestored struct {
	AssetID uuid.UUID `json:"asset_id"`
}

// AssetReplaced: AssetID (cũ) được thay bằng ReplacementID; ReplacedOn "YYYY-MM-DD"
type AssetReplaced struct {
	AssetID        uuid.UUID `json:"asset_id"`
	ReplacementID  uuid.UUID `json:"replacement_id"`
	ReplacementTag string    `json:"replacement_tag"`
	ReplacedOn     string    `json:"replaced_on"`
	Reason         string    `json:"reason"`
}

// AssetReplaces: AssetID (mới) thay ReplacedID
type AssetReplaces struct {
	AssetID     uuid.UUID `json:"asset_id"`
	ReplacedID  uuid.UUID `json:"replaced_id"`
	ReplacedTag string    `json:"replaced_tag"`
	ReplacedOn  string    `json:"replaced_on"`
	Reason      string    `json:"reason"`
}

// AssetReplacementUndone: liên kết giữa AssetID và OtherID bị huỷ; Role là vai của AssetID
// ("old" hay "new")
type AssetReplacementUndone struct {
	AssetID  uuid.UUID `json:"asset_id"`
	OtherID  uuid.UUID `json:"other_id"`
	OtherTag string    `json:"other_tag"`
	Role     string    `json:"role"`
}

// AssetReserved: tài sản được giữ chỗ cho người gửi phiếu (AccountID)
type AssetReserved struct {
	AssetID      uuid.UUID `json:"asset_id"`
	TicketID     uuid.UUID `json:"ticket_id"`
	TicketNumber int64     `json:"ticket_number"`
	AccountID    uuid.UUID `json:"account_id"`
}

// AssetReservationCancelled: phiếu đã duyệt bị huỷ; ByManager là người quản lý huỷ (không
// phải người gửi phiếu)
type AssetReservationCancelled struct {
	AssetID      uuid.UUID `json:"asset_id"`
	TicketID     uuid.UUID `json:"ticket_id"`
	TicketNumber int64     `json:"ticket_number"`
	AccountID    uuid.UUID `json:"account_id"`
	ByManager    bool      `json:"by_manager"`
}

// AssetLent: khoản mượn bắt đầu; TicketID nil là cho mượn trực tiếp; ReturnBy "YYYY-MM-DD" hay rỗng
type AssetLent struct {
	AssetID      uuid.UUID  `json:"asset_id"`
	LoanID       uuid.UUID  `json:"loan_id"`
	BorrowerID   uuid.UUID  `json:"borrower_id"`
	TicketID     *uuid.UUID `json:"ticket_id,omitempty"`
	TicketNumber int64      `json:"ticket_number,omitempty"`
	ReturnBy     string     `json:"return_by,omitempty"`
}

// BorrowTicketChanged: payload chung của các event trên phiếu mượn
type BorrowTicketChanged struct {
	TicketID uuid.UUID  `json:"ticket_id"`
	Number   int64      `json:"number"`
	AssetID  *uuid.UUID `json:"asset_id,omitempty"`
	Reason   string     `json:"reason,omitempty"`
}

type AssetTypeCreated struct {
	AssetTypeID uuid.UUID `json:"asset_type_id"`
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Attributes  []string  `json:"attributes"` // key của thuộc tính khởi tạo
}

type AssetTypeUpdated struct {
	AssetTypeID uuid.UUID     `json:"asset_type_id"`
	Changes     []FieldChange `json:"changes"`
}

type AssetTypeArchived struct {
	AssetTypeID uuid.UUID `json:"asset_type_id"`
}

type AssetTypeRestored struct {
	AssetTypeID uuid.UUID `json:"asset_type_id"`
}

type StatusCreated struct {
	StatusID uuid.UUID `json:"status_id"`
	Name     string    `json:"name"`
	Kind     string    `json:"kind"`
}

type StatusUpdated struct {
	StatusID uuid.UUID     `json:"status_id"`
	Changes  []FieldChange `json:"changes"`
}

type StatusArchived struct {
	StatusID uuid.UUID `json:"status_id"`
}

type StatusRestored struct {
	StatusID uuid.UUID `json:"status_id"`
}

type ExportProfileCreated struct {
	ProfileID uuid.UUID `json:"profile_id"`
	Name      string    `json:"name"`
	Shared    bool      `json:"shared"`
}

// ExportProfileUpdated: bố cục đổi thì một FieldChange "layout" không kèm giá trị
type ExportProfileUpdated struct {
	ProfileID uuid.UUID     `json:"profile_id"`
	Changes   []FieldChange `json:"changes"`
}

type ExportProfileDeleted struct {
	ProfileID uuid.UUID `json:"profile_id"`
}

type ExportProfileRestored struct {
	ProfileID uuid.UUID `json:"profile_id"`
}

// AssetsExported: ai export gì (người làm là actor của event)
type AssetsExported struct {
	Mode      string         `json:"mode"`
	ProfileID *uuid.UUID     `json:"profile_id,omitempty"`
	Rows      int64          `json:"rows"`
	Sheets    int            `json:"sheets"`
	Filters   map[string]any `json:"filters"`
}
