package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	idcontract "storeit/internal/identity/contract"
	"storeit/internal/inventory/domain"
	"storeit/internal/platform/auth"
	"storeit/internal/platform/errs"
)

// Mượn tài sản (US-10). Người có quyền request gửi phiếu, xem và huỷ phiếu của mình; người có
// quyền manage xem mọi phiếu, duyệt (giữ chỗ một tài sản), từ chối, huỷ phiếu đã duyệt, giao và
// cho mượn trực tiếp. Ngày nghiệp vụ theo múi giờ của ứng dụng (s.loc).

// TicketInput: phiếu mới; NeededFrom và ReturnBy chỉ có phần ngày
type TicketInput struct {
	TypeID           uuid.UUID
	Requirements     []string
	SuggestedAssetID *uuid.UUID
	NeededFrom       time.Time
	ReturnBy         *time.Time
	Note             string
}

// AssetRef: tài sản được nhắc tới (tag, tên)
type AssetRef struct {
	ID        uuid.UUID
	Tag, Name string
}

type TicketView struct {
	domain.BorrowTicket
	Requester      idcontract.Account
	TypeName       string
	Asset          *AssetRef
	SuggestedAsset *AssetRef
	// Loan: khoản mượn của phiếu đã giao, chỉ có ở trang phiếu (GetTicket)
	Loan *domain.Loan
}

// ReturnInput: ghi nhận trả; StatusID nil là status mặc định Available (tình trạng tốt)
type ReturnInput struct {
	ReturnedOn time.Time
	StatusID   *uuid.UUID
	Note       string
}

type LoanView struct {
	domain.Loan
	Asset       AssetRef
	TypeName    string
	Borrower    idcontract.Account
	OverdueDays int
}

// BorrowingInfo: tài sản đang được giữ chỗ (reserved) hay đang cho mượn (on_loan)
type BorrowingInfo struct {
	State        string // "reserved" | "on_loan"
	Account      idcontract.Account
	TicketID     *uuid.UUID
	TicketNumber int64
	Since        *time.Time
	ReturnBy     *time.Time
	OverdueDays  int
}

type TicketCounts struct{ Waiting, Approved int64 }

const maxBorrowerResults = 10

// maxTicketSearchPeople: số account tối đa xét khi tìm phiếu theo tên người (đủ cho cả công ty)
const maxTicketSearchPeople = 1000

// Today: ngày nghiệp vụ hôm nay (cmd/seed dùng để đặt ngày cho phiếu demo)
func (s *Service) Today() time.Time { return s.today() }

// today: ngày hiện tại theo múi giờ ứng dụng, ở dạng UTC 00:00 (khớp cột date)
func (s *Service) today() time.Time {
	y, m, d := s.now().In(s.loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// BusinessDay: "hôm nay" theo múi giờ của ứng dụng, để form phiếu và hộp Lend kiểm ngày như
// server thay vì theo đồng hồ của trình duyệt
func (s *Service) BusinessDay(ctx context.Context) (time.Time, error) {
	if _, _, err := borrowActor(ctx); err != nil {
		return time.Time{}, err
	}
	return s.today(), nil
}

// borrowActor: actor có ít nhất một trong hai quyền mượn; canManage là có quyền manage
func borrowActor(ctx context.Context) (auth.Actor, bool, error) {
	a, ok := auth.FromContext(ctx)
	if !ok {
		return auth.Actor{}, false, auth.ErrUnauthenticated
	}
	if a.Can(domain.PermBorrowManage) {
		return a, true, nil
	}
	if _, err := auth.Require(ctx, domain.PermBorrowRequest); err != nil {
		return auth.Actor{}, false, err
	}
	return a, false, nil
}

func ticketField(field, detail string) error {
	return domain.ErrInvalidTicket.With(errs.WithFields(errs.FieldError{Field: field, Detail: detail}))
}

func (s *Service) CreateTicket(ctx context.Context, in TicketInput) (TicketView, error) {
	actor, err := auth.Require(ctx, domain.PermBorrowRequest)
	if err != nil {
		return TicketView{}, err
	}
	t, err := s.types.Get(ctx, in.TypeID)
	if errors.Is(err, domain.ErrTypeNotFound) {
		return TicketView{}, ticketField("asset_type_id", "unknown asset type")
	}
	if err != nil {
		return TicketView{}, err
	}
	if !t.CanBorrow || t.Archived() {
		return TicketView{}, ticketField("asset_type_id", "this type can't be borrowed")
	}
	reqs, err := resolveRequirements(t, in.Requirements)
	if err != nil {
		return TicketView{}, err
	}
	note, err := cleanText(in.Note, domain.MaxTicketNote)
	if err != nil {
		return TicketView{}, ticketField("note", "must be text up to 500 characters")
	}
	if err := domain.CheckTicketDates(in.NeededFrom, in.ReturnBy, s.today()); err != nil {
		return TicketView{}, err
	}
	if in.SuggestedAssetID != nil {
		a, err := s.assets.Get(ctx, *in.SuggestedAssetID)
		if errors.Is(err, domain.ErrAssetNotFound) || (err == nil && a.TypeID != t.ID) {
			return TicketView{}, ticketField("suggested_asset_id", "must be an asset of the requested type")
		}
		if err != nil {
			return TicketView{}, err
		}
	}
	tk, err := s.borrowing.CreateTicket(ctx, domain.NewBorrowTicket{
		RequesterID: actor.AccountID, TypeID: t.ID, Requirements: reqs, SuggestedAssetID: in.SuggestedAssetID,
		NeededFrom: in.NeededFrom, ReturnBy: in.ReturnBy, Note: note,
	})
	if err != nil {
		return TicketView{}, err
	}
	return s.ticketView(ctx, tk)
}

// resolveRequirements kiểm điều kiện thuộc tính như bộ lọc của danh sách; lỗi đổi field
// "attr[i]" thành "requirements[i]"
func resolveRequirements(t domain.AssetType, reqs []string) ([]string, error) {
	if len(reqs) > domain.MaxTicketRequirements {
		return nil, ticketField("requirements", "at most 10 specs")
	}
	if len(reqs) == 0 {
		return []string{}, nil
	}
	if _, _, err := domain.ResolveAttrQuery(t, reqs, ""); err != nil {
		var e *errs.Error
		if !errors.As(err, &e) {
			return nil, err
		}
		fields := make([]errs.FieldError, 0, len(e.Fields()))
		for _, f := range e.Fields() {
			fields = append(fields, errs.FieldError{Field: strings.Replace(f.Field, "attr[", "requirements[", 1), Detail: f.Detail})
		}
		return nil, domain.ErrInvalidTicket.With(errs.WithFields(fields...))
	}
	return reqs, nil
}

// GetTicket: người quản lý xem mọi phiếu; người khác chỉ phiếu của mình (phiếu người khác là 404)
func (s *Service) GetTicket(ctx context.Context, id uuid.UUID) (TicketView, error) {
	tk, err := s.visibleTicket(ctx, id)
	if err != nil {
		return TicketView{}, err
	}
	v, err := s.ticketView(ctx, tk)
	if err != nil || tk.State != domain.TicketHandedOver {
		return v, err
	}
	v.Loan, err = s.borrowing.LoanForTicket(ctx, tk.ID)
	return v, err
}

func (s *Service) visibleTicket(ctx context.Context, id uuid.UUID) (domain.BorrowTicket, error) {
	actor, canManage, err := borrowActor(ctx)
	if err != nil {
		return domain.BorrowTicket{}, err
	}
	tk, err := s.borrowing.GetTicket(ctx, id)
	if err != nil {
		return domain.BorrowTicket{}, err
	}
	if !canManage && tk.RequesterID != actor.AccountID {
		return domain.BorrowTicket{}, domain.ErrTicketNotFound
	}
	return tk, nil
}

// ListTickets: không có quyền manage thì luôn chỉ phiếu của mình; mine lọc phiếu của mình cho
// người quản lý. f.Q tìm theo tên/email người gửi hay tên loại.
func (s *Service) ListTickets(ctx context.Context, f domain.TicketFilter, mine bool) ([]TicketView, int64, error) {
	actor, canManage, err := borrowActor(ctx)
	if err != nil {
		return nil, 0, err
	}
	if !canManage || mine {
		id := actor.AccountID
		f.RequesterID = &id
	}
	if f.Q = strings.TrimSpace(f.Q); f.Q != "" {
		if f.MatchRequesters, f.MatchTypes, err = s.matchTicketQuery(ctx, f.Q); err != nil {
			return nil, 0, err
		}
	}
	items, total, err := s.borrowing.ListTickets(ctx, f)
	if err != nil {
		return nil, 0, err
	}
	out, err := s.ticketViews(ctx, items)
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// matchTicketQuery: account có tên/email chứa q và loại có tên chứa q
func (s *Service) matchTicketQuery(ctx context.Context, q string) ([]uuid.UUID, []uuid.UUID, error) {
	accs, err := s.accounts.SearchAccounts(ctx, q, maxTicketSearchPeople)
	if err != nil {
		return nil, nil, err
	}
	people := make([]uuid.UUID, len(accs))
	for i, a := range accs {
		people[i] = a.ID
	}
	types, err := s.types.List(ctx, true)
	if err != nil {
		return nil, nil, err
	}
	var typeIDs []uuid.UUID
	lq := strings.ToLower(q)
	for _, t := range types {
		if strings.Contains(strings.ToLower(t.Name), lq) {
			typeIDs = append(typeIDs, t.ID)
		}
	}
	return people, typeIDs, nil
}

func (s *Service) TicketCounts(ctx context.Context) (TicketCounts, error) {
	if _, err := auth.Require(ctx, domain.PermBorrowManage); err != nil {
		return TicketCounts{}, err
	}
	w, a, err := s.borrowing.TicketCounts(ctx)
	return TicketCounts{Waiting: w, Approved: a}, err
}

func (s *Service) ApproveTicket(ctx context.Context, id, assetID uuid.UUID, version int32) (TicketView, error) {
	if _, err := auth.Require(ctx, domain.PermBorrowManage); err != nil {
		return TicketView{}, err
	}
	tk, err := s.borrowing.Approve(ctx, id, assetID, version)
	if err != nil {
		return TicketView{}, err
	}
	return s.ticketView(ctx, tk)
}

func (s *Service) RejectTicket(ctx context.Context, id uuid.UUID, reason string, version int32) (TicketView, error) {
	if _, err := auth.Require(ctx, domain.PermBorrowManage); err != nil {
		return TicketView{}, err
	}
	r, err := cleanText(reason, domain.MaxTicketNote)
	if err != nil {
		return TicketView{}, ticketField("reason", "must be text up to 500 characters")
	}
	if r == "" {
		return TicketView{}, ticketField("reason", "is required")
	}
	tk, err := s.borrowing.Reject(ctx, id, r, version)
	if err != nil {
		return TicketView{}, err
	}
	return s.ticketView(ctx, tk)
}

// CancelTicket: người gửi huỷ phiếu của mình khi đang chờ hay đã duyệt; người quản lý huỷ phiếu
// đã duyệt của người khác (phiếu đang chờ thì từ chối), người gửi được báo qua thư
func (s *Service) CancelTicket(ctx context.Context, id uuid.UUID, reason string, version int32) (TicketView, error) {
	actor, _, err := borrowActor(ctx)
	if err != nil {
		return TicketView{}, err
	}
	tk, err := s.visibleTicket(ctx, id)
	if err != nil {
		return TicketView{}, err
	}
	own := tk.RequesterID == actor.AccountID
	if !own && tk.State != domain.TicketApproved {
		return TicketView{}, domain.ErrTicketState
	}
	r, err := cleanText(reason, domain.MaxTicketNote)
	if err != nil {
		return TicketView{}, ticketField("reason", "must be text up to 500 characters")
	}
	out, err := s.borrowing.Cancel(ctx, id, r, !own, version)
	if err != nil {
		return TicketView{}, err
	}
	return s.ticketView(ctx, out)
}

// HandOverTicket giao tài sản đã giữ chỗ; account người gửi phải còn hoạt động
func (s *Service) HandOverTicket(ctx context.Context, id uuid.UUID, version int32) (TicketView, error) {
	if _, err := auth.Require(ctx, domain.PermBorrowManage); err != nil {
		return TicketView{}, err
	}
	tk, err := s.borrowing.GetTicket(ctx, id)
	if err != nil {
		return TicketView{}, err
	}
	acc, err := s.accounts.GetAccount(ctx, tk.RequesterID)
	if errors.Is(err, idcontract.ErrAccountNotFound) || (err == nil && !acc.Active) {
		return TicketView{}, ticketField("requester", "the account is disabled; cancel the reservation instead")
	}
	if err != nil {
		return TicketView{}, err
	}
	out, _, err := s.borrowing.HandOver(ctx, id, s.today(), version)
	if err != nil {
		return TicketView{}, err
	}
	return s.ticketView(ctx, out)
}

// ReturnLoan ghi nhận trả khoản mượn đang hiệu lực của tài sản (US-11): ngày trả từ ngày bắt
// đầu tới hôm nay; tình trạng là status sau khi trả (không phải retired, Reserved, On loan hay
// status đã lưu trữ)
func (s *Service) ReturnLoan(ctx context.Context, assetID uuid.UUID, in ReturnInput, version int32) (AssetView, error) {
	if _, err := auth.Require(ctx, domain.PermBorrowManage); err != nil {
		return AssetView{}, err
	}
	loan, err := s.borrowing.ActiveLoan(ctx, assetID)
	if err != nil {
		return AssetView{}, err
	}
	if loan == nil {
		return AssetView{}, domain.ErrNoActiveLoan
	}
	if err := domain.CheckReturnedOn(loan.StartedOn, in.ReturnedOn, s.today()); err != nil {
		return AssetView{}, err
	}
	st, err := s.returnStatus(ctx, in.StatusID)
	if err != nil {
		return AssetView{}, err
	}
	note, err := cleanText(in.Note, domain.MaxTicketNote)
	if err != nil {
		return AssetView{}, loanField("note", "must be text up to 500 characters")
	}
	if _, err := s.borrowing.Return(ctx, assetID, domain.LoanReturn{ReturnedOn: in.ReturnedOn, StatusID: st, Note: note}, version); err != nil {
		return AssetView{}, err
	}
	a, err := s.assets.Get(ctx, assetID)
	if err != nil {
		return AssetView{}, err
	}
	return s.view(ctx, a)
}

// returnStatus: status tài sản sau khi trả; nil là status mặc định của kind available
func (s *Service) returnStatus(ctx context.Context, id *uuid.UUID) (uuid.UUID, error) {
	if id == nil {
		st, err := s.statuses.Default(ctx, domain.KindAvailable)
		return st.ID, err
	}
	st, err := s.statuses.Get(ctx, *id)
	if errors.Is(err, domain.ErrStatusNotFound) {
		return uuid.Nil, loanField("status_id", "unknown status")
	}
	if err != nil {
		return uuid.Nil, err
	}
	if st.Archived() || st.Kind == domain.KindRetired || domain.IsBorrowingStatus(st.ID) {
		return uuid.Nil, loanField("status_id", "pick a status an asset can be returned to")
	}
	return st.ID, nil
}

func loanField(field, detail string) error {
	return domain.ErrInvalidLoan.With(errs.WithFields(errs.FieldError{Field: field, Detail: detail}))
}

// LendAsset cho mượn trực tiếp, bắt đầu hôm nay
func (s *Service) LendAsset(ctx context.Context, assetID, borrowerID uuid.UUID, returnBy *time.Time, version int32) (AssetView, error) {
	if _, err := auth.Require(ctx, domain.PermBorrowManage); err != nil {
		return AssetView{}, err
	}
	acc, err := s.accounts.GetAccount(ctx, borrowerID)
	if errors.Is(err, idcontract.ErrAccountNotFound) || (err == nil && !acc.Active) {
		return AssetView{}, domain.ErrInvalidLoan.With(errs.WithFields(errs.FieldError{Field: "borrower_id", Detail: "unknown or disabled account"}))
	}
	if err != nil {
		return AssetView{}, err
	}
	if err := domain.CheckLoanReturnBy(returnBy, s.today()); err != nil {
		return AssetView{}, err
	}
	if _, err := s.borrowing.Lend(ctx, assetID, borrowerID, s.today(), returnBy, version); err != nil {
		return AssetView{}, err
	}
	a, err := s.assets.Get(ctx, assetID)
	if err != nil {
		return AssetView{}, err
	}
	return s.view(ctx, a)
}

// LoanQuery: danh sách khoản mượn (US-12). Mine: của chính actor; State "active" (mặc định),
// "returned", "all"; From/To là kỳ; Q tìm theo tag/tên tài sản hay người mượn
type LoanQuery struct {
	Mine           bool
	State          string
	From, To       *time.Time
	AssetID        *uuid.UUID
	Q, Sort        string
	Page, PageSize int
}

// ListLoans: khoản mượn; Mine là của chính actor (quyền mượn nào cũng được), còn lại (cả lịch sử
// một tài sản) cần borrow.manage
func (s *Service) ListLoans(ctx context.Context, lq LoanQuery) ([]LoanView, int64, error) {
	f := domain.LoanFilter{State: lq.State, From: lq.From, To: lq.To, AssetID: lq.AssetID, Sort: lq.Sort, Page: lq.Page, PageSize: lq.PageSize}
	if lq.Mine && lq.AssetID == nil {
		actor, _, err := borrowActor(ctx)
		if err != nil {
			return nil, 0, err
		}
		f.BorrowerID = &actor.AccountID
	} else if a, err := auth.Require(ctx, domain.PermBorrowManage); err != nil {
		return nil, 0, err
	} else if lq.Mine {
		f.BorrowerID = &a.AccountID
	}
	switch f.State {
	case "":
		f.State = "active"
	case "active", "returned", "all":
	default:
		return nil, 0, loanField("state", "must be active, returned or all")
	}
	if f.From != nil && f.To != nil && f.From.After(*f.To) {
		return nil, 0, loanField("from", "must not be after to")
	}
	if f.Q = strings.TrimSpace(lq.Q); f.Q != "" {
		accs, err := s.accounts.SearchAccounts(ctx, f.Q, maxTicketSearchPeople)
		if err != nil {
			return nil, 0, err
		}
		f.MatchBorrowers = make([]uuid.UUID, len(accs))
		for i, acc := range accs {
			f.MatchBorrowers[i] = acc.ID
		}
	}
	today := s.today()
	loans, total, err := s.borrowing.ListLoans(ctx, today, f)
	if err != nil {
		return nil, 0, err
	}
	// đọc gộp: một lần cho người mượn, một lần cho tài sản (kèm tên loại)
	borrowers := make([]uuid.UUID, len(loans))
	assetIDs := make([]uuid.UUID, len(loans))
	for i, l := range loans {
		borrowers[i], assetIDs[i] = l.BorrowerID, l.AssetID
	}
	people, err := s.accountMap(ctx, borrowers)
	if err != nil {
		return nil, 0, err
	}
	assets, err := s.assetItems(ctx, assetIDs)
	if err != nil {
		return nil, 0, err
	}
	out := make([]LoanView, len(loans))
	for i, l := range loans {
		a := assets[l.AssetID]
		out[i] = LoanView{
			Loan: l, Asset: AssetRef{ID: l.AssetID, Tag: a.Tag, Name: a.Name}, TypeName: a.TypeName,
			Borrower: people[l.BorrowerID], OverdueDays: l.OverdueDays(today),
		}
	}
	return out, total, nil
}

// SearchBorrowers: account đang hoạt động để chọn người mượn (hộp Lend)
func (s *Service) SearchBorrowers(ctx context.Context, q string) ([]idcontract.Account, error) {
	if _, err := auth.Require(ctx, domain.PermBorrowManage); err != nil {
		return nil, err
	}
	return s.accounts.SearchAccounts(ctx, q, maxBorrowerResults)
}

func (s *Service) ticketView(ctx context.Context, tk domain.BorrowTicket) (TicketView, error) {
	vs, err := s.ticketViews(ctx, []domain.BorrowTicket{tk})
	if err != nil {
		return TicketView{}, err
	}
	return vs[0], nil
}

// ticketViews: các phiếu kèm người gửi, tên loại và tài sản, đọc gộp (một lần cho người gửi,
// một cho loại, một cho tài sản) thay vì vài truy vấn mỗi phiếu
func (s *Service) ticketViews(ctx context.Context, tks []domain.BorrowTicket) ([]TicketView, error) {
	requesters := make([]uuid.UUID, 0, len(tks))
	var assetIDs []uuid.UUID
	for _, tk := range tks {
		requesters = append(requesters, tk.RequesterID)
		for _, id := range []*uuid.UUID{tk.AssetID, tk.SuggestedAssetID} {
			if id != nil {
				assetIDs = append(assetIDs, *id)
			}
		}
	}
	people, err := s.accountMap(ctx, requesters)
	if err != nil {
		return nil, err
	}
	types, err := s.types.List(ctx, true)
	if err != nil {
		return nil, err
	}
	typeNames := make(map[uuid.UUID]string, len(types))
	for _, t := range types {
		typeNames[t.ID] = t.Name
	}
	assets, err := s.assetItems(ctx, assetIDs)
	if err != nil {
		return nil, err
	}
	ref := func(id *uuid.UUID) *AssetRef {
		if id == nil {
			return nil
		}
		a, ok := assets[*id]
		if !ok {
			return nil
		}
		return &AssetRef{ID: a.ID, Tag: a.Tag, Name: a.Name}
	}
	out := make([]TicketView, len(tks))
	for i, tk := range tks {
		out[i] = TicketView{
			BorrowTicket: tk, Requester: people[tk.RequesterID], TypeName: typeNames[tk.TypeID],
			Asset: ref(tk.AssetID), SuggestedAsset: ref(tk.SuggestedAssetID),
		}
	}
	return out, nil
}

// assetItems: các tài sản theo ID (kể cả đã retire) trong một lần đọc, kèm tên loại
func (s *Service) assetItems(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]domain.AssetListItem, error) {
	out := make(map[uuid.UUID]domain.AssetListItem, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	items, _, err := s.assets.List(ctx, domain.AssetFilter{IDs: ids, IncludeRetired: true, Limit: int32(len(ids))})
	if err != nil {
		return nil, err
	}
	for _, a := range items {
		out[a.ID] = a
	}
	return out, nil
}

// accountMap: account theo ID; account không còn thì tên "Unknown user"
func (s *Service) accountMap(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]idcontract.Account, error) {
	out := make(map[uuid.UUID]idcontract.Account, len(ids))
	accs, err := s.accounts.GetAccounts(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, a := range accs {
		out[a.ID] = a
	}
	for _, id := range ids {
		if _, ok := out[id]; !ok {
			out[id] = idcontract.Account{ID: id, Name: unknownUser}
		}
	}
	return out, nil
}

// borrowingInfo: tài sản Reserved thì phiếu giữ chỗ, On loan thì khoản đang mượn; khác thì nil
func (s *Service) borrowingInfo(ctx context.Context, a domain.Asset) (*BorrowingInfo, error) {
	if s.borrowing == nil || !domain.IsBorrowingStatus(a.StatusID) {
		return nil, nil
	}
	if a.StatusID == domain.ReservedStatusID {
		tk, err := s.borrowing.ApprovedTicketFor(ctx, a.ID)
		if err != nil || tk == nil {
			return nil, err
		}
		people, err := s.accountMap(ctx, []uuid.UUID{tk.RequesterID})
		if err != nil {
			return nil, err
		}
		id := tk.ID
		return &BorrowingInfo{State: "reserved", Account: people[tk.RequesterID], TicketID: &id, TicketNumber: tk.Number, ReturnBy: tk.ReturnBy}, nil
	}
	l, err := s.borrowing.ActiveLoan(ctx, a.ID)
	if err != nil || l == nil {
		return nil, err
	}
	people, err := s.accountMap(ctx, []uuid.UUID{l.BorrowerID})
	if err != nil {
		return nil, err
	}
	since := l.StartedOn
	return &BorrowingInfo{
		State: "on_loan", Account: people[l.BorrowerID], TicketID: l.TicketID, TicketNumber: l.TicketNumber,
		Since: &since, ReturnBy: l.ReturnBy, OverdueDays: l.OverdueDays(s.today()),
	}, nil
}
