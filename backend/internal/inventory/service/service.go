// Package service là use case của inventory: loại tài sản và thuộc tính riêng,
// status, tài sản. Mọi use case bắt đầu bằng auth.Require; service không thấy
// HTTP, SQL hay transaction.
package service

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"storeit/internal/inventory/domain"
)

type Deps struct {
	Types    domain.TypeRepository
	Statuses domain.StatusRepository
	Assets   domain.AssetRepository
}

type Service struct {
	types    domain.TypeRepository
	statuses domain.StatusRepository
	assets   domain.AssetRepository
}

func New(d Deps) *Service {
	return &Service{types: d.Types, statuses: d.Statuses, assets: d.Assets}
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
