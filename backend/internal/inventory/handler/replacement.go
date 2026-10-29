package handler

import (
	"context"

	"storeit/internal/inventory/domain"
	"storeit/internal/inventory/handler/api"
	"storeit/internal/inventory/service"
)

// RecordReplacement: ghi tài sản được thay; quyền và quy tắc kiểm trong service
func (h *Handler) RecordReplacement(ctx context.Context, req api.RecordReplacementRequestObject) (api.RecordReplacementResponseObject, error) {
	b := req.Body
	v, err := h.svc.RecordReplacement(ctx, req.AssetID, service.ReplacementInput{
		ReplacementID: b.ReplacementAssetId, ReplacedOn: b.ReplacedOn.Time, Reason: b.Reason,
	}, b.Version)
	if err != nil {
		return nil, err
	}
	return api.RecordReplacement200JSONResponse(toAPIAsset(v)), nil
}

// UndoReplacement: hoàn tác thay thế, khôi phục tài sản cũ
func (h *Handler) UndoReplacement(ctx context.Context, req api.UndoReplacementRequestObject) (api.UndoReplacementResponseObject, error) {
	v, err := h.svc.UndoReplacement(ctx, req.AssetID, req.Body.Version)
	if err != nil {
		return nil, err
	}
	return api.UndoReplacement200JSONResponse(toAPIAsset(v)), nil
}

// toAPIReplacementLink: liên kết thay thế nhìn từ một tài sản
func toAPIReplacementLink(l domain.ReplacementLink) api.ReplacementLink {
	return api.ReplacementLink{
		Id: l.AssetID, Tag: l.Tag, Name: l.Name, Reason: l.Reason, ReplacedOn: *toAPIDate(&l.ReplacedOn),
	}
}
