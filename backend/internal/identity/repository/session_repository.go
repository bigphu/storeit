package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"storeit/internal/identity/domain"
	"storeit/internal/identity/repository/db"
	"storeit/internal/platform/database"
	"storeit/internal/platform/logger"
)

// SessionRepository lưu refresh token theo family (port từ mimir-2.0).
// Đăng nhập, refresh, đăng xuất không ghi event: chúng sẽ làm ngập lịch sử.
type SessionRepository struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

var _ domain.SessionRepository = (*SessionRepository)(nil)

func NewSessionRepository(pool *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{pool: pool, q: db.New(pool)}
}

func (r *SessionRepository) Start(ctx context.Context, s domain.NewSession) (uuid.UUID, error) {
	famID, err := newID()
	if err != nil {
		return uuid.Nil, err
	}
	tokID, err := newID()
	if err != nil {
		return uuid.Nil, err
	}
	err = database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		if err := q.CreateFamily(ctx, db.CreateFamilyParams{
			ID: famID, AccountID: s.AccountID, UserAgent: s.UserAgent, Ip: s.IP,
			AbsoluteExpiresAt: s.AbsoluteExpiresAt,
		}); err != nil {
			return fmt.Errorf("identity: create session: %w", err)
		}
		if err := q.CreateRefreshToken(ctx, db.CreateRefreshTokenParams{
			ID: tokID, FamilyID: famID, TokenHash: s.TokenHash, ExpiresAt: s.ExpiresAt,
		}); err != nil {
			return fmt.Errorf("identity: create refresh token: %w", err)
		}
		// bảng riêng: chỉ lấy KEY SHARE trên account (FK), không chờ khoá hàng account
		if err := q.RecordSignIn(ctx, s.AccountID); err != nil {
			return fmt.Errorf("identity: record sign-in: %w", err)
		}
		return nil
	})
	return famID, err
}

// Refresh khoá token và family (FOR UPDATE OF t, f), quyết định bằng
// domain.RefreshState.Decide rồi áp dụng, tất cả trong một transaction. Các
// nhánh thu hồi vẫn commit (fn trả nil) dù request bị từ chối: thu hồi phải
// được ghi lại.
func (r *SessionRepository) Refresh(ctx context.Context, in domain.RefreshInput) (domain.RefreshResult, error) {
	var res domain.RefreshResult
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		row, err := q.GetRefreshForUpdate(ctx, in.TokenHash)
		if errors.Is(err, pgx.ErrNoRows) {
			res = domain.RefreshResult{Decision: domain.RefreshReject}
			return nil
		}
		if err != nil {
			return fmt.Errorf("identity: load refresh token: %w", err)
		}
		state := domain.RefreshState{
			TokenID: row.TokenID, FamilyID: row.FamilyID, AccountID: row.AccountID,
			UsedAt: row.UsedAt, ExpiresAt: row.ExpiresAt, AbsoluteExpiresAt: row.AbsoluteExpiresAt,
			RevokedAt: row.RevokedAt, AccountActive: row.AccountActive,
		}
		res = domain.RefreshResult{
			Decision:  state.Decide(in.Now, in.Grace),
			AccountID: row.AccountID,
			FamilyID:  row.FamilyID,
		}

		target := row.TokenID
		switch res.Decision {
		case domain.RefreshReject:
			return nil
		case domain.RefreshAccountDisabled:
			return revoke(ctx, q, row.FamilyID, domain.RevokeAdmin)
		case domain.RefreshExpired:
			return revoke(ctx, q, row.FamilyID, domain.RevokeExpired)
		case domain.RefreshReuse:
			// Ghi family và account, không bao giờ ghi token
			logger.FromContext(ctx).WarnContext(ctx, "refresh token reuse detected, revoking session",
				slog.String("family_id", row.FamilyID.String()),
				slog.String("account_id", row.AccountID.String()))
			return revoke(ctx, q, row.FamilyID, domain.RevokeReuseDetected)
		case domain.RefreshGrace:
			// Không phát lại được token vừa nhận (chỉ có hash). Xoay ngọn của
			// family; tuyệt đối không tạo token anh em dưới token cũ, vì chẻ
			// family thành hai nhánh sống là tắt phát hiện dùng lại.
			tip, err := q.GetFamilyTip(ctx, row.FamilyID)
			if errors.Is(err, pgx.ErrNoRows) {
				res.Decision = domain.RefreshReject
				return nil
			}
			if err != nil {
				return fmt.Errorf("identity: family tip: %w", err)
			}
			target = tip
		}

		// Rotate hoặc Grace: đánh dấu target đã dùng rồi tạo token con
		n, err := q.MarkRefreshTokenUsed(ctx, db.MarkRefreshTokenUsedParams{ID: target, UsedAt: &in.Now})
		if err != nil {
			return fmt.Errorf("identity: mark token used: %w", err)
		}
		if n == 0 {
			// Khoá FOR UPDATE lẽ ra đã xếp hàng mọi request trên family; tới
			// đây là có ai ghi used_at ngoài đường khoá đó. Từ chối thay vì chẻ family.
			res.Decision = domain.RefreshReject
			return nil
		}
		expires := in.Now.Add(in.Sliding)
		if expires.After(row.AbsoluteExpiresAt) {
			expires = row.AbsoluteExpiresAt
		}
		id, err := newID()
		if err != nil {
			return err
		}
		if err := q.CreateRefreshToken(ctx, db.CreateRefreshTokenParams{
			ID: id, FamilyID: row.FamilyID, TokenHash: in.NextHash, ParentID: &target, ExpiresAt: expires,
		}); err != nil {
			return fmt.Errorf("identity: create refresh token: %w", err)
		}
		res.ExpiresAt = expires
		return nil
	})
	if err != nil {
		return domain.RefreshResult{}, err
	}
	return res, nil
}

func (r *SessionRepository) Revoke(ctx context.Context, tokenHash []byte, reason domain.RevokeReason) (*uuid.UUID, error) {
	fam, err := r.FamilyOf(ctx, tokenHash)
	if err != nil || fam == nil {
		return nil, err
	}
	if err := revoke(ctx, r.q, *fam, reason); err != nil {
		return nil, err
	}
	return fam, nil
}

func (r *SessionRepository) FamilyOf(ctx context.Context, tokenHash []byte) (*uuid.UUID, error) {
	fam, err := r.q.FamilyOfToken(ctx, tokenHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("identity: find session: %w", err)
	}
	return &fam, nil
}

func (r *SessionRepository) Prune(ctx context.Context, cutoff time.Time) (families, tokens int64, err error) {
	err = database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		if families, err = q.DeleteDeadFamilies(ctx, cutoff); err != nil {
			return fmt.Errorf("identity: prune families: %w", err)
		}
		if tokens, err = q.DeleteUsedTokens(ctx, cutoff); err != nil {
			return fmt.Errorf("identity: prune tokens: %w", err)
		}
		return nil
	})
	return families, tokens, err
}

func revoke(ctx context.Context, q *db.Queries, family uuid.UUID, reason domain.RevokeReason) error {
	if err := q.RevokeFamily(ctx, db.RevokeFamilyParams{ID: family, Reason: ptr(string(reason))}); err != nil {
		return fmt.Errorf("identity: revoke session: %w", err)
	}
	return nil
}
