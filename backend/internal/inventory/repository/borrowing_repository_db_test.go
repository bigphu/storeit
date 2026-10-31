package repository_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"storeit/internal/inventory/contract"
	"storeit/internal/inventory/domain"
	"storeit/internal/inventory/repository"
	"storeit/internal/platform/auth"
	"storeit/internal/platform/events"
	"storeit/internal/platform/jobs"
)

var (
	borrowToday = time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)
	pastDay     = time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	laterDay    = time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
)

// borrowType: loại không có thuộc tính, bật hay tắt Can be borrowed
func borrowType(t *testing.T, r repos, canBorrow bool) domain.AssetType {
	t.Helper()
	u := uniq()
	ty, err := r.types.Create(actorCtx(), domain.NewAssetType{Code: "BR" + u, Name: "Borrow " + u, CanBorrow: canBorrow})
	if err != nil {
		t.Fatal(err)
	}
	return ty
}

func borrowAsset(t *testing.T, r repos, typeID, statusID uuid.UUID) domain.Asset {
	t.Helper()
	a, err := r.assets.Create(actorCtx(), "BA-"+uniq(), domain.AssetFields{Name: "Projector", TypeID: typeID, StatusID: statusID})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func newTicket(t *testing.T, r repos, typeID uuid.UUID) domain.BorrowTicket {
	t.Helper()
	tk, err := r.borrowing.CreateTicket(actorCtx(), domain.NewBorrowTicket{
		RequesterID: uuid.New(), TypeID: typeID, NeededFrom: borrowToday, Requirements: []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return tk
}

func getAsset(t *testing.T, r repos, id uuid.UUID) domain.Asset {
	t.Helper()
	a, err := r.assets.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// emailJobs: số job gửi thư loại kind về phiếu/khoản id
func emailJobs(t *testing.T, r repos, kind string, id uuid.UUID) int {
	t.Helper()
	var n int
	err := r.pool.QueryRow(context.Background(), `
		SELECT count(*) FROM river_job WHERE kind = 'inventory.send_borrowing_email'
		AND args->>'kind' = $1 AND (args->>'ticket_id' = $2 OR args->>'loan_id' = $2)`, kind, id.String()).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestBorrowing_TicketLifecycle(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	ty := borrowType(t, r, true)
	a := borrowAsset(t, r, ty.ID, domain.AvailableStatusID)
	tk := newTicket(t, r, ty.ID)
	if tk.State != domain.TicketWaiting || tk.Number == 0 {
		t.Fatalf("new ticket = %+v", tk)
	}

	ap, err := r.borrowing.Approve(ctx, tk.ID, a.ID, tk.Version)
	if err != nil {
		t.Fatal(err)
	}
	if ap.State != domain.TicketApproved || ap.AssetID == nil || *ap.AssetID != a.ID {
		t.Errorf("approved ticket = %+v", ap)
	}
	if got := getAsset(t, r, a.ID); got.StatusID != domain.ReservedStatusID {
		t.Errorf("asset status after approve = %v, want Reserved", got.StatusID)
	}
	if c := countEvents(t, r, contract.EventAssetReserved, a.ID); c != 1 {
		t.Errorf("asset_reserved = %d, want 1", c)
	}
	if c := countEvents(t, r, contract.EventAssetUpdated, a.ID); c != 0 {
		t.Errorf("asset_updated = %d, want 0 (status set quietly)", c)
	}
	if n := emailJobs(t, r, "ticket_approved", tk.ID); n != 1 {
		t.Errorf("approval email jobs = %d, want 1", n)
	}
	if held, err := r.borrowing.ApprovedTicketFor(ctx, a.ID); err != nil || held == nil || held.ID != tk.ID {
		t.Errorf("ApprovedTicketFor = %+v, %v", held, err)
	}

	done, loan, err := r.borrowing.HandOver(ctx, tk.ID, borrowToday, ap.Version)
	if err != nil {
		t.Fatal(err)
	}
	if done.State != domain.TicketHandedOver || loan.AssetID != a.ID || loan.BorrowerID != tk.RequesterID ||
		!loan.StartedOn.Equal(borrowToday) || loan.TicketID == nil || loan.TicketNumber != tk.Number {
		t.Errorf("hand over = %+v / %+v", done, loan)
	}
	if got := getAsset(t, r, a.ID); got.StatusID != domain.OnLoanStatusID {
		t.Errorf("asset status after hand over = %v, want On loan", got.StatusID)
	}
	if active, err := r.borrowing.ActiveLoan(ctx, a.ID); err != nil || active == nil || active.ID != loan.ID {
		t.Errorf("ActiveLoan = %+v, %v", active, err)
	}
	if c := countEvents(t, r, contract.EventAssetLent, a.ID); c != 1 {
		t.Errorf("asset_lent = %d, want 1", c)
	}
	if c := countEvents(t, r, contract.EventBorrowTicketHandedOver, tk.ID); c != 1 {
		t.Errorf("borrow_ticket_handed_over = %d, want 1", c)
	}
}

func TestBorrowing_RejectAndCancel(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	ty := borrowType(t, r, true)

	w := newTicket(t, r, ty.ID)
	rej, err := r.borrowing.Reject(ctx, w.ID, "No spare projector this week", w.Version)
	if err != nil || rej.State != domain.TicketRejected || rej.Reason != "No spare projector this week" {
		t.Fatalf("reject = %+v, %v", rej, err)
	}
	if n := emailJobs(t, r, "ticket_rejected", w.ID); n != 1 {
		t.Errorf("rejection email jobs = %d, want 1", n)
	}
	if _, err := r.borrowing.Cancel(ctx, w.ID, "", false, rej.Version); !errors.Is(err, domain.ErrTicketState) {
		t.Errorf("cancel rejected: %v, want ErrTicketState", err)
	}

	// Người quản lý huỷ phiếu đã duyệt: tài sản về Available, có thư
	a := borrowAsset(t, r, ty.ID, domain.AvailableStatusID)
	tk := newTicket(t, r, ty.ID)
	ap, err := r.borrowing.Approve(ctx, tk.ID, a.ID, tk.Version)
	if err != nil {
		t.Fatal(err)
	}
	c, err := r.borrowing.Cancel(ctx, tk.ID, "Not picked up", true, ap.Version)
	if err != nil || c.State != domain.TicketCancelled {
		t.Fatalf("cancel approved = %+v, %v", c, err)
	}
	if got := getAsset(t, r, a.ID); got.StatusID != domain.AvailableStatusID {
		t.Errorf("asset after cancel = %v, want Available", got.StatusID)
	}
	if n := countEvents(t, r, contract.EventAssetReservationCancelled, a.ID); n != 1 {
		t.Errorf("asset_reservation_cancelled = %d, want 1", n)
	}
	if n := emailJobs(t, r, "ticket_cancelled", tk.ID); n != 1 {
		t.Errorf("manager-cancel email jobs = %d, want 1", n)
	}

	// Người gửi tự huỷ: không thư
	own := newTicket(t, r, ty.ID)
	if _, err := r.borrowing.Cancel(ctx, own.ID, "", false, own.Version); err != nil {
		t.Fatal(err)
	}
	if n := emailJobs(t, r, "ticket_cancelled", own.ID); n != 0 {
		t.Errorf("requester-cancel email jobs = %d, want 0", n)
	}
}

func TestBorrowing_ApproveRules(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	ty := borrowType(t, r, true)
	tk := newTicket(t, r, ty.ID)

	other := borrowAsset(t, r, borrowType(t, r, true).ID, domain.AvailableStatusID)
	inUse := borrowAsset(t, r, ty.ID, domain.InUseStatusID)
	off := borrowType(t, r, false)
	offTicket := newTicket(t, r, off.ID)
	offAsset := borrowAsset(t, r, off.ID, domain.AvailableStatusID)
	for name, c := range map[string]struct {
		ticket domain.BorrowTicket
		asset  uuid.UUID
	}{
		"wrong type": {tk, other.ID}, "in use": {tk, inUse.ID}, "type switch off": {offTicket, offAsset.ID},
	} {
		if _, err := r.borrowing.Approve(ctx, c.ticket.ID, c.asset, c.ticket.Version); !errors.Is(err, domain.ErrNotBorrowable) {
			t.Errorf("%s: %v, want ErrNotBorrowable", name, err)
		}
	}
	free := borrowAsset(t, r, ty.ID, domain.AvailableStatusID)
	if _, err := r.borrowing.Approve(ctx, tk.ID, free.ID, tk.Version+3); !errors.Is(err, domain.ErrTicketChanged) {
		t.Errorf("stale version: %v, want ErrTicketChanged", err)
	}
	if _, err := r.borrowing.Approve(ctx, uuid.New(), free.ID, 1); !errors.Is(err, domain.ErrTicketNotFound) {
		t.Errorf("unknown ticket: %v, want ErrTicketNotFound", err)
	}
	if _, _, err := r.borrowing.HandOver(ctx, tk.ID, borrowToday, tk.Version); !errors.Is(err, domain.ErrTicketState) {
		t.Errorf("hand over a waiting ticket: %v, want ErrTicketState", err)
	}
}

// Hai người duyệt hai phiếu với cùng một tài sản cùng lúc: đúng một người được
func TestBorrowing_ConcurrentApproveSameAsset(t *testing.T) {
	r := newRepos(t)
	ty := borrowType(t, r, true)
	for round := range 20 {
		a := borrowAsset(t, r, ty.ID, domain.AvailableStatusID)
		t1, t2 := newTicket(t, r, ty.ID), newTicket(t, r, ty.ID)
		errs := make([]error, 2)
		var wg sync.WaitGroup
		for i, tk := range []domain.BorrowTicket{t1, t2} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, errs[i] = r.borrowing.Approve(actorCtx(), tk.ID, a.ID, tk.Version)
			}()
		}
		wg.Wait()
		ok := 0
		for _, err := range errs {
			switch {
			case err == nil:
				ok++
			case !errors.Is(err, domain.ErrNotBorrowable):
				t.Errorf("round %d: loser error %v, want ErrNotBorrowable", round, err)
			}
		}
		if ok != 1 {
			t.Fatalf("round %d: successes = %d, want 1 (%v)", round, ok, errs)
		}
	}
}

// Duyệt phiếu và cho mượn trực tiếp cùng một tài sản cùng lúc: đúng một bên được
func TestBorrowing_ApproveVsLend(t *testing.T) {
	r := newRepos(t)
	ty := borrowType(t, r, true)
	for round := range 20 {
		a := borrowAsset(t, r, ty.ID, domain.AvailableStatusID)
		tk := newTicket(t, r, ty.ID)
		var approveErr, lendErr error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, approveErr = r.borrowing.Approve(actorCtx(), tk.ID, a.ID, tk.Version)
		}()
		go func() {
			defer wg.Done()
			_, lendErr = r.borrowing.Lend(actorCtx(), a.ID, uuid.New(), borrowToday, nil, a.Version)
		}()
		wg.Wait()
		if (approveErr == nil) == (lendErr == nil) {
			t.Fatalf("round %d: approve %v, lend %v; want exactly one success", round, approveErr, lendErr)
		}
		loser := approveErr
		if loser == nil {
			loser = lendErr
		}
		if !errors.Is(loser, domain.ErrNotBorrowable) && !errors.Is(loser, domain.ErrAssetChanged) {
			t.Errorf("round %d: loser error %v", round, loser)
		}
	}
}

// Cho mượn và retire cùng lúc: không bao giờ cả hai; cho mượn thắng thì retire bị từ chối
func TestBorrowing_LendVsRetire(t *testing.T) {
	r := newRepos(t)
	ty := borrowType(t, r, true)
	for round := range 20 {
		a := borrowAsset(t, r, ty.ID, domain.AvailableStatusID)
		var lendErr, retireErr error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, lendErr = r.borrowing.Lend(actorCtx(), a.ID, uuid.New(), borrowToday, nil, a.Version)
		}()
		go func() {
			defer wg.Done()
			_, retireErr = r.assets.Retire(actorCtx(), a.ID, "x", domain.RetiredStatusID, a.Version)
		}()
		wg.Wait()
		if lendErr == nil && retireErr == nil {
			t.Fatalf("round %d: both lend and retire succeeded", round)
		}
		if lendErr == nil && !errors.Is(retireErr, domain.ErrAssetInBorrowing) && !errors.Is(retireErr, domain.ErrAssetChanged) {
			t.Errorf("round %d: retire after lend: %v", round, retireErr)
		}
	}
}

// Tắt Can be borrowed sau khi duyệt: vẫn giao được; cho mượn mới của loại đó thì không
func TestBorrowing_SwitchOffAfterApprove(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	ty := borrowType(t, r, true)
	a := borrowAsset(t, r, ty.ID, domain.AvailableStatusID)
	tk := newTicket(t, r, ty.ID)
	ap, err := r.borrowing.Approve(ctx, tk.ID, a.ID, tk.Version)
	if err != nil {
		t.Fatal(err)
	}
	off := false
	if _, err := r.types.Update(ctx, ty.ID, nil, nil, &off, ty.Version); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.borrowing.HandOver(ctx, tk.ID, borrowToday, ap.Version); err != nil {
		t.Errorf("hand over after switch off: %v", err)
	}
	b := borrowAsset(t, r, ty.ID, domain.AvailableStatusID)
	if _, err := r.borrowing.Lend(ctx, b.ID, uuid.New(), borrowToday, nil, b.Version); !errors.Is(err, domain.ErrNotBorrowable) {
		t.Errorf("lend after switch off: %v, want ErrNotBorrowable", err)
	}
}

func TestBorrowing_QueueOverdueEmails(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	ty := borrowType(t, r, true)
	late, soon := borrowAsset(t, r, ty.ID, domain.AvailableStatusID), borrowAsset(t, r, ty.ID, domain.AvailableStatusID)
	lateLoan, err := r.borrowing.Lend(ctx, late.ID, uuid.New(), time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC), &pastDay, late.Version)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.borrowing.Lend(ctx, soon.ID, uuid.New(), time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC), &laterDay, soon.Version); err != nil {
		t.Fatal(err)
	}
	n, err := r.borrowing.QueueOverdueEmails(ctx, borrowToday, 100)
	if err != nil || n != 1 {
		t.Fatalf("first scan = %d, %v; want 1", n, err)
	}
	if c := emailJobs(t, r, "loan_overdue", lateLoan.ID); c != 1 {
		t.Errorf("overdue email jobs = %d, want 1", c)
	}
	if n, err := r.borrowing.QueueOverdueEmails(ctx, borrowToday, 100); err != nil || n != 0 {
		t.Errorf("second scan = %d, %v; want 0", n, err)
	}
	loans, total, err := r.borrowing.ListLoans(ctx, borrowToday, domain.LoanFilter{State: "active", Page: 1, PageSize: 50})
	if err != nil || total < 2 || loans[0].ID != lateLoan.ID {
		t.Errorf("active loans: first = %+v (total %d, %v), want the overdue one first", loans, total, err)
	}
	// Sắp theo cột: ngày bắt đầu mới nhất trước
	loans, _, err = r.borrowing.ListLoans(ctx, borrowToday, domain.LoanFilter{State: "active", Sort: "-started_on", Page: 1, PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(loans); i++ {
		if loans[i].StartedOn.After(loans[i-1].StartedOn) {
			t.Errorf("-started_on: %v after %v", loans[i].StartedOn, loans[i-1].StartedOn)
		}
	}
}

// lentAsset: tài sản đang cho mượn trực tiếp từ 2026-10-20; trả tài sản (đã đọc lại) và khoản mượn
func lentAsset(t *testing.T, r repos, borrower uuid.UUID) (domain.Asset, domain.Loan) {
	t.Helper()
	ty := borrowType(t, r, true)
	a := borrowAsset(t, r, ty.ID, domain.AvailableStatusID)
	loan, err := r.borrowing.Lend(actorCtx(), a.ID, borrower, time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC), nil, a.Version)
	if err != nil {
		t.Fatal(err)
	}
	return getAsset(t, r, a.ID), loan
}

func TestBorrowing_Return(t *testing.T) {
	r := newRepos(t)
	a, loan := lentAsset(t, r, uuid.New())
	officer := uuid.New()
	ctx := auth.WithActor(context.Background(), auth.Actor{AccountID: officer})
	got, err := r.borrowing.Return(ctx, a.ID, domain.LoanReturn{ReturnedOn: pastDay.AddDate(0, 0, 3), StatusID: domain.AvailableStatusID, Note: "scratched lid"}, a.Version)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != loan.ID || got.ReturnedOn == nil || !got.ReturnedOn.Equal(pastDay.AddDate(0, 0, 3)) ||
		got.ReturnedBy == nil || *got.ReturnedBy != officer || got.ReturnNote != "scratched lid" {
		t.Errorf("returned loan = %+v", got)
	}
	if s := getAsset(t, r, a.ID).StatusID; s != domain.AvailableStatusID {
		t.Errorf("status = %v, want Available", s)
	}
	if l, err := r.borrowing.ActiveLoan(context.Background(), a.ID); err != nil || l != nil {
		t.Errorf("active loan after return = %+v, %v", l, err)
	}
	if n := emailJobs(t, r, "loan_returned", loan.ID); n != 1 {
		t.Errorf("receipt jobs = %d, want 1", n)
	}
	if n := countEvents(t, r, contract.EventAssetReturned, a.ID); n != 1 {
		t.Errorf("asset_returned events = %d, want 1", n)
	}
	again := getAsset(t, r, a.ID)
	if _, err := r.borrowing.Lend(actorCtx(), a.ID, uuid.New(), borrowToday, nil, again.Version); err != nil {
		t.Errorf("lend again after return: %v", err)
	}
}

func TestBorrowing_ReturnRules(t *testing.T) {
	r := newRepos(t)
	ret := domain.LoanReturn{ReturnedOn: borrowToday, StatusID: domain.AvailableStatusID}
	ty := borrowType(t, r, true)
	free := borrowAsset(t, r, ty.ID, domain.AvailableStatusID)
	if _, err := r.borrowing.Return(actorCtx(), free.ID, ret, free.Version); !errors.Is(err, domain.ErrNoActiveLoan) {
		t.Errorf("available asset: %v, want ErrNoActiveLoan", err)
	}
	held := borrowAsset(t, r, ty.ID, domain.AvailableStatusID)
	tk := newTicket(t, r, ty.ID)
	if _, err := r.borrowing.Approve(actorCtx(), tk.ID, held.ID, tk.Version); err != nil {
		t.Fatal(err)
	}
	held = getAsset(t, r, held.ID)
	if _, err := r.borrowing.Return(actorCtx(), held.ID, ret, held.Version); !errors.Is(err, domain.ErrNoActiveLoan) {
		t.Errorf("reserved asset: %v, want ErrNoActiveLoan", err)
	}
	if s := getAsset(t, r, held.ID).StatusID; s != domain.ReservedStatusID {
		t.Errorf("reserved asset status changed to %v", s)
	}
	a, _ := lentAsset(t, r, uuid.New())
	if _, err := r.borrowing.Return(actorCtx(), a.ID, ret, a.Version-1); !errors.Is(err, domain.ErrAssetChanged) {
		t.Errorf("stale version: %v, want ErrAssetChanged", err)
	}
	ret.StatusID = domain.RepairStatusID
	if _, err := r.borrowing.Return(actorCtx(), a.ID, ret, a.Version); err != nil {
		t.Fatal(err)
	}
	if s := getAsset(t, r, a.ID).StatusID; s != domain.RepairStatusID {
		t.Errorf("status = %v, want In repair", s)
	}
}

// Hai người cùng ghi nhận trả một khoản: đúng một lần thành công, một thư
func TestBorrowing_ConcurrentReturn(t *testing.T) {
	r := newRepos(t)
	for round := range 10 {
		a, loan := lentAsset(t, r, uuid.New())
		errs := make([]error, 2)
		var wg sync.WaitGroup
		for i := range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, errs[i] = r.borrowing.Return(actorCtx(), a.ID, domain.LoanReturn{ReturnedOn: borrowToday, StatusID: domain.AvailableStatusID}, a.Version)
			}()
		}
		wg.Wait()
		if (errs[0] == nil) == (errs[1] == nil) {
			t.Fatalf("round %d: %v, %v; want exactly one success", round, errs[0], errs[1])
		}
		loser := errs[0]
		if loser == nil {
			loser = errs[1]
		}
		if !errors.Is(loser, domain.ErrNoActiveLoan) && !errors.Is(loser, domain.ErrAssetChanged) {
			t.Errorf("round %d: loser %v", round, loser)
		}
		if n := emailJobs(t, r, "loan_returned", loan.ID); n != 1 {
			t.Errorf("round %d: receipt jobs = %d, want 1", round, n)
		}
	}
}

// Khoản đang mượn của một người (My loans)
func TestBorrowing_ActiveLoansOfBorrower(t *testing.T) {
	r := newRepos(t)
	me := uuid.New()
	_, mine := lentAsset(t, r, me)
	lentAsset(t, r, uuid.New())
	loans, total, err := r.borrowing.ListLoans(context.Background(), borrowToday, domain.LoanFilter{State: "active", BorrowerID: &me, Page: 1, PageSize: 50})
	if err != nil || total != 1 || len(loans) != 1 || loans[0].ID != mine.ID {
		t.Errorf("my loans = %+v (total %d, %v), want only mine", loans, total, err)
	}
}

// pastLoan: khoản mượn bắt đầu start, đã trả ngày back
func pastLoan(t *testing.T, r repos, borrower uuid.UUID, start, back string) (domain.Asset, domain.Loan) {
	t.Helper()
	d := func(s string) time.Time { v, _ := time.Parse(time.DateOnly, s); return v }
	ty := borrowType(t, r, true)
	a := borrowAsset(t, r, ty.ID, domain.AvailableStatusID)
	l, err := r.borrowing.Lend(actorCtx(), a.ID, borrower, d(start), nil, a.Version)
	if err != nil {
		t.Fatal(err)
	}
	a = getAsset(t, r, a.ID)
	if _, err := r.borrowing.Return(actorCtx(), a.ID, domain.LoanReturn{ReturnedOn: d(back), StatusID: domain.AvailableStatusID}, a.Version); err != nil {
		t.Fatal(err)
	}
	return a, l
}

// loanSet: id các khoản trong danh sách (DB test dùng chung, chỉ xét khoản của test)
func loanSet(t *testing.T, r repos, f domain.LoanFilter) map[uuid.UUID]bool {
	t.Helper()
	f.Page, f.PageSize = 1, 200
	loans, _, err := r.borrowing.ListLoans(context.Background(), borrowToday, f)
	if err != nil {
		t.Fatal(err)
	}
	out := map[uuid.UUID]bool{}
	for _, l := range loans {
		out[l.ID] = true
	}
	return out
}

// Kỳ = khoản đang ở ngoài lúc nào đó trong kỳ: bắt đầu trước kỳ mà trả trong kỳ vẫn tính
func TestBorrowing_ListLoansPeriod(t *testing.T) {
	r := newRepos(t)
	who := uuid.New()
	_, a := pastLoan(t, r, who, "2026-10-01", "2026-10-05")
	_, b := pastLoan(t, r, who, "2026-09-20", "2026-10-02")
	_, c := pastLoan(t, r, who, "2026-09-01", "2026-09-30")
	from, to := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	got := loanSet(t, r, domain.LoanFilter{State: "all", BorrowerID: &who, From: &from, To: &to})
	if !got[a.ID] || !got[b.ID] || got[c.ID] {
		t.Errorf("October: a %v b %v c %v; want a and b only", got[a.ID], got[b.ID], got[c.ID])
	}
	if got := loanSet(t, r, domain.LoanFilter{State: "returned", BorrowerID: &who}); len(got) != 3 {
		t.Errorf("returned = %d loans, want 3", len(got))
	}
	if got := loanSet(t, r, domain.LoanFilter{State: "active", BorrowerID: &who}); len(got) != 0 {
		t.Errorf("active = %d loans, want 0", len(got))
	}
}

// Tìm theo tag tài sản hoặc người mượn: hợp của hai, không trùng dòng; lọc theo một tài sản
func TestBorrowing_ListLoansSearch(t *testing.T) {
	r := newRepos(t)
	assetA, a := pastLoan(t, r, uuid.New(), "2026-10-01", "2026-10-05")
	assetB, b := pastLoan(t, r, uuid.New(), "2026-10-01", "2026-10-05")
	cWho := uuid.New()
	_, c := pastLoan(t, r, cWho, "2026-10-01", "2026-10-05")
	got := loanSet(t, r, domain.LoanFilter{State: "all", Q: assetA.Tag, MatchBorrowers: []uuid.UUID{cWho}})
	if len(got) != 2 || !got[a.ID] || !got[c.ID] {
		t.Errorf("search = %v, want a and c", got)
	}
	if got := loanSet(t, r, domain.LoanFilter{State: "all", AssetID: &assetB.ID}); len(got) != 1 || !got[b.ID] {
		t.Errorf("asset filter = %v, want only b", got)
	}
}

type failingEnqueuer struct{}

func (failingEnqueuer) Enqueue(context.Context, jobs.Job, ...jobs.Option) error {
	return errors.New("queue down")
}
func (failingEnqueuer) EnqueueTx(context.Context, pgx.Tx, jobs.Job, ...jobs.Option) error {
	return errors.New("queue down")
}

// Xếp thư hỏng thì cả lần duyệt rollback: phiếu vẫn chờ, tài sản vẫn Available, không event
func TestBorrowing_AtomicOnEnqueueFailure(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	client, err := jobs.NewInsertClient(r.pool, nil)
	if err != nil {
		t.Fatal(err)
	}
	broken := repository.NewBorrowingRepository(r.pool, events.NewOutbox(events.NewRegistry(), client), failingEnqueuer{})
	ty := borrowType(t, r, true)
	a := borrowAsset(t, r, ty.ID, domain.AvailableStatusID)
	tk := newTicket(t, r, ty.ID)
	if _, err := broken.Approve(ctx, tk.ID, a.ID, tk.Version); err == nil {
		t.Fatal("approve succeeded with a broken queue")
	}
	if got, _ := r.borrowing.GetTicket(ctx, tk.ID); got.State != domain.TicketWaiting {
		t.Errorf("ticket state = %s, want waiting", got.State)
	}
	if got := getAsset(t, r, a.ID); got.StatusID != domain.AvailableStatusID {
		t.Errorf("asset status = %v, want Available", got.StatusID)
	}
	if c := countEvents(t, r, contract.EventAssetReserved, a.ID); c != 0 {
		t.Errorf("asset_reserved = %d, want 0", c)
	}
}

// Giao muộn sau hạn trả của phiếu (giữ chỗ không tự hết hạn): vẫn giao được, khoản mượn có hạn
// trả là ngày giao (quá hạn từ hôm sau) thay vì lỗi ràng buộc return_by >= started_on
func TestBorrowing_HandOverAfterReturnBy(t *testing.T) {
	r := newRepos(t)
	ctx := actorCtx()
	ty := borrowType(t, r, true)
	a := borrowAsset(t, r, ty.ID, domain.AvailableStatusID)
	due := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)
	tk, err := r.borrowing.CreateTicket(ctx, domain.NewBorrowTicket{RequesterID: uuid.New(), TypeID: ty.ID, NeededFrom: pastDay, ReturnBy: &due})
	if err != nil {
		t.Fatal(err)
	}
	ap, err := r.borrowing.Approve(ctx, tk.ID, a.ID, tk.Version)
	if err != nil {
		t.Fatal(err)
	}
	_, loan, err := r.borrowing.HandOver(ctx, tk.ID, borrowToday, ap.Version)
	if err != nil {
		t.Fatalf("late hand over: %v", err)
	}
	if loan.ReturnBy == nil || !loan.ReturnBy.Equal(borrowToday) {
		t.Errorf("loan return_by = %v, want the hand-over day %v", loan.ReturnBy, borrowToday)
	}
}
