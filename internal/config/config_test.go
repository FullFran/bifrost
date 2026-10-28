package config_test

import (
	"testing"

	"github.com/FullFran/bifrost/internal/config"
)

func TestDefaultConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	if cfg.DefaultRealm != "auto" {
		t.Errorf("expected default_realm to be 'auto', got '%s'", cfg.DefaultRealm)
	}
	if len(cfg.Priority) < 2 {
		t.Errorf("expected at least 2 realms in priority, got %d", len(cfg.Priority))
	}
	if !cfg.FallbackOnError {
		t.Errorf("expected fallback_on_error to be true")
	}
	if !cfg.Worktree.AutoCleanupOnMerge {
		t.Errorf("expected auto_cleanup_on_merge to be true")
	}
	if _, ok := cfg.Realms["agy"]; !ok {
		t.Errorf("expected 'agy' in realms config")
	}
}

func TestDefaultConfigLeavesModelAndEffortToRealm(t *testing.T) {
	cfg := config.DefaultConfig()
	for id, rc := range cfg.Realms {
		if rc.DefaultModel != "" || rc.DefaultEffort != "" {
			t.Errorf("realm %q: expected no pinned model/effort so the agent's own default applies, got model=%q effort=%q", id, rc.DefaultModel, rc.DefaultEffort)
		}
	}
}

func TestSetRealmOption(t *testing.T) {
	cfg := config.DefaultConfig()

	if err := cfg.SetRealmOption("agy", "model", "gemini-4-argon-high"); err != nil {
		t.Fatalf("set model: %v", err)
	}
	if err := cfg.SetRealmOption("agy", "effort", "low"); err != nil {
		t.Fatalf("set effort: %v", err)
	}
	if got := cfg.Realms["agy"]; got.DefaultModel != "gemini-4-argon-high" || got.DefaultEffort != "low" {
		t.Errorf("unexpected agy config after set: %+v", got)
	}

	if err := cfg.SetRealmOption("agy", "model", ""); err != nil {
		t.Fatalf("unset model: %v", err)
	}
	if got := cfg.Realms["agy"].DefaultModel; got != "" {
		t.Errorf("expected model to be unset, got %q", got)
	}

	if err := cfg.SetRealmOption("newrealm", "model", "x"); err != nil {
		t.Fatalf("set on new realm: %v", err)
	}
	if got := cfg.Realms["newrealm"].DefaultModel; got != "x" {
		t.Errorf("expected new realm entry, got %q", got)
	}

	if err := cfg.SetRealmOption("agy", "temperature", "1"); err == nil {
		t.Errorf("expected error for unknown key")
	}
}
