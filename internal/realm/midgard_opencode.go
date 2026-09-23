package realm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type MidgardOpenCode struct{}

func init() {
	Register(&MidgardOpenCode{})
}

func (m *MidgardOpenCode) ID() string {
	return "opencode"
}

func (m *MidgardOpenCode) Name() string {
	return "Midgard (OpenCode)"
}

func (m *MidgardOpenCode) Description() string {
	return "Modular agent runner for local and open-source models via opencode CLI"
}

func (m *MidgardOpenCode) findBinary() string {
	if p, err := exec.LookPath("opencode"); err == nil {
		return p
	}
	home, _ := os.UserHomeDir()
	fallbacks := []string{
		filepath.Join(home, ".opencode", "bin", "opencode"),
		filepath.Join(home, ".local", "bin", "opencode"),
	}
	for _, fb := range fallbacks {
		if info, err := os.Stat(fb); err == nil && !info.IsDir() {
			return fb
		}
	}
	return ""
}

func (m *MidgardOpenCode) IsAvailable(ctx context.Context) (bool, string) {
	bin := m.findBinary()
	if bin == "" {
		return false, "binary 'opencode' not found in PATH or ~/.opencode/bin/opencode"
	}
	return true, bin
}

func (m *MidgardOpenCode) BuildCommand(ctx context.Context, task Task, targetDir string) (*exec.Cmd, error) {
	bin := m.findBinary()
	if bin == "" {
		return nil, fmt.Errorf("opencode binary not found")
	}

	args := []string{
		"run",
		"--auto",
	}

	if task.Model != "" {
		args = append(args, "--model", task.Model)
	}
	if task.Effort != "" {
		args = append(args, "--variant", task.Effort)
	}

	// Message argument
	args = append(args, task.Prompt)

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = targetDir
	return cmd, nil
}
