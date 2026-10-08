package repository

import (
	"context"
	"encoding/json"
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

// ExportProfileRepository cài đặt domain.ExportProfileRepository
type ExportProfileRepository struct {
	pool   *pgxpool.Pool
	q      *db.Queries
	outbox *events.Outbox
}

var _ domain.ExportProfileRepository = (*ExportProfileRepository)(nil)

func NewExportProfileRepository(pool *pgxpool.Pool, outbox *events.Outbox) *ExportProfileRepository {
	return &ExportProfileRepository{pool: pool, q: db.New(pool), outbox: outbox}
}

func (r *ExportProfileRepository) List(ctx context.Context, ownerID uuid.UUID) ([]domain.ExportProfile, error) {
	rows, err := r.q.ListExportProfiles(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("inventory: list export profiles: %w", err)
	}
	out := make([]domain.ExportProfile, 0, len(rows))
	for _, row := range rows {
		p, err := toExportProfile(row)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (r *ExportProfileRepository) Get(ctx context.Context, id uuid.UUID) (domain.ExportProfile, error) {
	return profileOrNotFound(r.q.GetExportProfile(ctx, id))
}

func (r *ExportProfileRepository) Create(ctx context.Context, in domain.NewExportProfile) (domain.ExportProfile, error) {
	id, err := newID()
	if err != nil {
		return domain.ExportProfile{}, err
	}
	layout, err := json.Marshal(in.Layout)
	if err != nil {
		return domain.ExportProfile{}, fmt.Errorf("inventory: encode layout: %w", err)
	}
	var out domain.ExportProfile
	err = database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		row, err := r.q.WithTx(tx).CreateExportProfile(ctx, db.CreateExportProfileParams{
			ID: id, OwnerID: in.OwnerID, Name: in.Name, Shared: in.Shared, Layout: layout,
		})
		if err != nil {
			return mapWriteErr(err, "create export profile")
		}
		if out, err = toExportProfile(row); err != nil {
			return err
		}
		return appendEvent(ctx, tx, r.outbox, contract.EventExportProfileCreated, contract.AggregateExportProfile, id,
			contract.ExportProfileCreated{ProfileID: id, Name: in.Name, Shared: in.Shared})
	})
	return out, err
}

func (r *ExportProfileRepository) Update(ctx context.Context, id uuid.UUID, ch domain.ExportProfileChange) (domain.ExportProfile, error) {
	var out domain.ExportProfile
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		cur, err := profileOrNotFound(q.GetExportProfileForUpdate(ctx, id))
		if err != nil {
			return err
		}
		if cur.Version != ch.Version {
			return domain.ErrExportProfileChanged
		}
		name, shared, layout := cur.Name, cur.Shared, cur.Layout
		var changes []contract.FieldChange
		if ch.Name != nil && *ch.Name != cur.Name {
			changes = append(changes, change("name", cur.Name, *ch.Name))
			name = *ch.Name
		}
		if ch.Shared != nil && *ch.Shared != cur.Shared {
			changes = append(changes, change("shared", cur.Shared, *ch.Shared))
			shared = *ch.Shared
		}
		if ch.Layout != nil {
			changes = append(changes, contract.FieldChange{Field: "layout"})
			layout = *ch.Layout
		}
		raw, err := json.Marshal(layout)
		if err != nil {
			return fmt.Errorf("inventory: encode layout: %w", err)
		}
		row, err := q.UpdateExportProfile(ctx, db.UpdateExportProfileParams{
			ID: id, Name: name, Shared: shared, Layout: raw, Version: ch.Version,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrExportProfileChanged
		}
		if err != nil {
			return mapWriteErr(err, "update export profile")
		}
		if out, err = toExportProfile(row); err != nil {
			return err
		}
		return appendEvent(ctx, tx, r.outbox, contract.EventExportProfileUpdated, contract.AggregateExportProfile, id,
			contract.ExportProfileUpdated{ProfileID: id, Changes: changes})
	})
	return out, err
}

func (r *ExportProfileRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		n, err := r.q.WithTx(tx).DeleteExportProfile(ctx, id)
		if err != nil {
			return fmt.Errorf("inventory: delete export profile: %w", err)
		}
		if n == 0 {
			return domain.ErrExportProfileNotFound
		}
		return appendEvent(ctx, tx, r.outbox, contract.EventExportProfileDeleted, contract.AggregateExportProfile, id,
			contract.ExportProfileDeleted{ProfileID: id})
	})
}

func (r *ExportProfileRepository) GetAny(ctx context.Context, id uuid.UUID) (domain.ExportProfile, error) {
	return profileOrNotFound(r.q.GetExportProfileAny(ctx, id))
}

func (r *ExportProfileRepository) Restore(ctx context.Context, id uuid.UUID, check func(domain.ExportProfile) error) (domain.ExportProfile, error) {
	var out domain.ExportProfile
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		cur, err := profileOrNotFound(q.GetExportProfileAnyForUpdate(ctx, id))
		if err != nil {
			return err
		}
		if check != nil {
			if err := check(cur); err != nil {
				return err
			}
		}
		if cur.DeletedAt == nil {
			out = cur
			return nil
		}
		row, err := q.RestoreExportProfile(ctx, id)
		if err != nil {
			return mapWriteErr(err, "restore export profile")
		}
		if out, err = toExportProfile(row); err != nil {
			return err
		}
		return appendEvent(ctx, tx, r.outbox, contract.EventExportProfileRestored, contract.AggregateExportProfile, id,
			contract.ExportProfileRestored{ProfileID: id})
	})
	return out, err
}

func (r *ExportProfileRepository) RecordExport(ctx context.Context, rec domain.ExportRecord) error {
	id, err := newID()
	if err != nil {
		return err
	}
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		return appendEvent(ctx, tx, r.outbox, contract.EventAssetsExported, contract.AggregateAssetExport, id,
			contract.AssetsExported{Mode: rec.Mode, ProfileID: rec.ProfileID, Rows: rec.Rows, Sheets: rec.Sheets, Filters: rec.Filters})
	})
}

func profileOrNotFound(row db.InventoryExportProfile, err error) (domain.ExportProfile, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ExportProfile{}, domain.ErrExportProfileNotFound
	}
	if err != nil {
		return domain.ExportProfile{}, fmt.Errorf("inventory: get export profile: %w", err)
	}
	return toExportProfile(row)
}

func toExportProfile(row db.InventoryExportProfile) (domain.ExportProfile, error) {
	var l domain.ExportLayout
	if err := json.Unmarshal(row.Layout, &l); err != nil {
		return domain.ExportProfile{}, fmt.Errorf("inventory: decode layout of profile %s: %w", row.ID, err)
	}
	return domain.ExportProfile{
		ID: row.ID, OwnerID: row.OwnerID, Name: row.Name, Shared: row.Shared, Layout: l,
		Version: row.Version, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, DeletedAt: row.DeletedAt,
	}, nil
}
