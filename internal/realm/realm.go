package realm

import (
	"context"
	"fmt"
	"os/exec"
	"time"
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
	if preference != "" && preference != "auto" {
		r, ok := Get(preference)
		if !ok {
			return nil, fmt.Errorf("realm '%s' is not registered", preference)
		}
		if ok, reason := r.IsAvailable(ctx); !ok {
			return nil, fmt.Errorf("realm '%s' is unavailable: %s", preference, reason)
		}
		return r, nil
	}

	// Check realms according to configured priority first
	for _, id := range priority {
		if r, ok := Get(id); ok {
			if avail, _ := r.IsAvailable(ctx); avail {
				return r, nil
			}
		}
	}

	// Fallback to any available realm
	for _, r := range All() {
		if ok, _ := r.IsAvailable(ctx); ok {
			return r, nil
		}
	}

	return nil, fmt.Errorf("no operational realms found. Install 'agy' or 'opencode'")
}
