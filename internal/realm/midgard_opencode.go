package realm

import (
	"context"
	"fmt"
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
	return findBinary("opencode",
		filepath.Join(".opencode", "bin", "opencode"),
		filepath.Join(".local", "bin", "opencode"))
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

	cmd := exec.CommandContext(ctx, bin, openCodeArgs(task)...)
	cmd.Dir = targetDir
	return cmd, nil
}

func openCodeArgs(task Task) []string {
	args := []string{
		"run",
		"--auto",
	}
	if task.StreamJSON {
		args = append(args, "--format", "json")
	}
	if task.Model != "" {
		args = append(args, "--model", task.Model)
	}
	if task.Effort != "" {
		args = append(args, "--variant", task.Effort)
	}
	// Message argument
	return append(args, task.Prompt)
}

// ModelsCommand lists the models opencode can use.
func (m *MidgardOpenCode) ModelsCommand(ctx context.Context) (*exec.Cmd, error) {
	bin := m.findBinary()
	if bin == "" {
		return nil, fmt.Errorf("opencode binary not found")
	}
	return exec.CommandContext(ctx, bin, "models"), nil
}
