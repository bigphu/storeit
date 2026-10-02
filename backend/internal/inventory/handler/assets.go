package handler

import (
	"context"

	"storeit/internal/inventory/domain"
	"storeit/internal/inventory/handler/api"
	"storeit/internal/inventory/service"
)

func (h *Handler) ListAssets(ctx context.Context, req api.ListAssetsRequestObject) (api.ListAssetsResponseObject, error) {
	p := req.Params
	page, size := 1, defaultPageSize
	if p.Page != nil {
		page = *p.Page
	}
	if p.PageSize != nil {
		size = *p.PageSize
	}
	f := domain.AssetFilter{
		Query: deref(p.Q), TypeID: p.TypeId, StatusID: p.StatusId, LocationID: p.LocationId,
		HolderMemberID: p.HolderMemberId, IncludeRetired: deref(p.IncludeRetired),
		Sort: domain.AssetSort(deref(p.Sort)), Limit: int32(size), Offset: int32((page - 1) * size),
	}
	if p.StatusKind != nil {
		k := domain.StatusKind(*p.StatusKind)
		f.StatusKind = &k
	}
	items, total, err := h.svc.ListAssets(ctx, f)
	if err != nil {
		return nil, err
	}
	out := api.ListAssets200JSONResponse{Items: make([]api.AssetListItem, len(items)), Total: total}
	for i, a := range items {
		out.Items[i] = toAPIListItem(a)
	}
	// lọc theo loại: kèm cột thuộc tính. Không có dòng thì không tra loại, để
	// type_id lạ vẫn trả danh sách rỗng như các bộ lọc khác.
	if p.TypeId != nil && len(items) > 0 {
		t, err := h.svc.GetAssetType(ctx, *p.TypeId)
		if err != nil {
			return nil, err
		}
		for i, a := range items {
			attrs := attributeValues(t, a.Values)
			out.Items[i].Attributes = &attrs
		}
	}
	return out, nil
}

func (h *Handler) CreateAsset(ctx context.Context, req api.CreateAssetRequestObject) (api.CreateAssetResponseObject, error) {
	b := req.Body
	v, err := h.svc.CreateAsset(ctx, b.Tag, service.AssetInput{
		Name: b.Name, Description: deref(b.Description), TypeID: b.AssetTypeId, StatusID: b.StatusId,
		LocationID: b.LocationId, HolderMemberID: b.HolderMemberId, PurchaseDate: fromAPIDate(b.PurchaseDate),
		Attributes: deref(b.Attributes),
	})
	if err != nil {
		return nil, err
	}
	return api.CreateAsset201JSONResponse(toAPIAsset(v)), nil
}

func (h *Handler) GetAsset(ctx context.Context, req api.GetAssetRequestObject) (api.GetAssetResponseObject, error) {
	v, err := h.svc.GetAsset(ctx, req.AssetID)
	if err != nil {
		return nil, err
	}
	return api.GetAsset200JSONResponse(toAPIAsset(v)), nil
}

func (h *Handler) UpdateAsset(ctx context.Context, req api.UpdateAssetRequestObject) (api.UpdateAssetResponseObject, error) {
	b := req.Body
	v, err := h.svc.UpdateAsset(ctx, req.AssetID, service.AssetInput{
		Name: b.Name, Description: deref(b.Description), TypeID: b.AssetTypeId, StatusID: b.StatusId,
		LocationID: b.LocationId, HolderMemberID: b.HolderMemberId, PurchaseDate: fromAPIDate(b.PurchaseDate),
		Attributes: deref(b.Attributes),
	}, b.Version)
	if err != nil {
		return nil, err
	}
	return api.UpdateAsset200JSONResponse(toAPIAsset(v)), nil
}

func (h *Handler) RetireAsset(ctx context.Context, req api.RetireAssetRequestObject) (api.RetireAssetResponseObject, error) {
	v, err := h.svc.RetireAsset(ctx, req.AssetID, deref(req.Body.Reason), req.Body.Version)
	if err != nil {
		return nil, err
	}
	return api.RetireAsset200JSONResponse(toAPIAsset(v)), nil
}

func (h *Handler) RestoreAsset(ctx context.Context, req api.RestoreAssetRequestObject) (api.RestoreAssetResponseObject, error) {
	v, err := h.svc.RestoreAsset(ctx, req.AssetID, req.Body.Version)
	if err != nil {
		return nil, err
	}
	return api.RestoreAsset200JSONResponse(toAPIAsset(v)), nil
}
