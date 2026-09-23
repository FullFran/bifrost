package realm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type AsgardAgy struct{}

func init() {
	Register(&AsgardAgy{})
}

func (a *AsgardAgy) ID() string {
	return "agy"
}

func (a *AsgardAgy) Name() string {
	return "Asgard (Google Antigravity)"
}

func (a *AsgardAgy) Description() string {
	return "Flat-rate Google AI Pro quota, grounded search, deep reasoning via agy CLI"
}

func (a *AsgardAgy) findBinary() string {
	if p, err := exec.LookPath("agy"); err == nil {
		return p
	}
	home, _ := os.UserHomeDir()
	fallback := filepath.Join(home, ".local", "bin", "agy")
	if info, err := os.Stat(fallback); err == nil && !info.IsDir() {
		return fallback
	}
	return ""
}

func (a *AsgardAgy) IsAvailable(ctx context.Context) (bool, string) {
	bin := a.findBinary()
	if bin == "" {
		return false, "binary 'agy' not found in PATH or ~/.local/bin/agy"
	}
	return true, bin
}

func (a *AsgardAgy) BuildCommand(ctx context.Context, task Task, targetDir string) (*exec.Cmd, error) {
	bin := a.findBinary()
	if bin == "" {
		return nil, fmt.Errorf("agy binary not found")
	}

	args := []string{
		"--dangerously-skip-permissions",
		"-p", task.Prompt,
	}

	if task.Model != "" {
		args = append(args, "--model", task.Model)
	}
	if task.Effort != "" {
		args = append(args, "--effort", task.Effort)
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = targetDir
	return cmd, nil
}
