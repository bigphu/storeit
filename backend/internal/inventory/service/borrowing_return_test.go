package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	idcontract "storeit/internal/identity/contract"
	"storeit/internal/inventory/contract"
	"storeit/internal/inventory/domain"
	"storeit/internal/inventory/job"
	"storeit/internal/platform/events"
)

var officer = as(domain.PermAssetRead, domain.PermBorrowManage)

// onLoan: tài sản LAP-0042 cho mượn từ 2026-10-20; hôm nay là 2026-10-30 (giờ Việt Nam)
func onLoan(t *testing.T) (*borrowEnv, domain.Asset, domain.Loan) {
	t.Helper()
	b := newBorrowEnv(t, time.Date(2026, 10, 30, 3, 0, 0, 0, time.UTC))
	a, err := b.svc.CreateAsset(manager, "LAP-0042", AssetInput{Name: "Dell Latitude 7440", TypeID: b.typeID})
	if err != nil {
		t.Fatal(err)
	}
	l := domain.Loan{ID: uuid.New(), AssetID: a.ID, BorrowerID: uuid.New(), StartedOn: day("2026-10-20")}
	b.borrowing.loans[l.ID] = l
	return b, a.Asset, l
}

func TestReturnLoan_Dates(t *testing.T) {
	b, a, _ := onLoan(t)
	for _, bad := range []string{"2026-10-19", "2026-10-31"} {
		_, err := b.svc.ReturnLoan(officer, a.ID, ReturnInput{ReturnedOn: day(bad)}, a.Version)
		if f := fieldName(t, err, domain.ErrInvalidLoan); f != "returned_on" {
			t.Errorf("%s: field %q", bad, f)
		}
	}
	if len(b.borrowing.returns) != 0 {
		t.Errorf("repository Return called %d times, want 0", len(b.borrowing.returns))
	}
}

func TestReturnLoan_Condition(t *testing.T) {
	b, a, _ := onLoan(t)
	archived := time.Now()
	lost := domain.Status{ID: uuid.New(), Name: "Lost", Kind: domain.KindUnavailable, ArchivedAt: &archived}
	b.statuses.statuses[lost.ID] = lost
	for _, id := range []uuid.UUID{domain.RetiredStatusID, lost.ID, domain.ReservedStatusID, domain.OnLoanStatusID, uuid.New()} {
		_, err := b.svc.ReturnLoan(officer, a.ID, ReturnInput{ReturnedOn: day("2026-10-30"), StatusID: &id}, a.Version)
		if f := fieldName(t, err, domain.ErrInvalidLoan); f != "status_id" {
			t.Errorf("status %v: field %q", id, f)
		}
	}
	if _, err := b.svc.ReturnLoan(officer, a.ID, ReturnInput{ReturnedOn: day("2026-10-30")}, a.Version); err != nil {
		t.Fatal(err)
	}
	if got := b.borrowing.returns[0].StatusID; got != domain.AvailableStatusID {
		t.Errorf("no condition: status %v, want the default Available", got)
	}

	b2, a2, _ := onLoan(t)
	repair := domain.RepairStatusID
	if _, err := b2.svc.ReturnLoan(officer, a2.ID, ReturnInput{ReturnedOn: day("2026-10-30"), StatusID: &repair, Note: "  cracked hinge "}, a2.Version); err != nil {
		t.Fatal(err)
	}
	if r := b2.borrowing.returns[0]; r.StatusID != domain.RepairStatusID || r.Note != "cracked hinge" {
		t.Errorf("return = %+v, want Under repair with the trimmed note", r)
	}
}

func TestReturnLoan_Permissions(t *testing.T) {
	b, a, _ := onLoan(t)
	in := ReturnInput{ReturnedOn: day("2026-10-30")}
	if _, err := b.svc.ReturnLoan(as(domain.PermAssetRead, domain.PermBorrowRequest), a.ID, in, a.Version); status(err) != 403 {
		t.Errorf("borrow.request only = %v, want 403", err)
	}
	in.Note = strings.Repeat("x", 501)
	_, err := b.svc.ReturnLoan(officer, a.ID, in, a.Version)
	if f := fieldName(t, err, domain.ErrInvalidLoan); f != "note" {
		t.Errorf("long note: field %q", f)
	}
}

// Không có khoản mượn đang hiệu lực: 409, không gọi repository
func TestReturnLoan_NotOnLoan(t *testing.T) {
	b, a, l := onLoan(t)
	delete(b.borrowing.loans, l.ID)
	if _, err := b.svc.ReturnLoan(officer, a.ID, ReturnInput{ReturnedOn: day("2026-10-30")}, a.Version); status(err) != 409 {
		t.Errorf("no active loan = %v, want 409", err)
	}
}

// Account người mượn bị khoá sau khi mượn: vẫn nhận lại được tài sản
func TestReturnLoan_DisabledBorrower(t *testing.T) {
	b, a, l := onLoan(t)
	b.accounts.inactive[l.BorrowerID] = true
	if _, err := b.svc.ReturnLoan(officer, a.ID, ReturnInput{ReturnedOn: day("2026-10-30")}, a.Version); err != nil {
		t.Errorf("disabled borrower: %v", err)
	}
}

func TestListLoans_Mine(t *testing.T) {
	b := newBorrowEnv(t, time.Date(2026, 10, 30, 3, 0, 0, 0, time.UTC))
	me := uuid.New()
	if _, _, err := b.svc.ListLoans(asAccount(me, domain.PermBorrowRequest), true, "", 1, 50); err != nil {
		t.Fatal(err)
	}
	if f := b.borrowing.loansOf; f == nil || *f != me {
		t.Errorf("mine: borrower filter = %v, want the actor", f)
	}
	if _, _, err := b.svc.ListLoans(asAccount(me, domain.PermBorrowRequest), false, "", 1, 50); status(err) != 403 {
		t.Errorf("everyone's loans without manage = %v, want 403", err)
	}
	if _, _, err := b.svc.ListLoans(officer, false, "", 1, 50); err != nil || b.borrowing.loansOf != nil {
		t.Errorf("manager, everyone: err %v, filter %v", err, b.borrowing.loansOf)
	}
}

func TestSendBorrowingEmail_Returned(t *testing.T) {
	b, a, l := onLoan(t)
	returned := day("2026-10-28")
	l.ReturnedOn = &returned
	b.borrowing.loans[l.ID] = l
	if err := b.svc.SendBorrowingEmail(context.Background(), job.BorrowingEmailArgs{EmailKind: job.EmailLoanReturned, LoanID: &l.ID}); err != nil {
		t.Fatal(err)
	}
	if len(b.mail.sent) != 1 {
		t.Fatalf("sent %d, want 1", len(b.mail.sent))
	}
	m := b.mail.sent[0]
	if m.Subject != "LAP-0042 return recorded" {
		t.Errorf("subject = %q", m.Subject)
	}
	want := "We recorded LAP-0042 · Dell Latitude 7440 as returned on 28 Oct 2026. Thanks."
	if !strings.Contains(m.Text, want) || !strings.Contains(m.HTML, want) {
		t.Errorf("text or HTML lacks %q:\n%s", want, m.Text)
	}
	if m.IdempotencyKey != "inventory/borrowing/loan_returned/"+l.ID.String() {
		t.Errorf("idempotency key = %q", m.IdempotencyKey)
	}

	// khoản chưa trả: không gửi
	open := domain.Loan{ID: uuid.New(), AssetID: a.ID, BorrowerID: uuid.New(), StartedOn: day("2026-10-20")}
	b.borrowing.loans[open.ID] = open
	if err := b.svc.SendBorrowingEmail(context.Background(), job.BorrowingEmailArgs{EmailKind: job.EmailLoanReturned, LoanID: &open.ID}); err != nil || len(b.mail.sent) != 1 {
		t.Errorf("open loan: err %v, sent %d; want nothing sent", err, len(b.mail.sent))
	}
}

func TestResolveActivity_Returned(t *testing.T) {
	assetID, ticketID, person := uuid.New(), uuid.New(), uuid.New()
	lk := activityLookups{
		accounts: map[uuid.UUID]idcontract.Account{person: {ID: person, Name: "Phạm Thu Hà"}},
		statuses: map[uuid.UUID]domain.Status{
			domain.AvailableStatusID: {ID: domain.AvailableStatusID, Name: "Available", Kind: domain.KindAvailable},
			domain.RepairStatusID:    {ID: domain.RepairStatusID, Name: "In repair", Kind: domain.KindUnavailable},
		},
	}
	got := resolveActivity([]events.Event{
		ev(t, contract.EventAssetReturned, contract.AssetReturned{AssetID: assetID, LoanID: uuid.New(), BorrowerID: person, TicketID: &ticketID, TicketNumber: 12, ReturnedOn: "2026-10-30", StatusID: domain.RepairStatusID}, uuid.Nil),
		ev(t, contract.EventAssetReturned, contract.AssetReturned{AssetID: assetID, LoanID: uuid.New(), BorrowerID: person, ReturnedOn: "2026-10-30", StatusID: domain.AvailableStatusID}, uuid.Nil),
	}, lk)
	if len(got) != 2 {
		t.Fatalf("entries = %+v", got)
	}
	if g := got[0]; g.Kind != "returned" || g.Person == nil || g.Person.Name != "Phạm Thu Hà" || g.TicketNumber != 12 || g.StatusName != "In repair" {
		t.Errorf("returned to repair = %+v", g)
	}
	if g := got[1]; g.Kind != "returned" || g.TicketNumber != 0 || g.StatusName != "" {
		t.Errorf("returned fine = %+v", g)
	}
}

// Trang phiếu: phiếu đã giao kèm khoản mượn; phiếu đang chờ thì không
func TestGetTicket_Loan(t *testing.T) {
	b := newBorrowEnv(t, time.Date(2026, 10, 30, 3, 0, 0, 0, time.UTC))
	tk, requester := approvedTicket(t, b)
	if _, err := b.svc.HandOverTicket(as(domain.PermBorrowManage), tk.ID, tk.Version); err != nil {
		t.Fatal(err)
	}
	v, err := b.svc.GetTicket(asAccount(requester, domain.PermBorrowRequest), tk.ID)
	if err != nil || v.Loan == nil || !v.Loan.StartedOn.Equal(day("2026-10-30")) {
		t.Errorf("handed-over ticket loan = %+v, %v", v.Loan, err)
	}
	w, err := b.svc.CreateTicket(asAccount(requester, domain.PermBorrowRequest), TicketInput{TypeID: b.typeID, NeededFrom: day("2026-10-30")})
	if err != nil {
		t.Fatal(err)
	}
	if v, err := b.svc.GetTicket(asAccount(requester, domain.PermBorrowRequest), w.ID); err != nil || v.Loan != nil {
		t.Errorf("waiting ticket loan = %+v, %v; want nil", v.Loan, err)
	}
}
