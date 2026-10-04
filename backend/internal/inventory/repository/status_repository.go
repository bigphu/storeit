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

// StatusRepository cài đặt domain.StatusRepository
type StatusRepository struct {
	pool   *pgxpool.Pool
	q      *db.Queries
	outbox *events.Outbox
}

var _ domain.StatusRepository = (*StatusRepository)(nil)

func NewStatusRepository(pool *pgxpool.Pool, outbox *events.Outbox) *StatusRepository {
	return &StatusRepository{pool: pool, q: db.New(pool), outbox: outbox}
}

func (r *StatusRepository) List(ctx context.Context, includeArchived bool) ([]domain.Status, error) {
	rows, err := r.q.ListStatuses(ctx, includeArchived)
	if err != nil {
		return nil, fmt.Errorf("inventory: list statuses: %w", err)
	}
	out := make([]domain.Status, len(rows))
	for i, row := range rows {
		out[i] = toStatus(row)
	}
	return out, nil
}

func (r *StatusRepository) Get(ctx context.Context, id uuid.UUID) (domain.Status, error) {
	return statusOrNotFound(r.q.GetStatus(ctx, id))
}

func (r *StatusRepository) Default(ctx context.Context, kind domain.StatusKind) (domain.Status, error) {
	row, err := r.q.GetDefaultStatus(ctx, string(kind))
	if err != nil {
		return domain.Status{}, fmt.Errorf("inventory: default status for %s: %w", kind, err)
	}
	return toStatus(row), nil
}

func (r *StatusRepository) Create(ctx context.Context, in domain.NewStatus) (domain.Status, error) {
	id, err := newID()
	if err != nil {
		return domain.Status{}, err
	}
	var out domain.Status
	err = database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		row, err := r.q.WithTx(tx).CreateStatus(ctx, db.CreateStatusParams{ID: id, Name: in.Name, Kind: string(in.Kind), Position: in.Position})
		if err != nil {
			return mapWriteErr(err, "create status")
		}
		out = toStatus(row)
		return appendEvent(ctx, tx, r.outbox, contract.EventStatusCreated, contract.AggregateStatus, id,
			contract.StatusCreated{StatusID: id, Name: in.Name, Kind: string(in.Kind)})
	})
	return out, err
}

func (r *StatusRepository) Update(ctx context.Context, id uuid.UUID, ch domain.StatusChange) (domain.Status, error) {
	var out domain.Status
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		cur, err := statusOrNotFound(q.GetStatusForUpdate(ctx, id))
		if err != nil {
			return err
		}
		name, position := cur.Name, cur.Position
		var changes []contract.FieldChange
		if ch.Name != nil && *ch.Name != cur.Name {
			changes = append(changes, change("name", cur.Name, *ch.Name))
			name = *ch.Name
		}
		if ch.Position != nil && *ch.Position != cur.Position {
			changes = append(changes, change("position", cur.Position, *ch.Position))
			position = *ch.Position
		}
		row, err := q.UpdateStatus(ctx, db.UpdateStatusParams{ID: id, Name: name, Position: position})
		if err != nil {
			return mapWriteErr(err, "update status")
		}
		if ch.MakeDefault && !cur.IsDefault {
			if cur.Archived() {
				return domain.ErrStatusArchived
			}
			// Mỗi kind đúng một mặc định: bỏ cờ cũ rồi đặt cờ mới, cùng transaction
			if err := q.ClearDefaultStatus(ctx, string(cur.Kind)); err != nil {
				return fmt.Errorf("inventory: clear default status: %w", err)
			}
			if row, err = q.SetDefaultStatus(ctx, id); err != nil {
				return fmt.Errorf("inventory: set default status: %w", err)
			}
			changes = append(changes, change("is_default", false, true))
		}
		out = toStatus(row)
		if len(changes) == 0 {
			return nil
		}
		return appendEvent(ctx, tx, r.outbox, contract.EventStatusUpdated, contract.AggregateStatus, id,
			contract.StatusUpdated{StatusID: id, Changes: changes})
	})
	return out, err
}

func (r *StatusRepository) Archive(ctx context.Context, id uuid.UUID) (domain.Status, error) {
	var out domain.Status
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		cur, err := statusOrNotFound(q.GetStatusForUpdate(ctx, id))
		if err != nil {
			return err
		}
		switch {
		case cur.IsSystem:
			return domain.ErrSystemStatus
		case cur.IsDefault:
			return domain.ErrStatusIsDefault
		case cur.Archived():
			out = cur
			return nil
		}
		row, err := q.ArchiveStatus(ctx, id)
		if err != nil {
			return fmt.Errorf("inventory: archive status: %w", err)
		}
		out = toStatus(row)
		return appendEvent(ctx, tx, r.outbox, contract.EventStatusArchived, contract.AggregateStatus, id,
			contract.StatusArchived{StatusID: id})
	})
	return out, err
}

func (r *StatusRepository) Restore(ctx context.Context, id uuid.UUID) (domain.Status, error) {
	var out domain.Status
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		cur, err := statusOrNotFound(q.GetStatusForUpdate(ctx, id))
		if err != nil {
			return err
		}
		if !cur.Archived() {
			out = cur
			return nil
		}
		row, err := q.RestoreStatus(ctx, id)
		if err != nil {
			return fmt.Errorf("inventory: restore status: %w", err)
		}
		out = toStatus(row)
		return appendEvent(ctx, tx, r.outbox, contract.EventStatusRestored, contract.AggregateStatus, id,
			contract.StatusRestored{StatusID: id})
	})
	return out, err
}

func (r *StatusRepository) Reorder(ctx context.Context, ids []uuid.UUID) ([]domain.Status, error) {
	var out []domain.Status
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		// khoá mọi status đang dùng: sửa hay archive cùng lúc phải đợi
		rows, err := q.ListActiveStatusesForUpdate(ctx)
		if err != nil {
			return fmt.Errorf("inventory: list statuses: %w", err)
		}
		cur := make(map[uuid.UUID]db.InventoryAssetStatus, len(rows))
		for _, row := range rows {
			cur[row.ID] = row
		}
		if !sameIDs(ids, cur) {
			return domain.ErrInvalidOrder
		}
		out = make([]domain.Status, len(ids))
		for i, id := range ids {
			row, pos := cur[id], int32(i+1)
			row.Position = pos
			out[i] = toStatus(row)
			if cur[id].Position == pos {
				continue
			}
			if err := q.SetStatusPosition(ctx, db.SetStatusPositionParams{ID: id, Position: pos}); err != nil {
				return fmt.Errorf("inventory: set status position: %w", err)
			}
			if err := appendEvent(ctx, tx, r.outbox, contract.EventStatusUpdated, contract.AggregateStatus, id,
				contract.StatusUpdated{StatusID: id, Changes: []contract.FieldChange{change("position", cur[id].Position, pos)}}); err != nil {
				return err
			}
		}
		return nil
	})
	return out, err
}

func statusOrNotFound(row db.InventoryAssetStatus, err error) (domain.Status, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Status{}, domain.ErrStatusNotFound
	}
	if err != nil {
		return domain.Status{}, fmt.Errorf("inventory: get status: %w", err)
	}
	return toStatus(row), nil
}
