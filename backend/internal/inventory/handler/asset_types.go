package handler

import (
	"context"

	"storeit/internal/inventory/domain"
	"storeit/internal/inventory/handler/api"
)

func (h *Handler) ListAssetTypes(ctx context.Context, req api.ListAssetTypesRequestObject) (api.ListAssetTypesResponseObject, error) {
	types, err := h.svc.ListAssetTypes(ctx, deref(req.Params.IncludeArchived))
	if err != nil {
		return nil, err
	}
	out := api.ListAssetTypes200JSONResponse{Items: make([]api.AssetType, len(types))}
	for i, t := range types {
		out.Items[i] = toAPIType(t)
	}
	if deref(req.Params.WithCounts) {
		sums, err := h.svc.TypeSummaries(ctx)
		if err != nil {
			return nil, err
		}
		for i, t := range types {
			sum := sums[t.ID]
			total, nAttrs, labels := sum.Counts.Total, int32(len(sum.AttributeLabels)), sum.AttributeLabels
			if labels == nil {
				labels = []string{}
			}
			out.Items[i].AssetCount = &total
			out.Items[i].KindCounts = &api.KindCounts{
				Available:   sum.Counts.ByKind[domain.KindAvailable],
				InUse:       sum.Counts.ByKind[domain.KindInUse],
				Unavailable: sum.Counts.ByKind[domain.KindUnavailable],
			}
			out.Items[i].AttributeCount = &nAttrs
			out.Items[i].AttributeLabels = &labels
		}
	}
	return out, nil
}

func (h *Handler) CreateAssetType(ctx context.Context, req api.CreateAssetTypeRequestObject) (api.CreateAssetTypeResponseObject, error) {
	in := domain.NewAssetType{Code: req.Body.Code, Name: req.Body.Name, Description: deref(req.Body.Description)}
	for _, a := range deref(req.Body.Attributes) {
		in.Attributes = append(in.Attributes, toNewAttribute(a))
	}
	t, err := h.svc.CreateAssetType(ctx, in)
	if err != nil {
		return nil, err
	}
	return api.CreateAssetType201JSONResponse(toAPITypeDetail(t)), nil
}

func (h *Handler) GetAssetType(ctx context.Context, req api.GetAssetTypeRequestObject) (api.GetAssetTypeResponseObject, error) {
	t, err := h.svc.GetAssetType(ctx, req.TypeID)
	if err != nil {
		return nil, err
	}
	return api.GetAssetType200JSONResponse(toAPITypeDetail(t)), nil
}

func (h *Handler) UpdateAssetType(ctx context.Context, req api.UpdateAssetTypeRequestObject) (api.UpdateAssetTypeResponseObject, error) {
	t, err := h.svc.UpdateAssetType(ctx, req.TypeID, req.Body.Name, req.Body.Description, req.Body.Version)
	if err != nil {
		return nil, err
	}
	return api.UpdateAssetType200JSONResponse(toAPITypeDetail(t)), nil
}

func (h *Handler) ArchiveAssetType(ctx context.Context, req api.ArchiveAssetTypeRequestObject) (api.ArchiveAssetTypeResponseObject, error) {
	t, err := h.svc.ArchiveAssetType(ctx, req.TypeID)
	if err != nil {
		return nil, err
	}
	return api.ArchiveAssetType200JSONResponse(toAPITypeDetail(t)), nil
}

func (h *Handler) RestoreAssetType(ctx context.Context, req api.RestoreAssetTypeRequestObject) (api.RestoreAssetTypeResponseObject, error) {
	t, err := h.svc.RestoreAssetType(ctx, req.TypeID)
	if err != nil {
		return nil, err
	}
	return api.RestoreAssetType200JSONResponse(toAPITypeDetail(t)), nil
}

func (h *Handler) AddAttribute(ctx context.Context, req api.AddAttributeRequestObject) (api.AddAttributeResponseObject, error) {
	a, err := h.svc.AddAttribute(ctx, req.TypeID, toNewAttribute(*req.Body))
	if err != nil {
		return nil, err
	}
	return api.AddAttribute201JSONResponse(toAPIAttribute(a)), nil
}

func (h *Handler) UpdateAttribute(ctx context.Context, req api.UpdateAttributeRequestObject) (api.UpdateAttributeResponseObject, error) {
	ch := domain.AttributeChange{
		Label: req.Body.Label, Unit: req.Body.Unit, Required: req.Body.IsRequired, Position: req.Body.Position,
	}
	if req.Body.DataType != nil {
		dt := domain.DataType(*req.Body.DataType)
		ch.DataType = &dt
	}
	a, err := h.svc.UpdateAttribute(ctx, req.TypeID, req.AttributeID, ch)
	if err != nil {
		return nil, err
	}
	return api.UpdateAttribute200JSONResponse(toAPIAttribute(a)), nil
}

func (h *Handler) RemoveAttribute(ctx context.Context, req api.RemoveAttributeRequestObject) (api.RemoveAttributeResponseObject, error) {
	if err := h.svc.RemoveAttribute(ctx, req.TypeID, req.AttributeID); err != nil {
		return nil, err
	}
	return api.RemoveAttribute204Response{}, nil
}

func (h *Handler) ReorderAttributes(ctx context.Context, req api.ReorderAttributesRequestObject) (api.ReorderAttributesResponseObject, error) {
	t, err := h.svc.ReorderAttributes(ctx, req.TypeID, req.Body.Ids)
	if err != nil {
		return nil, err
	}
	return api.ReorderAttributes200JSONResponse(toAPITypeDetail(t)), nil
}

func (h *Handler) ReorderOptions(ctx context.Context, req api.ReorderOptionsRequestObject) (api.ReorderOptionsResponseObject, error) {
	a, err := h.svc.ReorderOptions(ctx, req.TypeID, req.AttributeID, req.Body.Ids)
	if err != nil {
		return nil, err
	}
	return api.ReorderOptions200JSONResponse(toAPIAttribute(a)), nil
}

func (h *Handler) AddOption(ctx context.Context, req api.AddOptionRequestObject) (api.AddOptionResponseObject, error) {
	o, err := h.svc.AddOption(ctx, req.TypeID, req.AttributeID, req.Body.Label, deref(req.Body.Position))
	if err != nil {
		return nil, err
	}
	return api.AddOption201JSONResponse(toAPIOption(o)), nil
}

func (h *Handler) UpdateOption(ctx context.Context, req api.UpdateOptionRequestObject) (api.UpdateOptionResponseObject, error) {
	o, err := h.svc.UpdateOption(ctx, req.TypeID, req.AttributeID, req.OptionID, req.Body.Label, req.Body.Position)
	if err != nil {
		return nil, err
	}
	return api.UpdateOption200JSONResponse(toAPIOption(o)), nil
}

func (h *Handler) RemoveOption(ctx context.Context, req api.RemoveOptionRequestObject) (api.RemoveOptionResponseObject, error) {
	if err := h.svc.RemoveOption(ctx, req.TypeID, req.AttributeID, req.OptionID); err != nil {
		return nil, err
	}
	return api.RemoveOption204Response{}, nil
}

func toNewAttribute(a api.NewAttribute) domain.NewAttribute {
	return domain.NewAttribute{
		Key: a.Key, Label: a.Label, DataType: domain.DataType(a.DataType), Unit: deref(a.Unit),
		Required: deref(a.IsRequired), Position: deref(a.Position), Options: deref(a.Options),
	}
}
