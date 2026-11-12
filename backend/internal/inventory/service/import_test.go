package service

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"storeit/internal/inventory/domain"
	"storeit/internal/inventory/spreadsheet"
	"storeit/internal/platform/errs"
)

// xlsx dựng một workbook; ô là string, số hay nil (ô trống)
func xlsx(t *testing.T, order []string, sheets map[string][][]any) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	for i, name := range order {
		if i == 0 {
			_ = f.SetSheetName("Sheet1", name)
		} else if _, err := f.NewSheet(name); err != nil {
			t.Fatal(err)
		}
		for r, row := range sheets[name] {
			cell, _ := excelize.CoordinatesToCellName(1, r+1)
			if err := f.SetSheetRow(name, cell, &row); err != nil {
				t.Fatal(err)
			}
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

var importHeader = []any{"tag", "name", "status", "attr:serial", "attr:ram_gb", "attr:os"}

// importEnv: một loại Laptop (serial bắt buộc, ram_gb, os) và vài tài sản có sẵn
type importEnv struct {
	*env
	typ     domain.AssetType
	options map[string]uuid.UUID
}

func newImportEnv(t *testing.T) *importEnv {
	t.Helper()
	e := newEnv()
	typ := e.laptop(t)
	ie := &importEnv{env: e, typ: typ, options: map[string]uuid.UUID{}}
	for _, a := range typ.Attributes {
		for _, o := range a.Options {
			ie.options[o.Label] = o.ID
		}
	}
	return ie
}

// asset tạo một tài sản có sẵn của loại Laptop
func (ie *importEnv) asset(t *testing.T, tag, name, serial string, ram float64) AssetView {
	t.Helper()
	v, err := ie.svc.CreateAsset(manager, tag, AssetInput{Name: name, TypeID: ie.typ.ID, Attributes: map[string]any{
		"serial": serial, "ram_gb": ram, "os": ie.options["Windows"].String(),
	}})
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func rowOf(rep ImportReport, sheet string, row int) ImportRowResult {
	for _, r := range rep.Rows {
		if r.Sheet == sheet && r.Row == row {
			return r
		}
	}
	return ImportRowResult{Message: "ROW NOT FOUND"}
}

func TestPreviewImportDecidesEveryRow(t *testing.T) {
	ie := newImportEnv(t)
	ie.asset(t, "LAP-0001", "MacBook Air", "S1", 8)
	ie.asset(t, "LAP-0002", "Dell Latitude", "S2", 8)
	ie.asset(t, "LAP-0003", "ThinkPad", "S3", 8)
	ie.asset(t, "LAP-0004", "HP EliteBook", "S4", 8)
	old := ie.asset(t, "LAP-0099", "Old laptop", "S99", 8)
	if _, err := ie.svc.RetireAsset(manager, old.ID, "broken", old.Version); err != nil {
		t.Fatal(err)
	}
	loaned := ie.asset(t, "LAP-0005", "On loan one", "S5", 8)
	a := ie.assets.assets[loaned.ID]
	a.StatusID = domain.OnLoanStatusID
	ie.assets.assets[loaned.ID] = a
	mon, err := ie.svc.CreateAssetType(manager, domain.NewAssetType{Code: "MONITOR", Name: "Monitor"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ie.svc.CreateAsset(manager, "MON-0001", AssetInput{Name: "Dell P2422H", TypeID: mon.ID}); err != nil {
		t.Fatal(err)
	}
	before := len(ie.assets.assets)

	data := xlsx(t, []string{"LAPTOP"}, map[string][][]any{"LAPTOP": {
		importHeader,
		{"LAP-0001", "MacBook Air", nil, nil, 16, nil},              // 2 update: RAM 8 → 16
		{"lap-0002", "Dell Latitude", nil, "S2", 8, "Windows"},      // 3 skip: nothing differs
		{nil, "ThinkPad T14", nil, "S9", 16, "macOS"},               // 4 create, generated tag
		{"LAP-0500", "Acer Swift", "Under repair", "S10", nil, nil}, // 5 create with its own tag
		{nil, "No serial", nil, nil, 8, nil},                        // 6 fail: serial required
		{"LAP-0033", "Twin A", nil, "T1", nil, nil},                 // 7 fail: tag twice
		{"lap-0033", "Twin B", nil, "T2", nil, nil},                 // 8 fail: tag twice
		{"LAP-0099", "Old laptop", nil, nil, 16, nil},               // 9 fail: retired
		{"LAP-0003", nil, "Nope", nil, nil, nil},                    // 10 fail: no such status
		{"LAP-0004", nil, nil, nil, "sixteen", nil},                 // 11 fail: not a number
		{"bad tag!", "x", nil, "S", nil, nil},                       // 12 fail: bad tag
		{"MON-0001", "Dell P2422H", nil, "S", nil, nil},             // 13 fail: other type
		{"LAP-0005", nil, "Available", nil, nil, nil},               // 14 fail: on loan
		{nil, "Retired status", "Retired", "S", nil, nil},           // 15 fail: needs Retire
	}})
	rep, err := ie.svc.PreviewImport(manager, "assets.xlsx", data)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Committed || rep.FileName != "assets.xlsx" || len(rep.Sheets) != 1 || rep.Sheets[0].TypeCode != "LAPTOP" || rep.Sheets[0].Rows != 14 {
		t.Fatalf("report header = %+v", rep)
	}
	want := map[int]struct {
		action domain.ImportAction
		msg    string
	}{
		2:  {domain.ImportUpdate, "RAM 8 → 16 GB"},
		3:  {domain.ImportSkip, "No changes"},
		4:  {domain.ImportCreate, "New asset. Tag is given on import"},
		5:  {domain.ImportCreate, "New asset with tag LAP-0500"},
		6:  {domain.ImportFail, "Serial is required"},
		7:  {domain.ImportFail, "Tag LAP-0033 is also in row 8"},
		8:  {domain.ImportFail, "Tag LAP-0033 is also in row 7"},
		9:  {domain.ImportFail, "Retired asset. Restore it before importing changes"},
		10: {domain.ImportFail, "Status: “Nope” doesn't exist. Use one of:"},
		11: {domain.ImportFail, "RAM: “sixteen” is not a number"},
		12: {domain.ImportFail, "Tag “bad tag!” isn't valid"},
		13: {domain.ImportFail, "This tag belongs to type Monitor, not Laptop"},
		14: {domain.ImportFail, "On loan: status can't change. Return it first"},
		15: {domain.ImportFail, "retire"},
	}
	for row, w := range want {
		got := rowOf(rep, "LAPTOP", row)
		if got.Action != w.action || !strings.Contains(strings.ToLower(got.Message), strings.ToLower(w.msg)) {
			t.Errorf("row %d = %s %q, want %s containing %q", row, got.Action, got.Message, w.action, w.msg)
		}
	}
	if rep.Counts != (domain.ImportCounts{Create: 2, Update: 1, Skip: 1, Fail: 10}) {
		t.Errorf("counts = %+v", rep.Counts)
	}
	if len(ie.assets.assets) != before || len(ie.imports.records) != 0 {
		t.Error("a preview must not write anything")
	}
}

func TestCommitImportSavesEachRowOnItsOwn(t *testing.T) {
	ie := newImportEnv(t)
	ie.asset(t, "LAP-0001", "MacBook Air", "S1", 8)
	data := xlsx(t, []string{"LAPTOP"}, map[string][][]any{"LAPTOP": {
		importHeader,
		{"LAP-0001", nil, nil, nil, 16, nil},
		{nil, "ThinkPad T14", nil, "S9", 16, "macOS"},
		{"LAP-0500", "Acer Swift", nil, "S10", nil, nil},
		{nil, "No serial", nil, nil, 8, nil},
	}})
	before := len(ie.assets.assets)
	rep, err := ie.svc.CommitImport(manager, "assets.xlsx", data)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Committed || rep.Counts != (domain.ImportCounts{Create: 2, Update: 1, Fail: 1}) {
		t.Fatalf("report = %+v", rep)
	}
	if got := len(ie.assets.assets) - before; got != 2 {
		t.Errorf("assets created = %d, want 2 (the failed row is left out)", got)
	}
	gen := rowOf(rep, "LAPTOP", 3)
	if gen.AssetID == nil || gen.Tag != "AUTO-0002" || gen.Message != "Created" {
		t.Errorf("generated row = %+v: the report must carry the tag the repository gave", gen)
	}
	if upd := rowOf(rep, "LAPTOP", 2); upd.AssetID == nil || !strings.HasPrefix(upd.Message, "Updated: RAM 8 → 16 GB") {
		t.Errorf("update row = %+v", upd)
	}
	for _, a := range ie.assets.assets {
		if a.Tag == "LAP-0001" && (a.Version != 2 || a.Name != "MacBook Air") {
			t.Errorf("updated asset = version %d name %q: blank cells keep the current values", a.Version, a.Name)
		}
	}
	if len(ie.imports.records) != 1 || !ie.imports.records[0].HasCreates || ie.imports.records[0].Rows != 4 {
		t.Fatalf("history = %+v", ie.imports.records)
	}
	if c := ie.imports.counts[ie.imports.records[0].ID]; c != rep.Counts {
		t.Errorf("history counts = %+v, want %+v", c, rep.Counts)
	}
}

func TestCommitImportRefusesTheSameFileTwice(t *testing.T) {
	ie := newImportEnv(t)
	data := xlsx(t, []string{"LAPTOP"}, map[string][][]any{"LAPTOP": {importHeader, {nil, "ThinkPad T14", nil, "S9", 16, nil}}})
	if _, err := ie.svc.CommitImport(manager, "a.xlsx", data); err != nil {
		t.Fatal(err)
	}
	n := len(ie.assets.assets)
	if _, err := ie.svc.CommitImport(manager, "a.xlsx", data); !errors.Is(err, domain.ErrImportAlreadyDone) {
		t.Fatalf("second commit: %v, want ErrImportAlreadyDone", err)
	}
	if len(ie.assets.assets) != n {
		t.Error("the refused import created assets anyway")
	}

	// file không tạo gì thì chạy lại không có rủi ro
	ie.asset(t, "LAP-0001", "MacBook Air", "S1", 8)
	upd := xlsx(t, []string{"LAPTOP"}, map[string][][]any{"LAPTOP": {importHeader, {"LAP-0001", nil, nil, nil, 16, nil}}})
	for i := 0; i < 2; i++ {
		if _, err := ie.svc.CommitImport(manager, "u.xlsx", upd); err != nil {
			t.Fatalf("update-only commit %d: %v", i+1, err)
		}
	}
}

func TestCommitImportReportsARowThatChangedMeanwhile(t *testing.T) {
	ie := newImportEnv(t)
	ie.asset(t, "LAP-0001", "MacBook Air", "S1", 8)
	ie.assets.replaceErr = domain.ErrAssetChanged
	data := xlsx(t, []string{"LAPTOP"}, map[string][][]any{"LAPTOP": {importHeader, {"LAP-0001", nil, nil, nil, 16, nil}, {nil, "New", nil, "S", nil, nil}}})
	rep, err := ie.svc.CommitImport(manager, "a.xlsx", data)
	if err != nil {
		t.Fatal(err)
	}
	if r := rowOf(rep, "LAPTOP", 2); r.Action != domain.ImportFail || !strings.Contains(r.Message, "Changed by someone else") {
		t.Errorf("row = %+v", r)
	}
	if rep.Counts != (domain.ImportCounts{Create: 1, Fail: 1}) {
		t.Errorf("counts = %+v: the other row must still be saved", rep.Counts)
	}
}

func TestImportFileProblems(t *testing.T) {
	ie := newImportEnv(t)
	var many [][]any
	many = append(many, importHeader)
	for i := 0; i < domain.MaxImportRows+1; i++ {
		many = append(many, []any{nil, "X", nil, "S", nil, nil})
	}
	cases := map[string]struct {
		data   []byte
		want   error
		field  string
		detail string
	}{
		"csv":         {[]byte("tag,name\nA,B\n"), domain.ErrInvalidImportFile, "file", "Excel workbook"},
		"unknown":     {xlsx(t, []string{"Sheet1"}, map[string][][]any{"Sheet1": {importHeader, {nil, "x"}}}), domain.ErrInvalidImportFile, "Sheet1", "isn't an asset type code. Rename the sheet to one of: GENERAL, LAPTOP"},
		"no name":     {xlsx(t, []string{"LAPTOP"}, map[string][][]any{"LAPTOP": {{"tag", "attr:serial"}, {nil, "S"}}}), domain.ErrInvalidImportFile, "LAPTOP", `column "name" is missing`},
		"odd column":  {xlsx(t, []string{"LAPTOP"}, map[string][][]any{"LAPTOP": {{"tag", "name", "Asset name"}, {nil, "x", "y"}}}), domain.ErrInvalidImportFile, "LAPTOP", `unknown column "Asset name"`},
		"no rows":     {xlsx(t, []string{"LAPTOP"}, map[string][][]any{"LAPTOP": {importHeader}}), domain.ErrInvalidImportFile, "file", "no rows"},
		"too many":    {xlsx(t, []string{"LAPTOP"}, map[string][][]any{"LAPTOP": many}), domain.ErrImportTooLarge, "", "limit is 5000"},
		"too big":     {bytes.Repeat([]byte{0}, domain.MaxImportBytes+1), domain.ErrImportTooLarge, "", "5 MB"},
		"same type":   {xlsx(t, []string{"LAPTOP", " LAPTOP"}, map[string][][]any{"LAPTOP": {importHeader, {nil, "x", nil, "S"}}, " LAPTOP": {importHeader, {nil, "y", nil, "S"}}}), domain.ErrInvalidImportFile, " LAPTOP", "Keep one sheet per type"},
		"empty bytes": {nil, domain.ErrInvalidImportFile, "file", "Excel workbook"},
	}
	for name, c := range cases {
		_, err := ie.svc.PreviewImport(manager, "f.xlsx", c.data)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", name, err, c.want)
			continue
		}
		e := errs.NewFrom(err)
		found := c.field == "" && strings.Contains(e.Detail(), c.detail)
		for _, f := range e.Fields() {
			if f.Field == c.field && strings.Contains(f.Detail, c.detail) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: fields %+v detail %q, want %s: %q", name, e.Fields(), e.Detail(), c.field, c.detail)
		}
	}
}

func TestImportNeedsTheManagePermission(t *testing.T) {
	ie := newImportEnv(t)
	data := xlsx(t, []string{"LAPTOP"}, map[string][][]any{"LAPTOP": {importHeader, {nil, "x", nil, "S"}}})
	for name, ctx := range map[string]context.Context{"anonymous": context.Background(), "reader": as(domain.PermAssetRead, domain.PermAssetExport)} {
		want := 401
		if name == "reader" {
			want = 403
		}
		if _, err := ie.svc.PreviewImport(ctx, "f.xlsx", data); status(err) != want {
			t.Errorf("%s preview: %v, want %d", name, err, want)
		}
		if _, err := ie.svc.CommitImport(ctx, "f.xlsx", data); status(err) != want {
			t.Errorf("%s commit: %v, want %d", name, err, want)
		}
		if _, err := ie.svc.ImportTemplate(ctx); status(err) != want {
			t.Errorf("%s template: %v, want %d", name, err, want)
		}
	}
	if len(ie.assets.assets) != 0 || len(ie.imports.records) != 0 {
		t.Error("a refused request must not write anything")
	}
}

func TestImportTemplateHasASheetPerType(t *testing.T) {
	ie := newImportEnv(t)
	if _, err := ie.svc.CreateAssetType(manager, domain.NewAssetType{Code: "DESK", Name: "Desk"}); err != nil {
		t.Fatal(err)
	}
	data, err := ie.svc.ImportTemplate(manager)
	if err != nil {
		t.Fatal(err)
	}
	sheets, err := spreadsheet.ReadWorkbook(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, s := range sheets {
		got[s.Name] = strings.Join(s.Header, ",")
		if len(s.Rows) != 0 {
			t.Errorf("sheet %s has example rows: they would be imported by accident", s.Name)
		}
	}
	if got["LAPTOP"] != "tag,name,description,status,purchase_date,attr:serial,attr:ram_gb,attr:os" || got["DESK"] != "tag,name,description,status,purchase_date" {
		t.Errorf("headers = %v", got)
	}
	// export rồi import lại: file mẫu tự đọc được bằng chính bộ đọc của import
	rep, err := ie.svc.PreviewImport(manager, "template.xlsx", append([]byte(nil), data...))
	if !errors.Is(err, domain.ErrInvalidImportFile) || !strings.Contains(errs.NewFrom(err).Fields()[0].Detail, "no rows") {
		t.Errorf("an empty template previews as %v / %+v", err, rep)
	}
}
