package service

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"storeit/internal/inventory/domain"
)

// Lịch sử mượn (US-12): kiểm kỳ, trạng thái, quyền; từ khoá dịch sang người mượn
func TestListLoans_Filters(t *testing.T) {
	b := newBorrowEnv(t, time.Date(2026, 10, 30, 3, 0, 0, 0, time.UTC))
	from, to := day("2026-11-02"), day("2026-11-01")
	_, _, err := b.svc.ListLoans(officer, LoanQuery{State: "all", From: &from, To: &to})
	if f := fieldName(t, err, domain.ErrInvalidLoan); f != "from" {
		t.Errorf("from after to: field %q", f)
	}
	_, _, err = b.svc.ListLoans(officer, LoanQuery{State: "x"})
	if f := fieldName(t, err, domain.ErrInvalidLoan); f != "state" {
		t.Errorf("bad state: field %q", f)
	}

	me := uuid.New()
	emp := asAccount(me, domain.PermBorrowRequest)
	if _, _, err := b.svc.ListLoans(emp, LoanQuery{Mine: true, State: "returned"}); err != nil {
		t.Fatal(err)
	}
	if f := b.borrowing.lastLoanFilter; f.BorrowerID == nil || *f.BorrowerID != me || f.State != "returned" {
		t.Errorf("mine: filter = %+v, want the actor's returned loans", f)
	}
	if _, _, err := b.svc.ListLoans(emp, LoanQuery{State: "all"}); status(err) != 403 {
		t.Errorf("everyone's loans without manage = %v, want 403", err)
	}
	asset := uuid.New()
	if _, _, err := b.svc.ListLoans(emp, LoanQuery{Mine: true, AssetID: &asset}); status(err) != 403 {
		t.Errorf("asset history without manage = %v, want 403", err)
	}

	if _, _, err := b.svc.ListLoans(officer, LoanQuery{State: "all", Q: "  nguyen "}); err != nil {
		t.Fatal(err)
	}
	if got := *b.accounts.searchLimit; got < 1000 {
		t.Errorf("people considered = %d, want at least 1000", got)
	}
	if f := b.borrowing.lastLoanFilter; f.Q != "nguyen" || f.MatchBorrowers == nil {
		t.Errorf("search filter = %+v, want trimmed q and the matched people", f)
	}
	if _, _, err := b.svc.ListLoans(officer, LoanQuery{}); err != nil || b.borrowing.lastLoanFilter.State != "active" {
		t.Errorf("default state = %q, %v; want active", b.borrowing.lastLoanFilter.State, err)
	}
}
