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
