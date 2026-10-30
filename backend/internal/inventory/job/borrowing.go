package job

import (
	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"storeit/internal/platform/jobs"
)

// Loại thư của việc mượn (US-10)
const (
	EmailTicketApproved  = "ticket_approved"
	EmailTicketRejected  = "ticket_rejected"
	EmailTicketCancelled = "ticket_cancelled" // người quản lý huỷ phiếu đã duyệt
	EmailLoanOverdue     = "loan_overdue"
)

// BorrowingEmailArgs: gửi một thư về phiếu mượn (TicketID) hay khoản mượn (LoanID). Repository
// xếp job trong cùng transaction với thay đổi; worker đọc lại phiếu/khoản lúc chạy và bỏ qua
// nếu nó đã đổi (vd phiếu đã duyệt nhưng bị huỷ trước khi thư đi).
type BorrowingEmailArgs struct {
	EmailKind string     `json:"kind"`
	TicketID  *uuid.UUID `json:"ticket_id,omitempty"`
	LoanID    *uuid.UUID `json:"loan_id,omitempty"`
}

func (BorrowingEmailArgs) Kind() string { return "inventory.send_borrowing_email" }

func (BorrowingEmailArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: jobs.QueueDefault}
}

// OverdueScanArgs: quét khoản mượn quá hạn chưa báo, xếp thư cho từng khoản (periodic, mỗi giờ)
type OverdueScanArgs struct{}

func (OverdueScanArgs) Kind() string { return "inventory.scan_overdue_loans" }

func (OverdueScanArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: jobs.QueueDefault}
}
