// Package repository là cài đặt Postgres của các interface trong
// inventory/domain: sqlc (repository/db), transaction và event vào outbox.
package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"storeit/internal/inventory/contract"
	"storeit/internal/inventory/domain"
	"storeit/internal/inventory/repository/db"
	"storeit/internal/platform/events"
)

const (
	codeUniqueViolation     = "23505"
	codeForeignKeyViolation = "23503"
)

// pgCode trả mã lỗi Postgres và tên constraint, rỗng nếu không phải lỗi Postgres
func pgCode(err error) (code, constraint string) {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return pe.Code, pe.ConstraintName
	}
	return "", ""
}

// uniqueErrors: tên constraint/index duy nhất -> lỗi domain
var uniqueErrors = map[string]error{
	"assets_tag_key":                      domain.ErrTagTaken,
	"asset_types_code_key":                domain.ErrTypeCodeTaken,
	"asset_types_name_lower":              domain.ErrTypeNameTaken,
	"asset_type_attributes_key_key":       domain.ErrAttributeKeyTaken,
	"asset_type_attributes_label_lower":   domain.ErrAttributeLabelTaken,
	"asset_attribute_options_label_lower": domain.ErrOptionLabelTaken,
	"asset_statuses_name_lower":           domain.ErrStatusNameTaken,
}

// mapWriteErr đổi lỗi trùng của Postgres sang lỗi domain; lỗi khác bọc kèm what
func mapWriteErr(err error, what string) error {
	if code, constraint := pgCode(err); code == codeUniqueViolation {
		if e, ok := uniqueErrors[constraint]; ok {
			return e
		}
	}
	return fmt.Errorf("inventory: %s: %w", what, err)
}

func newID() (uuid.UUID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, fmt.Errorf("inventory: new id: %w", err)
	}
	return id, nil
}

func ptr[T any](v T) *T { return &v }

func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

// nilIfEmpty: chuỗi rỗng ghi NULL
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func toPgDate(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: *t, Valid: true}
}

func fromPgDate(d pgtype.Date) *time.Time {
	if !d.Valid {
		return nil
	}
	t := d.Time
	return &t
}

// likeEscaper: % _ và dấu gạch ngược người dùng gõ là chữ, không phải ký tự đại diện của ILIKE
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// appendEvent ghi event vào outbox trong tx (cùng transaction với thay đổi)
func appendEvent(ctx context.Context, tx pgx.Tx, outbox *events.Outbox, typ, aggregate string, id uuid.UUID, payload any) error {
	e, err := events.New(typ, aggregate, id, payload)
	if err != nil {
		return err
	}
	return outbox.Append(ctx, tx, e)
}

func toAssetType(t db.InventoryAssetType) domain.AssetType {
	return domain.AssetType{
		ID: t.ID, Code: t.Code, Name: t.Name, Description: t.Description, IsSystem: t.IsSystem,
		ArchivedAt: t.ArchivedAt, Version: t.Version, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
}

func toAttribute(a db.InventoryAssetTypeAttribute) domain.Attribute {
	return domain.Attribute{
		ID: a.ID, TypeID: a.AssetTypeID, Key: a.Key, Label: a.Label, DataType: domain.DataType(a.DataType),
		Unit: deref(a.Unit), Required: a.IsRequired, Position: a.Position, RemovedAt: a.RemovedAt,
	}
}

func toOption(o db.InventoryAssetAttributeOption) domain.Option {
	return domain.Option{ID: o.ID, AttributeID: o.AttributeID, Label: o.Label, Position: o.Position, RemovedAt: o.RemovedAt}
}

func toStatus(s db.InventoryAssetStatus) domain.Status {
	return domain.Status{
		ID: s.ID, Name: s.Name, Kind: domain.StatusKind(s.Kind), IsDefault: s.IsDefault, IsSystem: s.IsSystem,
		Position: s.Position, ArchivedAt: s.ArchivedAt, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
	}
}

func toAsset(a db.InventoryAsset) domain.Asset {
	return domain.Asset{
		ID: a.ID, Tag: a.Tag, Name: a.Name, Description: a.Description, TypeID: a.AssetTypeID, StatusID: a.StatusID,
		LocationID: a.LocationID, HolderMemberID: a.HolderMemberID, PurchaseDate: fromPgDate(a.PurchaseDate),
		RetiredAt: a.RetiredAt, RetiredReason: deref(a.RetiredReason), Version: a.Version,
		CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
}

func toValue(v db.ListAssetValuesRow) domain.Value {
	out := domain.Value{
		AttributeID: v.AttributeID, DataType: domain.DataType(v.DataType),
		Text: v.ValueText, Date: fromPgDate(v.ValueDate), Bool: v.ValueBool, OptionID: v.ValueOptionID,
	}
	if v.ValueNumber != "" {
		out.Number = &v.ValueNumber
	}
	return out
}

// loadType đọc loại kèm mọi thuộc tính (kể cả đã gỡ) và option, theo thứ tự hiển thị
func loadType(ctx context.Context, q *db.Queries, id uuid.UUID) (domain.AssetType, error) {
	row, err := q.GetAssetType(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AssetType{}, domain.ErrTypeNotFound
	}
	if err != nil {
		return domain.AssetType{}, fmt.Errorf("inventory: get asset type: %w", err)
	}
	t := toAssetType(row)
	attrs, err := q.ListAttributes(ctx, id)
	if err != nil {
		return domain.AssetType{}, fmt.Errorf("inventory: list attributes: %w", err)
	}
	opts, err := q.ListOptionsForType(ctx, id)
	if err != nil {
		return domain.AssetType{}, fmt.Errorf("inventory: list options: %w", err)
	}
	byAttr := map[uuid.UUID][]domain.Option{}
	for _, o := range opts {
		byAttr[o.AttributeID] = append(byAttr[o.AttributeID], toOption(o))
	}
	t.Attributes = make([]domain.Attribute, len(attrs))
	for i, a := range attrs {
		t.Attributes[i] = toAttribute(a)
		t.Attributes[i].Options = byAttr[a.ID]
	}
	return t, nil
}

// attributeKeys: id thuộc tính -> key, cho event của tài sản
func attributeKeys(ctx context.Context, q *db.Queries, typeIDs ...uuid.UUID) (map[uuid.UUID]string, error) {
	out := map[uuid.UUID]string{}
	for _, id := range typeIDs {
		attrs, err := q.ListAttributes(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("inventory: list attributes: %w", err)
		}
		for _, a := range attrs {
			out[a.ID] = a.Key
		}
	}
	return out, nil
}

// event giữ thứ tự field ổn định để activity hiện đúng
func change(field string, from, to any) contract.FieldChange {
	return contract.FieldChange{Field: field, From: from, To: to}
}
