package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"storeit/internal/inventory/contract"
	"storeit/internal/inventory/domain"
	"storeit/internal/inventory/job"
	"storeit/internal/inventory/repository/db"
	"storeit/internal/platform/database"
	"storeit/internal/platform/errs"
	"storeit/internal/platform/events"
	"storeit/internal/platform/jobs"
)

// Mượn tài sản (US-10). Mỗi thao tác ghi là một transaction: khoá phiếu (nếu có) rồi tới tài
// sản, theo thứ tự đó ở mọi nơi; kiểm tra; ghi phiếu/khoản mượn; đổi status tài sản bằng
// SetAssetStatusQuiet (không event asset_updated); thêm event; xếp thư bằng EnqueueTx. Lỗi ở
// bất kỳ bước nào rollback cả (US10-AC6).

type BorrowingRepository struct {
	pool   *pgxpool.Pool
	q      *db.Queries
	outbox *events.Outbox
	jobs   jobs.Enqueuer
}

var _ domain.BorrowingRepository = (*BorrowingRepository)(nil)

func NewBorrowingRepository(pool *pgxpool.Pool, outbox *events.Outbox, enq jobs.Enqueuer) *BorrowingRepository {
	return &BorrowingRepository{pool: pool, q: db.New(pool), outbox: outbox, jobs: enq}
}

func (r *BorrowingRepository) CreateTicket(ctx context.Context, t domain.NewBorrowTicket) (domain.BorrowTicket, error) {
	id, err := newID()
	if err != nil {
		return domain.BorrowTicket{}, err
	}
	var out domain.BorrowTicket
	err = database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		reqs := t.Requirements
		if reqs == nil {
			reqs = []string{}
		}
		needed := t.NeededFrom
		row, err := q.InsertBorrowTicket(ctx, db.InsertBorrowTicketParams{
			ID: id, RequesterID: t.RequesterID, AssetTypeID: t.TypeID, Requirements: reqs,
			SuggestedAssetID: t.SuggestedAssetID, NeededFrom: toPgDate(&needed), ReturnBy: toPgDate(t.ReturnBy), Note: t.Note,
		})
		if err != nil {
			return fmt.Errorf("inventory: insert borrow ticket: %w", err)
		}
		out = toTicket(row)
		return r.ticketEvent(ctx, tx, contract.EventBorrowTicketCreated, out)
	})
	return out, err
}

func (r *BorrowingRepository) GetTicket(ctx context.Context, id uuid.UUID) (domain.BorrowTicket, error) {
	return ticketOrNotFound(r.q.GetBorrowTicket(ctx, id))
}

func (r *BorrowingRepository) ListTickets(ctx context.Context, f domain.TicketFilter) ([]domain.BorrowTicket, int64, error) {
	states := make([]string, len(f.States))
	for i, s := range f.States {
		states[i] = string(s)
	}
	page, size := pageOf(f.Page, f.PageSize)
	hasQ := f.Q != ""
	rows, err := r.q.ListBorrowTickets(ctx, db.ListBorrowTicketsParams{
		States: states, RequesterID: f.RequesterID, HasQ: hasQ,
		QRequesters: nonNil(f.MatchRequesters), QTypes: nonNil(f.MatchTypes),
		Sort: ticketSort(f.Sort), Off: int32((page - 1) * size), Lim: int32(size),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("inventory: list borrow tickets: %w", err)
	}
	total, err := r.q.CountBorrowTickets(ctx, db.CountBorrowTicketsParams{
		States: states, RequesterID: f.RequesterID, HasQ: hasQ,
		QRequesters: nonNil(f.MatchRequesters), QTypes: nonNil(f.MatchTypes),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("inventory: count borrow tickets: %w", err)
	}
	out := make([]domain.BorrowTicket, len(rows))
	for i, row := range rows {
		out[i] = toTicket(row)
	}
	return out, total, nil
}

func (r *BorrowingRepository) TicketCounts(ctx context.Context) (waiting, approved int64, err error) {
	rows, err := r.q.CountBorrowTicketsByState(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("inventory: count tickets: %w", err)
	}
	for _, row := range rows {
		switch domain.TicketState(row.State) {
		case domain.TicketWaiting:
			waiting = row.N
		case domain.TicketApproved:
			approved = row.N
		}
	}
	return waiting, approved, nil
}

// Approve giữ chỗ assetID cho phiếu đang chờ: tài sản phải đúng loại của phiếu và cho mượn
// được; tài sản → Reserved; xếp thư báo người gửi
func (r *BorrowingRepository) Approve(ctx context.Context, ticketID, assetID uuid.UUID, version int32) (domain.BorrowTicket, error) {
	var out domain.BorrowTicket
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		tk, err := lockTicket(ctx, q, ticketID, version, domain.TicketWaiting)
		if err != nil {
			return err
		}
		a, err := lockBorrowable(ctx, q, assetID)
		if err != nil {
			return err
		}
		if a.TypeID != tk.TypeID {
			return domain.ErrNotBorrowable
		}
		if err := setStatus(ctx, q, assetID, domain.ReservedStatusID); err != nil {
			return err
		}
		row, err := q.ApproveBorrowTicket(ctx, db.ApproveBorrowTicketParams{ID: ticketID, AssetID: &assetID, DecidedBy: actorID(ctx)})
		if err != nil {
			return mapBorrowErr(err, "approve ticket")
		}
		out = toTicket(row)
		if err := appendEvent(ctx, tx, r.outbox, contract.EventAssetReserved, contract.AggregateAsset, assetID, contract.AssetReserved{
			AssetID: assetID, TicketID: ticketID, TicketNumber: out.Number, AccountID: out.RequesterID,
		}); err != nil {
			return err
		}
		if err := r.ticketEvent(ctx, tx, contract.EventBorrowTicketApproved, out); err != nil {
			return err
		}
		return r.email(ctx, tx, job.EmailTicketApproved, &ticketID, nil)
	})
	return out, err
}

func (r *BorrowingRepository) Reject(ctx context.Context, ticketID uuid.UUID, reason string, version int32) (domain.BorrowTicket, error) {
	var out domain.BorrowTicket
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		if _, err := lockTicket(ctx, q, ticketID, version, domain.TicketWaiting); err != nil {
			return err
		}
		row, err := q.RejectBorrowTicket(ctx, db.RejectBorrowTicketParams{ID: ticketID, Reason: reason, DecidedBy: actorID(ctx)})
		if err != nil {
			return fmt.Errorf("inventory: reject ticket: %w", err)
		}
		out = toTicket(row)
		if err := r.ticketEvent(ctx, tx, contract.EventBorrowTicketRejected, out); err != nil {
			return err
		}
		return r.email(ctx, tx, job.EmailTicketRejected, &ticketID, nil)
	})
	return out, err
}

// Cancel huỷ phiếu đang chờ hay đã duyệt; đã duyệt thì tài sản về status mặc định Available.
// byManager: người quản lý huỷ (không phải người gửi), người gửi được báo qua thư.
func (r *BorrowingRepository) Cancel(ctx context.Context, ticketID uuid.UUID, reason string, byManager bool, version int32) (domain.BorrowTicket, error) {
	var out domain.BorrowTicket
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		tk, err := lockTicket(ctx, q, ticketID, version, domain.TicketWaiting, domain.TicketApproved)
		if err != nil {
			return err
		}
		if tk.State == domain.TicketApproved && tk.AssetID != nil {
			if err := r.releaseReservation(ctx, tx, q, tk, byManager); err != nil {
				return err
			}
		}
		row, err := q.CancelBorrowTicket(ctx, db.CancelBorrowTicketParams{ID: ticketID, Reason: reason, CancelledBy: actorID(ctx)})
		if err != nil {
			return fmt.Errorf("inventory: cancel ticket: %w", err)
		}
		out = toTicket(row)
		if err := r.ticketEvent(ctx, tx, contract.EventBorrowTicketCancelled, out); err != nil {
			return err
		}
		if !byManager {
			return nil
		}
		return r.email(ctx, tx, job.EmailTicketCancelled, &ticketID, nil)
	})
	return out, err
}

// releaseReservation đưa tài sản đang giữ chỗ cho tk về status mặc định Available
func (r *BorrowingRepository) releaseReservation(ctx context.Context, tx pgx.Tx, q *db.Queries, tk domain.BorrowTicket, byManager bool) error {
	assetID := *tk.AssetID
	locked, err := lockAssets(ctx, q, assetID)
	if err != nil {
		return err
	}
	if a, ok := locked[assetID]; ok && a.StatusID == domain.ReservedStatusID {
		avail, err := q.GetDefaultStatus(ctx, string(domain.KindAvailable))
		if err != nil {
			return fmt.Errorf("inventory: default available status: %w", err)
		}
		if err := setStatus(ctx, q, assetID, avail.ID); err != nil {
			return err
		}
	}
	return appendEvent(ctx, tx, r.outbox, contract.EventAssetReservationCancelled, contract.AggregateAsset, assetID, contract.AssetReservationCancelled{
		AssetID: assetID, TicketID: tk.ID, TicketNumber: tk.Number, AccountID: tk.RequesterID, ByManager: byManager,
	})
}

// HandOver giao tài sản của phiếu đã duyệt: khoản mượn bắt đầu startedOn, tài sản → On loan
func (r *BorrowingRepository) HandOver(ctx context.Context, ticketID uuid.UUID, startedOn time.Time, version int32) (domain.BorrowTicket, domain.Loan, error) {
	var out domain.BorrowTicket
	var loan domain.Loan
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		tk, err := lockTicket(ctx, q, ticketID, version, domain.TicketApproved)
		if err != nil {
			return err
		}
		assetID := *tk.AssetID
		locked, err := lockAssets(ctx, q, assetID)
		if err != nil {
			return err
		}
		if a, ok := locked[assetID]; !ok || a.StatusID != domain.ReservedStatusID {
			return domain.ErrTicketState
		}
		ticket := tk.ID
		// giữ chỗ không tự hết hạn: giao muộn sau hạn trả của phiếu thì hạn trả là ngày giao
		// (quá hạn từ hôm sau) thay vì vi phạm return_by >= started_on
		returnBy := tk.ReturnBy
		if returnBy != nil && returnBy.Before(startedOn) {
			due := startedOn
			returnBy = &due
		}
		if loan, err = r.insertLoan(ctx, q, assetID, tk.RequesterID, &ticket, startedOn, returnBy); err != nil {
			return err
		}
		loan.TicketNumber = tk.Number
		row, err := q.HandOverBorrowTicket(ctx, ticketID)
		if err != nil {
			return fmt.Errorf("inventory: hand over ticket: %w", err)
		}
		out = toTicket(row)
		if err := r.lent(ctx, tx, loan); err != nil {
			return err
		}
		return r.ticketEvent(ctx, tx, contract.EventBorrowTicketHandedOver, out)
	})
	return out, loan, err
}

// Lend cho mượn trực tiếp: tài sản cho mượn được, đúng version; tài sản → On loan
func (r *BorrowingRepository) Lend(ctx context.Context, assetID, borrowerID uuid.UUID, startedOn time.Time, returnBy *time.Time, version int32) (domain.Loan, error) {
	var loan domain.Loan
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		a, err := lockBorrowable(ctx, q, assetID)
		if err != nil {
			return err
		}
		if a.Version != version {
			return domain.ErrAssetChanged
		}
		if loan, err = r.insertLoan(ctx, q, assetID, borrowerID, nil, startedOn, returnBy); err != nil {
			return err
		}
		return r.lent(ctx, tx, loan)
	})
	return loan, err
}

func (r *BorrowingRepository) insertLoan(ctx context.Context, q *db.Queries, assetID, borrowerID uuid.UUID, ticketID *uuid.UUID, startedOn time.Time, returnBy *time.Time) (domain.Loan, error) {
	id, err := newID()
	if err != nil {
		return domain.Loan{}, err
	}
	row, err := q.InsertLoan(ctx, db.InsertLoanParams{
		ID: id, AssetID: assetID, BorrowerID: borrowerID, TicketID: ticketID, LentBy: actorID(ctx),
		StartedOn: toPgDate(&startedOn), ReturnBy: toPgDate(returnBy),
	})
	if err != nil {
		return domain.Loan{}, mapBorrowErr(err, "insert loan")
	}
	if err := setStatus(ctx, q, assetID, domain.OnLoanStatusID); err != nil {
		return domain.Loan{}, err
	}
	return toLoan(row, 0), nil
}

// lent: event asset_lent của khoản mượn vừa bắt đầu
func (r *BorrowingRepository) lent(ctx context.Context, tx pgx.Tx, l domain.Loan) error {
	p := contract.AssetLent{AssetID: l.AssetID, LoanID: l.ID, BorrowerID: l.BorrowerID, TicketID: l.TicketID, TicketNumber: l.TicketNumber}
	if l.ReturnBy != nil {
		p.ReturnBy = l.ReturnBy.Format(time.DateOnly)
	}
	return appendEvent(ctx, tx, r.outbox, contract.EventAssetLent, contract.AggregateAsset, l.AssetID, p)
}

// Return: khoá tài sản (phải đang On loan, đúng version) rồi khoản mượn đang hiệu lực; ghi ngày trả,
// người nhận lại, ghi chú; tài sản sang r.StatusID; event asset_returned; xếp thư biên nhận
func (r *BorrowingRepository) Return(ctx context.Context, assetID uuid.UUID, ret domain.LoanReturn, version int32) (domain.Loan, error) {
	var out domain.Loan
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		locked, err := lockAssets(ctx, q, assetID)
		if err != nil {
			return err
		}
		a, ok := locked[assetID]
		if !ok {
			return domain.ErrAssetNotFound
		}
		// người khác vừa ghi nhận trả: status đã đổi, báo "không còn khoản mượn" thay vì lệch version
		if a.StatusID != domain.OnLoanStatusID {
			return domain.ErrNoActiveLoan
		}
		if a.Version != version {
			return domain.ErrAssetChanged
		}
		active, err := q.GetActiveLoanForUpdate(ctx, assetID)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrNoActiveLoan
		}
		if err != nil {
			return fmt.Errorf("inventory: lock loan: %w", err)
		}
		var note *string
		if ret.Note != "" {
			note = &ret.Note
		}
		row, err := q.ReturnLoan(ctx, db.ReturnLoanParams{ID: active.ID, ReturnedOn: toPgDate(&ret.ReturnedOn), ReturnedBy: actorID(ctx), ReturnNote: note})
		if code, constraint := pgCode(err); code == codeCheckViolation && constraint == "loans_returned_on_check" {
			return domain.ErrInvalidLoan.With(errs.WithFields(errs.FieldError{Field: "returned_on", Detail: "can't be before the loan started"}))
		}
		if err != nil {
			return fmt.Errorf("inventory: return loan: %w", err)
		}
		var number int64
		if row.TicketID != nil {
			tk, err := ticketOrNotFound(q.GetBorrowTicket(ctx, *row.TicketID))
			if err != nil {
				return err
			}
			number = tk.Number
		}
		out = toLoan(row, number)
		if err := setStatus(ctx, q, assetID, ret.StatusID); err != nil {
			return err
		}
		p := contract.AssetReturned{
			AssetID: assetID, LoanID: out.ID, BorrowerID: out.BorrowerID, TicketID: out.TicketID, TicketNumber: number,
			ReturnedOn: ret.ReturnedOn.Format(time.DateOnly), StatusID: ret.StatusID, Note: ret.Note,
		}
		if err := appendEvent(ctx, tx, r.outbox, contract.EventAssetReturned, contract.AggregateAsset, assetID, p); err != nil {
			return err
		}
		return r.email(ctx, tx, job.EmailLoanReturned, nil, &out.ID)
	})
	return out, err
}

func (r *BorrowingRepository) ActiveLoan(ctx context.Context, assetID uuid.UUID) (*domain.Loan, error) {
	row, err := r.q.GetActiveLoanForAsset(ctx, assetID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inventory: active loan: %w", err)
	}
	l := toLoan(db.InventoryLoan{
		ID: row.ID, AssetID: row.AssetID, BorrowerID: row.BorrowerID, TicketID: row.TicketID, LentBy: row.LentBy,
		StartedOn: row.StartedOn, ReturnBy: row.ReturnBy, ReturnedOn: row.ReturnedOn, CreatedAt: row.CreatedAt,
		ReturnedBy: row.ReturnedBy, ReturnNote: row.ReturnNote,
	}, row.TicketNumber)
	return &l, nil
}

func (r *BorrowingRepository) LoanForTicket(ctx context.Context, ticketID uuid.UUID) (*domain.Loan, error) {
	row, err := r.q.GetLoanByTicket(ctx, &ticketID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inventory: loan of ticket: %w", err)
	}
	l := toLoan(row, 0)
	return &l, nil
}

func (r *BorrowingRepository) ApprovedTicketFor(ctx context.Context, assetID uuid.UUID) (*domain.BorrowTicket, error) {
	row, err := r.q.GetApprovedTicketForAsset(ctx, &assetID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inventory: approved ticket: %w", err)
	}
	t := toTicket(row)
	return &t, nil
}

func (r *BorrowingRepository) GetLoan(ctx context.Context, id uuid.UUID) (domain.Loan, error) {
	row, err := r.q.GetLoan(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Loan{}, domain.ErrLoanNotFound
	}
	if err != nil {
		return domain.Loan{}, fmt.Errorf("inventory: get loan: %w", err)
	}
	return toLoan(db.InventoryLoan{
		ID: row.ID, AssetID: row.AssetID, BorrowerID: row.BorrowerID, TicketID: row.TicketID, LentBy: row.LentBy,
		StartedOn: row.StartedOn, ReturnBy: row.ReturnBy, ReturnedOn: row.ReturnedOn, CreatedAt: row.CreatedAt,
		ReturnedBy: row.ReturnedBy, ReturnNote: row.ReturnNote,
	}, row.TicketNumber), nil
}

func (r *BorrowingRepository) ListActiveLoans(ctx context.Context, today time.Time, borrowerID *uuid.UUID, sort string, page, pageSize int) ([]domain.Loan, int64, error) {
	page, size := pageOf(page, pageSize)
	rows, err := r.q.ListActiveLoans(ctx, db.ListActiveLoansParams{BorrowerID: borrowerID, Today: toPgDate(&today), Sort: sort, Off: int32((page - 1) * size), Lim: int32(size)})
	if err != nil {
		return nil, 0, fmt.Errorf("inventory: list loans: %w", err)
	}
	total, err := r.q.CountActiveLoans(ctx, borrowerID)
	if err != nil {
		return nil, 0, fmt.Errorf("inventory: count loans: %w", err)
	}
	out := make([]domain.Loan, len(rows))
	for i, row := range rows {
		out[i] = toLoan(db.InventoryLoan{
			ID: row.ID, AssetID: row.AssetID, BorrowerID: row.BorrowerID, TicketID: row.TicketID, LentBy: row.LentBy,
			StartedOn: row.StartedOn, ReturnBy: row.ReturnBy, ReturnedOn: row.ReturnedOn, CreatedAt: row.CreatedAt,
			ReturnedBy: row.ReturnedBy, ReturnNote: row.ReturnNote,
		}, row.TicketNumber)
	}
	return out, total, nil
}

// QueueOverdueEmails: mỗi khoản quá hạn chưa báo được đánh dấu đã báo hôm nay và xếp một thư,
// trong cùng transaction; hai lần quét cùng lúc không báo trùng (SKIP LOCKED). Trả số đã xếp.
func (r *BorrowingRepository) QueueOverdueEmails(ctx context.Context, today time.Time, limit int) (int, error) {
	n := 0
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		rows, err := q.ListOverdueUnnotifiedLoans(ctx, db.ListOverdueUnnotifiedLoansParams{Today: toPgDate(&today), Lim: int32(limit)})
		if err != nil {
			return fmt.Errorf("inventory: overdue loans: %w", err)
		}
		for _, row := range rows {
			if err := q.MarkLoanOverdueNotified(ctx, db.MarkLoanOverdueNotifiedParams{ID: row.ID, Today: toPgDate(&today)}); err != nil {
				return fmt.Errorf("inventory: mark overdue notified: %w", err)
			}
			id := row.ID
			if err := r.email(ctx, tx, job.EmailLoanOverdue, nil, &id); err != nil {
				return err
			}
		}
		n = len(rows)
		return nil
	})
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (r *BorrowingRepository) ticketEvent(ctx context.Context, tx pgx.Tx, typ string, t domain.BorrowTicket) error {
	return appendEvent(ctx, tx, r.outbox, typ, contract.AggregateBorrowTicket, t.ID, contract.BorrowTicketChanged{
		TicketID: t.ID, Number: t.Number, AssetID: t.AssetID, Reason: t.Reason,
	})
}

func (r *BorrowingRepository) email(ctx context.Context, tx pgx.Tx, kind string, ticketID, loanID *uuid.UUID) error {
	if err := r.jobs.EnqueueTx(ctx, tx, job.BorrowingEmailArgs{EmailKind: kind, TicketID: ticketID, LoanID: loanID}); err != nil {
		return fmt.Errorf("inventory: queue borrowing email: %w", err)
	}
	return nil
}

// lockTicket khoá phiếu: không có thì ErrTicketNotFound, lệch version thì ErrTicketChanged,
// trạng thái không thuộc states thì ErrTicketState
func lockTicket(ctx context.Context, q *db.Queries, id uuid.UUID, version int32, states ...domain.TicketState) (domain.BorrowTicket, error) {
	tk, err := ticketOrNotFound(q.GetBorrowTicketForUpdate(ctx, id))
	if err != nil {
		return domain.BorrowTicket{}, err
	}
	if tk.Version != version {
		return domain.BorrowTicket{}, domain.ErrTicketChanged
	}
	for _, s := range states {
		if tk.State == s {
			return tk, nil
		}
	}
	return domain.BorrowTicket{}, domain.ErrTicketState
}

// lockBorrowable khoá tài sản và kiểm nó cho mượn được ngay (loại bật Can be borrowed, status
// kind available); không có thì ErrAssetNotFound, không cho mượn được thì ErrNotBorrowable
func lockBorrowable(ctx context.Context, q *db.Queries, id uuid.UUID) (domain.Asset, error) {
	locked, err := lockAssets(ctx, q, id)
	if err != nil {
		return domain.Asset{}, err
	}
	a, ok := locked[id]
	if !ok {
		return domain.Asset{}, domain.ErrAssetNotFound
	}
	if a.Retired() || domain.IsBorrowingStatus(a.StatusID) {
		return domain.Asset{}, domain.ErrNotBorrowable
	}
	t, err := q.GetAssetType(ctx, a.TypeID)
	if err != nil {
		return domain.Asset{}, fmt.Errorf("inventory: get asset type: %w", err)
	}
	st, err := q.GetStatus(ctx, a.StatusID)
	if err != nil {
		return domain.Asset{}, fmt.Errorf("inventory: get status: %w", err)
	}
	if !domain.Borrowable(toAssetType(t), toStatus(st)) {
		return domain.Asset{}, domain.ErrNotBorrowable
	}
	return a, nil
}

func setStatus(ctx context.Context, q *db.Queries, assetID, statusID uuid.UUID) error {
	if _, err := q.SetAssetStatusQuiet(ctx, db.SetAssetStatusQuietParams{ID: assetID, StatusID: statusID}); err != nil {
		return fmt.Errorf("inventory: set asset status: %w", err)
	}
	return nil
}

// mapBorrowErr: hai index "một phiếu giữ chỗ / một khoản mượn mỗi tài sản" là tài sản không
// còn cho mượn được (lưới cuối, khoá hàng đã chặn trước)
func mapBorrowErr(err error, what string) error {
	if code, constraint := pgCode(err); code == codeUniqueViolation &&
		(constraint == "borrow_tickets_one_approved_per_asset" || constraint == "loans_one_active_per_asset") {
		return domain.ErrNotBorrowable
	}
	return fmt.Errorf("inventory: %s: %w", what, err)
}

func ticketOrNotFound(row db.InventoryBorrowTicket, err error) (domain.BorrowTicket, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.BorrowTicket{}, domain.ErrTicketNotFound
	}
	if err != nil {
		return domain.BorrowTicket{}, fmt.Errorf("inventory: get borrow ticket: %w", err)
	}
	return toTicket(row), nil
}

func toTicket(t db.InventoryBorrowTicket) domain.BorrowTicket {
	out := domain.BorrowTicket{
		ID: t.ID, RequesterID: t.RequesterID, TypeID: t.AssetTypeID, Requirements: t.Requirements,
		SuggestedAssetID: t.SuggestedAssetID, NeededFrom: dateOf(t.NeededFrom), ReturnBy: fromPgDate(t.ReturnBy),
		Note: t.Note, State: domain.TicketState(t.State), AssetID: t.AssetID, DecidedBy: t.DecidedBy,
		DecidedAt: t.DecidedAt, Reason: t.Reason, CancelledBy: t.CancelledBy, CancelledAt: t.CancelledAt,
		Version: t.Version, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
	if t.Number != nil {
		out.Number = *t.Number
	}
	if out.Requirements == nil {
		out.Requirements = []string{}
	}
	return out
}

func toLoan(l db.InventoryLoan, number int64) domain.Loan {
	return domain.Loan{
		ID: l.ID, AssetID: l.AssetID, BorrowerID: l.BorrowerID, TicketID: l.TicketID, TicketNumber: number,
		LentBy: l.LentBy, StartedOn: dateOf(l.StartedOn), ReturnBy: fromPgDate(l.ReturnBy),
		ReturnedOn: fromPgDate(l.ReturnedOn), ReturnedBy: l.ReturnedBy, ReturnNote: deref(l.ReturnNote), CreatedAt: l.CreatedAt,
	}
}

func ticketSort(s string) string {
	switch s {
	case "number", "needed_from", "-needed_from":
		return s
	}
	return "-number"
}

// pageOf: trang từ 1, cỡ trang 1..200 (mặc định 50), như tham số Page/PageSize chung
func pageOf(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 50
	}
	return page, size
}

func nonNil(ids []uuid.UUID) []uuid.UUID {
	if ids == nil {
		return []uuid.UUID{}
	}
	return ids
}
