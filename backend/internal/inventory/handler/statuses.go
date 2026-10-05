package handler

import (
	"context"

	"storeit/internal/inventory/domain"
	"storeit/internal/inventory/handler/api"
)

func (h *Handler) ListStatuses(ctx context.Context, req api.ListStatusesRequestObject) (api.ListStatusesResponseObject, error) {
	statuses, err := h.svc.ListStatuses(ctx, deref(req.Params.IncludeArchived))
	if err != nil {
		return nil, err
	}
	out := api.ListStatuses200JSONResponse{Items: make([]api.Status, len(statuses))}
	for i, s := range statuses {
		out.Items[i] = toAPIStatus(s)
	}
	if deref(req.Params.WithCounts) {
		counts, err := h.svc.StatusAssetCounts(ctx)
		if err != nil {
			return nil, err
		}
		for i, s := range statuses {
			n := counts[s.ID]
			out.Items[i].AssetCount = &n
		}
	}
	return out, nil
}

func (h *Handler) ReorderStatuses(ctx context.Context, req api.ReorderStatusesRequestObject) (api.ReorderStatusesResponseObject, error) {
	statuses, err := h.svc.ReorderStatuses(ctx, req.Body.Ids)
	if err != nil {
		return nil, err
	}
	out := api.ReorderStatuses200JSONResponse{Items: make([]api.Status, len(statuses))}
	for i, s := range statuses {
		out.Items[i] = toAPIStatus(s)
	}
	return out, nil
}

func (h *Handler) RestoreStatus(ctx context.Context, req api.RestoreStatusRequestObject) (api.RestoreStatusResponseObject, error) {
	s, err := h.svc.RestoreStatus(ctx, req.StatusID)
	if err != nil {
		return nil, err
	}
	return api.RestoreStatus200JSONResponse(toAPIStatus(s)), nil
}

func (h *Handler) CreateStatus(ctx context.Context, req api.CreateStatusRequestObject) (api.CreateStatusResponseObject, error) {
	s, err := h.svc.CreateStatus(ctx, domain.NewStatus{
		Name: req.Body.Name, Kind: domain.StatusKind(req.Body.Kind), Position: deref(req.Body.Position),
	})
	if err != nil {
		return nil, err
	}
	return api.CreateStatus201JSONResponse(toAPIStatus(s)), nil
}

func (h *Handler) UpdateStatus(ctx context.Context, req api.UpdateStatusRequestObject) (api.UpdateStatusResponseObject, error) {
	s, err := h.svc.UpdateStatus(ctx, req.StatusID, domain.StatusChange{
		Name: req.Body.Name, Position: req.Body.Position, MakeDefault: deref(req.Body.MakeDefault),
	})
	if err != nil {
		return nil, err
	}
	return api.UpdateStatus200JSONResponse(toAPIStatus(s)), nil
}

func (h *Handler) ArchiveStatus(ctx context.Context, req api.ArchiveStatusRequestObject) (api.ArchiveStatusResponseObject, error) {
	s, err := h.svc.ArchiveStatus(ctx, req.StatusID)
	if err != nil {
		return nil, err
	}
	return api.ArchiveStatus200JSONResponse(toAPIStatus(s)), nil
}
