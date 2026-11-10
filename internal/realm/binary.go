package realm

import (
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
)

// Lookups used by findBinary; replaced in tests.
var (
	lookPath = exec.LookPath
	// realHomeDir returns the account's home from the user database, which
	// stays correct when a host (e.g. Gentle Shell) redirects HOME.
	realHomeDir = func() string {
		u, err := user.Current()
		if err != nil {
			return ""
		}
		return u.HomeDir
	}
	envHomeDir = os.UserHomeDir
)

// findBinary resolves a realm CLI: PATH first, then each homeRelative path
// under the account's real home, then under HOME when it differs. The first
// regular executable file wins (symlinks are followed). Returns "" otherwise.
func findBinary(name string, homeRelative ...string) string {
	if p, err := lookPath(name); err == nil {
		return p
	}
	var homes []string
	real := realHomeDir()
	if real != "" {
		homes = append(homes, real)
	}
	if env, err := envHomeDir(); err == nil && env != "" && env != real {
		homes = append(homes, env)
	}
	for _, home := range homes {
		for _, rel := range homeRelative {
			p := filepath.Join(home, rel)
			if info, err := os.Stat(p); err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0 {
				return p
			}
		}
	}
	return ""
}
