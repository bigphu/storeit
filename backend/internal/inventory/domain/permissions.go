package domain

import "github.com/google/uuid"

// Mã quyền của inventory, seed ở migrations/00003_inventory.sql cùng phân quyền
// cho role hệ thống của identity
const (
	PermAssetRead    = "inventory.asset.read"
	PermAssetManage  = "inventory.asset.manage"
	PermTypeManage   = "inventory.type.manage"
	PermStatusManage = "inventory.status.manage"

	PermAssetExport         = "inventory.asset.export"          // tải danh sách tài sản dạng Excel, lưu profile của mình
	PermExportProfileManage = "inventory.export_profile.manage" // sửa, xoá profile người khác chia sẻ
)

// ID cố định của dữ liệu seed
var (
	GeneralTypeID     = uuid.MustParse("00000000-0000-7000-8000-000000000101")
	AvailableStatusID = uuid.MustParse("00000000-0000-7000-8000-000000000201")
	InUseStatusID     = uuid.MustParse("00000000-0000-7000-8000-000000000202")
	RepairStatusID    = uuid.MustParse("00000000-0000-7000-8000-000000000203")
	RetiredStatusID   = uuid.MustParse("00000000-0000-7000-8000-000000000204")
)
