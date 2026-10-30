package domain

import (
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"storeit/internal/platform/errs"
)

// Thay thế tài sản (US-13): tài sản cũ được thay bằng một tài sản khác, cũ bị retire;
// liên kết giữ để lần ngược nguồn gốc. Lịch sử nằm ở event, bảng chỉ giữ liên kết hiện tại.

// MaxReplacementReason: độ dài tối đa của lý do, bằng lý do retire
const MaxReplacementReason = 500

// ReplacementLink là một liên kết thay thế nhìn từ một tài sản: AssetID là tài sản ở đầu
// kia (tài sản thay nó, hay tài sản nó thay)
type ReplacementLink struct {
	AssetID    uuid.UUID
	Tag, Name  string
	ReplacedOn time.Time // chỉ ngày
	Reason     string
}

// CheckReplacedOn: ngày thay thế không ở tương lai theo ngày của người dùng, nên chấp nhận
// tới ngày UTC của server cộng một ngày
func CheckReplacedOn(day, now time.Time) error {
	y, m, d := now.UTC().Date()
	latest := time.Date(y, m, d, 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1)
	dy, dm, dd := day.Date()
	if time.Date(dy, dm, dd, 0, 0, 0, 0, time.UTC).After(latest) {
		return ErrInvalidReplacement.With(errs.WithFields(errs.FieldError{Field: "replaced_on", Detail: "can't be in the future"}))
	}
	return nil
}

// CheckReplacedAfterPurchase: ngày thay không trước ngày mua của tài sản cũ hay tài sản thay
// (chưa mua thì chưa dùng hay thay được); tài sản không có ngày mua thì không xét
func CheckReplacedAfterPurchase(day time.Time, old, repl Asset) error {
	dy, dm, dd := day.Date()
	d := time.Date(dy, dm, dd, 0, 0, 0, 0, time.UTC)
	for _, a := range []Asset{old, repl} {
		if a.PurchaseDate == nil {
			continue
		}
		py, pm, pd := a.PurchaseDate.Date()
		if p := time.Date(py, pm, pd, 0, 0, 0, 0, time.UTC); d.Before(p) {
			return ErrInvalidReplacement.With(errs.WithFields(errs.FieldError{
				Field: "replaced_on", Detail: "can't be before " + a.Tag + " was purchased (" + p.Format(time.DateOnly) + ")",
			}))
		}
	}
	return nil
}

// ReplacementRetireReason: lý do retire của tài sản cũ, "Replaced by <tag>: <lý do>", cắt
// còn MaxReplacementReason ký tự
func ReplacementRetireReason(newTag, reason string) string {
	s := "Replaced by " + newTag + ": " + reason
	if utf8.RuneCountInString(s) <= MaxReplacementReason {
		return s
	}
	return string([]rune(s)[:MaxReplacementReason])
}
