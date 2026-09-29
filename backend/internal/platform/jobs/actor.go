package jobs

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"storeit/internal/platform/auth"
	"storeit/internal/platform/logger"
)

// ActorArgs nhúng vào args của job để job chạy dưới tên account đã yêu cầu nó:
//
//	type CommitImportArgs struct {
//		jobs.ActorArgs
//		ImportID uuid.UUID `json:"import_id"`
//	}
//
//	args := CommitImportArgs{ActorArgs: jobs.ActorArgsFrom(ctx), ImportID: im.ID}
type ActorArgs struct {
	// uuid.Nil là SystemActor
	ActorID uuid.UUID `json:"actor_id"`
}

func ActorArgsFrom(ctx context.Context) ActorArgs {
	return ActorArgs{ActorID: auth.ActorIDFrom(ctx)}
}

// ActorLoader nạp quyền hiện tại của account. identity cung cấp qua contract,
// cmd/worker truyền vào worker.
type ActorLoader func(ctx context.Context, accountID uuid.UUID) (auth.Actor, error)

// RestoreActor đặt lại actor vào ctx trước khi worker gọi service, để kiểm
// tra quyền và event ghi ra giống hệt khi chạy trong API. Quyền nạp lại lúc
// chạy (không lấy lúc enqueue), nên account bị bớt quyền thì job cũng mất
// quyền đó. accountID là uuid.Nil thì chạy bằng SystemActor.
func RestoreActor(ctx context.Context, accountID uuid.UUID, load ActorLoader) (context.Context, error) {
	if accountID == uuid.Nil {
		return auth.WithActor(ctx, auth.SystemActor), nil
	}
	actor, err := load(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("jobs: load actor %s: %w", accountID, err)
	}
	ctx = logger.With(ctx, slog.String("actor_id", accountID.String()))
	return auth.WithActor(ctx, actor), nil
}
