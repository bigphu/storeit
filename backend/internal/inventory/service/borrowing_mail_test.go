package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"storeit/internal/inventory/domain"
	"storeit/internal/inventory/job"
)

// approvedTicket: phiếu của một người, đã duyệt với tài sản LAP-0042
func approvedTicket(t *testing.T, b *borrowEnv) (domain.BorrowTicket, uuid.UUID) {
	t.Helper()
	asset, err := b.svc.CreateAsset(manager, "LAP-0042", AssetInput{Name: "Dell Latitude 7440", TypeID: b.typeID})
	if err != nil {
		t.Fatal(err)
	}
	requester := uuid.New()
	tk, err := b.borrowing.CreateTicket(context.Background(), domain.NewBorrowTicket{RequesterID: requester, TypeID: b.typeID, NeededFrom: day("2026-10-30")})
	if err != nil {
		t.Fatal(err)
	}
	ap, err := b.borrowing.Approve(context.Background(), tk.ID, asset.ID, tk.Version)
	if err != nil {
		t.Fatal(err)
	}
	return ap, requester
}

func TestSendBorrowingEmail(t *testing.T) {
	b := newBorrowEnv(t, time.Date(2026, 10, 30, 3, 0, 0, 0, time.UTC))
	tk, requester := approvedTicket(t, b)
	if err := b.svc.SendBorrowingEmail(context.Background(), job.BorrowingEmailArgs{EmailKind: job.EmailTicketApproved, TicketID: &tk.ID}); err != nil {
		t.Fatal(err)
	}
	if len(b.mail.sent) != 1 {
		t.Fatalf("sent %d messages, want 1", len(b.mail.sent))
	}
	m := b.mail.sent[0]
	if m.To.Email != requester.String()[:8]+"@storeit.test" {
		t.Errorf("to = %+v, want the requester", m.To)
	}
	if m.Subject != "Ticket #1 approved: LAP-0042 is ready for pickup" {
		t.Errorf("subject = %q", m.Subject)
	}
	for _, want := range []string{"LAP-0042 · Dell Latitude 7440", "http://app.test/borrowing/mine"} {
		if !strings.Contains(m.Text, want) || !strings.Contains(m.HTML, want) {
			t.Errorf("text or HTML lacks %q:\n%s", want, m.Text)
		}
	}
	if m.IdempotencyKey != "inventory/borrowing/ticket_approved/"+tk.ID.String() {
		t.Errorf("idempotency key = %q", m.IdempotencyKey)
	}
}

func TestSendBorrowingEmail_Overdue(t *testing.T) {
	b := newBorrowEnv(t, time.Date(2026, 10, 30, 3, 0, 0, 0, time.UTC))
	asset, err := b.svc.CreateAsset(manager, "LAP-0031", AssetInput{Name: "ThinkPad T14", TypeID: b.typeID})
	if err != nil {
		t.Fatal(err)
	}
	due := day("2026-10-25")
	loan := domain.Loan{ID: uuid.New(), AssetID: asset.ID, BorrowerID: uuid.New(), StartedOn: day("2026-10-01"), ReturnBy: &due}
	b.borrowing.loans[loan.ID] = loan
	if err := b.svc.SendBorrowingEmail(context.Background(), job.BorrowingEmailArgs{EmailKind: job.EmailLoanOverdue, LoanID: &loan.ID}); err != nil {
		t.Fatal(err)
	}
	if len(b.mail.sent) != 1 || b.mail.sent[0].Subject != "LAP-0031 was due back on 25 Oct" ||
		!strings.Contains(b.mail.sent[0].Text, "It was due back on 25 Oct 2026.") {
		t.Errorf("overdue email = %+v", b.mail.sent)
	}
}

// Phiếu đã đổi (vd đã duyệt rồi bị huỷ) hay khoản đã trả trước khi job chạy: không gửi gì
func TestSendBorrowingEmail_SkipsStale(t *testing.T) {
	b := newBorrowEnv(t, time.Date(2026, 10, 30, 3, 0, 0, 0, time.UTC))
	tk, _ := approvedTicket(t, b)
	if _, err := b.borrowing.Cancel(context.Background(), tk.ID, "", true, tk.Version); err != nil {
		t.Fatal(err)
	}
	if err := b.svc.SendBorrowingEmail(context.Background(), job.BorrowingEmailArgs{EmailKind: job.EmailTicketApproved, TicketID: &tk.ID}); err != nil {
		t.Errorf("stale approval: %v, want nil", err)
	}
	returned := day("2026-10-28")
	due := day("2026-10-25")
	loan := domain.Loan{ID: uuid.New(), AssetID: uuid.New(), BorrowerID: uuid.New(), ReturnBy: &due, ReturnedOn: &returned}
	b.borrowing.loans[loan.ID] = loan
	if err := b.svc.SendBorrowingEmail(context.Background(), job.BorrowingEmailArgs{EmailKind: job.EmailLoanOverdue, LoanID: &loan.ID}); err != nil {
		t.Errorf("returned loan: %v, want nil", err)
	}
	if len(b.mail.sent) != 0 {
		t.Errorf("sent %d messages, want 0", len(b.mail.sent))
	}
}

// Quét quá hạn chỉ từ 08:00 giờ Việt Nam
func TestScanOverdueLoans_BusinessHours(t *testing.T) {
	early := newBorrowEnv(t, time.Date(2026, 10, 29, 17, 30, 0, 0, time.UTC)) // 00:30 ở HCM
	if err := early.svc.ScanOverdueLoans(context.Background()); err != nil || len(early.borrowing.overdueOn) != 0 {
		t.Errorf("00:30: err %v, scans %v; want no scan", err, early.borrowing.overdueOn)
	}
	morning := newBorrowEnv(t, time.Date(2026, 10, 30, 1, 5, 0, 0, time.UTC)) // 08:05 ở HCM
	if err := morning.svc.ScanOverdueLoans(context.Background()); err != nil || len(morning.borrowing.overdueOn) != 1 ||
		!morning.borrowing.overdueOn[0].Equal(day("2026-10-30")) {
		t.Errorf("08:05: err %v, scans %v; want one scan for 2026-10-30", err, morning.borrowing.overdueOn)
	}
}
