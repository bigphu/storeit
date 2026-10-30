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
	if _, err := r.borrowing.Lend(ctx, soon.ID, uuid.New(), time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC), &laterDay, soon.Version); err != nil {
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
	loans, total, err := r.borrowing.ListActiveLoans(ctx, borrowToday, 1, 50)
	if err != nil || total < 2 || loans[0].ID != lateLoan.ID {
		t.Errorf("active loans: first = %+v (total %d, %v), want the overdue one first", loans, total, err)
	}
}

type failingEnqueuer struct{}

func (failingEnqueuer) Enqueue(context.Context, jobs.Job, ...jobs.Option) error { return errors.New("queue down") }
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
