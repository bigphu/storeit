package domain

import (
	"errors"
	"testing"
	"time"

	"storeit/internal/platform/errs"
)

func day(s string) time.Time {
	d, _ := time.Parse(time.DateOnly, s)
	return d
}

func dayPtr(s string) *time.Time {
	d := day(s)
	return &d
}

// fieldOf: lỗi là err mong đợi với đúng một field, trả tên field
func fieldOf(t *testing.T, err, want error) string {
	t.Helper()
	var e *errs.Error
	if !errors.Is(err, want) || !errors.As(err, &e) || len(e.Fields()) != 1 {
		t.Fatalf("err = %v, want %v with one field", err, want)
	}
	return e.Fields()[0].Field
}

func TestCheckTicketDates(t *testing.T) {
	today := day("2026-10-30")
	if f := fieldOf(t, CheckTicketDates(day("2026-10-29"), nil, today), ErrInvalidTicket); f != "needed_from" {
		t.Errorf("needed_from in the past: field %q", f)
	}
	if err := CheckTicketDates(today, nil, today); err != nil {
		t.Errorf("needed_from today: %v", err)
	}
	if f := fieldOf(t, CheckTicketDates(day("2026-11-01"), dayPtr("2026-10-31"), today), ErrInvalidTicket); f != "return_by" {
		t.Errorf("return_by before needed_from: field %q", f)
	}
	if err := CheckTicketDates(day("2026-11-01"), dayPtr("2026-11-01"), today); err != nil {
		t.Errorf("return_by = needed_from: %v", err)
	}
}

func TestCheckLoanReturnBy(t *testing.T) {
	today := day("2026-10-30")
	if f := fieldOf(t, CheckLoanReturnBy(dayPtr("2026-10-29"), today), ErrInvalidLoan); f != "return_by" {
		t.Errorf("field %q", f)
	}
	for _, ok := range []*time.Time{dayPtr("2026-10-30"), nil} {
		if err := CheckLoanReturnBy(ok, today); err != nil {
			t.Errorf("%v: %v", ok, err)
		}
	}
}

func TestIsBorrowingStatus(t *testing.T) {
	if !IsBorrowingStatus(ReservedStatusID) || !IsBorrowingStatus(OnLoanStatusID) {
		t.Error("Reserved and On loan must be borrowing statuses")
	}
	if IsBorrowingStatus(AvailableStatusID) || IsBorrowingStatus(InUseStatusID) {
		t.Error("Available and In use are not borrowing statuses")
	}
}

func TestBorrowable(t *testing.T) {
	avail := Status{Kind: KindAvailable}
	if Borrowable(AssetType{CanBorrow: false}, avail) {
		t.Error("type switch off: not borrowable")
	}
	if !Borrowable(AssetType{CanBorrow: true}, avail) {
		t.Error("switch on and available: borrowable")
	}
	if Borrowable(AssetType{CanBorrow: true}, Status{Kind: KindInUse}) {
		t.Error("in use: not borrowable")
	}
	now := time.Now()
	if Borrowable(AssetType{CanBorrow: true, ArchivedAt: &now}, avail) {
		t.Error("archived type: not borrowable")
	}
}

func TestLoanOverdueDays(t *testing.T) {
	today := day("2026-10-30")
	if n := (Loan{ReturnBy: dayPtr("2026-10-25")}).OverdueDays(today); n != 5 {
		t.Errorf("due 10-25 = %d, want 5", n)
	}
	if n := (Loan{ReturnBy: dayPtr("2026-10-25"), ReturnedOn: dayPtr("2026-10-28")}).OverdueDays(today); n != 0 {
		t.Errorf("returned = %d, want 0", n)
	}
	if n := (Loan{}).OverdueDays(today); n != 0 {
		t.Errorf("no return date = %d, want 0", n)
	}
	if n := (Loan{ReturnBy: dayPtr("2026-10-30")}).OverdueDays(today); n != 0 {
		t.Errorf("due today = %d, want 0", n)
	}
}
