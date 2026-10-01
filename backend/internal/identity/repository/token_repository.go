package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"storeit/internal/identity/contract"
	"storeit/internal/identity/domain"
	"storeit/internal/identity/job"
	"storeit/internal/identity/repository/db"
	"storeit/internal/platform/auth"
	"storeit/internal/platform/database"
	"storeit/internal/platform/events"
	"storeit/internal/platform/jobs"
)

// TokenRepository cài đặt domain.TokenRepository: link đặt mật khẩu gửi qua
// email. Phát token luôn đi kèm job gửi thư trong cùng transaction, nên không
// có token nào mà thư không bao giờ được gửi (hay ngược lại).
type TokenRepository struct {
	pool   *pgxpool.Pool
	q      *db.Queries
	outbox *events.Outbox
	jobs   jobs.Enqueuer
}

var _ domain.TokenRepository = (*TokenRepository)(nil)

func NewTokenRepository(pool *pgxpool.Pool, outbox *events.Outbox, enq jobs.Enqueuer) *TokenRepository {
	return &TokenRepository{pool: pool, q: db.New(pool), outbox: outbox, jobs: enq}
}

func (r *TokenRepository) Issue(ctx context.Context, accountID uuid.UUID, t domain.IssuedToken, ev domain.TokenEvent) error {
	return database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		if err := issueToken(ctx, tx, r.q.WithTx(tx), r.jobs, accountID, t); err != nil {
			return err
		}
		switch ev {
		case domain.TokenEventInvitationResent:
			return appendAccountEvent(ctx, tx, r.outbox, contract.EventInvitationResent, accountID,
				contract.InvitationResent{AccountID: accountID})
		case domain.TokenEventResetSent:
			return appendAccountEvent(ctx, tx, r.outbox, contract.EventPasswordResetSent, accountID,
				contract.PasswordResetSent{AccountID: accountID})
		}
		return nil
	})
}

func (r *TokenRepository) Use(ctx context.Context, hash []byte, now time.Time, passwordHash string) (domain.UsedToken, error) {
	var out domain.UsedToken
	err := database.WithTx(ctx, r.pool, func(tx pgx.Tx) error {
		q := r.q.WithTx(tx)
		// Xoá trước rồi mới kiểm tra: hai lần gửi cùng lúc thì lần sau chờ khoá
		// hàng, rồi không thấy gì để xoá
		row, err := q.ConsumePasswordToken(ctx, hash)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrInvalidPasswordToken
		}
		if err != nil {
			return fmt.Errorf("identity: consume password token: %w", err)
		}
		acc, err := accountOrNotFound(q.GetAccountForUpdate(ctx, row.AccountID))
		if err != nil {
			return err
		}
		tok := toPasswordToken(row)
		// Không dùng được thì rollback: token hết hạn còn nằm đó tới lần dọn rác,
		// vô hại vì nó không bao giờ qua được Usable
		if err := tok.Usable(acc, now); err != nil {
			return err
		}
		if err := q.SetAccountPassword(ctx, db.SetAccountPasswordParams{ID: acc.ID, PasswordHash: &passwordHash}); err != nil {
			return fmt.Errorf("identity: set password: %w", err)
		}
		// Có mật khẩu rồi thì mọi link khác của account đều thừa
		if err := q.DeleteAccountPasswordTokens(ctx, acc.ID); err != nil {
			return fmt.Errorf("identity: delete password tokens: %w", err)
		}
		out = domain.UsedToken{AccountID: acc.ID, Purpose: tok.Purpose}
		if tok.Purpose == domain.PurposeReset {
			// Quên mật khẩu có thể vì mật khẩu đã lộ: đăng xuất mọi thiết bị
			if _, err := q.RevokeAccountFamilies(ctx, db.RevokeAccountFamiliesParams{
				AccountID: acc.ID, Reason: ptr(string(domain.RevokeAdmin)),
			}); err != nil {
				return fmt.Errorf("identity: revoke sessions: %w", err)
			}
			return nil
		}
		// Request không có actor (link công khai): người làm là chính account
		actx := auth.WithActor(ctx, auth.Actor{AccountID: acc.ID})
		return appendAccountEvent(actx, tx, r.outbox, contract.EventInvitationAccepted, acc.ID,
			contract.InvitationAccepted{AccountID: acc.ID})
	})
	return out, err
}

func (r *TokenRepository) Get(ctx context.Context, id uuid.UUID) (domain.PasswordToken, error) {
	row, err := r.q.GetPasswordToken(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PasswordToken{}, domain.ErrInvalidPasswordToken
	}
	if err != nil {
		return domain.PasswordToken{}, fmt.Errorf("identity: get password token: %w", err)
	}
	return toPasswordToken(row), nil
}

func (r *TokenRepository) Latest(ctx context.Context, accountID uuid.UUID, p domain.TokenPurpose) (*domain.PasswordToken, error) {
	row, err := r.q.GetAccountPasswordToken(ctx, db.GetAccountPasswordTokenParams{AccountID: accountID, Purpose: string(p)})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("identity: get password token: %w", err)
	}
	t := toPasswordToken(row)
	return &t, nil
}

func (r *TokenRepository) Prune(ctx context.Context, cutoff time.Time) (int64, error) {
	n, err := r.q.DeleteExpiredPasswordTokens(ctx, cutoff)
	if err != nil {
		return 0, fmt.Errorf("identity: prune password tokens: %w", err)
	}
	return n, nil
}

// issueToken ghi token (đè token cùng loại) và xếp job gửi thư, trong tx của
// người gọi. Dùng chung cho tạo account kèm lời mời và Issue.
func issueToken(ctx context.Context, tx pgx.Tx, q *db.Queries, enq jobs.Enqueuer, accountID uuid.UUID, t domain.IssuedToken) error {
	if !t.Purpose.Valid() {
		return fmt.Errorf("identity: invalid token purpose %q", t.Purpose)
	}
	_, err := q.UpsertPasswordToken(ctx, db.UpsertPasswordTokenParams{
		AccountID: accountID, Purpose: string(t.Purpose), ID: t.ID, TokenHash: t.Hash, ExpiresAt: t.ExpiresAt,
	})
	if err != nil {
		if code, _ := pgCode(err); code == codeForeignKeyViolation {
			return domain.ErrAccountNotFound
		}
		return fmt.Errorf("identity: issue password token: %w", err)
	}
	return enq.EnqueueTx(ctx, tx, job.SendAccountEmailArgs{TokenID: t.ID, Purpose: string(t.Purpose), Token: t.Raw})
}

// appendAccountEvent ghi event của aggregate account vào outbox trong tx
func appendAccountEvent(ctx context.Context, tx pgx.Tx, outbox *events.Outbox, typ string, id uuid.UUID, payload any) error {
	e, err := events.New(typ, contract.AggregateAccount, id, payload)
	if err != nil {
		return err
	}
	return outbox.Append(ctx, tx, e)
}
