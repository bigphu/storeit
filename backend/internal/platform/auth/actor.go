package auth

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"storeit/internal/platform/errs"
)

// Actor là người (hoặc hệ thống) đang thực hiện thao tác
type Actor struct {
	AccountID   uuid.UUID
	Permissions []string

	// Chỉ SystemActor có system = true; field ẩn nên không dựng tay được
	system bool
}

// SystemActor chạy scheduled job và việc không do ai yêu cầu. Có mọi quyền,
// event của nó không có actor_id.
var SystemActor = Actor{system: true}

func (a Actor) IsSystem() bool {
	return a.system
}

// Can báo actor có quyền perm hay không. SystemActor có mọi quyền.
func (a Actor) Can(perm string) bool {
	return a.system || slices.Contains(a.Permissions, perm)
}

var (
	ErrUnauthenticated = errs.Unauthorized(
		"/errors/unauthenticated", "Authentication required")

	ErrForbidden = errs.Forbidden(
		"/errors/forbidden", "Permission denied")
)

type ctxKey struct{}

func WithActor(ctx context.Context, a Actor) context.Context {
	return context.WithValue(ctx, ctxKey{}, a)
}

func FromContext(ctx context.Context) (Actor, bool) {
	a, ok := ctx.Value(ctxKey{}).(Actor)
	return a, ok
}

// ActorIDFrom trả account ID của actor, uuid.Nil nếu không có actor hoặc là
// SystemActor. Dùng để ghi actor_id của event.
func ActorIDFrom(ctx context.Context) uuid.UUID {
	a, _ := FromContext(ctx)
	return a.AccountID
}

// Require là dòng đầu của mọi use case cần quyền, trong service:
//
//	actor, err := auth.Require(ctx, domain.PermAssetCreate)
//	if err != nil {
//		return err
//	}
//
// Không có actor thì 401, thiếu quyền thì 403.
func Require(ctx context.Context, perm string) (Actor, error) {
	a, ok := FromContext(ctx)
	if !ok {
		return Actor{}, ErrUnauthenticated
	}
	if !a.Can(perm) {
		return Actor{}, ErrForbidden.With(errs.WithDetailf("Missing permission %q.", perm))
	}
	return a, nil
}
