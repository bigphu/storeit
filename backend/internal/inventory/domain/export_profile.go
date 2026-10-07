package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ExportProfile là một bố cục báo cáo đã lưu; Shared thì mọi người export được đều thấy
type ExportProfile struct {
	ID        uuid.UUID
	OwnerID   uuid.UUID
	Name      string
	Shared    bool
	Layout    ExportLayout
	Version   int32
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

type NewExportProfile struct {
	OwnerID uuid.UUID
	Name    string
	Shared  bool
	Layout  ExportLayout
}

// ExportProfileChange: field nil là giữ nguyên; Version cũ là ErrExportProfileChanged
type ExportProfileChange struct {
	Name    *string
	Shared  *bool
	Layout  *ExportLayout
	Version int32
}

// ExportRecord: một lần export, ghi thành event assets_exported cho lịch sử sau này
type ExportRecord struct {
	Mode      string
	ProfileID *uuid.UUID
	Rows      int64
	Sheets    int
	Filters   map[string]any
}

type ExportProfileRepository interface {
	// List: của owner và mọi profile được chia sẻ, theo tên
	List(ctx context.Context, ownerID uuid.UUID) ([]ExportProfile, error)
	// Get: ErrExportProfileNotFound
	Get(ctx context.Context, id uuid.UUID) (ExportProfile, error)
	// Create: ErrExportProfileNameTaken; event export_profile_created
	Create(ctx context.Context, in NewExportProfile) (ExportProfile, error)
	// Update: ErrExportProfileChanged, ErrExportProfileNameTaken; event export_profile_updated
	Update(ctx context.Context, id uuid.UUID, ch ExportProfileChange) (ExportProfile, error)
	// Delete: xoá mềm; ErrExportProfileNotFound; event export_profile_deleted
	Delete(ctx context.Context, id uuid.UUID) error
	// GetAny: kể cả đã xoá (để kiểm tra quyền khôi phục); ErrExportProfileNotFound
	GetAny(ctx context.Context, id uuid.UUID) (ExportProfile, error)
	// Restore: khôi phục profile đã xoá (chưa xoá thì trả nguyên); ErrExportProfileNotFound,
	// ErrExportProfileNameTaken; event export_profile_restored
	Restore(ctx context.Context, id uuid.UUID) (ExportProfile, error)
	// RecordExport ghi event assets_exported trong transaction riêng
	RecordExport(ctx context.Context, r ExportRecord) error
}
