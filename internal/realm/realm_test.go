package realm_test

import (
	"context"
	"os/exec"
	"testing"

	"github.com/FullFran/bifrost/internal/realm"
)

type mockRealm struct {
	id        string
	available bool
}

func (m *mockRealm) ID() string          { return m.id }
func (m *mockRealm) Name() string        { return "Mock " + m.id }
func (m *mockRealm) Description() string { return "Mock description" }
func (m *mockRealm) IsAvailable(ctx context.Context) (bool, string) {
	if m.available {
		return true, "/bin/mock"
	}
	return false, "binary not found"
}
func (m *mockRealm) BuildCommand(ctx context.Context, task realm.Task, targetDir string) (*exec.Cmd, error) {
	return exec.Command("true"), nil
}

func TestRealmRegistry(t *testing.T) {
	realms := realm.All()
	if len(realms) < 2 {
		t.Fatalf("expected at least 2 default realms registered, got %d", len(realms))
	}

	agy, ok := realm.Get("agy")
	if !ok || agy.ID() != "agy" {
		t.Errorf("failed to get 'agy' realm")
	}

	opencode, ok := realm.Get("opencode")
	if !ok || opencode.ID() != "opencode" {
		t.Errorf("failed to get 'opencode' realm")
	}
}

func TestResolveRealmWithMocks(t *testing.T) {
	mock1 := &mockRealm{id: "test-avail-1", available: true}
	mock2 := &mockRealm{id: "test-avail-2", available: true}
	mockUnavail := &mockRealm{id: "test-unavail", available: false}

	realm.Register(mock1)
	realm.Register(mock2)
	realm.Register(mockUnavail)

	ctx := context.Background()

	// 1. Explicit available realm
	r, err := realm.ResolveRealm(ctx, "test-avail-1", nil)
	if err != nil {
		t.Fatalf("expected test-avail-1 to resolve, got error: %v", err)
	}
	if r.ID() != "test-avail-1" {
		t.Errorf("expected ID 'test-avail-1', got '%s'", r.ID())
	}

	// 2. Explicit unavailable realm should error
	_, err = realm.ResolveRealm(ctx, "test-unavail", nil)
	if err == nil {
		t.Fatalf("expected error for unavailable realm, got nil")
	}

	// 3. Priority resolution order
	r, err = realm.ResolveRealm(ctx, "auto", []string{"test-unavail", "test-avail-2", "test-avail-1"})
	if err != nil {
		t.Fatalf("unexpected error resolving with priority: %v", err)
	}
	if r.ID() != "test-avail-2" {
		t.Errorf("expected priority to pick 'test-avail-2', got '%s'", r.ID())
	}
}
