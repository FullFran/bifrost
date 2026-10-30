package realm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ValhallaClaude delegates authentication, billing, and execution to native Claude Code.
type ValhallaClaude struct{}

func init() { Register(&ValhallaClaude{}) }

func (*ValhallaClaude) ID() string          { return "claude" }
func (*ValhallaClaude) Name() string        { return "Valhalla (Claude Code)" }
func (*ValhallaClaude) Description() string { return "Native Anthropic Claude Code CLI" }

func (*ValhallaClaude) findBinary() string {
	if p, err := exec.LookPath("claude"); err == nil {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	p := filepath.Join(home, ".local", "bin", "claude")
	if info, err := os.Stat(p); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
		return p
	}
	return ""
}

func (c *ValhallaClaude) IsAvailable(context.Context) (bool, string) {
	if p := c.findBinary(); p != "" {
		return true, p
	}
	return false, "binary 'claude' not found in PATH or ~/.local/bin/claude"
}

func (c *ValhallaClaude) BuildCommand(ctx context.Context, task Task, targetDir string) (*exec.Cmd, error) {
	bin := c.findBinary()
	if bin == "" {
		return nil, fmt.Errorf("claude binary not found")
	}
	args := []string{"--dangerously-skip-permissions", "-p", task.Prompt}
	if task.StreamJSON {
		args = append(args, "--output-format", "stream-json", "--verbose", "--include-partial-messages")
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
