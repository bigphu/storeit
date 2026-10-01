// Package contract là phần công khai của identity cho module khác: đọc thông
// tin account (vd hiện "ai đã sửa" trong lịch sử) và tên các event.
package contract

import (
	"context"

	"github.com/google/uuid"

	"storeit/internal/platform/errs"
)

// Account là thông tin công khai của một account, không có gì nhạy cảm
type Account struct {
	ID     uuid.UUID
	Name   string
	Email  string
	Active bool
}

var ErrAccountNotFound = errs.NotFound("/errors/account-not-found", "Account not found")

// AccountReader đọc account theo ID. Không kiểm tra quyền: module gọi tự
// quyết định ai được xem gì.
type AccountReader interface {
	// GetAccount: ErrAccountNotFound nếu không có
	GetAccount(ctx context.Context, id uuid.UUID) (Account, error)
	// GetAccounts trả các account tìm thấy, bỏ qua ID không tồn tại
	GetAccounts(ctx context.Context, ids []uuid.UUID) ([]Account, error)
}
