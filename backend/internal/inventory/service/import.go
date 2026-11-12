package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"storeit/internal/inventory/domain"
	"storeit/internal/inventory/spreadsheet"
	"storeit/internal/platform/auth"
	"storeit/internal/platform/errs"
)

// Import tài sản từ Excel (US-06). File mẫu là bố cục export dữ liệu: mỗi sheet một loại (tên sheet
// là mã loại), hàng 1 là tên cột (tag, name, description, status, purchase_date, attr:<key>).
//
// Quy tắc (đã duyệt với PO):
//   - tag trống: tạo tài sản mới, kho tự sinh tag (LAP-0091); tag chưa có: tạo mới với tag đó;
//     tag đã có: cập nhật, chỉ các cột có trong sheet và ô không trống (ô trống giữ giá trị hiện có)
//   - không bao giờ đổi tag, loại, tài sản đã retire, status Reserved / On loan của tài sản đang mượn
//   - tag trùng trong file: mọi dòng có tag đó lỗi; tag của loại khác: lỗi; tài sản retire: lỗi
//   - mỗi dòng là một giao dịch riêng: dòng lỗi bị bỏ và nằm trong báo cáo, dòng tốt vẫn được lưu
//   - xem trước (preview) không ghi gì; xác nhận (commit) đọc lại file và kiểm lại từng dòng

// ImportRowResult là kết quả của một dòng. Sau commit: Tag là tag thật của tài sản (kể cả tag
// vừa sinh) và AssetID là tài sản đã tạo hay cập nhật.
type ImportRowResult struct {
	Sheet   string
	Row     int
	Tag     string
	Name    string
	Action  domain.ImportAction
	Message string
	AssetID *uuid.UUID
}

// ImportSheetInfo: một sheet của file và loại nó khớp
type ImportSheetInfo struct {
	Name     string
	TypeCode string
	Rows     int
}

// ImportReport là báo cáo của xem trước hay xác nhận
type ImportReport struct {
	FileName  string
	Sheets    []ImportSheetInfo
	Rows      []ImportRowResult
	Counts    domain.ImportCounts
	Committed bool
}

type importRow struct {
	ImportRowResult
	typ     domain.AssetType
	create  bool
	assetID uuid.UUID
	version int32
	newTag  string // tag tạo với ("" là kho tự sinh)
	fields  domain.AssetFields
}

type importPlan struct {
	sheets []ImportSheetInfo
	rows   []*importRow
	sha    string
}

func (p *importPlan) counts() domain.ImportCounts {
	var c domain.ImportCounts
	for _, r := range p.rows {
		switch r.Action {
		case domain.ImportCreate:
			c.Create++
		case domain.ImportUpdate:
			c.Update++
		case domain.ImportSkip:
			c.Skip++
		default:
			c.Fail++
		}
	}
	return c
}

func (p *importPlan) report(name string, committed bool) ImportReport {
	rows := make([]ImportRowResult, len(p.rows))
	for i, r := range p.rows {
		rows[i] = r.ImportRowResult
	}
	return ImportReport{FileName: name, Sheets: p.sheets, Rows: rows, Counts: p.counts(), Committed: committed}
}

// PreviewImport kiểm file và cho biết từng dòng sẽ ra sao, không ghi gì
func (s *Service) PreviewImport(ctx context.Context, fileName string, data []byte) (ImportReport, error) {
	if _, err := auth.Require(ctx, domain.PermAssetManage); err != nil {
		return ImportReport{}, err
	}
	plan, err := s.planImport(ctx, data)
	if err != nil {
		return ImportReport{}, err
	}
	return plan.report(fileName, false), nil
}

// CommitImport kiểm lại file rồi lưu từng dòng tạo / cập nhật, mỗi dòng một giao dịch. Dòng lỗi bị bỏ
// và nằm trong báo cáo. File có dòng tạo mới chỉ xác nhận được một lần (ErrImportAlreadyDone).
func (s *Service) CommitImport(ctx context.Context, fileName string, data []byte) (ImportReport, error) {
	actor, err := auth.Require(ctx, domain.PermAssetManage)
	if err != nil {
		return ImportReport{}, err
	}
	plan, err := s.planImport(ctx, data)
	if err != nil {
		return ImportReport{}, err
	}
	hasCreates := false
	for _, r := range plan.rows {
		hasCreates = hasCreates || r.Action == domain.ImportCreate
	}
	id, err := s.imports.Claim(ctx, domain.ImportRecord{
		AccountID: actor.AccountID, FileName: cleanFileName(fileName), SHA256: plan.sha, HasCreates: hasCreates, Rows: len(plan.rows),
	})
	if err != nil {
		return ImportReport{}, err
	}
	var abort error
	for _, r := range plan.rows {
		if r.Action != domain.ImportCreate && r.Action != domain.ImportUpdate {
			continue
		}
		if abort = s.saveImportRow(ctx, r); abort != nil {
			break
		}
	}
	// ghi kết quả kể cả khi bị ngắt giữa chừng: những dòng đã lưu là thật
	if err := s.imports.Finish(ctx, id, plan.counts()); err != nil && abort == nil {
		abort = err
	}
	if abort != nil {
		return ImportReport{}, abort
	}
	return plan.report(fileName, true), nil
}

// saveImportRow lưu một dòng đã lên kế hoạch. Lỗi do dữ liệu (đã đổi từ lúc xem trước, tag bị người
// khác lấy) biến dòng thành lỗi trong báo cáo; lỗi hệ thống thì dừng cả lần import.
func (s *Service) saveImportRow(ctx context.Context, r *importRow) error {
	var a domain.Asset
	var err error
	if r.create {
		a, err = s.assets.Create(ctx, r.newTag, r.fields)
	} else {
		a, err = s.assets.Replace(ctx, r.assetID, r.fields, r.version)
	}
	if err != nil {
		var e *errs.Error
		if !errors.As(err, &e) || e.Status() >= 500 {
			return err
		}
		r.Action = domain.ImportFail
		switch {
		case errors.Is(err, domain.ErrAssetChanged):
			r.Message = "Changed by someone else while importing. Import the file again to apply this row"
		case errors.Is(err, domain.ErrTagTaken):
			r.Message = fmt.Sprintf("Tag %s is already used by another asset", r.newTag)
		case errors.Is(err, domain.ErrAssetRetired):
			r.Message = "Retired asset. Restore it before importing changes"
		default:
			r.Message = importMessage(err, r.typ)
		}
		return nil
	}
	r.AssetID, r.Tag = &a.ID, a.Tag
	if r.create {
		r.Message = "Created"
	} else {
		r.Message = "Updated: " + r.Message
	}
	return nil
}

// ImportTemplate dựng file mẫu: mỗi loại đang dùng một sheet, hàng tiêu đề và danh sách chọn
func (s *Service) ImportTemplate(ctx context.Context) ([]byte, error) {
	if _, err := auth.Require(ctx, domain.PermAssetManage); err != nil {
		return nil, err
	}
	types, err := s.types.List(ctx, false)
	if err != nil {
		return nil, err
	}
	statuses, err := s.statuses.List(ctx, false)
	if err != nil {
		return nil, err
	}
	var statusNames []string
	for _, st := range statuses {
		if assignableStatus(st) {
			statusNames = append(statusNames, st.Name)
		}
	}
	var sheets []spreadsheet.TemplateSheet
	for _, t := range types {
		full, err := s.types.Get(ctx, t.ID)
		if err != nil {
			return nil, err
		}
		sh := spreadsheet.TemplateSheet{
			Name:    full.Code,
			Headers: []string{domain.ColTag, domain.ColName, domain.ColDescription, domain.ColStatus, domain.ColPurchaseDate},
			Lists:   map[int][]string{3: statusNames},
		}
		for _, a := range full.ActiveAttributes() {
			switch a.DataType {
			case domain.TypeSelect:
				var labels []string
				for _, o := range a.Options {
					if o.RemovedAt == nil {
						labels = append(labels, o.Label)
					}
				}
				sh.Lists[len(sh.Headers)] = labels
			case domain.TypeBoolean:
				sh.Lists[len(sh.Headers)] = []string{"TRUE", "FALSE"}
			}
			sh.Headers = append(sh.Headers, domain.ColAttrPrefix+a.Key)
		}
		sheets = append(sheets, sh)
	}
	if len(sheets) == 0 {
		return nil, fileProblem("file", "there is no asset type to make a template from")
	}
	var buf bytes.Buffer
	if err := spreadsheet.WriteTemplate(&buf, sheets); err != nil {
		return nil, fmt.Errorf("inventory: import template: %w", err)
	}
	return buf.Bytes(), nil
}

// assignableStatus: status mà import được đặt (không phải retire, Reserved, On loan hay đã archive)
func assignableStatus(st domain.Status) bool {
	return st.Kind != domain.KindRetired && !domain.IsBorrowingStatus(st.ID) && !st.Archived()
}

func fileProblem(field, detail string) error {
	return domain.ErrInvalidImportFile.With(errs.WithFields(errs.FieldError{Field: field, Detail: detail}))
}

// cleanFileName: tên file để ghi lịch sử (không có đường dẫn, tối đa 255 ký tự)
func cleanFileName(name string) string {
	name = strings.TrimSpace(name)
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	r := []rune(name)
	if len(r) > 255 {
		name = string(r[:255])
	}
	if name == "" {
		return "import.xlsx"
	}
	return name
}

type importSheet struct {
	sheet spreadsheet.ReadSheet
	typ   domain.AssetType
	cols  []domain.ImportColumn
}

// planImport đọc file và quyết định kết quả từng dòng; không ghi gì
func (s *Service) planImport(ctx context.Context, data []byte) (*importPlan, error) {
	if len(data) > domain.MaxImportBytes {
		return nil, domain.ErrImportTooLarge.With(errs.WithDetailf("The file is larger than %d MB.", domain.MaxImportBytes>>20))
	}
	read, err := spreadsheet.ReadWorkbook(bytes.NewReader(data))
	if err != nil {
		return nil, fileProblem("file", "can't be opened as an Excel workbook (.xlsx). Save it as “Excel Workbook” and choose it again")
	}
	total := 0
	for _, sh := range read {
		total += len(sh.Rows)
	}
	if total == 0 {
		return nil, fileProblem("file", "has no rows to import")
	}
	if total > domain.MaxImportRows {
		return nil, domain.ErrImportTooLarge.With(errs.WithDetailf("The file has %d rows; the limit is %d.", total, domain.MaxImportRows))
	}
	sum := sha256.Sum256(data)
	plan := &importPlan{sha: hex.EncodeToString(sum[:])}

	allTypes, err := s.types.List(ctx, true)
	if err != nil {
		return nil, err
	}
	typeNames := make(map[uuid.UUID]string, len(allTypes))
	var active []domain.AssetType
	for _, t := range allTypes {
		typeNames[t.ID] = t.Name
		if !t.Archived() {
			active = append(active, t)
		}
	}

	var problems []errs.FieldError
	var sheets []importSheet
	usedBy := map[uuid.UUID]string{}
	for _, sh := range read {
		t, ok := domain.ImportTypeCode(sh.Name, active)
		if !ok {
			codes := make([]string, 0, len(active))
			for _, a := range active {
				codes = append(codes, a.Code)
			}
			if len(codes) > 12 {
				codes = append(codes[:12], "…")
			}
			problems = append(problems, errs.FieldError{Field: sh.Name, Detail: "isn't an asset type code. Rename the sheet to one of: " + strings.Join(codes, ", ")})
			continue
		}
		if other, dup := usedBy[t.ID]; dup {
			problems = append(problems, errs.FieldError{Field: sh.Name, Detail: fmt.Sprintf("is the same type (%s) as sheet %s. Keep one sheet per type", t.Code, other)})
			continue
		}
		usedBy[t.ID] = sh.Name
		full, err := s.types.Get(ctx, t.ID)
		if err != nil {
			return nil, err
		}
		cols, probs := domain.ResolveImportColumns(full, sh.Header)
		for _, p := range probs {
			problems = append(problems, errs.FieldError{Field: sh.Name, Detail: p})
		}
		sheets = append(sheets, importSheet{sheet: sh, typ: full, cols: cols})
		plan.sheets = append(plan.sheets, ImportSheetInfo{Name: sh.Name, TypeCode: full.Code, Rows: len(sh.Rows)})
	}
	if len(problems) > 0 {
		return nil, domain.ErrInvalidImportFile.With(errs.WithFields(problems...))
	}

	statuses, err := s.statuses.List(ctx, true)
	if err != nil {
		return nil, err
	}
	lk := importLookups{statusByName: map[string]domain.Status{}, typeNames: typeNames}
	for _, st := range statuses {
		key := strings.ToLower(st.Name)
		if prev, dup := lk.statusByName[key]; !dup || (prev.Archived() && !st.Archived()) {
			lk.statusByName[key] = st
		}
		if assignableStatus(st) {
			lk.assignable = append(lk.assignable, st.Name)
		}
	}

	type parsed struct {
		row *importRow
		v   domain.ImportValues
		tag string // chuẩn hoá; "" là trống
		typ domain.AssetType
	}
	var rows []parsed
	byTag := map[string][]*importRow{}
	for _, sh := range sheets {
		for _, raw := range sh.sheet.Rows {
			v := domain.ParseImportRow(sh.cols, raw.Cells)
			r := &importRow{typ: sh.typ, ImportRowResult: ImportRowResult{Sheet: sh.sheet.Name, Row: raw.Row, Tag: strings.TrimSpace(v.Tag)}}
			if v.Name != nil {
				r.Name = *v.Name
			}
			p := parsed{row: r, v: v, typ: sh.typ}
			if strings.TrimSpace(v.Tag) != "" {
				tag, err := domain.NormalizeTag(v.Tag)
				if err != nil {
					r.fail(fmt.Sprintf("Tag “%s” isn't valid: use 1 to 64 characters, A–Z, 0–9, “.”, “_” or “-”", strings.TrimSpace(v.Tag)))
				} else {
					p.tag, r.Tag = tag, tag
					byTag[tag] = append(byTag[tag], r)
				}
			}
			rows = append(rows, p)
			plan.rows = append(plan.rows, r)
		}
	}
	for tag, same := range byTag {
		if len(same) < 2 {
			continue
		}
		for _, r := range same {
			r.fail(fmt.Sprintf("Tag %s is also in %s. Keep one row per tag", tag, otherRows(r, same)))
		}
	}

	var tags []string
	for tag, same := range byTag {
		if len(same) == 1 {
			tags = append(tags, tag)
		}
	}
	slices.Sort(tags)
	existing := map[string]domain.Asset{}
	for chunk := range slices.Chunk(tags, 1000) {
		found, err := s.assets.ByTags(ctx, chunk)
		if err != nil {
			return nil, err
		}
		for _, a := range found {
			existing[a.Tag] = a
		}
	}

	for _, p := range rows {
		if p.row.Action == domain.ImportFail {
			continue
		}
		if len(p.v.Errs) > 0 {
			p.row.fail(strings.Join(p.v.Errs, "; "))
			continue
		}
		if cur, ok := existing[p.tag]; ok && p.tag != "" {
			s.planUpdate(ctx, p.row, p.v, cur, lk)
		} else {
			s.planCreate(ctx, p.row, p.v, p.tag, lk)
		}
	}
	return plan, nil
}

type importLookups struct {
	statusByName map[string]domain.Status
	assignable   []string
	typeNames    map[uuid.UUID]string
}

func (r *importRow) fail(msg string) {
	r.Action, r.Message = domain.ImportFail, msg
}

// otherRows: "row 12", "rows 12 and 15" hay "SHEET row 4" cho các dòng cùng tag với r
func otherRows(r *importRow, same []*importRow) string {
	var parts []string
	for _, o := range same {
		if o == r {
			continue
		}
		if o.Sheet == r.Sheet {
			parts = append(parts, strconv.Itoa(o.Row))
		} else {
			parts = append(parts, fmt.Sprintf("%s row %d", o.Sheet, o.Row))
		}
	}
	switch len(parts) {
	case 1:
		if _, err := strconv.Atoi(parts[0]); err == nil {
			return "row " + parts[0]
		}
		return parts[0]
	default:
		return "rows " + strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
	}
}

// status tra tên status trong file; ok=false kèm lý do khi tên không có hay không được đặt
func (lk importLookups) status(name string) (domain.Status, string) {
	st, ok := lk.statusByName[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return domain.Status{}, fmt.Sprintf("Status: “%s” doesn't exist. Use one of: %s", name, strings.Join(lk.assignable, ", "))
	}
	return st, ""
}

func (s *Service) planCreate(ctx context.Context, r *importRow, v domain.ImportValues, tag string, lk importLookups) {
	if v.Name == nil {
		r.fail("Name is required")
		return
	}
	in := AssetInput{Name: *v.Name, TypeID: r.typ.ID, PurchaseDate: v.PurchaseDate, Attributes: v.Attrs}
	if v.Description != nil {
		in.Description = *v.Description
	}
	if v.Status != nil {
		st, msg := lk.status(*v.Status)
		if msg != "" {
			r.fail(msg)
			return
		}
		in.StatusID = &st.ID
	}
	f, err := s.fields(ctx, in, nil)
	if err != nil {
		r.fail(importMessage(err, r.typ))
		return
	}
	r.create, r.fields, r.newTag, r.Name = true, f, tag, f.Name
	r.Action = domain.ImportCreate
	if tag == "" {
		r.Message = "New asset. Tag is given on import"
	} else {
		r.Message = "New asset with tag " + tag
	}
}

func (s *Service) planUpdate(ctx context.Context, r *importRow, v domain.ImportValues, cur domain.Asset, lk importLookups) {
	r.Name = cur.Name
	if cur.Retired() {
		r.fail("Retired asset. Restore it before importing changes")
		return
	}
	if cur.TypeID != r.typ.ID {
		r.fail(fmt.Sprintf("This tag belongs to type %s, not %s. Move it to the other sheet", lk.typeNames[cur.TypeID], r.typ.Name))
		return
	}
	in := currentInput(cur, r.typ)
	if v.Name != nil {
		in.Name = *v.Name
	}
	if v.Description != nil {
		in.Description = *v.Description
	}
	if v.PurchaseDate != nil {
		in.PurchaseDate = v.PurchaseDate
	}
	for k, val := range v.Attrs {
		in.Attributes[k] = val
	}
	if v.Status != nil {
		st, msg := lk.status(*v.Status)
		if msg != "" {
			r.fail(msg)
			return
		}
		if st.ID != cur.StatusID {
			if domain.IsBorrowingStatus(cur.StatusID) {
				name := "On loan"
				if cur.StatusID == domain.ReservedStatusID {
					name = "Reserved"
				}
				r.fail(name + ": status can't change. Return it first")
				return
			}
			in.StatusID = &st.ID
		}
	}
	f, err := s.fields(ctx, in, &cur)
	if err != nil {
		r.fail(importMessage(err, r.typ))
		return
	}
	changes := s.importChanges(ctx, cur, f, r.typ, lk)
	if len(changes) == 0 {
		r.Action, r.Message = domain.ImportSkip, "No changes"
		return
	}
	r.Action, r.Message = domain.ImportUpdate, strings.Join(changes, " · ")
	r.fields, r.assetID, r.version, r.Name = f, cur.ID, cur.Version, f.Name
}

// currentInput: tài sản hiện có thành AssetInput đủ mọi trường, để áp các ô của file lên rồi gửi lại
// như một lần sửa
func currentInput(a domain.Asset, t domain.AssetType) AssetInput {
	byID := map[uuid.UUID]domain.Attribute{}
	for _, at := range t.ActiveAttributes() {
		byID[at.ID] = at
	}
	attrs := map[string]any{}
	for _, v := range a.Values {
		at, ok := byID[v.AttributeID]
		if !ok {
			continue
		}
		switch {
		case v.Text != nil:
			attrs[at.Key] = *v.Text
		case v.Number != nil:
			if f, err := strconv.ParseFloat(*v.Number, 64); err == nil {
				attrs[at.Key] = f
			}
		case v.Date != nil:
			attrs[at.Key] = v.Date.Format(time.DateOnly)
		case v.Bool != nil:
			attrs[at.Key] = *v.Bool
		case v.OptionID != nil:
			attrs[at.Key] = v.OptionID.String()
		}
	}
	st := a.StatusID
	return AssetInput{
		Name: a.Name, Description: a.Description, TypeID: a.TypeID, StatusID: &st,
		LocationID: a.LocationID, HolderMemberID: a.HolderMemberID, PurchaseDate: a.PurchaseDate, Attributes: attrs,
	}
}

// importChanges liệt kê điều khác nhau giữa tài sản hiện có và trường mới, dạng "RAM 8 → 16 GB"
func (s *Service) importChanges(ctx context.Context, cur domain.Asset, f domain.AssetFields, t domain.AssetType, lk importLookups) []string {
	var out []string
	if f.Name != cur.Name {
		out = append(out, fmt.Sprintf("Name “%s” → “%s”", clip(cur.Name), clip(f.Name)))
	}
	if f.Description != cur.Description {
		out = append(out, "Description changed")
	}
	if f.StatusID != cur.StatusID {
		from, to := "?", "?"
		if st, err := s.statuses.Get(ctx, cur.StatusID); err == nil {
			from = st.Name
		}
		if st, err := s.statuses.Get(ctx, f.StatusID); err == nil {
			to = st.Name
		}
		out = append(out, fmt.Sprintf("Status %s → %s", from, to))
	}
	if !sameDay(cur.PurchaseDate, f.PurchaseDate) {
		out = append(out, fmt.Sprintf("Purchase date %s → %s", dayText(cur.PurchaseDate), dayText(f.PurchaseDate)))
	}
	before := map[uuid.UUID]domain.Value{}
	for _, v := range cur.Values {
		before[v.AttributeID] = v
	}
	after := map[uuid.UUID]domain.Value{}
	for _, v := range f.Values {
		after[v.AttributeID] = v
	}
	for _, a := range t.ActiveAttributes() {
		b, hadB := before[a.ID]
		n, hasN := after[a.ID]
		if hadB == hasN && (!hadB || sameValue(b, n)) {
			continue
		}
		line := a.Label + " " + valueText(a, b, hadB) + " → " + valueText(a, n, hasN)
		if a.Unit != "" && a.DataType == domain.TypeNumber {
			line += " " + a.Unit
		}
		out = append(out, line)
	}
	return out
}

func clip(s string) string {
	if r := []rune(s); len(r) > 40 {
		return string(r[:39]) + "…"
	}
	return s
}

func sameDay(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Format(time.DateOnly) == b.Format(time.DateOnly)
}

func dayText(t *time.Time) string {
	if t == nil {
		return "—"
	}
	return t.Format("2 Jan 2006")
}

func sameValue(a, b domain.Value) bool {
	switch {
	case a.Number != nil && b.Number != nil:
		x, e1 := strconv.ParseFloat(*a.Number, 64)
		y, e2 := strconv.ParseFloat(*b.Number, 64)
		return e1 == nil && e2 == nil && x == y
	case a.Text != nil && b.Text != nil:
		return *a.Text == *b.Text
	case a.Date != nil && b.Date != nil:
		return a.Date.Format(time.DateOnly) == b.Date.Format(time.DateOnly)
	case a.Bool != nil && b.Bool != nil:
		return *a.Bool == *b.Bool
	case a.OptionID != nil && b.OptionID != nil:
		return *a.OptionID == *b.OptionID
	}
	return false
}

// valueText: giá trị để đọc trong báo cáo (số không kèm đơn vị: đơn vị đứng sau cả "trước → sau")
func valueText(a domain.Attribute, v domain.Value, has bool) string {
	if !has {
		return "—"
	}
	switch {
	case v.Text != nil:
		return clip(*v.Text)
	case v.Number != nil:
		if f, err := strconv.ParseFloat(*v.Number, 64); err == nil {
			return strconv.FormatFloat(f, 'f', -1, 64)
		}
		return *v.Number
	case v.Date != nil:
		return v.Date.Format("2 Jan 2006")
	case v.Bool != nil:
		if *v.Bool {
			return "Yes"
		}
		return "No"
	case v.OptionID != nil:
		for _, o := range a.Options {
			if o.ID == *v.OptionID {
				return o.Label
			}
		}
	}
	return "—"
}

// importMessage đổi lỗi của việc kiểm tài sản thành một dòng cho người dùng: "CPU is required"
func importMessage(err error, t domain.AssetType) string {
	e := errs.NewFrom(err)
	if fields := e.Fields(); len(fields) > 0 {
		labels := map[string]string{}
		for _, a := range t.ActiveAttributes() {
			labels["attributes."+a.Key] = a.Label
		}
		parts := make([]string, len(fields))
		for i, f := range fields {
			name := f.Field
			if l, ok := labels[f.Field]; ok {
				name = l
			}
			parts[i] = name + " " + f.Detail
		}
		return strings.Join(parts, "; ")
	}
	if d := e.Detail(); d != "" {
		return d
	}
	return e.Title()
}
