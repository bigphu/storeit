package service

import (
	"bytes"
	"errors"
	"math"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"storeit/internal/inventory/domain"
	"storeit/internal/platform/errs"
)

func open(t *testing.T, f ExportFile) *excelize.File {
	t.Helper()
	x, err := excelize.OpenReader(bytes.NewReader(f.Data))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = x.Close() })
	return x
}

var exporter = as(domain.PermAssetRead, domain.PermAssetExport, domain.PermAssetManage, domain.PermTypeManage)

func TestExport_Permissions(t *testing.T) {
	e := newEnv()
	if _, err := e.svc.Export(as(domain.PermAssetRead), ExportRequest{Mode: ExportData}); status(err) != 403 {
		t.Errorf("without export: %v", err)
	}
	if _, err := e.svc.Export(as(domain.PermAssetExport), ExportRequest{Mode: ExportData}); status(err) != 403 {
		t.Errorf("without read: %v", err)
	}
}

func TestExport_DataPerTypeWithKeyHeaders(t *testing.T) {
	e := newEnv()
	lap := e.laptop(t)
	e.mustAsset(t, lap, "LAP-1")
	e.mustAsset(t, lap, "LAP-2")
	gen := e.generalAsset(t, "GEN-1")
	_ = gen

	f, err := e.svc.Export(exporter, ExportRequest{Mode: ExportData})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(f.Name, "storeit-assets-") || f.Rows != 3 {
		t.Errorf("file = %s, rows %d", f.Name, f.Rows)
	}
	x := open(t, f)
	sheets := x.GetSheetList()
	if len(sheets) != 2 || sheets[0] != "GENERAL" || sheets[1] != lap.Code {
		t.Fatalf("sheets = %v, want one per type code in name order", sheets)
	}
	rows, _ := x.GetRows(lap.Code)
	if rows[0][0] != "tag" || !slices.Contains(rows[0], "attr:serial") || len(rows) != 3 {
		t.Errorf("laptop sheet = %v", rows)
	}
	if len(e.profiles.records) != 1 || e.profiles.records[0].Mode != "data" {
		t.Errorf("export event = %+v", e.profiles.records)
	}
}

func TestExport_SelectedIDsPerType(t *testing.T) {
	e := newEnv()
	lap := e.laptop(t)
	a := e.mustAsset(t, lap, "LAP-1")
	e.mustAsset(t, lap, "LAP-2")
	g := e.generalAsset(t, "GEN-1")
	f, err := e.svc.Export(exporter, ExportRequest{Mode: ExportData, IDs: []uuid.UUID{a.ID, g.ID}})
	if err != nil {
		t.Fatal(err)
	}
	x := open(t, f)
	rows, _ := x.GetRows(lap.Code)
	if len(rows) != 2 || rows[1][0] != "LAP-1" {
		t.Errorf("laptop sheet must hold only the selected laptop: %v", rows)
	}
	if _, err := e.svc.Export(exporter, ExportRequest{Mode: ExportData, IDs: []uuid.UUID{}}); !errors.Is(err, domain.ErrInvalidExportIDs) {
		t.Errorf("empty selection: %v", err)
	}
}

func TestExport_ReportLayoutAndTooLarge(t *testing.T) {
	e := newEnv()
	lap := e.laptop(t)
	e.mustAsset(t, lap, "LAP-1")
	l := domain.DefaultReportLayout()
	l.TitleRow, l.SheetName, l.Summary = true, "Laptops", true
	l.Columns = []domain.ExportColumn{{Field: "tag", Header: "Asset tag"}, {Field: "attr:serial"}}
	f, err := e.svc.Export(exporter, ExportRequest{Mode: ExportReport, Layout: &l})
	if err != nil {
		t.Fatal(err)
	}
	x := open(t, f)
	if s := x.GetSheetList(); len(s) != 2 || s[0] != "Laptops" || s[1] != "Summary" {
		t.Errorf("sheets = %v", s)
	}
	if v, _ := x.GetCellValue("Laptops", "A3"); v != "Asset tag" {
		t.Errorf("renamed header below two title rows = %q", v)
	}

	e.svc.exportMaxRows = 0
	if _, err := e.svc.Export(exporter, ExportRequest{Mode: ExportData}); !errors.Is(err, domain.ErrExportTooLarge) {
		t.Errorf("over the cap: %v", err)
	}
}

func TestExport_SkipsRemovedAttribute(t *testing.T) {
	e := newEnv()
	lap := e.laptop(t)
	e.mustAsset(t, lap, "LAP-1")
	owner := uuid.New()
	ctx := actorWith(owner, domain.PermAssetRead, domain.PermAssetExport)
	l := domain.DefaultReportLayout()
	l.Columns = []domain.ExportColumn{{Field: "tag"}, {Field: "attr:warranty_until"}}
	p, err := e.svc.CreateExportProfile(ctx, ExportProfileInput{Name: "Warranty", Layout: l})
	if err != nil {
		t.Fatal(err)
	}
	f, err := e.svc.Export(ctx, ExportRequest{Mode: ExportReport, ProfileID: &p.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Skipped) != 1 || f.Skipped[0] != "warranty_until" || !strings.HasPrefix(f.Name, "warranty-") {
		t.Errorf("file = %+v", f)
	}
	other := actorWith(uuid.New(), domain.PermAssetRead, domain.PermAssetExport)
	if _, err := e.svc.Export(other, ExportRequest{Mode: ExportReport, ProfileID: &p.ID}); !errors.Is(err, domain.ErrExportProfileNotFound) {
		t.Errorf("someone else's private profile: %v", err)
	}
}

func sortLayout(sort string, mode domain.SheetMode) domain.ExportLayout {
	l := domain.DefaultReportLayout()
	l.Sheets, l.Sort = mode, sort
	l.Columns = []domain.ExportColumn{{Field: "tag"}}
	return l
}

func TestExport_LayoutAttributeSort(t *testing.T) {
	e := newEnv()
	lap := e.laptop(t)
	a := e.mustAsset(t, lap, "LAP-1")
	e.generalAsset(t, "GEN-1")

	// per_type, không type_id: loại laptop giải được sort theo serial (chọn riêng laptop
	// vì loại General không có serial và đúng ra bị bỏ ở sheet của nó)
	l := sortLayout("-attributes.serial", domain.SheetPerType)
	f, err := e.svc.Export(exporter, ExportRequest{Mode: ExportReport, Layout: &l, IDs: []uuid.UUID{a.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(f.Skipped, "serial") {
		t.Errorf("per-type sort must resolve: skipped %v", f.Skipped)
	}

	// single, không type_id: không có một loại duy nhất nên bỏ qua sort, không lỗi
	l = sortLayout("-attributes.serial", domain.SheetSingle)
	f, err = e.svc.Export(exporter, ExportRequest{Mode: ExportReport, Layout: &l})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(f.Skipped, "serial") {
		t.Errorf("single sheet without type must skip the sort: %v", f.Skipped)
	}

	// key không còn tồn tại trên loại
	l = sortLayout("attributes.gone", domain.SheetSingle)
	f, err = e.svc.Export(exporter, ExportRequest{Mode: ExportReport, Layout: &l, Filter: domain.AssetFilter{TypeID: &lap.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(f.Skipped, "gone") || f.Rows != 1 {
		t.Errorf("missing attribute: skipped %v rows %d", f.Skipped, f.Rows)
	}
}

func TestExport_RequestAttributeSortStaysStrict(t *testing.T) {
	e := newEnv()
	lap := e.laptop(t)
	e.mustAsset(t, lap, "LAP-1")
	_, err := e.svc.Export(exporter, ExportRequest{Mode: ExportData, Filter: domain.AssetFilter{Sort: "attributes.serial"}})
	if status(err) != 422 {
		t.Errorf("request attribute sort without type_id: %v", err)
	}
}

func updatedCell(t *testing.T, e *env, tz string) float64 {
	t.Helper()
	l := domain.DefaultReportLayout()
	l.Columns = []domain.ExportColumn{{Field: "updated_at"}}
	f, err := e.svc.Export(exporter, ExportRequest{Mode: ExportReport, Layout: &l, TZ: tz})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := open(t, f).GetCellValue("Assets", "A2", excelize.Options{RawCellValue: true})
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		t.Fatalf("raw %q: %v", raw, err)
	}
	return n
}

func TestExport_TimeZoneShiftsUpdatedAt(t *testing.T) {
	e := newEnv()
	e.mustAsset(t, e.laptop(t), "LAP-1")
	utc, empty, hcm := updatedCell(t, e, "UTC"), updatedCell(t, e, ""), updatedCell(t, e, "Asia/Ho_Chi_Minh")
	if utc != empty {
		t.Errorf("empty tz = %v, want UTC %v", empty, utc)
	}
	if h := (hcm - utc) * 24; math.Abs(h-7) > 0.02 {
		t.Errorf("Ho Chi Minh is %.3f hours after UTC, want 7", h)
	}
}

func TestExport_InvalidTimeZone(t *testing.T) {
	e := newEnv()
	_, err := e.svc.Export(exporter, ExportRequest{Mode: ExportData, TZ: "Mars/Olympus"})
	var pe *errs.Error
	if !errors.Is(err, domain.ErrInvalidTimeZone) || status(err) != 422 || !errors.As(err, &pe) || pe.Fields()[0].Field != "tz" {
		t.Errorf("err = %v, want invalid-time-zone with field tz", err)
	}
}

func TestExport_InvalidIDsHasDetail(t *testing.T) {
	e := newEnv()
	_, err := e.svc.Export(exporter, ExportRequest{Mode: ExportData, IDs: []uuid.UUID{}})
	var pe *errs.Error
	if !errors.As(err, &pe) || pe.Detail() == "" {
		t.Errorf("err = %v, want a readable detail", err)
	}
}

func TestExportFileName(t *testing.T) {
	now := time.Date(2026, 10, 6, 1, 0, 0, 0, time.UTC)
	nfd := &domain.ExportProfile{Name: "Kiểm kê"}
	if got := exportFileName(ExportReport, nfd, now); got != "kiểm-kê-2026-10-06.xlsx" {
		t.Errorf("NFD name = %q", got)
	}
	if got := exportFileName(ExportData, nil, now); got != "storeit-assets-2026-10-06.xlsx" {
		t.Errorf("data = %q", got)
	}
}

func TestAttrFilterText_SelectLabels(t *testing.T) {
	win, mac := uuid.New(), uuid.New()
	attr := domain.Attribute{ID: uuid.New(), Label: "OS", DataType: domain.TypeSelect,
		Options: []domain.Option{{ID: win, Label: "Windows"}, {ID: mac, Label: "macOS"}}}
	types := []domain.AssetType{{Attributes: []domain.Attribute{attr}}}
	eq := attrFilterText(types, domain.AttrFilter{AttributeID: attr.ID, DataType: domain.TypeSelect, Op: domain.OpEq, Value: win.String()})
	in := attrFilterText(types, domain.AttrFilter{AttributeID: attr.ID, DataType: domain.TypeSelect, Op: domain.OpIn, Value: win.String() + "," + mac.String()})
	if eq != "OS = Windows" || in != "OS in Windows, macOS" {
		t.Errorf("eq = %q, in = %q", eq, in)
	}
}
