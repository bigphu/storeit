package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"storeit/internal/inventory/contract"
	"storeit/internal/inventory/domain"
	"storeit/internal/inventory/repository/db"
	"storeit/internal/platform/database"
	"storeit/internal/platform/events"
)

// AssetRepository cài đặt domain.AssetRepository. Tài sản không bao giờ bị xoá
// cứng; giá trị thuộc tính được ghi lại toàn bộ mỗi lần sửa.
type AssetRepository struct {
	pool   *pgxpool.Pool
	q      *db.Queries
	outbox *events.Outbox
}

var _ domain.AssetRepository = (*AssetRepository)(nil)

func NewAssetRepository(pool *pgxpool.Pool, outbox *events.Outbox) *AssetRepository {
	return &AssetRepository{pool: pool, q: db.New(pool), outbox: outbox}
}

func (r *AssetRepository) Create(ctx context.Context, tag string, f domain.AssetFields) (domain.Asset, error) {
	id, err := newID()
	if err != nil {
		return domain.Asset{}, err
	}
	var out domain.Asset
	err = database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		if _, err := q.CreateAsset(ctx, db.CreateAssetParams{
			ID: id, Tag: tag, Name: f.Name, Description: f.Description, AssetTypeID: f.TypeID, StatusID: f.StatusID,
			LocationID: f.LocationID, HolderMemberID: f.HolderMemberID, PurchaseDate: toPgDate(f.PurchaseDate),
		}); err != nil {
			return mapWriteErr(err, "create asset")
		}
		if err := insertValues(ctx, q, id, f.TypeID, f.Values); err != nil {
			return err
		}
		if out, err = loadAsset(ctx, q, id); err != nil {
			return err
		}
		return appendEvent(ctx, tx, r.outbox, contract.EventAssetCreated, contract.AggregateAsset, id,
			contract.AssetCreated{AssetID: id, Tag: tag, Name: f.Name, TypeID: f.TypeID, StatusID: f.StatusID})
	})
	return out, err
}

func (r *AssetRepository) Get(ctx context.Context, id uuid.UUID) (domain.Asset, error) {
	return loadAsset(ctx, r.q, id)
}

func (r *AssetRepository) List(ctx context.Context, f domain.AssetFilter) ([]domain.AssetListItem, int64, error) {
	var q *string
	if f.Query != "" {
		q = ptr(likeEscaper.Replace(f.Query))
	}
	var kind *string
	if f.StatusKind != nil {
		kind = ptr(string(*f.StatusKind))
	}
	sort := string(f.Sort)
	var sortAttr *uuid.UUID
	switch {
	case f.AttrOrder != nil:
		// khoá sắp theo kiểu dữ liệu: "attr_number", "-attr_date"...
		sort, sortAttr = "attr_"+string(f.AttrOrder.DataType), &f.AttrOrder.AttributeID
		if f.AttrOrder.Desc {
			sort = "-" + sort
		}
	case sort == "":
		sort = string(domain.SortTag)
	}
	fAttrs, fOps, fVals := attrFilterArgs(f.AttrFilters)
	rows, err := r.q.ListAssets(ctx, db.ListAssetsParams{
		Q: q, TypeID: f.TypeID, StatusID: f.StatusID, StatusKind: kind, LocationID: f.LocationID,
		HolderMemberID: f.HolderMemberID, IncludeRetired: f.IncludeRetired, FAttrs: fAttrs, FOps: fOps, FVals: fVals,
		SortAttr: sortAttr, Sort: sort, Lim: f.Limit, Off: f.Offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("inventory: list assets: %w", err)
	}
	total, err := r.q.CountAssets(ctx, db.CountAssetsParams{
		Q: q, TypeID: f.TypeID, StatusID: f.StatusID, StatusKind: kind, LocationID: f.LocationID,
		HolderMemberID: f.HolderMemberID, IncludeRetired: f.IncludeRetired, FAttrs: fAttrs, FOps: fOps, FVals: fVals,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("inventory: count assets: %w", err)
	}
	out := make([]domain.AssetListItem, len(rows))
	for i, row := range rows {
		out[i] = domain.AssetListItem{
			Asset: toAsset(db.InventoryAsset{
				ID: row.ID, Tag: row.Tag, Name: row.Name, Description: row.Description, AssetTypeID: row.AssetTypeID,
				StatusID: row.StatusID, LocationID: row.LocationID, HolderMemberID: row.HolderMemberID,
				PurchaseDate: row.PurchaseDate, RetiredAt: row.RetiredAt, RetiredReason: row.RetiredReason,
				Version: row.Version, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
			}),
			TypeName: row.TypeName, StatusName: row.StatusName, StatusKind: domain.StatusKind(row.StatusKind),
		}
	}
	if f.IncludeValues && len(out) > 0 {
		if err := r.attachValues(ctx, out); err != nil {
			return nil, 0, err
		}
	}
	return out, total, nil
}

// attrFilterArgs đổi điều kiện thành ba mảng song song cho ListAssets/CountAssets;
// toán tử SQL là "<kiểu>_<op>" ("number_gte", "select_in")
func attrFilterArgs(filters []domain.AttrFilter) (attrs []uuid.UUID, ops, vals []string) {
	attrs, ops, vals = []uuid.UUID{}, []string{}, []string{}
	for _, f := range filters {
		val := f.Value
		if f.Op == domain.OpContains {
			val = likeEscaper.Replace(val)
		}
		attrs = append(attrs, f.AttributeID)
		ops = append(ops, string(f.DataType)+"_"+string(f.Op))
		vals = append(vals, val)
	}
	return attrs, ops, vals
}

func (r *AssetRepository) CountByType(ctx context.Context) (map[uuid.UUID]domain.TypeCounts, error) {
	rows, err := r.q.CountActiveAssetsByTypeAndKind(ctx)
	if err != nil {
		return nil, fmt.Errorf("inventory: count assets by type: %w", err)
	}
	out := make(map[uuid.UUID]domain.TypeCounts)
	for _, row := range rows {
		c := out[row.AssetTypeID]
		if c.ByKind == nil {
			c.ByKind = map[domain.StatusKind]int64{}
		}
		c.Total += row.N
		c.ByKind[domain.StatusKind(row.Kind)] += row.N
		out[row.AssetTypeID] = c
	}
	return out, nil
}

func (r *AssetRepository) CountByStatus(ctx context.Context) (map[uuid.UUID]int64, error) {
	rows, err := r.q.CountAssetsByStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("inventory: count assets by status: %w", err)
	}
	out := make(map[uuid.UUID]int64, len(rows))
	for _, row := range rows {
		out[row.StatusID] = row.N
	}
	return out, nil
}

// attachValues nạp giá trị thuộc tính của mọi dòng trong một truy vấn
func (r *AssetRepository) attachValues(ctx context.Context, items []domain.AssetListItem) error {
	ids := make([]uuid.UUID, len(items))
	at := make(map[uuid.UUID]int, len(items))
	for i, it := range items {
		ids[i] = it.ID
		at[it.ID] = i
	}
	rows, err := r.q.ListValuesForAssets(ctx, ids)
	if err != nil {
		return fmt.Errorf("inventory: list values: %w", err)
	}
	for _, v := range rows {
		i := at[v.AssetID]
		items[i].Values = append(items[i].Values, toValue(db.ListAssetValuesRow{
			AttributeID: v.AttributeID, DataType: v.DataType, ValueText: v.ValueText, ValueNumber: v.ValueNumber,
			ValueDate: v.ValueDate, ValueBool: v.ValueBool, ValueOptionID: v.ValueOptionID,
		}))
	}
	return nil
}

func (r *AssetRepository) Replace(ctx context.Context, id uuid.UUID, f domain.AssetFields, version int32) (domain.Asset, error) {
	var out domain.Asset
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		cur, err := lockAsset(ctx, q, id, version)
		if err != nil {
			return err
		}
		oldValues, err := loadValues(ctx, q, id)
		if err != nil {
			return err
		}
		// Xoá giá trị trước khi đổi loại: khoá ngoại giữ giá trị đúng loại hiện tại
		if err := q.DeleteAssetValues(ctx, id); err != nil {
			return fmt.Errorf("inventory: delete values: %w", err)
		}
		if _, err := q.UpdateAsset(ctx, db.UpdateAssetParams{
			ID: id, Name: f.Name, Description: f.Description, AssetTypeID: f.TypeID, StatusID: f.StatusID,
			LocationID: f.LocationID, HolderMemberID: f.HolderMemberID, PurchaseDate: toPgDate(f.PurchaseDate),
		}); err != nil {
			return mapWriteErr(err, "update asset")
		}
		if err := insertValues(ctx, q, id, f.TypeID, f.Values); err != nil {
			return err
		}
		if out, err = loadAsset(ctx, q, id); err != nil {
			return err
		}
		keys, err := attributeKeys(ctx, q, cur.TypeID, f.TypeID)
		if err != nil {
			return err
		}
		changes := assetChanges(cur, out, oldValues, keys)
		if len(changes) == 0 {
			return nil
		}
		return appendEvent(ctx, tx, r.outbox, contract.EventAssetUpdated, contract.AggregateAsset, id,
			contract.AssetUpdated{AssetID: id, Changes: changes})
	})
	return out, err
}

func (r *AssetRepository) Retire(ctx context.Context, id uuid.UUID, reason string, retiredStatus uuid.UUID, version int32) (domain.Asset, error) {
	var out domain.Asset
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		if _, err := lockAsset(ctx, q, id, version); err != nil {
			return err
		}
		if _, err := q.RetireAsset(ctx, db.RetireAssetParams{ID: id, Reason: nilIfEmpty(reason), StatusID: retiredStatus}); err != nil {
			return fmt.Errorf("inventory: retire asset: %w", err)
		}
		var err error
		if out, err = loadAsset(ctx, q, id); err != nil {
			return err
		}
		return appendEvent(ctx, tx, r.outbox, contract.EventAssetRetired, contract.AggregateAsset, id,
			contract.AssetRetired{AssetID: id, Reason: reason})
	})
	return out, err
}

func (r *AssetRepository) Restore(ctx context.Context, id uuid.UUID, availableStatus uuid.UUID, version int32) (domain.Asset, error) {
	var out domain.Asset
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		cur, err := assetOrNotFound(q.GetAssetForUpdate(ctx, id))
		if err != nil {
			return err
		}
		if cur.Version != version {
			return domain.ErrAssetChanged
		}
		if !cur.Retired() { // chưa retire: không có gì để khôi phục
			out, err = loadAsset(ctx, q, id)
			return err
		}
		if _, err := q.RestoreAsset(ctx, db.RestoreAssetParams{ID: id, StatusID: availableStatus}); err != nil {
			return fmt.Errorf("inventory: restore asset: %w", err)
		}
		if out, err = loadAsset(ctx, q, id); err != nil {
			return err
		}
		return appendEvent(ctx, tx, r.outbox, contract.EventAssetRestored, contract.AggregateAsset, id,
			contract.AssetRestored{AssetID: id})
	})
	return out, err
}

// lockAsset khoá tài sản để sửa: không có thì ErrAssetNotFound, đã retire thì
// ErrAssetRetired, lệch version thì ErrAssetChanged
func lockAsset(ctx context.Context, q *db.Queries, id uuid.UUID, version int32) (domain.Asset, error) {
	cur, err := assetOrNotFound(q.GetAssetForUpdate(ctx, id))
	if err != nil {
		return domain.Asset{}, err
	}
	if cur.Retired() {
		return domain.Asset{}, domain.ErrAssetRetired
	}
	if cur.Version != version {
		return domain.Asset{}, domain.ErrAssetChanged
	}
	return cur, nil
}

func assetOrNotFound(row db.InventoryAsset, err error) (domain.Asset, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Asset{}, domain.ErrAssetNotFound
	}
	if err != nil {
		return domain.Asset{}, fmt.Errorf("inventory: get asset: %w", err)
	}
	return toAsset(row), nil
}

func loadAsset(ctx context.Context, q *db.Queries, id uuid.UUID) (domain.Asset, error) {
	a, err := assetOrNotFound(q.GetAsset(ctx, id))
	if err != nil {
		return domain.Asset{}, err
	}
	if a.Values, err = loadValues(ctx, q, id); err != nil {
		return domain.Asset{}, err
	}
	return a, nil
}

func loadValues(ctx context.Context, q *db.Queries, id uuid.UUID) ([]domain.Value, error) {
	rows, err := q.ListAssetValues(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("inventory: list values: %w", err)
	}
	out := make([]domain.Value, len(rows))
	for i, row := range rows {
		out[i] = toValue(row)
	}
	return out, nil
}

func insertValues(ctx context.Context, q *db.Queries, assetID, typeID uuid.UUID, values []domain.Value) error {
	for _, v := range values {
		if err := q.InsertAssetValue(ctx, db.InsertAssetValueParams{
			AssetID: assetID, AttributeID: v.AttributeID, AssetTypeID: typeID, DataType: string(v.DataType),
			ValueText: v.Text, ValueNumber: v.Number, ValueDate: toPgDate(v.Date), ValueBool: v.Bool, ValueOptionID: v.OptionID,
		}); err != nil {
			return fmt.Errorf("inventory: insert attribute value: %w", err)
		}
	}
	return nil
}

// assetChanges so trường chung và giá trị thuộc tính (theo key) trước và sau
func assetChanges(cur, next domain.Asset, oldValues []domain.Value, keys map[uuid.UUID]string) []contract.FieldChange {
	var out []contract.FieldChange
	add := func(field string, from, to any) { out = append(out, change(field, from, to)) }
	if cur.Name != next.Name {
		add("name", cur.Name, next.Name)
	}
	if cur.Description != next.Description {
		add("description", cur.Description, next.Description)
	}
	if cur.TypeID != next.TypeID {
		add("asset_type_id", cur.TypeID, next.TypeID)
	}
	if cur.StatusID != next.StatusID {
		add("status_id", cur.StatusID, next.StatusID)
	}
	if !sameID(cur.LocationID, next.LocationID) {
		add("location_id", cur.LocationID, next.LocationID)
	}
	if !sameID(cur.HolderMemberID, next.HolderMemberID) {
		add("holder_member_id", cur.HolderMemberID, next.HolderMemberID)
	}
	if dateString(cur.PurchaseDate) != dateString(next.PurchaseDate) {
		add("purchase_date", dateString(cur.PurchaseDate), dateString(next.PurchaseDate))
	}

	before := map[uuid.UUID]string{}
	for _, v := range oldValues {
		before[v.AttributeID] = valueString(v)
	}
	after := map[uuid.UUID]string{}
	for _, v := range next.Values {
		after[v.AttributeID] = valueString(v)
	}
	var ids []uuid.UUID
	for id := range before {
		ids = append(ids, id)
	}
	for id := range after {
		if _, ok := before[id]; !ok {
			ids = append(ids, id)
		}
	}
	// Thứ tự ổn định theo key
	sortByKey(ids, keys)
	for _, id := range ids {
		b, hadB := before[id]
		a, hasA := after[id]
		if hadB && hasA && a == b {
			continue
		}
		var from, to any
		if hadB {
			from = b
		}
		if hasA {
			to = a
		}
		add("attributes."+keys[id], from, to)
	}
	return out
}

func sortByKey(ids []uuid.UUID, keys map[uuid.UUID]string) {
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && keys[ids[j]] < keys[ids[j-1]]; j-- {
			ids[j], ids[j-1] = ids[j-1], ids[j]
		}
	}
}

func sameID(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func dateString(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.DateOnly)
}

// valueString: dạng chữ của giá trị cho event (option ghi id)
func valueString(v domain.Value) string {
	switch {
	case v.Text != nil:
		return *v.Text
	case v.Number != nil:
		return *v.Number
	case v.Date != nil:
		return v.Date.Format(time.DateOnly)
	case v.Bool != nil:
		if *v.Bool {
			return "true"
		}
		return "false"
	case v.OptionID != nil:
		return v.OptionID.String()
	}
	return ""
}
