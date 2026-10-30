// Package service là use case của inventory: loại tài sản và thuộc tính riêng,
// status, tài sản. Mọi use case bắt đầu bằng auth.Require; service không thấy
// HTTP, SQL hay transaction.
package service

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	idcontract "storeit/internal/identity/contract"
	"storeit/internal/inventory/domain"
	"storeit/internal/platform/mail"
)

type Deps struct {
	Types    domain.TypeRepository
	Statuses domain.StatusRepository
	Assets   domain.AssetRepository
	Profiles domain.ExportProfileRepository
	Accounts idcontract.AccountReader // tên chủ profile, tên người export trong tiêu đề báo cáo
	// Events đọc event của tài sản cho lịch sử hoạt động (repository tài sản cài sẵn)
	Events AssetEventReader
	// ExportMaxRows: số dòng tối đa một lần export; 0 là mặc định 50.000
	ExportMaxRows int

	// Mượn tài sản (US-10). Location: múi giờ của ngày nghiệp vụ (nil là UTC); Now: đồng hồ
	// (nil là time.Now), test thay được
	Borrowing domain.BorrowingRepository
	Location  *time.Location
	Now       func() time.Time
	// Mail gửi thư của việc mượn (chỉ worker cần); AppURL là địa chỉ frontend cho link trong thư
	Mail   mail.Sender
	AppURL string
}

type Service struct {
	types         domain.TypeRepository
	statuses      domain.StatusRepository
	assets        domain.AssetRepository
	profiles      domain.ExportProfileRepository
	accounts      idcontract.AccountReader
	events        AssetEventReader
	exportMaxRows int
	borrowing     domain.BorrowingRepository
	loc           *time.Location
	now           func() time.Time
	mail          mail.Sender
	appURL        string
}

func New(d Deps) *Service {
	maxRows := d.ExportMaxRows
	if maxRows == 0 {
		maxRows = defaultExportMaxRows
	}
	loc, now := d.Location, d.Now
	if loc == nil {
		loc = time.UTC
	}
	if now == nil {
		now = time.Now
	}
	return &Service{
		types: d.Types, statuses: d.Statuses, assets: d.Assets, profiles: d.Profiles, accounts: d.Accounts, events: d.Events,
		exportMaxRows: maxRows, borrowing: d.Borrowing, loc: loc, now: now, mail: d.Mail, appURL: d.AppURL,
	}
}

// Độ dài tối đa (ký tự), khớp CHECK của migration và maxLength của OpenAPI
const (
	maxNameLen        = 100
	maxAssetNameLen   = 200
	maxTypeDescLen    = 500
	maxAssetDescLen   = 2000
	maxRetireReasonLn = 500
)

// cleanText bỏ khoảng trắng hai đầu cho văn bản tự do (mô tả, lý do): cho phép
// xuống dòng và tab, chặn ký tự điều khiển khác và quá dài
func cleanText(s string, maxLen int) (string, error) {
	t := strings.TrimSpace(s)
	bad := func(r rune) bool { return unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' }
	if utf8.RuneCountInString(t) > maxLen || strings.ContainsFunc(t, bad) {
		return "", domain.ErrInvalidLabel
	}
	return t, nil
}
