package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"storeit/internal/inventory/domain"
	"storeit/internal/inventory/repository/db"
	"storeit/internal/platform/errs"
)

// ImportRepository cài đặt domain.ImportRepository: lịch sử các lần import tài sản từ Excel
type ImportRepository struct {
	q *db.Queries
}

var _ domain.ImportRepository = (*ImportRepository)(nil)

func NewImportRepository(pool *pgxpool.Pool) *ImportRepository {
	return &ImportRepository{q: db.New(pool)}
}

// Claim đặt chỗ cho lần import. File có dòng tạo mới mà đã giữ chỗ trước đó (kể cả lần bị ngắt
// giữa chừng, đã tạo một phần tài sản) là ErrImportAlreadyDone kèm ngày giờ lần trước.
func (r *ImportRepository) Claim(ctx context.Context, rec domain.ImportRecord) (uuid.UUID, error) {
	id, err := newID()
	if err != nil {
		return uuid.Nil, err
	}
	err = r.q.ClaimImport(ctx, db.ClaimImportParams{
		ID: id, AccountID: rec.AccountID, FileName: rec.FileName, FileSha256: rec.SHA256,
		HasCreates: rec.HasCreates, RowCount: int32(rec.Rows),
	})
	if err == nil {
		return id, nil
	}
	mapped := mapWriteErr(err, "claim import")
	if !errors.Is(mapped, domain.ErrImportAlreadyDone) {
		return uuid.Nil, mapped
	}
	prev, getErr := r.q.GetCreatingImportBySHA(ctx, rec.SHA256)
	if errors.Is(getErr, pgx.ErrNoRows) {
		return uuid.Nil, domain.ErrImportAlreadyDone
	}
	if getErr != nil {
		return uuid.Nil, fmt.Errorf("inventory: read earlier import: %w", getErr)
	}
	return uuid.Nil, domain.ErrImportAlreadyDone.With(errs.WithDetailf(
		"%s was imported on %s. Importing it again would create the new assets twice. Remove the rows that were imported, or change the file, and try again.",
		prev.FileName, prev.CreatedAt.UTC().Format("2 Jan 2006 15:04 UTC")))
}

func (r *ImportRepository) Finish(ctx context.Context, id uuid.UUID, c domain.ImportCounts) error {
	if err := r.q.FinishImport(ctx, db.FinishImportParams{
		ID: id, CreatedCount: int32(c.Create), UpdatedCount: int32(c.Update), SkippedCount: int32(c.Skip), FailedCount: int32(c.Fail),
	}); err != nil {
		return fmt.Errorf("inventory: finish import: %w", err)
	}
	return nil
}
