package handler

import (
	"encoding/json"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"storeit/internal/inventory/domain"
	"storeit/internal/inventory/handler/api"
	"storeit/internal/inventory/service"
)

func toAPIType(t domain.AssetType) api.AssetType {
	return api.AssetType{
		Id: t.ID, Code: t.Code, Name: t.Name, Description: t.Description, IsSystem: t.IsSystem,
		ArchivedAt: t.ArchivedAt, Version: t.Version, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
}

func toAPITypeDetail(t domain.AssetType) api.AssetTypeDetail {
	attrs := make([]api.Attribute, len(t.Attributes))
	for i, a := range t.Attributes {
		attrs[i] = toAPIAttribute(a)
	}
	return api.AssetTypeDetail{
		Id: t.ID, Code: t.Code, Name: t.Name, Description: t.Description, IsSystem: t.IsSystem,
		ArchivedAt: t.ArchivedAt, Version: t.Version, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
		Attributes: attrs,
	}
}

func toAPIAttribute(a domain.Attribute) api.Attribute {
	opts := make([]api.Option, len(a.Options))
	for i, o := range a.Options {
		opts[i] = toAPIOption(o)
	}
	var unit *string
	if a.Unit != "" {
		unit = &a.Unit
	}
	return api.Attribute{
		Id: a.ID, Key: a.Key, Label: a.Label, DataType: api.DataType(a.DataType), Unit: unit,
		IsRequired: a.Required, Position: a.Position, Removed: a.RemovedAt != nil, Options: opts,
	}
}

func toAPIOption(o domain.Option) api.Option {
	return api.Option{Id: o.ID, Label: o.Label, Position: o.Position, Removed: o.RemovedAt != nil}
}

func toAPIStatus(s domain.Status) api.Status {
	return api.Status{
		Id: s.ID, Name: s.Name, Kind: api.StatusKind(s.Kind), IsDefault: s.IsDefault, IsSystem: s.IsSystem,
		Position: s.Position, ArchivedAt: s.ArchivedAt,
	}
}

func toAPIListItem(a domain.AssetListItem) api.AssetListItem {
	return api.AssetListItem{
		Id: a.ID, Tag: a.Tag, Name: a.Name, AssetTypeId: a.TypeID, AssetTypeName: a.TypeName,
		StatusId: a.StatusID, StatusName: a.StatusName, StatusKind: api.StatusKind(a.StatusKind),
		LocationId: a.LocationID, HolderMemberId: a.HolderMemberID, PurchaseDate: toAPIDate(a.PurchaseDate),
		RetiredAt: a.RetiredAt, Version: a.Version, UpdatedAt: a.UpdatedAt,
	}
}

func toAPIAsset(v service.AssetView) api.AssetDetail {
	var reason *string
	if v.RetiredReason != "" {
		reason = &v.RetiredReason
	}
	return api.AssetDetail{
		Id: v.ID, Tag: v.Tag, Name: v.Name, Description: v.Description,
		AssetType:  api.TypeSummary{Id: v.Type.ID, Code: v.Type.Code, Name: v.Type.Name},
		Status:     api.StatusSummary{Id: v.Status.ID, Name: v.Status.Name, Kind: api.StatusKind(v.Status.Kind)},
		LocationId: v.LocationID, HolderMemberId: v.HolderMemberID, PurchaseDate: toAPIDate(v.PurchaseDate),
		RetiredAt: v.RetiredAt, RetiredReason: reason, Version: v.Version, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt,
		Attributes: attributeValues(v),
	}
}

// attributeValues: mọi thuộc tính đang hoạt động của loại, theo thứ tự hiển
// thị, kèm giá trị (null khi trống). Giá trị của thuộc tính đã gỡ không trả.
func attributeValues(v service.AssetView) []api.AttributeValue {
	byAttr := make(map[openapi_types.UUID]domain.Value, len(v.Values))
	for _, val := range v.Values {
		byAttr[val.AttributeID] = val
	}
	active := v.Type.ActiveAttributes()
	out := make([]api.AttributeValue, 0, len(active))
	for _, a := range active {
		av := api.AttributeValue{Key: a.Key, Label: a.Label, DataType: api.DataType(a.DataType)}
		if a.Unit != "" {
			av.Unit = &a.Unit
		}
		if val, ok := byAttr[a.ID]; ok {
			av.Value = jsonValue(val)
			if val.OptionID != nil {
				for _, o := range a.Options {
					if o.ID == *val.OptionID {
						label, removed := o.Label, o.RemovedAt != nil
						av.OptionLabel, av.OptionRemoved = &label, &removed
					}
				}
			}
		}
		out = append(out, av)
	}
	return out
}

// jsonValue: số trả dạng số JSON (giữ đúng chữ số đã lưu), ngày dạng YYYY-MM-DD
func jsonValue(v domain.Value) any {
	switch {
	case v.Text != nil:
		return *v.Text
	case v.Number != nil:
		return json.Number(*v.Number)
	case v.Date != nil:
		return v.Date.Format(time.DateOnly)
	case v.Bool != nil:
		return *v.Bool
	case v.OptionID != nil:
		return v.OptionID.String()
	}
	return nil
}

func toAPIDate(t *time.Time) *openapi_types.Date {
	if t == nil {
		return nil
	}
	return &openapi_types.Date{Time: *t}
}

func fromAPIDate(d *openapi_types.Date) *time.Time {
	if d == nil {
		return nil
	}
	t := d.Time
	return &t
}
