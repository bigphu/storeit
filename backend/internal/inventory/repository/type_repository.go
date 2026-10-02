package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"storeit/internal/inventory/contract"
	"storeit/internal/inventory/domain"
	"storeit/internal/inventory/repository/db"
	"storeit/internal/platform/database"
	"storeit/internal/platform/events"
)

// TypeRepository cài đặt domain.TypeRepository: loại tài sản, thuộc tính, option.
// Mọi thay đổi ghi event asset_type_* trong cùng transaction.
type TypeRepository struct {
	pool   *pgxpool.Pool
	q      *db.Queries
	outbox *events.Outbox
}

var _ domain.TypeRepository = (*TypeRepository)(nil)

func NewTypeRepository(pool *pgxpool.Pool, outbox *events.Outbox) *TypeRepository {
	return &TypeRepository{pool: pool, q: db.New(pool), outbox: outbox}
}

func (r *TypeRepository) List(ctx context.Context, includeArchived bool) ([]domain.AssetType, error) {
	rows, err := r.q.ListAssetTypes(ctx, includeArchived)
	if err != nil {
		return nil, fmt.Errorf("inventory: list asset types: %w", err)
	}
	out := make([]domain.AssetType, len(rows))
	for i, row := range rows {
		out[i] = toAssetType(row)
	}
	return out, nil
}

func (r *TypeRepository) Get(ctx context.Context, id uuid.UUID) (domain.AssetType, error) {
	return loadType(ctx, r.q, id)
}

func (r *TypeRepository) Create(ctx context.Context, in domain.NewAssetType) (domain.AssetType, error) {
	id, err := newID()
	if err != nil {
		return domain.AssetType{}, err
	}
	var out domain.AssetType
	err = database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		if _, err := q.CreateAssetType(ctx, db.CreateAssetTypeParams{
			ID: id, Code: in.Code, Name: in.Name, Description: in.Description,
		}); err != nil {
			return mapWriteErr(err, "create asset type")
		}
		keys := make([]string, 0, len(in.Attributes))
		for _, a := range in.Attributes {
			if _, err := insertAttribute(ctx, q, id, a); err != nil {
				return err
			}
			keys = append(keys, a.Key)
		}
		if out, err = loadType(ctx, q, id); err != nil {
			return err
		}
		return appendEvent(ctx, tx, r.outbox, contract.EventAssetTypeCreated, contract.AggregateAssetType, id,
			contract.AssetTypeCreated{AssetTypeID: id, Code: in.Code, Name: in.Name, Attributes: keys})
	})
	return out, err
}

func (r *TypeRepository) Update(ctx context.Context, id uuid.UUID, name, description *string, version int32) (domain.AssetType, error) {
	var out domain.AssetType
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		cur, err := lockType(ctx, q, id)
		if err != nil {
			return err
		}
		if cur.Version != version {
			return domain.ErrAssetTypeChanged
		}
		newName, newDesc := cur.Name, cur.Description
		var changes []contract.FieldChange
		if name != nil && *name != cur.Name {
			changes = append(changes, change("name", cur.Name, *name))
			newName = *name
		}
		if description != nil && *description != cur.Description {
			changes = append(changes, change("description", cur.Description, *description))
			newDesc = *description
		}
		row, err := q.UpdateAssetType(ctx, db.UpdateAssetTypeParams{ID: id, Name: newName, Description: newDesc, Version: version})
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrAssetTypeChanged
		}
		if err != nil {
			return mapWriteErr(err, "update asset type")
		}
		if out, err = loadType(ctx, q, row.ID); err != nil {
			return err
		}
		return r.updated(ctx, tx, id, changes)
	})
	return out, err
}

func (r *TypeRepository) SetArchived(ctx context.Context, id uuid.UUID, archived bool) (domain.AssetType, error) {
	var out domain.AssetType
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		cur, err := lockType(ctx, q, id)
		if err != nil {
			return err
		}
		if cur.IsSystem && archived {
			return domain.ErrSystemType
		}
		if (cur.ArchivedAt != nil) == archived { // không đổi gì, không event
			out, err = loadType(ctx, q, id)
			return err
		}
		if _, err := q.SetAssetTypeArchived(ctx, db.SetAssetTypeArchivedParams{ID: id, Archived: archived}); err != nil {
			return fmt.Errorf("inventory: archive asset type: %w", err)
		}
		if out, err = loadType(ctx, q, id); err != nil {
			return err
		}
		if archived {
			return appendEvent(ctx, tx, r.outbox, contract.EventAssetTypeArchived, contract.AggregateAssetType, id,
				contract.AssetTypeArchived{AssetTypeID: id})
		}
		return appendEvent(ctx, tx, r.outbox, contract.EventAssetTypeRestored, contract.AggregateAssetType, id,
			contract.AssetTypeRestored{AssetTypeID: id})
	})
	return out, err
}

func (r *TypeRepository) AddAttribute(ctx context.Context, typeID uuid.UUID, in domain.NewAttribute) (domain.Attribute, error) {
	var out domain.Attribute
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		if _, err := lockType(ctx, q, typeID); err != nil {
			return err
		}
		a, err := insertAttribute(ctx, q, typeID, in)
		if err != nil {
			return err
		}
		out = a
		return r.updated(ctx, tx, typeID, []contract.FieldChange{change("attributes."+in.Key, nil, "added")})
	})
	return out, err
}

func (r *TypeRepository) UpdateAttribute(ctx context.Context, typeID, attrID uuid.UUID, ch domain.AttributeChange) (domain.Attribute, error) {
	var out domain.Attribute
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		cur, err := lockAttribute(ctx, q, typeID, attrID)
		if err != nil {
			return err
		}
		next := cur
		if ch.Label != nil {
			next.Label = *ch.Label
		}
		if ch.Required != nil {
			next.Required = *ch.Required
		}
		if ch.Position != nil {
			next.Position = *ch.Position
		}
		if ch.DataType != nil {
			next.DataType = *ch.DataType
		}
		if ch.Unit != nil {
			next.Unit = *ch.Unit
		}
		// Rời kiểu number thì đơn vị cũng đi theo
		if next.DataType != domain.TypeNumber {
			next.Unit = ""
		}
		if next.DataType != cur.DataType || next.Unit != cur.Unit {
			// Đổi kiểu hay đơn vị khi đã có giá trị sẽ đọc sai mọi giá trị cũ
			has, err := q.AttributeHasValues(ctx, attrID)
			if err != nil {
				return fmt.Errorf("inventory: attribute values: %w", err)
			}
			if has {
				return domain.ErrAttributeInUse
			}
		}
		if cur.DataType == domain.TypeSelect && next.DataType != domain.TypeSelect {
			if err := q.DeleteAttributeOptions(ctx, attrID); err != nil {
				return fmt.Errorf("inventory: delete options: %w", err)
			}
		}
		row, err := q.UpdateAttribute(ctx, db.UpdateAttributeParams{
			ID: attrID, Label: next.Label, Unit: nilIfEmpty(next.Unit), DataType: string(next.DataType),
			IsRequired: next.Required, Position: next.Position,
		})
		if err != nil {
			// Giá trị chen vào giữa lúc kiểm tra và lúc đổi kiểu: khoá ngoại chặn
			if code, _ := pgCode(err); code == codeForeignKeyViolation {
				return domain.ErrAttributeInUse
			}
			return mapWriteErr(err, "update attribute")
		}
		out = toAttribute(row)
		if out.Options, err = attributeOptions(ctx, q, typeID, attrID); err != nil {
			return err
		}
		return r.updated(ctx, tx, typeID, attributeChanges(cur, out))
	})
	return out, err
}

func (r *TypeRepository) RemoveAttribute(ctx context.Context, typeID, attrID uuid.UUID) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		cur, err := lockAttribute(ctx, q, typeID, attrID)
		if err != nil {
			return err
		}
		if _, err := q.RemoveAttribute(ctx, db.RemoveAttributeParams{ID: attrID, AssetTypeID: typeID}); err != nil {
			return fmt.Errorf("inventory: remove attribute: %w", err)
		}
		return r.updated(ctx, tx, typeID, []contract.FieldChange{change("attributes."+cur.Key, "active", "removed")})
	})
}

func (r *TypeRepository) AddOption(ctx context.Context, typeID, attrID uuid.UUID, label string, position int32) (domain.Option, error) {
	var out domain.Option
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		a, err := lockAttribute(ctx, q, typeID, attrID)
		if err != nil {
			return err
		}
		if a.DataType != domain.TypeSelect {
			return domain.ErrNotSelectAttribute
		}
		o, err := insertOption(ctx, q, attrID, label, position)
		if err != nil {
			return err
		}
		out = o
		return r.updated(ctx, tx, typeID, []contract.FieldChange{change("attributes."+a.Key+".options."+label, nil, "added")})
	})
	return out, err
}

func (r *TypeRepository) UpdateOption(ctx context.Context, typeID, attrID, optID uuid.UUID, label *string, position *int32) (domain.Option, error) {
	var out domain.Option
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		a, err := lockAttribute(ctx, q, typeID, attrID)
		if err != nil {
			return err
		}
		cur, err := lockOption(ctx, q, attrID, optID)
		if err != nil {
			return err
		}
		next := cur
		if label != nil {
			next.Label = *label
		}
		if position != nil {
			next.Position = *position
		}
		row, err := q.UpdateOption(ctx, db.UpdateOptionParams{ID: optID, Label: next.Label, Position: next.Position})
		if err != nil {
			return mapWriteErr(err, "update option")
		}
		out = toOption(row)
		var changes []contract.FieldChange
		if cur.Label != out.Label {
			changes = append(changes, change("attributes."+a.Key+".options."+cur.Label, cur.Label, out.Label))
		}
		if cur.Position != out.Position {
			changes = append(changes, change("attributes."+a.Key+".options."+out.Label+".position", cur.Position, out.Position))
		}
		return r.updated(ctx, tx, typeID, changes)
	})
	return out, err
}

func (r *TypeRepository) RemoveOption(ctx context.Context, typeID, attrID, optID uuid.UUID) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		a, err := lockAttribute(ctx, q, typeID, attrID)
		if err != nil {
			return err
		}
		o, err := lockOption(ctx, q, attrID, optID)
		if err != nil {
			return err
		}
		if _, err := q.RemoveOption(ctx, db.RemoveOptionParams{ID: optID, AttributeID: attrID}); err != nil {
			return fmt.Errorf("inventory: remove option: %w", err)
		}
		return r.updated(ctx, tx, typeID, []contract.FieldChange{change("attributes."+a.Key+".options."+o.Label, "active", "removed")})
	})
}

// updated ghi asset_type_updated nếu có thay đổi
func (r *TypeRepository) updated(ctx context.Context, tx pgx.Tx, typeID uuid.UUID, changes []contract.FieldChange) error {
	if len(changes) == 0 {
		return nil
	}
	return appendEvent(ctx, tx, r.outbox, contract.EventAssetTypeUpdated, contract.AggregateAssetType, typeID,
		contract.AssetTypeUpdated{AssetTypeID: typeID, Changes: changes})
}

func lockType(ctx context.Context, q *db.Queries, id uuid.UUID) (domain.AssetType, error) {
	row, err := q.GetAssetTypeForUpdate(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AssetType{}, domain.ErrTypeNotFound
	}
	if err != nil {
		return domain.AssetType{}, fmt.Errorf("inventory: lock asset type: %w", err)
	}
	return toAssetType(row), nil
}

// lockAttribute: thuộc tính của đúng loại và chưa gỡ, không thì ErrAttributeNotFound
func lockAttribute(ctx context.Context, q *db.Queries, typeID, attrID uuid.UUID) (domain.Attribute, error) {
	row, err := q.GetAttributeForUpdate(ctx, db.GetAttributeForUpdateParams{ID: attrID, AssetTypeID: typeID})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Attribute{}, domain.ErrAttributeNotFound
	}
	if err != nil {
		return domain.Attribute{}, fmt.Errorf("inventory: lock attribute: %w", err)
	}
	if row.RemovedAt != nil {
		return domain.Attribute{}, domain.ErrAttributeNotFound
	}
	return toAttribute(row), nil
}

func lockOption(ctx context.Context, q *db.Queries, attrID, optID uuid.UUID) (domain.Option, error) {
	row, err := q.GetOptionForUpdate(ctx, db.GetOptionForUpdateParams{ID: optID, AttributeID: attrID})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Option{}, domain.ErrOptionNotFound
	}
	if err != nil {
		return domain.Option{}, fmt.Errorf("inventory: lock option: %w", err)
	}
	if row.RemovedAt != nil {
		return domain.Option{}, domain.ErrOptionNotFound
	}
	return toOption(row), nil
}

func insertAttribute(ctx context.Context, q *db.Queries, typeID uuid.UUID, in domain.NewAttribute) (domain.Attribute, error) {
	id, err := newID()
	if err != nil {
		return domain.Attribute{}, err
	}
	row, err := q.CreateAttribute(ctx, db.CreateAttributeParams{
		ID: id, AssetTypeID: typeID, Key: in.Key, Label: in.Label, DataType: string(in.DataType),
		Unit: nilIfEmpty(in.Unit), IsRequired: in.Required, Position: in.Position,
	})
	if err != nil {
		return domain.Attribute{}, mapWriteErr(err, "create attribute")
	}
	a := toAttribute(row)
	for i, label := range in.Options {
		o, err := insertOption(ctx, q, id, label, int32(i+1))
		if err != nil {
			return domain.Attribute{}, err
		}
		a.Options = append(a.Options, o)
	}
	return a, nil
}

func insertOption(ctx context.Context, q *db.Queries, attrID uuid.UUID, label string, position int32) (domain.Option, error) {
	id, err := newID()
	if err != nil {
		return domain.Option{}, err
	}
	row, err := q.CreateOption(ctx, db.CreateOptionParams{ID: id, AttributeID: attrID, Label: label, Position: position})
	if err != nil {
		return domain.Option{}, mapWriteErr(err, "create option")
	}
	return toOption(row), nil
}

func attributeOptions(ctx context.Context, q *db.Queries, typeID, attrID uuid.UUID) ([]domain.Option, error) {
	rows, err := q.ListOptionsForType(ctx, typeID)
	if err != nil {
		return nil, fmt.Errorf("inventory: list options: %w", err)
	}
	var out []domain.Option
	for _, o := range rows {
		if o.AttributeID == attrID {
			out = append(out, toOption(o))
		}
	}
	return out, nil
}

func attributeChanges(cur, next domain.Attribute) []contract.FieldChange {
	prefix := "attributes." + cur.Key + "."
	var out []contract.FieldChange
	if cur.Label != next.Label {
		out = append(out, change(prefix+"label", cur.Label, next.Label))
	}
	if cur.DataType != next.DataType {
		out = append(out, change(prefix+"data_type", cur.DataType, next.DataType))
	}
	if cur.Unit != next.Unit {
		out = append(out, change(prefix+"unit", cur.Unit, next.Unit))
	}
	if cur.Required != next.Required {
		out = append(out, change(prefix+"is_required", cur.Required, next.Required))
	}
	if cur.Position != next.Position {
		out = append(out, change(prefix+"position", cur.Position, next.Position))
	}
	return out
}
