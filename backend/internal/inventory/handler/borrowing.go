package handler

import (
	"context"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"storeit/internal/inventory/domain"
	"storeit/internal/inventory/handler/api"
	"storeit/internal/inventory/service"
)

// Mượn tài sản (US-10): quyền và quy tắc kiểm trong service

func (h *Handler) ListBorrowTickets(ctx context.Context, req api.ListBorrowTicketsRequestObject) (api.ListBorrowTicketsResponseObject, error) {
	p := req.Params
	f := domain.TicketFilter{Q: deref(p.Q), Sort: string(deref(p.Sort))}
	f.Page, f.PageSize = paging(p.Page, p.PageSize)
	for _, s := range deref(p.State) {
		f.States = append(f.States, domain.TicketState(s))
	}
	items, total, err := h.svc.ListTickets(ctx, f, deref(p.Mine))
	if err != nil {
		return nil, err
	}
	out := api.ListBorrowTickets200JSONResponse{Items: make([]api.BorrowTicket, len(items)), Total: total}
	for i, t := range items {
		out.Items[i] = toAPITicket(t)
	}
	return out, nil
}

func (h *Handler) CreateBorrowTicket(ctx context.Context, req api.CreateBorrowTicketRequestObject) (api.CreateBorrowTicketResponseObject, error) {
	b := req.Body
	t, err := h.svc.CreateTicket(ctx, service.TicketInput{
		TypeID: b.AssetTypeId, Requirements: deref(b.Requirements), SuggestedAssetID: b.SuggestedAssetId,
		NeededFrom: b.NeededFrom.Time, ReturnBy: fromAPIDate(b.ReturnBy), Note: deref(b.Note),
	})
	if err != nil {
		return nil, err
	}
	return api.CreateBorrowTicket201JSONResponse(toAPITicket(t)), nil
}

func (h *Handler) BorrowTicketSummary(ctx context.Context, _ api.BorrowTicketSummaryRequestObject) (api.BorrowTicketSummaryResponseObject, error) {
	c, err := h.svc.TicketCounts(ctx)
	if err != nil {
		return nil, err
	}
	return api.BorrowTicketSummary200JSONResponse{Waiting: c.Waiting, Approved: c.Approved}, nil
}

func (h *Handler) GetBusinessDay(ctx context.Context, _ api.GetBusinessDayRequestObject) (api.GetBusinessDayResponseObject, error) {
	d, err := h.svc.BusinessDay(ctx)
	if err != nil {
		return nil, err
	}
	return api.GetBusinessDay200JSONResponse{Today: *toAPIDate(&d)}, nil
}

func (h *Handler) GetBorrowTicket(ctx context.Context, req api.GetBorrowTicketRequestObject) (api.GetBorrowTicketResponseObject, error) {
	t, err := h.svc.GetTicket(ctx, req.TicketID)
	if err != nil {
		return nil, err
	}
	return api.GetBorrowTicket200JSONResponse(toAPITicket(t)), nil
}

func (h *Handler) ApproveBorrowTicket(ctx context.Context, req api.ApproveBorrowTicketRequestObject) (api.ApproveBorrowTicketResponseObject, error) {
	t, err := h.svc.ApproveTicket(ctx, req.TicketID, req.Body.AssetId, req.Body.Version)
	if err != nil {
		return nil, err
	}
	return api.ApproveBorrowTicket200JSONResponse(toAPITicket(t)), nil
}

func (h *Handler) RejectBorrowTicket(ctx context.Context, req api.RejectBorrowTicketRequestObject) (api.RejectBorrowTicketResponseObject, error) {
	t, err := h.svc.RejectTicket(ctx, req.TicketID, req.Body.Reason, req.Body.Version)
	if err != nil {
		return nil, err
	}
	return api.RejectBorrowTicket200JSONResponse(toAPITicket(t)), nil
}

func (h *Handler) CancelBorrowTicket(ctx context.Context, req api.CancelBorrowTicketRequestObject) (api.CancelBorrowTicketResponseObject, error) {
	t, err := h.svc.CancelTicket(ctx, req.TicketID, deref(req.Body.Reason), req.Body.Version)
	if err != nil {
		return nil, err
	}
	return api.CancelBorrowTicket200JSONResponse(toAPITicket(t)), nil
}

func (h *Handler) HandOverBorrowTicket(ctx context.Context, req api.HandOverBorrowTicketRequestObject) (api.HandOverBorrowTicketResponseObject, error) {
	t, err := h.svc.HandOverTicket(ctx, req.TicketID, req.Body.Version)
	if err != nil {
		return nil, err
	}
	return api.HandOverBorrowTicket200JSONResponse(toAPITicket(t)), nil
}

func (h *Handler) LendAsset(ctx context.Context, req api.LendAssetRequestObject) (api.LendAssetResponseObject, error) {
	v, err := h.svc.LendAsset(ctx, req.AssetID, req.Body.BorrowerId, fromAPIDate(req.Body.ReturnBy), req.Body.Version)
	if err != nil {
		return nil, err
	}
	return api.LendAsset200JSONResponse(toAPIAsset(v)), nil
}

func (h *Handler) ReturnAsset(ctx context.Context, req api.ReturnAssetRequestObject) (api.ReturnAssetResponseObject, error) {
	in := service.ReturnInput{ReturnedOn: req.Body.ReturnedOn.Time, Note: deref(req.Body.Note)}
	if req.Body.StatusId != nil {
		in.StatusID = req.Body.StatusId
	}
	v, err := h.svc.ReturnLoan(ctx, req.AssetID, in, req.Body.Version)
	if err != nil {
		return nil, err
	}
	return api.ReturnAsset200JSONResponse(toAPIAsset(v)), nil
}

func (h *Handler) ListLoans(ctx context.Context, req api.ListLoansRequestObject) (api.ListLoansResponseObject, error) {
	page, size := paging(req.Params.Page, req.Params.PageSize)
	loans, total, err := h.svc.ListLoans(ctx, service.LoanQuery{
		Mine: deref(req.Params.Mine), State: string(deref(req.Params.State)), From: fromAPIDate(req.Params.From),
		To: fromAPIDate(req.Params.To), AssetID: req.Params.AssetId, Q: deref(req.Params.Q),
		Sort: string(deref(req.Params.Sort)), Page: page, PageSize: size,
	})
	if err != nil {
		return nil, err
	}
	out := api.ListLoans200JSONResponse{Items: make([]api.Loan, len(loans)), Total: total}
	for i, l := range loans {
		out.Items[i] = api.Loan{
			Id: l.ID, Asset: api.AssetRef{Id: l.Asset.ID, Tag: l.Asset.Tag, Name: l.Asset.Name}, AssetTypeName: l.TypeName,
			Borrower: api.Person{Id: l.Borrower.ID, Name: l.Borrower.Name, Email: l.Borrower.Email},
			TicketId: l.TicketID, TicketNumber: l.TicketNumber, StartedOn: *toAPIDate(&l.StartedOn),
			ReturnBy: toAPIDate(l.ReturnBy), OverdueDays: l.OverdueDays, ReturnedOn: toAPIDate(l.ReturnedOn),
		}
		if l.ReturnNote != "" {
			out.Items[i].ReturnNote = &l.ReturnNote
		}
	}
	return out, nil
}

func (h *Handler) SearchBorrowers(ctx context.Context, req api.SearchBorrowersRequestObject) (api.SearchBorrowersResponseObject, error) {
	accs, err := h.svc.SearchBorrowers(ctx, deref(req.Params.Q))
	if err != nil {
		return nil, err
	}
	out := api.SearchBorrowers200JSONResponse{Items: make([]api.Person, len(accs))}
	for i, a := range accs {
		out.Items[i] = api.Person{Id: a.ID, Name: a.Name, Email: a.Email}
	}
	return out, nil
}

func toAPITicket(t service.TicketView) api.BorrowTicket {
	out := api.BorrowTicket{
		Id: t.ID, Number: t.Number, State: api.TicketState(t.State),
		Requester:    api.Person{Id: t.Requester.ID, Name: t.Requester.Name, Email: t.Requester.Email},
		Requirements: t.Requirements, NeededFrom: *toAPIDate(&t.NeededFrom), ReturnBy: toAPIDate(t.ReturnBy),
		Note: t.Note, Reason: t.Reason, DecidedAt: t.DecidedAt, CancelledAt: t.CancelledAt,
		Version: t.Version, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
	out.AssetType.Id, out.AssetType.Name = t.TypeID, t.TypeName
	if t.Asset != nil {
		out.Asset = &api.AssetRef{Id: t.Asset.ID, Tag: t.Asset.Tag, Name: t.Asset.Name}
	}
	if t.SuggestedAsset != nil {
		out.SuggestedAsset = &api.AssetRef{Id: t.SuggestedAsset.ID, Tag: t.SuggestedAsset.Tag, Name: t.SuggestedAsset.Name}
	}
	if t.Loan != nil {
		out.Loan = &api.TicketLoan{StartedOn: *toAPIDate(&t.Loan.StartedOn), ReturnBy: toAPIDate(t.Loan.ReturnBy), ReturnedOn: toAPIDate(t.Loan.ReturnedOn)}
	}
	return out
}

// toAPIBorrowing: tài sản đang giữ chỗ hay đang cho mượn
func toAPIBorrowing(b *service.BorrowingInfo) *api.AssetBorrowing {
	if b == nil {
		return nil
	}
	out := &api.AssetBorrowing{
		State: api.AssetBorrowingState(b.State), OverdueDays: b.OverdueDays,
		Since: toAPIDate(b.Since), ReturnBy: toAPIDate(b.ReturnBy),
	}
	out.Account.Id, out.Account.Name = b.Account.ID, b.Account.Name
	if b.TicketID != nil {
		out.Ticket = &struct {
			Id     openapi_types.UUID `json:"id"`
			Number int64              `json:"number"`
		}{Id: *b.TicketID, Number: b.TicketNumber}
	}
	return out
}

// paging: trang (mặc định 1) và cỡ trang (mặc định defaultPageSize); validator đã chặn giới hạn
func paging(page, size *int) (int, int) {
	p, s := 1, defaultPageSize
	if page != nil {
		p = *page
	}
	if size != nil {
		s = *size
	}
	return p, s
}
