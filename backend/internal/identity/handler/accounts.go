package handler

import (
	"context"

	"storeit/internal/identity/domain"
	"storeit/internal/identity/handler/api"
	"storeit/internal/identity/service"
)

// Mặc định phân trang: validator không chèn default của spec, nên đặt ở đây
const (
	defaultPageSize = 50
)

func (h *Handler) ListAccounts(ctx context.Context, req api.ListAccountsRequestObject) (api.ListAccountsResponseObject, error) {
	page, size := 1, defaultPageSize
	if req.Params.Page != nil {
		page = *req.Params.Page
	}
	if req.Params.PageSize != nil {
		size = *req.Params.PageSize
	}
	f := domain.AccountFilter{
		Query:  deref(req.Params.Q),
		Active: req.Params.Active,
		RoleID: req.Params.RoleId,
		Limit:  int32(size),
		Offset: int32((page - 1) * size),
	}
	if req.Params.Status != nil {
		st := domain.AccountStatus(*req.Params.Status)
		f.Status = &st
	}
	p, err := h.svc.ListAccounts(ctx, f)
	if err != nil {
		return nil, err
	}
	out := api.ListAccounts200JSONResponse{
		Items: make([]api.AccountListItem, len(p.Items)),
		Total: p.Total,
		StatusCounts: api.AccountStatusCounts{
			Invited:  p.StatusCounts[domain.StatusInvited],
			Active:   p.StatusCounts[domain.StatusActive],
			Disabled: p.StatusCounts[domain.StatusDisabled],
		},
	}
	for i, it := range p.Items {
		a := it.Account
		out.Items[i] = api.AccountListItem{
			Id: a.ID, Email: a.Email, Name: a.Name, MemberId: a.MemberID,
			Active: a.Active, Status: api.AccountStatus(a.Status()), Version: a.Version,
			LastSignInAt: a.LastSignInAt, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
			Roles: toSummaries(it.Roles), InviteExpiresAt: it.InviteExpiresAt,
		}
	}
	return out, nil
}

func (h *Handler) SignOutAccount(ctx context.Context, req api.SignOutAccountRequestObject) (api.SignOutAccountResponseObject, error) {
	n, err := h.svc.SignOutEverywhere(ctx, req.AccountID)
	if err != nil {
		return nil, err
	}
	return api.SignOutAccount200JSONResponse{Revoked: n}, nil
}

func (h *Handler) CreateAccount(ctx context.Context, req api.CreateAccountRequestObject) (api.CreateAccountResponseObject, error) {
	v, err := h.svc.CreateAccount(ctx, service.CreateAccountInput{
		Email: req.Body.Email, Name: req.Body.Name,
		MemberID: req.Body.MemberId, RoleIDs: ids(req.Body.RoleIds),
	})
	if err != nil {
		return nil, err
	}
	return api.CreateAccount201JSONResponse(toAPIDetail(v)), nil
}

func (h *Handler) GetAccount(ctx context.Context, req api.GetAccountRequestObject) (api.GetAccountResponseObject, error) {
	v, err := h.svc.GetAccount(ctx, req.AccountID)
	if err != nil {
		return nil, err
	}
	return api.GetAccount200JSONResponse(toAPIDetail(v)), nil
}

func (h *Handler) UpdateAccount(ctx context.Context, req api.UpdateAccountRequestObject) (api.UpdateAccountResponseObject, error) {
	v, err := h.svc.UpdateAccount(ctx, req.AccountID, domain.ProfileChange{
		Name:        req.Body.Name,
		MemberID:    req.Body.MemberId,
		ClearMember: deref(req.Body.ClearMemberId),
		Version:     req.Body.Version,
	})
	if err != nil {
		return nil, err
	}
	return api.UpdateAccount200JSONResponse(toAPIDetail(v)), nil
}

func (h *Handler) DisableAccount(ctx context.Context, req api.DisableAccountRequestObject) (api.DisableAccountResponseObject, error) {
	v, err := h.svc.DisableAccount(ctx, req.AccountID)
	if err != nil {
		return nil, err
	}
	return api.DisableAccount200JSONResponse(toAPIDetail(v)), nil
}

func (h *Handler) EnableAccount(ctx context.Context, req api.EnableAccountRequestObject) (api.EnableAccountResponseObject, error) {
	v, err := h.svc.EnableAccount(ctx, req.AccountID)
	if err != nil {
		return nil, err
	}
	return api.EnableAccount200JSONResponse(toAPIDetail(v)), nil
}

func (h *Handler) ResendInvitation(ctx context.Context, req api.ResendInvitationRequestObject) (api.ResendInvitationResponseObject, error) {
	if err := h.svc.ResendInvitation(ctx, req.AccountID); err != nil {
		return nil, err
	}
	return api.ResendInvitation202Response{}, nil
}

func (h *Handler) SendPasswordReset(ctx context.Context, req api.SendPasswordResetRequestObject) (api.SendPasswordResetResponseObject, error) {
	if err := h.svc.SendPasswordReset(ctx, req.AccountID); err != nil {
		return nil, err
	}
	return api.SendPasswordReset202Response{}, nil
}

func (h *Handler) AssignRoles(ctx context.Context, req api.AssignRolesRequestObject) (api.AssignRolesResponseObject, error) {
	v, err := h.svc.AssignRoles(ctx, req.AccountID, req.Body.RoleIds)
	if err != nil {
		return nil, err
	}
	return api.AssignRoles200JSONResponse(toAPIDetail(v)), nil
}
