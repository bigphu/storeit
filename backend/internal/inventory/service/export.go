package service

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"storeit/internal/inventory/domain"
	"storeit/internal/inventory/spreadsheet"
	"storeit/internal/platform/auth"
	"storeit/internal/platform/errs"
)

// ExportMode: "data" (mỗi loại một sheet, import đọc lại được) hay "report" (theo bố cục)
type ExportMode string

const (
	ExportData   ExportMode = "data"
	ExportReport ExportMode = "report"
)

type ExportRequest struct {
	Mode      ExportMode
	Filter    domain.AssetFilter // như danh sách; Limit/Offset bỏ qua
	IDs       []uuid.UUID        // "Export selected": 1..200; nil là không lọc
	ProfileID *uuid.UUID
	Layout    *domain.ExportLayout // gửi kèm thì thắng profile
}

type ExportFile struct {
	Name    string
	Data    []byte
	Skipped []string // khoá thuộc tính bị bỏ (không loại nào còn có)
	Rows    int64
}

const (
	maxExportIDs   = 200
	exportPageSize = 500
)

// Export dựng file .xlsx theo bộ lọc của danh sách. Vượt số dòng tối đa thì 422 trước
// khi ghi gì; file nằm trong bộ nhớ đến khi trả về.
func (s *Service) Export(ctx context.Context, req ExportRequest) (ExportFile, error) {
	actor, err := auth.Require(ctx, domain.PermAssetExport)
	if err != nil {
		return ExportFile{}, err
	}
	if _, err := auth.Require(ctx, domain.PermAssetRead); err != nil {
		return ExportFile{}, err
	}
	layout, profile, err := s.exportLayout(ctx, actor, req)
	if err != nil {
		return ExportFile{}, err
	}
	f := req.Filter
	f.Limit, f.Offset, f.IncludeValues = 0, 0, false
	if req.IDs != nil {
		if len(req.IDs) == 0 || len(req.IDs) > maxExportIDs {
			return ExportFile{}, domain.ErrInvalidExportIDs
		}
		f.IDs = req.IDs
	}
	if layout.Sort != "" {
		f.Sort = domain.AssetSort(layout.Sort)
	}
	if _, _, byAttr := f.Sort.Attribute(); byAttr || len(f.Attrs) > 0 {
		if err := s.resolveAttrQuery(ctx, &f); err != nil {
			return ExportFile{}, err
		}
	}
	total, err := s.assets.Count(ctx, f)
	if err != nil {
		return ExportFile{}, err
	}
	if total > int64(s.exportMaxRows) {
		return ExportFile{}, domain.ErrExportTooLarge.With(errs.WithDetailf(
			"This export has %d rows; the limit is %d. Narrow the filters and try again.", total, s.exportMaxRows))
	}
	types, err := s.exportTypes(ctx, f)
	if err != nil {
		return ExportFile{}, err
	}

	w := spreadsheet.NewWriter()
	defer func() { _ = w.Close() }()
	byID := make(map[uuid.UUID]domain.AssetType, len(types))
	for _, t := range types {
		byID[t.ID] = t
	}
	fm := spreadsheet.Formatter{Layout: layout, Types: byID, Loc: time.Local}
	title, err := s.exportTitle(ctx, actor, req, layout, profile, f, types)
	if err != nil {
		return ExportFile{}, err
	}

	type group struct {
		name   string
		types  []domain.AssetType
		filter domain.AssetFilter
	}
	var groups []group
	if layout.Sheets == domain.SheetPerType {
		for _, t := range types {
			gf := f
			gf.TypeID = &t.ID
			name := t.Name
			if req.Mode == ExportData {
				name = t.Code
			}
			groups = append(groups, group{name: name, types: []domain.AssetType{t}, filter: gf})
		}
	} else {
		groups = []group{{name: layout.SheetName, types: types, filter: f}}
	}
	if len(groups) == 0 {
		// không có dòng nào: vẫn trả file có một sheet với tên cột
		groups = []group{{name: layout.SheetName, filter: f}}
	}

	summary := map[uuid.UUID]map[domain.StatusKind]int64{}
	var rows int64
	for _, g := range groups {
		cols := layout.SheetColumns(g.types)
		headers, widths := make([]string, len(cols)), make([]float64, len(cols))
		for i, c := range cols {
			headers[i], widths[i] = c.Header, columnWidth(c)
		}
		sh, err := w.Sheet(spreadsheet.SheetOptions{
			Name: g.name, Widths: widths, Header: layout.Header, Freeze: layout.Freeze,
			Filter: layout.Filter, Stripes: layout.Stripes, Title: title,
		}, headers)
		if err != nil {
			return ExportFile{}, err
		}
		err = s.assets.Stream(ctx, g.filter, exportPageSize, func(items []domain.AssetListItem) error {
			for _, a := range items {
				cells := make([]spreadsheet.Cell, len(cols))
				for i, c := range cols {
					cells[i] = fm.Cell(a, c)
				}
				if err := sh.Row(cells); err != nil {
					return err
				}
				if summary[a.TypeID] == nil {
					summary[a.TypeID] = map[domain.StatusKind]int64{}
				}
				summary[a.TypeID][a.StatusKind]++
				rows++
			}
			return nil
		})
		if err != nil {
			return ExportFile{}, err
		}
		if err := sh.Close(); err != nil {
			return ExportFile{}, err
		}
	}
	if layout.Summary {
		if err := writeSummary(w, types, summary); err != nil {
			return ExportFile{}, err
		}
	}
	var buf bytes.Buffer
	if _, err := w.WriteTo(&buf); err != nil {
		return ExportFile{}, err
	}

	rec := domain.ExportRecord{Mode: string(req.Mode), Rows: rows, Sheets: len(groups), Filters: filterRecord(f)}
	if profile != nil {
		rec.ProfileID = &profile.ID
	}
	if err := s.profiles.RecordExport(ctx, rec); err != nil {
		// file đã dựng xong; mất một dòng lịch sử không đáng làm hỏng lần tải
		slog.ErrorContext(ctx, "inventory: record export", "error", err)
	}
	return ExportFile{Name: exportFileName(req.Mode, profile), Data: buf.Bytes(), Skipped: layout.SkippedKeys(types), Rows: rows}, nil
}

// exportLayout: dữ liệu dùng DataLayout; báo cáo dùng bố cục gửi kèm, của profile, hay mặc định
func (s *Service) exportLayout(ctx context.Context, actor auth.Actor, req ExportRequest) (domain.ExportLayout, *domain.ExportProfile, error) {
	if req.Mode == ExportData {
		return domain.DataLayout(), nil, nil
	}
	var profile *domain.ExportProfile
	if req.ProfileID != nil {
		p, err := s.visibleProfile(ctx, actor, *req.ProfileID)
		if err != nil {
			return domain.ExportLayout{}, nil, err
		}
		profile = &p
	}
	l := domain.DefaultReportLayout()
	switch {
	case req.Layout != nil:
		l = *req.Layout
	case profile != nil:
		l = profile.Layout
	}
	l = l.WithDefaults()
	if err := l.Validate(); err != nil {
		return domain.ExportLayout{}, nil, err
	}
	return l, profile, nil
}

// exportTypes: các loại (kèm thuộc tính) có dòng trong export, theo tên
func (s *Service) exportTypes(ctx context.Context, f domain.AssetFilter) ([]domain.AssetType, error) {
	var candidates []domain.AssetType
	if f.TypeID != nil {
		candidates = []domain.AssetType{{ID: *f.TypeID}}
	} else {
		all, err := s.types.List(ctx, true)
		if err != nil {
			return nil, err
		}
		candidates = all
	}
	var out []domain.AssetType
	for _, c := range candidates {
		tf := f
		tf.TypeID = &c.ID
		n, err := s.assets.Count(ctx, tf)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			continue
		}
		t, err := s.types.Get(ctx, c.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	slices.SortFunc(out, func(a, b domain.AssetType) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return out, nil
}

// exportTitle: hai dòng tiêu đề của báo cáo (tên báo cáo; ngày, người export, bộ lọc)
func (s *Service) exportTitle(ctx context.Context, actor auth.Actor, req ExportRequest, l domain.ExportLayout, p *domain.ExportProfile, f domain.AssetFilter, types []domain.AssetType) ([]string, error) {
	if req.Mode != ExportReport || !l.TitleRow {
		return nil, nil
	}
	name := "Asset report"
	if p != nil {
		name = p.Name
	}
	who, err := s.accounts.GetAccount(ctx, actor.AccountID)
	by := who.Name
	if err != nil || by == "" {
		by = "unknown"
	}
	summary, err := s.filterSummary(ctx, f, types)
	if err != nil {
		return nil, err
	}
	date := time.Now().Format("02/01/2006")
	return []string{name, fmt.Sprintf("Generated %s by %s · %s", date, by, summary)}, nil
}

// filterSummary: mô tả bộ lọc cho người đọc ("Laptop · In use · search \"think\"")
func (s *Service) filterSummary(ctx context.Context, f domain.AssetFilter, types []domain.AssetType) (string, error) {
	var parts []string
	if f.TypeID != nil && len(types) == 1 {
		parts = append(parts, types[0].Name)
	}
	if f.StatusID != nil {
		st, err := s.statuses.Get(ctx, *f.StatusID)
		if err != nil {
			return "", err
		}
		parts = append(parts, st.Name)
	}
	if f.StatusKind != nil {
		parts = append(parts, f.StatusKind.Label())
	}
	if f.Query != "" {
		parts = append(parts, fmt.Sprintf("search %q", f.Query))
	}
	for _, af := range f.AttrFilters {
		parts = append(parts, attrFilterText(types, af))
	}
	if f.IDs != nil {
		parts = append(parts, fmt.Sprintf("%d selected", len(f.IDs)))
	}
	if f.IncludeRetired {
		parts = append(parts, "including retired")
	}
	if len(parts) == 0 {
		return "All assets", nil
	}
	return strings.Join(parts, " · "), nil
}

var opSymbols = map[domain.AttrOp]string{
	domain.OpEq: "=", domain.OpGt: ">", domain.OpGte: "≥", domain.OpLt: "<", domain.OpLte: "≤",
	domain.OpContains: "contains", domain.OpIn: "in",
}

func attrFilterText(types []domain.AssetType, af domain.AttrFilter) string {
	for _, t := range types {
		for _, a := range t.Attributes {
			if a.ID == af.AttributeID {
				return fmt.Sprintf("%s %s %s", a.Label, opSymbols[af.Op], af.Value)
			}
		}
	}
	return "attribute filter"
}

func filterRecord(f domain.AssetFilter) map[string]any {
	m := map[string]any{}
	if f.Query != "" {
		m["q"] = f.Query
	}
	if f.TypeID != nil {
		m["type_id"] = f.TypeID.String()
	}
	if f.StatusID != nil {
		m["status_id"] = f.StatusID.String()
	}
	if f.StatusKind != nil {
		m["status_kind"] = string(*f.StatusKind)
	}
	if len(f.Attrs) > 0 {
		m["attr"] = f.Attrs
	}
	if f.IDs != nil {
		m["ids"] = len(f.IDs)
	}
	if f.IncludeRetired {
		m["include_retired"] = true
	}
	return m
}

// columnWidth: độ rộng người dùng chọn, hay mặc định theo trường; 0 để writer tự tính
func columnWidth(c domain.SheetColumn) float64 {
	if c.Width != 0 {
		return c.Width
	}
	switch c.Field {
	case "name":
		return 32
	case "description":
		return 40
	case "tag":
		return 14
	}
	return 0
}

func writeSummary(w *spreadsheet.Writer, types []domain.AssetType, counts map[uuid.UUID]map[domain.StatusKind]int64) error {
	kinds := []domain.StatusKind{domain.KindAvailable, domain.KindInUse, domain.KindUnavailable, domain.KindRetired}
	headers := []string{"Type"}
	for _, k := range kinds {
		headers = append(headers, k.Label())
	}
	headers = append(headers, "Total")
	sh, err := w.Sheet(spreadsheet.SheetOptions{Name: "Summary", Header: domain.HeaderBold, Freeze: true}, headers)
	if err != nil {
		return err
	}
	for _, t := range types {
		row := []spreadsheet.Cell{spreadsheet.TextCell(t.Name)}
		var total int64
		for _, k := range kinds {
			n := counts[t.ID][k]
			total += n
			row = append(row, spreadsheet.Cell{Kind: spreadsheet.Number, Num: float64(n)})
		}
		row = append(row, spreadsheet.Cell{Kind: spreadsheet.Number, Num: float64(total)})
		if err := sh.Row(row); err != nil {
			return err
		}
	}
	return sh.Close()
}

var slugBad = regexp.MustCompile(`[^\p{L}\p{N}]+`)

// exportFileName: storeit-assets-<ngày>.xlsx; báo cáo theo tên profile
func exportFileName(mode ExportMode, p *domain.ExportProfile) string {
	date := time.Now().Format("2006-01-02")
	switch {
	case mode == ExportData:
		return "storeit-assets-" + date + ".xlsx"
	case p != nil:
		slug := strings.Trim(slugBad.ReplaceAllString(strings.ToLower(p.Name), "-"), "-")
		if slug == "" {
			slug = "storeit-report"
		}
		return slug + "-" + date + ".xlsx"
	}
	return "storeit-report-" + date + ".xlsx"
}
