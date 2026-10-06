package handler

import (
	"bytes"
	"context"
	"mime"
	"strings"

	"github.com/google/uuid"

	"storeit/internal/inventory/domain"
	"storeit/internal/inventory/handler/api"
	"storeit/internal/inventory/service"
)

// ExportAssets tải .xlsx; quyền kiểm tra trong service
func (h *Handler) ExportAssets(ctx context.Context, req api.ExportAssetsRequestObject) (api.ExportAssetsResponseObject, error) {
	b := req.Body
	in := service.ExportRequest{Mode: service.ExportMode(b.Mode), ProfileID: b.ProfileId}
	if b.Filters != nil {
		p := b.Filters
		in.Filter = domain.AssetFilter{
			Query: deref(p.Q), TypeID: p.TypeId, StatusID: p.StatusId, IncludeRetired: deref(p.IncludeRetired),
			Attrs: deref(p.Attr), Sort: domain.AssetSort(deref(p.Sort)),
		}
		if p.StatusKind != nil {
			k := domain.StatusKind(*p.StatusKind)
			in.Filter.StatusKind = &k
		}
		if p.Ids != nil {
			in.IDs = append([]uuid.UUID{}, (*p.Ids)...)
		}
	}
	if b.Layout != nil {
		l := toDomainLayout(*b.Layout)
		in.Layout = &l
	}
	file, err := h.svc.Export(ctx, in)
	if err != nil {
		return nil, err
	}
	cd := mime.FormatMediaType("attachment", map[string]string{"filename": file.Name})
	skipped := strings.Join(file.Skipped, ",")
	return api.ExportAssets200ApplicationvndOpenxmlformatsOfficedocumentSpreadsheetmlSheetResponse{
		Body:          bytes.NewReader(file.Data),
		ContentLength: int64(len(file.Data)),
		Headers: api.ExportAssets200ResponseHeaders{
			ContentDisposition:    &cd,
			XExportSkippedColumns: &skipped,
		},
	}, nil
}

func toDomainLayout(l api.ExportLayout) domain.ExportLayout {
	out := domain.ExportLayout{
		Sheets: domain.SheetMode(l.Sheets), EachTypeAttrs: deref(l.EachTypeAttrs), SheetName: l.SheetName,
		TitleRow: l.TitleRow, Summary: l.Summary, Header: domain.HeaderStyle(l.Header), Freeze: l.Freeze,
		Filter: l.Filter, Stripes: l.Stripes, DateFormat: domain.DateFormat(l.DateFormat),
		BoolStyle: domain.BoolStyle(l.BoolStyle), StatusAs: domain.StatusAs(l.StatusAs), UnitIn: domain.UnitIn(l.UnitIn),
		Sort: deref(l.Sort),
	}
	for _, c := range l.Columns {
		out.Columns = append(out.Columns, domain.ExportColumn{Field: c.Field, Header: deref(c.Header), Width: float64(deref(c.Width))})
	}
	return out
}

func toAPILayout(l domain.ExportLayout) api.ExportLayout {
	cols := make([]api.ExportColumn, len(l.Columns))
	for i, c := range l.Columns {
		header, width := c.Header, float32(c.Width)
		cols[i] = api.ExportColumn{Field: c.Field, Header: &header, Width: &width}
	}
	each, sort := l.EachTypeAttrs, l.Sort
	return api.ExportLayout{
		Columns: cols, Sheets: api.ExportLayoutSheets(l.Sheets), EachTypeAttrs: &each, SheetName: l.SheetName,
		TitleRow: l.TitleRow, Summary: l.Summary, Header: api.ExportLayoutHeader(l.Header), Freeze: l.Freeze,
		Filter: l.Filter, Stripes: l.Stripes, DateFormat: api.ExportLayoutDateFormat(l.DateFormat),
		BoolStyle: api.ExportLayoutBoolStyle(l.BoolStyle), StatusAs: api.ExportLayoutStatusAs(l.StatusAs),
		UnitIn: api.ExportLayoutUnitIn(l.UnitIn), Sort: &sort,
	}
}
