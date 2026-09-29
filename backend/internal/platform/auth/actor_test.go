package auth

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"storeit/internal/platform/errs"
)

const permAssetRead = "inventory.asset.read"

func TestRequire(t *testing.T) {
	reader := Actor{AccountID: uuid.New(), Permissions: []string{permAssetRead}}

	tests := []struct {
		name       string
		ctx        context.Context
		wantErr    error
		wantStatus int
	}{
		{"no actor", context.Background(), ErrUnauthenticated, http.StatusUnauthorized},
		{"missing permission", WithActor(context.Background(), Actor{AccountID: uuid.New()}), ErrForbidden, http.StatusForbidden},
		{"has permission", WithActor(context.Background(), reader), nil, 0},
		{"system has every permission", WithActor(context.Background(), SystemActor), nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actor, err := Require(tt.ctx, permAssetRead)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("err = %v", err)
				}
				want, _ := FromContext(tt.ctx)
				if actor.AccountID != want.AccountID || actor.IsSystem() != want.IsSystem() {
					t.Errorf("actor = %+v, want %+v", actor, want)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if got := errs.NewFrom(err).Status(); got != tt.wantStatus {
				t.Errorf("status = %d, want %d", got, tt.wantStatus)
			}
		})
	}
}

func TestForbiddenNamesMissingPermission(t *testing.T) {
	ctx := WithActor(context.Background(), Actor{AccountID: uuid.New()})

	_, err := Require(ctx, permAssetRead)

	if got, want := errs.NewFrom(err).Detail(), `Missing permission "inventory.asset.read".`; got != want {
		t.Errorf("detail = %q, want %q", got, want)
	}
}

func TestActorIDFrom(t *testing.T) {
	id := uuid.New()

	if got := ActorIDFrom(context.Background()); got != uuid.Nil {
		t.Errorf("no actor: got %v, want Nil", got)
	}
	if got := ActorIDFrom(WithActor(context.Background(), Actor{AccountID: id})); got != id {
		t.Errorf("account: got %v, want %v", got, id)
	}
	// System không phải account nào, event của nó có actor_id NULL
	if got := ActorIDFrom(WithActor(context.Background(), SystemActor)); got != uuid.Nil {
		t.Errorf("system: got %v, want Nil", got)
	}
}

// Actor thường không thể tự nhận là system bằng cách dựng struct
func TestOnlySystemActorIsSystem(t *testing.T) {
	if (Actor{AccountID: uuid.Nil, Permissions: nil}).IsSystem() {
		t.Error("zero Actor is system")
	}
	if !SystemActor.IsSystem() {
		t.Error("SystemActor is not system")
	}
}
