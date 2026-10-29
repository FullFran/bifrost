package realm

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/FullFran/bifrost/internal/config"
)

// Task represents a unit of work dispatched through Bifrost.
type Task struct {
	ID         string
	Prompt     string
	WorkDir    string
	Worktree   bool
	Background bool
	Model      string
	Effort     string
	// StreamJSON asks the agent for its machine-readable event stream
	// (one JSON event per line) instead of formatted text.
	StreamJSON bool
	Timeout    time.Duration
}

// Realm defines the contract for an agent backend (Asgard, Midgard, Alfheim, etc.)
type Realm interface {
	ID() string
	Name() string
	Description() string
	IsAvailable(ctx context.Context) (bool, string)
	BuildCommand(ctx context.Context, task Task, targetDir string) (*exec.Cmd, error)
}

// ModelLister is implemented by realms that can list their available models.
type ModelLister interface {
	ModelsCommand(ctx context.Context) (*exec.Cmd, error)
}

var registry = make(map[string]Realm)
var order []string

// Register adds a realm to the global registry.
func Register(r Realm) {
	id := r.ID()
	registry[id] = r
	order = append(order, id)
}

// Get returns a realm by ID.
func Get(id string) (Realm, bool) {
	r, ok := registry[id]
	return r, ok
}

// All returns all registered realms in registration order.
func All() []Realm {
	var list []Realm
	for _, id := range order {
		list = append(list, registry[id])
	}
	return list
}

// AvailableRealms returns realms that are currently installed and operational.
func AvailableRealms(ctx context.Context) []Realm {
	var available []Realm
	for _, r := range All() {
		if ok, _ := r.IsAvailable(ctx); ok {
			available = append(available, r)
		}
	}
	return available
}

// ResolveRealm determines which realm to use based on preference and priority list.
func ResolveRealm(ctx context.Context, preference string, priority []string) (Realm, error) {
	return ResolveRealmWithPolicy(ctx, preference, priority, nil)
}

// ResolveRealmWithPolicy applies the same policy to priority and registry fallback.
// Missing controls retain the historical enabled/auto behavior.
func ResolveRealmWithPolicy(ctx context.Context, preference string, priority []string, policies map[string]config.RealmConfig) (Realm, error) {
	if preference != "" && preference != "auto" {
		r, ok := Get(preference)
		if !ok {
			return nil, fmt.Errorf("realm '%s' is not registered", preference)
		}
		if !policies[preference].IsEnabled() {
			return nil, fmt.Errorf("realm '%s' is disabled", preference)
		}
		if ok, reason := r.IsAvailable(ctx); !ok {
			return nil, fmt.Errorf("realm '%s' is unavailable: %s", preference, reason)
		}
		return r, nil
	}

	// Check realms according to configured priority first
	eligible := func(id string) bool { return policies[id].IsEnabled() && policies[id].IsAuto() }
	for _, id := range priority {
		if r, ok := Get(id); ok && eligible(id) {
			if avail, _ := r.IsAvailable(ctx); avail {
				return r, nil
			}
		}
	}

	// Fallback to any available realm
	for _, r := range All() {
		if !eligible(r.ID()) {
			continue
		}
		if ok, _ := r.IsAvailable(ctx); ok {
			return r, nil
		}
	}

	return nil, fmt.Errorf("no operational realms found eligible for auto. Install 'agy', 'opencode', or 'claude' and enable automatic routing")
}
