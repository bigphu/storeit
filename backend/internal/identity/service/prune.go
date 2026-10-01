package service

import (
	"context"
	"log/slog"

	"storeit/internal/platform/logger"
)

// PruneResult là số hàng đã xoá, để job ghi log: bảng phình ra thì còn thấy
// được job có chạy hay không
type PruneResult struct {
	Families int64
	Tokens   int64
}

// PruneSessions xoá family đã chết và token đã dùng cũ hơn Retention. Không có
// nó refresh_tokens tăng một hàng mỗi lần refresh, mãi mãi. Mốc cắt tính một
// lần ở đây, bằng cùng đồng hồ đã ghi used_at.
func (s *Service) PruneSessions(ctx context.Context) (PruneResult, error) {
	cutoff := s.now().Add(-s.settings.Retention)
	families, tokens, err := s.sessions.Prune(ctx, cutoff)
	if err != nil {
		return PruneResult{}, err
	}
	logger.FromContext(ctx).InfoContext(ctx, "pruned expired sessions",
		slog.Int64("families", families), slog.Int64("tokens", tokens))
	return PruneResult{Families: families, Tokens: tokens}, nil
}
