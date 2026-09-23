package realm_test

import (
	"context"
	"testing"

	"github.com/FullFran/bifrost/internal/realm"
)

func TestRealmRegistry(t *testing.T) {
	realms := realm.All()
	if len(realms) < 2 {
		t.Fatalf("expected at least 2 realms, got %d", len(realms))
	}

	agy, ok := realm.Get("agy")
	if !ok || agy.ID() != "agy" {
		t.Errorf("failed to get 'agy' realm")
	}

	opencode, ok := realm.Get("opencode")
	if !ok || opencode.ID() != "opencode" {
		t.Errorf("failed to get 'opencode' realm")
	}

	ctx := context.Background()
	resolved, err := realm.ResolveRealm(ctx, "auto", []string{"agy", "opencode"})
	if err != nil {
		t.Fatalf("unexpected error resolving realm: %v", err)
	}
	if resolved.ID() != "agy" && resolved.ID() != "opencode" {
		t.Errorf("expected resolved realm to be agy or opencode, got %s", resolved.ID())
	}
}
