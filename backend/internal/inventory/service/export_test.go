package service

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"storeit/internal/inventory/domain"
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
