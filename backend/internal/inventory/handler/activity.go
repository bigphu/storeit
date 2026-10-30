package handler

import (
	"context"

	"storeit/internal/inventory/handler/api"
)

// defaultActivityLimit: số dòng mỗi trang khi không truyền limit (validator không áp mặc định)
const defaultActivityLimit = 50

// GetAssetActivity: lịch sử hoạt động của tài sản, mới nhất trước; quyền kiểm tra trong service
func (h *Handler) GetAssetActivity(ctx context.Context, req api.GetAssetActivityRequestObject) (api.GetAssetActivityResponseObject, error) {
	limit := defaultActivityLimit
	if req.Params.Limit != nil {
		limit = *req.Params.Limit
	}
	entries, next, err := h.svc.AssetActivity(ctx, req.AssetID, req.Params.Before, limit)
	if err != nil {
		return nil, err
	}
	out := api.GetAssetActivity200JSONResponse{Items: make([]api.AssetActivityEntry, len(entries)), Next: next}
	for i, e := range entries {
		item := api.AssetActivityEntry{
			Id: e.ID, Kind: api.AssetActivityEntryKind(e.Kind), Type: e.Type, OccurredAt: e.OccurredAt,
			Changes: make([]api.ActivityChange, len(e.Changes)),
		}
		if e.Actor != nil {
			item.Actor = &api.ActivityActor{Id: e.Actor.ID, Name: e.Actor.Name}
		}
		for j, c := range e.Changes {
			item.Changes[j] = api.ActivityChange{Field: c.Field, Label: c.Label, From: c.From, To: c.To}
		}
		if e.Reason != "" {
			item.Reason = &e.Reason
		}
		if e.Related != nil {
			rel := api.ActivityRelated{Id: e.Related.ID, Tag: e.Related.Tag}
			if e.Related.Role != "" {
				role := api.ActivityRelatedRole(e.Related.Role)
				rel.Role = &role
			}
			item.Related = &rel
		}
		if e.Person != nil {
			item.Person = &api.ActivityActor{Id: e.Person.ID, Name: e.Person.Name}
		}
		if e.TicketNumber != 0 {
			item.TicketNumber = &e.TicketNumber
		}
		item.ReturnBy = toAPIDate(e.ReturnBy)
		if e.StatusName != "" {
			item.StatusName = &e.StatusName
		}
		out.Items[i] = item
	}
	return out, nil
}
