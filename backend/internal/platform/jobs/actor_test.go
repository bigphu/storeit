package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"

	"storeit/internal/platform/auth"
)

func TestActorArgsFrom(t *testing.T) {
	id := uuid.New()

	if got := ActorArgsFrom(auth.WithActor(context.Background(), auth.Actor{AccountID: id})); got.ActorID != id {
		t.Errorf("account: ActorID = %v, want %v", got.ActorID, id)
	}
	if got := ActorArgsFrom(auth.WithActor(context.Background(), auth.SystemActor)); got.ActorID != uuid.Nil {
		t.Errorf("system: ActorID = %v, want Nil", got.ActorID)
	}
}

// Nhúng ActorArgs vào args của job thì actor_id nằm phẳng trong JSON
func TestActorArgs_EmbeddedJSON(t *testing.T) {
	type commitImportArgs struct {
		ActorArgs
		ImportID string `json:"import_id"`
	}
	id := uuid.New()

	b, err := json.Marshal(commitImportArgs{ActorArgs: ActorArgs{ActorID: id}, ImportID: "imp-1"})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got["actor_id"] != id.String() || got["import_id"] != "imp-1" {
		t.Errorf("json = %s", b)
	}
}

func TestRestoreActor(t *testing.T) {
	id := uuid.New()
	loaded := auth.Actor{AccountID: id, Permissions: []string{"inventory.asset.import"}}
	load := func(_ context.Context, got uuid.UUID) (auth.Actor, error) {
		if got != id {
			t.Errorf("loader asked for %v, want %v", got, id)
		}
		return loaded, nil
	}

	t.Run("account gets its current permissions", func(t *testing.T) {
		ctx, err := RestoreActor(context.Background(), id, load)
		if err != nil {
			t.Fatal(err)
		}
		a, ok := auth.FromContext(ctx)
		if !ok || a.AccountID != id || !a.Can("inventory.asset.import") {
			t.Errorf("actor = %+v, %v", a, ok)
		}
	})

	t.Run("no account runs as system", func(t *testing.T) {
		ctx, err := RestoreActor(context.Background(), uuid.Nil, load)
		if err != nil {
			t.Fatal(err)
		}
		if a, _ := auth.FromContext(ctx); !a.IsSystem() {
			t.Errorf("actor = %+v, want SystemActor", a)
		}
	})

	t.Run("loader error is returned so River retries", func(t *testing.T) {
		boom := errors.New("identity down")
		_, err := RestoreActor(context.Background(), id, func(context.Context, uuid.UUID) (auth.Actor, error) {
			return auth.Actor{}, boom
		})
		if !errors.Is(err, boom) {
			t.Errorf("err = %v, want %v", err, boom)
		}
	})
}
