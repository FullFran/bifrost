package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type WorktreeConfig struct {
	AutoCleanupOnMerge bool `json:"auto_cleanup_on_merge"`
}

type RealmConfig struct {
	Enabled       *bool  `json:"enabled,omitempty"`
	Auto          *bool  `json:"auto,omitempty"`
	DefaultModel  string `json:"default_model,omitempty"`
	DefaultEffort string `json:"default_effort,omitempty"`
}

// IsEnabled and IsAuto preserve opt-out semantics for older configuration files.
func (r RealmConfig) IsEnabled() bool { return r.Enabled == nil || *r.Enabled }
func (r RealmConfig) IsAuto() bool    { return r.Auto == nil || *r.Auto }

// UnmarshalJSON rejects null policies: only an omitted field means unspecified.
// Legacy model/effort fields retain encoding/json's null-as-no-op semantics.
func (r *RealmConfig) UnmarshalJSON(data []byte) error {
	type plain RealmConfig
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if fields == nil {
		return fmt.Errorf("realm configuration must be an object")
	}
	for _, key := range []string{"enabled", "auto"} {
		if raw, ok := fields[key]; ok && bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return fmt.Errorf("realm %s must not be null", key)
		}
	}
	return json.Unmarshal(data, (*plain)(r))
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
		Priority:        []string{"agy", "opencode", "claude"},
		FallbackOnError: true,
		Worktree: WorktreeConfig{
			AutoCleanupOnMerge: true,
		},
		// Empty model/effort means "use the agent's own default", which keeps
		// tracking the agent as it ships new models.
		Realms: map[string]RealmConfig{
			"agy":      {},
			"opencode": {},
			"claude":   {},
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
		if err := Save(cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	} else if err != nil {
		return DefaultConfig(), err
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	if fields == nil {
		return nil, fmt.Errorf("configuration must be an object")
	}
	// A null realm map could discard routing policies. Other legacy fields
	// retain their original encoding/json null semantics.
	if raw, ok := fields["realms"]; ok && bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, fmt.Errorf("configuration realms must not be null")
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

// SetRealmOption sets a per-realm default or routing policy.
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
	case "enabled", "auto":
		var policy *bool
		if value != "" {
			if value != "true" && value != "false" {
				return fmt.Errorf("invalid value %q for %s (expected true or false)", value, key)
			}
			v := value == "true"
			policy = &v
		}
		if key == "enabled" {
			rc.Enabled = policy
		} else {
			rc.Auto = policy
		}
	default:
		return fmt.Errorf("unknown realm option %q (expected model, effort, enabled, or auto)", key)
	}
	c.Realms[realmID] = rc
	return nil
}
