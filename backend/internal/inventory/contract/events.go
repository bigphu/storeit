package contract

import "github.com/google/uuid"

// Event công khai của inventory. Module khác (activity) nghe theo các tên này
// và đọc payload bằng events.Decode[T]. Payload chỉ mang những gì API cũng trả.
const (
	EventAssetCreated  = "inventory.asset_created"
	EventAssetUpdated  = "inventory.asset_updated"
	EventAssetRetired  = "inventory.asset_retired"
	EventAssetRestored = "inventory.asset_restored"

	EventAssetTypeCreated  = "inventory.asset_type_created"
	EventAssetTypeUpdated  = "inventory.asset_type_updated"
	EventAssetTypeArchived = "inventory.asset_type_archived"
	EventAssetTypeRestored = "inventory.asset_type_restored"

	EventStatusCreated  = "inventory.status_created"
	EventStatusUpdated  = "inventory.status_updated"
	EventStatusArchived = "inventory.status_archived"
	EventStatusRestored = "inventory.status_restored"

	EventExportProfileCreated = "inventory.export_profile_created"
	EventExportProfileUpdated = "inventory.export_profile_updated"
	EventExportProfileDeleted = "inventory.export_profile_deleted"
	EventAssetsExported       = "inventory.assets_exported"
)

// Loại aggregate trong platform.events
const (
	AggregateAsset     = "asset"
	AggregateAssetType = "asset_type"
	AggregateStatus    = "asset_status"

	AggregateExportProfile = "export_profile"
	AggregateAssetExport   = "asset_export"
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

// AssetsExported: ai export gì (người làm là actor của event)
type AssetsExported struct {
	Mode      string         `json:"mode"`
	ProfileID *uuid.UUID     `json:"profile_id,omitempty"`
	Rows      int64          `json:"rows"`
	Sheets    int            `json:"sheets"`
	Filters   map[string]any `json:"filters"`
}
