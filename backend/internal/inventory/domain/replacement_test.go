package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"storeit/internal/platform/errs"
)

// Ngày thay thế không ở tương lai, tính theo ngày của người dùng: chấp nhận tới ngày
// UTC của server cộng một ngày (người dùng UTC+7 lúc sáng sớm đã sang ngày mới)
func TestCheckReplacedOn(t *testing.T) {
	now := time.Date(2026, 10, 29, 20, 0, 0, 0, time.UTC)
	day := func(s string) time.Time { d, _ := time.Parse(time.DateOnly, s); return d }
	for _, ok := range []string{"2026-10-01", "2026-10-29", "2026-10-30"} {
		if err := CheckReplacedOn(day(ok), now); err != nil {
			t.Errorf("%s: %v, want ok", ok, err)
		}
	}
	err := CheckReplacedOn(day("2026-10-31"), now)
	if !errors.Is(err, ErrInvalidReplacement) {
		t.Fatalf("2026-10-31: %v, want ErrInvalidReplacement", err)
	}
	var e *errs.Error
	if !errors.As(err, &e) || len(e.Fields()) != 1 || e.Fields()[0].Field != "replaced_on" {
		t.Errorf("fields = %+v, want replaced_on", e)
	}
}

func TestReplacementRetireReason(t *testing.T) {
	if got := ReplacementRetireReason("LAP-0042", "screen broken"); got != "Replaced by LAP-0042: screen broken" {
		t.Errorf("got %q", got)
	}
	long := ReplacementRetireReason("LAP-0042", strings.Repeat("ê", 600))
	if n := utf8.RuneCountInString(long); n != MaxReplacementReason {
		t.Errorf("long reason has %d runes, want %d", n, MaxReplacementReason)
	}
}

// Ngày thay không trước ngày mua của tài sản nào trong hai (ngày mua có thì mới xét)
func TestCheckReplacedAfterPurchase(t *testing.T) {
	day := func(s string) time.Time { d, _ := time.Parse(time.DateOnly, s); return d }
	ptr := func(s string) *time.Time { d := day(s); return &d }
	old := Asset{Tag: "LAP-0001", PurchaseDate: ptr("2026-10-10")}
	repl := Asset{Tag: "LAP-0042", PurchaseDate: ptr("2026-10-15")}
	for _, ok := range []string{"2026-10-15", "2026-10-20"} {
		if err := CheckReplacedAfterPurchase(day(ok), old, repl); err != nil {
			t.Errorf("%s: %v, want ok", ok, err)
		}
	}
	for s, want := range map[string]string{
		"2026-10-09": "can't be before LAP-0001 was purchased (2026-10-10)",
		"2026-10-14": "can't be before LAP-0042 was purchased (2026-10-15)",
	} {
		err := CheckReplacedAfterPurchase(day(s), old, repl)
		var e *errs.Error
		if !errors.Is(err, ErrInvalidReplacement) || !errors.As(err, &e) || len(e.Fields()) != 1 ||
			e.Fields()[0].Field != "replaced_on" || e.Fields()[0].Detail != want {
			t.Errorf("%s: %v, want replaced_on %q", s, err, want)
		}
	}
	if err := CheckReplacedAfterPurchase(day("2001-01-01"), Asset{}, Asset{}); err != nil {
		t.Errorf("no purchase dates: %v, want ok", err)
	}
}
