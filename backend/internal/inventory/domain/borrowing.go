package domain

import (
	"time"

	"github.com/google/uuid"

	"storeit/internal/platform/errs"
)

// Mượn tài sản (US-10). Nhân viên gửi phiếu mượn (loại tài sản, yêu cầu thuộc tính); người
// quản lý duyệt bằng cách giữ chỗ một tài sản cụ thể (Reserved), giao tài sản thì khoản mượn
// bắt đầu (On loan). Người quản lý cũng cho mượn trực tiếp không cần phiếu. Phiếu và khoản
// mượn giữ mãi, không xoá.

const (
	MaxTicketNote         = 500 // cũng là giới hạn lý do từ chối / huỷ
	MaxTicketRequirements = 10
)

// Hai status hệ thống chỉ việc mượn đặt và gỡ; không ai chọn tay được
var (
	ReservedStatusID = uuid.MustParse("00000000-0000-7000-8000-000000000205")
	OnLoanStatusID   = uuid.MustParse("00000000-0000-7000-8000-000000000206")
)

// IsBorrowingStatus: status do việc mượn quản lý (Reserved, On loan). Tài sản mang status này
// không retire, không thay thế, không đổi status hay loại bằng tay được.
func IsBorrowingStatus(id uuid.UUID) bool { return id == ReservedStatusID || id == OnLoanStatusID }

// Borrowable: tài sản cho mượn được khi loại của nó bật "Can be borrowed" (và chưa archive) và
// status thuộc kind available
func Borrowable(t AssetType, st Status) bool {
	return t.CanBorrow && !t.Archived() && st.Kind == KindAvailable
}

type TicketState string

const (
	TicketWaiting    TicketState = "waiting"
	TicketApproved   TicketState = "approved" // đã giữ chỗ tài sản, chờ nhận
	TicketRejected   TicketState = "rejected"
	TicketCancelled  TicketState = "cancelled"
	TicketHandedOver TicketState = "handed_over" // đã giao, có khoản mượn
)

func (s TicketState) Valid() bool {
	switch s {
	case TicketWaiting, TicketApproved, TicketRejected, TicketCancelled, TicketHandedOver:
		return true
	}
	return false
}

// BorrowTicket là phiếu mượn. Ngày (NeededFrom, ReturnBy) chỉ có phần ngày, ở UTC 00:00.
type BorrowTicket struct {
	ID               uuid.UUID
	Number           int64 // hiện "#12"
	RequesterID      uuid.UUID
	TypeID           uuid.UUID
	Requirements     []string // "<key>:<op>:<value>" như bộ lọc thuộc tính của danh sách
	SuggestedAssetID *uuid.UUID
	NeededFrom       time.Time
	ReturnBy         *time.Time
	Note             string
	State            TicketState
	AssetID          *uuid.UUID // tài sản được giữ chỗ khi duyệt
	DecidedBy        *uuid.UUID
	DecidedAt        *time.Time
	Reason           string // lý do từ chối hay huỷ
	CancelledBy      *uuid.UUID
	CancelledAt      *time.Time
	Version          int32
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type NewBorrowTicket struct {
	RequesterID      uuid.UUID
	TypeID           uuid.UUID
	Requirements     []string
	SuggestedAssetID *uuid.UUID
	NeededFrom       time.Time
	ReturnBy         *time.Time
	Note             string
}

// Loan là một khoản mượn: bắt đầu khi giao tài sản (theo phiếu) hay cho mượn trực tiếp
type Loan struct {
	ID           uuid.UUID
	AssetID      uuid.UUID
	BorrowerID   uuid.UUID
	TicketID     *uuid.UUID
	TicketNumber int64 // 0: cho mượn trực tiếp
	LentBy       *uuid.UUID
	StartedOn    time.Time
	ReturnBy     *time.Time
	ReturnedOn   *time.Time // US-11
	ReturnedBy   *uuid.UUID // người ghi nhận trả
	ReturnNote   string
	CreatedAt    time.Time
}

// LoanReturn: ghi nhận trả một khoản mượn; StatusID là status tài sản sau khi trả (tình trạng)
type LoanReturn struct {
	ReturnedOn time.Time
	StatusID   uuid.UUID
	Note       string
}

// OverdueDays: số ngày đã quá hạn trả tính tới today; 0 khi chưa quá hạn, không có hạn hay
// đã trả
func (l Loan) OverdueDays(today time.Time) int {
	if l.ReturnBy == nil || l.ReturnedOn != nil {
		return 0
	}
	d := int(dateOnly(today).Sub(dateOnly(*l.ReturnBy)).Hours() / 24)
	return max(d, 0)
}

// TicketFilter lọc danh sách phiếu
type TicketFilter struct {
	States      []TicketState
	RequesterID *uuid.UUID // nil: mọi người (chỉ người quản lý)
	Q           string     // tên người mượn hay loại tài sản; service dịch sang hai danh sách dưới
	// Q khác rỗng: phiếu của một trong MatchRequesters hay thuộc một trong MatchTypes
	MatchRequesters []uuid.UUID
	MatchTypes      []uuid.UUID
	Sort            string // "-number" (mặc định) | "number" | "needed_from" | "-needed_from"
	Page            int
	PageSize        int
}

// CheckTicketDates: needed_from không trước today; return_by (nếu có) không trước needed_from
func CheckTicketDates(neededFrom time.Time, returnBy *time.Time, today time.Time) error {
	if dateOnly(neededFrom).Before(dateOnly(today)) {
		return invalid(ErrInvalidTicket, "needed_from", "can't be in the past")
	}
	if returnBy != nil && dateOnly(*returnBy).Before(dateOnly(neededFrom)) {
		return invalid(ErrInvalidTicket, "return_by", "can't be before needed_from")
	}
	return nil
}

// CheckLoanReturnBy: hạn trả (nếu có) của khoản cho mượn trực tiếp không trước today
func CheckLoanReturnBy(returnBy *time.Time, today time.Time) error {
	if returnBy != nil && dateOnly(*returnBy).Before(dateOnly(today)) {
		return invalid(ErrInvalidLoan, "return_by", "can't be in the past")
	}
	return nil
}

// CheckReturnedOn: ngày trả từ ngày bắt đầu khoản mượn tới hôm nay
func CheckReturnedOn(startedOn, returnedOn, today time.Time) error {
	if dateOnly(returnedOn).Before(dateOnly(startedOn)) {
		return invalid(ErrInvalidLoan, "returned_on", "can't be before the loan started")
	}
	if dateOnly(returnedOn).After(dateOnly(today)) {
		return invalid(ErrInvalidLoan, "returned_on", "can't be in the future")
	}
	return nil
}

func invalid(e *errs.Error, field, detail string) error {
	return e.With(errs.WithFields(errs.FieldError{Field: field, Detail: detail}))
}

// dateOnly: phần ngày ở UTC 00:00 (cột date của Postgres đọc ra như vậy)
func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
