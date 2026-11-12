package handler

import (
	"bytes"
	"context"
	"io"
	"mime"

	"storeit/internal/inventory/domain"
	"storeit/internal/inventory/handler/api"
	"storeit/internal/inventory/service"
)

// ImportAssets xem trước hay xác nhận import tài sản từ file .xlsx (thân request là file); quyền
// kiểm tra trong service. Đọc tối đa MaxImportBytes+1 byte để service báo file quá lớn.
func (h *Handler) ImportAssets(ctx context.Context, req api.ImportAssetsRequestObject) (api.ImportAssetsResponseObject, error) {
	data, err := io.ReadAll(io.LimitReader(req.Body, domain.MaxImportBytes+1))
	if err != nil {
		return nil, err
	}
	name := deref(req.Params.FileName)
	var rep service.ImportReport
	if req.Params.Mode == api.ImportAssetsParamsMode("commit") {
		rep, err = h.svc.CommitImport(ctx, name, data)
	} else {
		rep, err = h.svc.PreviewImport(ctx, name, data)
	}
	if err != nil {
		return nil, err
	}
	return api.ImportAssets200JSONResponse(toAPIImportReport(rep)), nil
}

// DownloadImportTemplate tải file mẫu import
func (h *Handler) DownloadImportTemplate(ctx context.Context, _ api.DownloadImportTemplateRequestObject) (api.DownloadImportTemplateResponseObject, error) {
	data, err := h.svc.ImportTemplate(ctx)
	if err != nil {
		return nil, err
	}
	cd := mime.FormatMediaType("attachment", map[string]string{"filename": "storeit-import-template.xlsx"})
	return api.DownloadImportTemplate200ApplicationvndOpenxmlformatsOfficedocumentSpreadsheetmlSheetResponse{
		Body:          bytes.NewReader(data),
		ContentLength: int64(len(data)),
		Headers:       api.DownloadImportTemplate200ResponseHeaders{ContentDisposition: &cd},
	}, nil
}

func toAPIImportReport(r service.ImportReport) api.ImportReport {
	out := api.ImportReport{FileName: r.FileName, Committed: r.Committed}
	out.Counts.Create, out.Counts.Update = int32(r.Counts.Create), int32(r.Counts.Update)
	out.Counts.Skip, out.Counts.Fail = int32(r.Counts.Skip), int32(r.Counts.Fail)
	out.Sheets = make([]struct {
		Name     string `json:"name"`
		Rows     int32  `json:"rows"`
		TypeCode string `json:"type_code"`
	}, len(r.Sheets))
	for i, s := range r.Sheets {
		out.Sheets[i].Name, out.Sheets[i].TypeCode, out.Sheets[i].Rows = s.Name, s.TypeCode, int32(s.Rows)
	}
	out.Rows = make([]api.ImportRow, len(r.Rows))
	for i, row := range r.Rows {
		out.Rows[i] = api.ImportRow{
			Sheet: row.Sheet, Row: int32(row.Row), Tag: row.Tag, Name: row.Name,
			Result: api.ImportRowResult(row.Action), Message: row.Message, AssetId: row.AssetID,
		}
	}
	return out
}
