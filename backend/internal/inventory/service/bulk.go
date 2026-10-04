package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"storeit/internal/inventory/domain"
	"storeit/internal/platform/auth"
	"storeit/internal/platform/errs"
)

// Thao tác hàng loạt trên danh sách tài sản: mỗi tài sản đi qua đúng quy tắc và
// transaction của thao tác đơn (một event mỗi tài sản), thành công hay thất bại
// riêng. Lỗi nghiệp vụ của một tài sản (version cũ, đã retire, không tồn tại) vào
// Failed; lỗi bất ngờ (DB) dừng cả lô, tài sản đã làm xong vẫn giữ.

// maxBulkItems: bằng page_size lớn nhất, đủ cho "chọn cả trang"
const maxBulkItems = 200

// BulkItem: một tài sản cùng version người dùng đã đọc
type BulkItem struct {
	ID      uuid.UUID
	Version int32
}

type BulkFailure struct {
	ID  uuid.UUID
	Err error
}

// BulkResult giữ thứ tự gửi lên
type BulkResult struct {
	Succeeded []uuid.UUID
	Failed    []BulkFailure
}

// RetireAssets retire nhiều tài sản với cùng một lý do
func (s *Service) RetireAssets(ctx context.Context, items []BulkItem, reason string) (BulkResult, error) {
	if _, err := auth.Require(ctx, domain.PermAssetManage); err != nil {
		return BulkResult{}, err
	}
	items, err := checkBulk(items)
	if err != nil {
		return BulkResult{}, err
	}
	r, err := cleanText(reason, maxRetireReasonLn)
	if err != nil {
		return BulkResult{}, err
	}
	st, err := s.statuses.Default(ctx, domain.KindRetired)
	if err != nil {
		return BulkResult{}, err
	}
	return eachItem(items, func(it BulkItem) error {
		_, err := s.assets.Retire(ctx, it.ID, r, st.ID, it.Version)
		return err
	})
}

// SetAssetsStatus đổi status của nhiều tài sản. Status phải dùng được (không thuộc
// kind retired, không archive); tài sản đã có status đó thì không ghi gì.
func (s *Service) SetAssetsStatus(ctx context.Context, items []BulkItem, statusID uuid.UUID) (BulkResult, error) {
	if _, err := auth.Require(ctx, domain.PermAssetManage); err != nil {
		return BulkResult{}, err
	}
	items, err := checkBulk(items)
	if err != nil {
		return BulkResult{}, err
	}
	st, err := s.statuses.Get(ctx, statusID)
	if err != nil {
		return BulkResult{}, err
	}
	if st.Kind == domain.KindRetired {
		return BulkResult{}, domain.ErrRetiredStatus
	}
	if st.Archived() {
		return BulkResult{}, domain.ErrStatusArchived
	}
	return eachItem(items, func(it BulkItem) error {
		cur, err := s.assets.Get(ctx, it.ID)
		if err != nil {
			return err
		}
		if cur.Retired() {
			return domain.ErrAssetRetired
		}
		if cur.StatusID == st.ID {
			return nil
		}
		// giữ mọi trường và giá trị, chỉ đổi status (PUT thay toàn bộ)
		_, err = s.assets.Replace(ctx, it.ID, domain.AssetFields{
			Name: cur.Name, Description: cur.Description, TypeID: cur.TypeID, StatusID: st.ID,
			LocationID: cur.LocationID, HolderMemberID: cur.HolderMemberID, PurchaseDate: cur.PurchaseDate,
			Values: cur.Values,
		}, it.Version)
		return err
	})
}

// checkBulk: 1..maxBulkItems tài sản; id trùng chỉ làm một lần
func checkBulk(items []BulkItem) ([]BulkItem, error) {
	if len(items) == 0 || len(items) > maxBulkItems {
		return nil, domain.ErrInvalidBulk
	}
	seen := make(map[uuid.UUID]bool, len(items))
	out := items[:0:0]
	for _, it := range items {
		if !seen[it.ID] {
			seen[it.ID] = true
			out = append(out, it)
		}
	}
	return out, nil
}

// eachItem chạy fn cho từng tài sản: lỗi nghiệp vụ ghi vào Failed, lỗi khác dừng lô
func eachItem(items []BulkItem, fn func(BulkItem) error) (BulkResult, error) {
	var res BulkResult
	for _, it := range items {
		err := fn(it)
		var e *errs.Error
		switch {
		case err == nil:
			res.Succeeded = append(res.Succeeded, it.ID)
		case errors.As(err, &e) && e.Status() < 500:
			res.Failed = append(res.Failed, BulkFailure{ID: it.ID, Err: err})
		default:
			return BulkResult{}, err
		}
	}
	return res, nil
}
