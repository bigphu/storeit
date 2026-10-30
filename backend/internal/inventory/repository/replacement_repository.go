package repository

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"storeit/internal/inventory/contract"
	"storeit/internal/inventory/domain"
	"storeit/internal/inventory/repository/db"
	"storeit/internal/platform/auth"
	"storeit/internal/platform/database"
	"storeit/internal/platform/errs"
)

// Thay thế tài sản (US-13). Ghi và hoàn tác đều trong một transaction, khoá hai tài sản theo
// thứ tự ID để hai lần thay chéo nhau không deadlock.

func invalidReplacement(field, detail string) error {
	return domain.ErrInvalidReplacement.With(errs.WithFields(errs.FieldError{Field: field, Detail: detail}))
}

// RecordReplacement: oldID được thay bằng newID; tài sản cũ retire với lý do "Replaced by …"
func (r *AssetRepository) RecordReplacement(ctx context.Context, oldID, newID uuid.UUID, replacedOn time.Time, reason string, retiredStatus uuid.UUID, version int32) (domain.Asset, error) {
	var out domain.Asset
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		locked, err := lockAssets(ctx, q, oldID, newID)
		if err != nil {
			return err
		}
		old, ok := locked[oldID]
		if !ok {
			return domain.ErrAssetNotFound
		}
		if old.Retired() {
			return domain.ErrAssetRetired
		}
		if old.Version != version {
			return domain.ErrAssetChanged
		}
		if domain.IsBorrowingStatus(old.StatusID) {
			return domain.ErrAssetInBorrowing
		}
		if newID == oldID {
			return invalidReplacement("replacement_asset_id", "can't be the same asset")
		}
		repl, ok := locked[newID]
		if !ok {
			return invalidReplacement("replacement_asset_id", "unknown asset")
		}
		if repl.Retired() {
			return invalidReplacement("replacement_asset_id", "is retired")
		}
		if err := domain.CheckReplacedAfterPurchase(replacedOn, old, repl); err != nil {
			return err
		}

		day := replacedOn
		if err := q.InsertReplacement(ctx, db.InsertReplacementParams{
			OldAssetID: oldID, NewAssetID: newID, ReplacedOn: toPgDate(&day), Reason: reason, CreatedBy: actorID(ctx),
		}); err != nil {
			return fmt.Errorf("inventory: insert replacement: %w", err)
		}
		if _, err := q.RetireAsset(ctx, db.RetireAssetParams{
			ID: oldID, Reason: nilIfEmpty(domain.ReplacementRetireReason(repl.Tag, reason)), StatusID: retiredStatus,
		}); err != nil {
			return fmt.Errorf("inventory: retire replaced asset: %w", err)
		}
		if out, err = loadAsset(ctx, q, oldID); err != nil {
			return err
		}
		on := replacedOn.Format(time.DateOnly)
		if err := appendEvent(ctx, tx, r.outbox, contract.EventAssetReplaced, contract.AggregateAsset, oldID, contract.AssetReplaced{
			AssetID: oldID, ReplacementID: newID, ReplacementTag: repl.Tag, ReplacedOn: on, Reason: reason,
		}); err != nil {
			return err
		}
		return appendEvent(ctx, tx, r.outbox, contract.EventAssetReplaces, contract.AggregateAsset, newID, contract.AssetReplaces{
			AssetID: newID, ReplacedID: oldID, ReplacedTag: old.Tag, ReplacedOn: on, Reason: reason,
		})
	})
	return out, err
}

// UndoReplacement: bỏ liên kết, khôi phục tài sản cũ; không có liên kết thì ErrAssetNotReplaced
func (r *AssetRepository) UndoReplacement(ctx context.Context, oldID uuid.UUID, availableStatus uuid.UUID, version int32) (domain.Asset, error) {
	return r.restoreOrUndo(ctx, oldID, availableStatus, version, true)
}

// restoreOrUndo: khôi phục tài sản đã retire. Tài sản bị thay thì khôi phục là hoàn tác (bỏ
// liên kết, event replacement_undone ở cả hai) để không có tài sản đang dùng mà vẫn ghi là
// đã bị thay. mustBeReplaced: không có liên kết thì ErrAssetNotReplaced (UndoReplacement)
func (r *AssetRepository) restoreOrUndo(ctx context.Context, id uuid.UUID, availableStatus uuid.UUID, version int32, mustBeReplaced bool) (domain.Asset, error) {
	var out domain.Asset
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		link, err := q.GetReplacementByOld(ctx, id)
		hasLink := err == nil
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("inventory: get replacement: %w", err)
		}
		ids := []uuid.UUID{id}
		if hasLink {
			ids = append(ids, link.NewAssetID)
		}
		locked, err := lockAssets(ctx, q, ids...)
		if err != nil {
			return err
		}
		cur, ok := locked[id]
		if !ok {
			return domain.ErrAssetNotFound
		}
		if cur.Version != version {
			return domain.ErrAssetChanged
		}

		if !hasLink {
			if mustBeReplaced {
				return domain.ErrAssetNotReplaced
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
		}

		if err := q.DeleteReplacement(ctx, id); err != nil {
			return fmt.Errorf("inventory: delete replacement: %w", err)
		}
		if _, err := q.RestoreAsset(ctx, db.RestoreAssetParams{ID: id, StatusID: availableStatus}); err != nil {
			return fmt.Errorf("inventory: restore replaced asset: %w", err)
		}
		if out, err = loadAsset(ctx, q, id); err != nil {
			return err
		}
		if err := appendEvent(ctx, tx, r.outbox, contract.EventAssetReplacementUndone, contract.AggregateAsset, id, contract.AssetReplacementUndone{
			AssetID: id, OtherID: link.NewAssetID, OtherTag: link.NewTag, Role: "old",
		}); err != nil {
			return err
		}
		return appendEvent(ctx, tx, r.outbox, contract.EventAssetReplacementUndone, contract.AggregateAsset, link.NewAssetID, contract.AssetReplacementUndone{
			AssetID: link.NewAssetID, OtherID: id, OtherTag: cur.Tag, Role: "new",
		})
	})
	return out, err
}

// Replacements: tài sản thay id (nil nếu không có) và các tài sản id đã thay, mới nhất trước
func (r *AssetRepository) Replacements(ctx context.Context, id uuid.UUID) (*domain.ReplacementLink, []domain.ReplacementLink, error) {
	var by *domain.ReplacementLink
	row, err := r.q.GetReplacementByOld(ctx, id)
	switch {
	case err == nil:
		by = &domain.ReplacementLink{AssetID: row.NewAssetID, Tag: row.NewTag, Name: row.NewName, ReplacedOn: dateOf(row.ReplacedOn), Reason: row.Reason}
	case !errors.Is(err, pgx.ErrNoRows):
		return nil, nil, fmt.Errorf("inventory: get replacement: %w", err)
	}
	rows, err := r.q.ListReplacementsByNew(ctx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("inventory: list replacements: %w", err)
	}
	replaces := make([]domain.ReplacementLink, len(rows))
	for i, x := range rows {
		replaces[i] = domain.ReplacementLink{AssetID: x.OldAssetID, Tag: x.OldTag, Name: x.OldName, ReplacedOn: dateOf(x.ReplacedOn), Reason: x.Reason}
	}
	return by, replaces, nil
}

// lockAssets khoá các tài sản theo thứ tự ID tăng dần (FOR UPDATE); tài sản không có thì
// vắng trong map
func lockAssets(ctx context.Context, q *db.Queries, ids ...uuid.UUID) (map[uuid.UUID]domain.Asset, error) {
	sorted := slices.Clone(ids)
	slices.SortFunc(sorted, func(a, b uuid.UUID) int { return bytes.Compare(a[:], b[:]) })
	sorted = slices.Compact(sorted)
	out := make(map[uuid.UUID]domain.Asset, len(sorted))
	for _, id := range sorted {
		a, err := assetOrNotFound(q.GetAssetForUpdate(ctx, id))
		if errors.Is(err, domain.ErrAssetNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out[id] = a
	}
	return out, nil
}

// actorID: account đang thao tác, nil với hệ thống
func actorID(ctx context.Context) *uuid.UUID {
	a, ok := auth.FromContext(ctx)
	if !ok || a.IsSystem() || a.AccountID == uuid.Nil {
		return nil
	}
	id := a.AccountID
	return &id
}

func dateOf(d pgtype.Date) time.Time {
	if t := fromPgDate(d); t != nil {
		return *t
	}
	return time.Time{}
}
