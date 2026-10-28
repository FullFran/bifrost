package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type WorktreeConfig struct {
	AutoCleanupOnMerge bool `json:"auto_cleanup_on_merge"`
}

type RealmConfig struct {
	DefaultModel  string `json:"default_model,omitempty"`
	DefaultEffort string `json:"default_effort,omitempty"`
}

type Config struct {
	DefaultRealm    string                 `json:"default_realm"`
	Priority        []string               `json:"priority"`
	FallbackOnError bool                   `json:"fallback_on_error"`
	Worktree        WorktreeConfig         `json:"worktree"`
	Realms          map[string]RealmConfig `json:"realms"`
}

func DefaultConfig() *Config {
	return &Config{
		DefaultRealm:    "auto",
		Priority:        []string{"agy", "opencode"},
		FallbackOnError: true,
		Worktree: WorktreeConfig{
			AutoCleanupOnMerge: true,
		},
		// Empty model/effort means "use the agent's own default", which keeps
		// tracking the agent as it ships new models.
		Realms: map[string]RealmConfig{
			"agy":      {},
			"opencode": {},
		},
	}
}

func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "bifrost")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Load loads the configuration from ~/.config/bifrost/config.json.
// If the file does not exist, it creates and saves the default configuration.
func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return DefaultConfig(), err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		cfg := DefaultConfig()
		_ = Save(cfg)
		return cfg, nil
	} else if err != nil {
		return DefaultConfig(), err
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return DefaultConfig(), err
	}

	return cfg, nil
}

// Save writes the configuration to disk.
func Save(cfg *Config) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// SetRealmOption sets a per-realm default ("model" or "effort").
// An empty value unsets it so the agent's own default applies.
func (c *Config) SetRealmOption(realmID, key, value string) error {
	if c.Realms == nil {
		c.Realms = map[string]RealmConfig{}
	}
	rc := c.Realms[realmID]
	switch key {
	case "model":
		rc.DefaultModel = value
	case "effort":
		rc.DefaultEffort = value
	default:
		return fmt.Errorf("unknown realm option %q (expected model or effort)", key)
	}
	c.Realms[realmID] = rc
	return nil
}
