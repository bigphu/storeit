package handler

import (
	"context"

	"storeit/internal/inventory/domain"
	"storeit/internal/inventory/handler/api"
	"storeit/internal/inventory/service"
)

func (h *Handler) ListExportProfiles(ctx context.Context, _ api.ListExportProfilesRequestObject) (api.ListExportProfilesResponseObject, error) {
	ps, err := h.svc.ListExportProfiles(ctx)
	if err != nil {
		return nil, err
	}
	out := api.ListExportProfiles200JSONResponse{Items: make([]api.ExportProfile, len(ps))}
	for i, p := range ps {
		out.Items[i] = toAPIProfile(p)
	}
	return out, nil
}

func (h *Handler) CreateExportProfile(ctx context.Context, req api.CreateExportProfileRequestObject) (api.CreateExportProfileResponseObject, error) {
	p, err := h.svc.CreateExportProfile(ctx, service.ExportProfileInput{
		Name: req.Body.Name, Shared: deref(req.Body.Shared), Layout: toDomainLayout(req.Body.Layout),
	})
	if err != nil {
		return nil, err
	}
	return api.CreateExportProfile201JSONResponse(toAPIProfile(p)), nil
}

func (h *Handler) GetExportProfile(ctx context.Context, req api.GetExportProfileRequestObject) (api.GetExportProfileResponseObject, error) {
	p, err := h.svc.GetExportProfile(ctx, req.ProfileID)
	if err != nil {
		return nil, err
	}
	return api.GetExportProfile200JSONResponse(toAPIProfile(p)), nil
}

func (h *Handler) UpdateExportProfile(ctx context.Context, req api.UpdateExportProfileRequestObject) (api.UpdateExportProfileResponseObject, error) {
	ch := domain.ExportProfileChange{Name: req.Body.Name, Shared: req.Body.Shared, Version: req.Body.Version}
	if req.Body.Layout != nil {
		l := toDomainLayout(*req.Body.Layout)
		ch.Layout = &l
	}
	p, err := h.svc.UpdateExportProfile(ctx, req.ProfileID, ch)
	if err != nil {
		return nil, err
	}
	return api.UpdateExportProfile200JSONResponse(toAPIProfile(p)), nil
}

func (h *Handler) DeleteExportProfile(ctx context.Context, req api.DeleteExportProfileRequestObject) (api.DeleteExportProfileResponseObject, error) {
	if err := h.svc.DeleteExportProfile(ctx, req.ProfileID); err != nil {
		return nil, err
	}
	return api.DeleteExportProfile204Response{}, nil
}

func toAPIProfile(p service.ExportProfileView) api.ExportProfile {
	out := api.ExportProfile{
		Id: p.ID, Name: p.Name, Shared: p.Shared, Layout: toAPILayout(p.Layout), CanEdit: p.CanEdit,
		Version: p.Version, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
	}
	out.Owner.Id, out.Owner.Name = p.OwnerID, p.OwnerName
	return out
}
