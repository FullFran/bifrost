package realm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// agyEffortSuffixes are the effort levels agy encodes in model IDs
// (e.g. gemini-3.8-flash-high). agy rejects --effort when it disagrees with them.
var agyEffortSuffixes = []string{"low", "medium", "high", "xhigh", "max"}

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

	cmd := exec.CommandContext(ctx, bin, agyArgs(task)...)
	cmd.Dir = targetDir
	return cmd, nil
}

func agyArgs(task Task) []string {
	args := []string{
		"--dangerously-skip-permissions",
		"-p", task.Prompt,
	}
	if task.StreamJSON {
		args = append(args, "--output-format", "stream-json")
	}
	return append(args, agyModelArgs(task.Model, task.Effort)...)
}

// agyModelArgs builds the --model/--effort flags. When the model ID already
// carries an effort suffix, the requested effort replaces that suffix instead
// of being passed separately, which agy would reject as a conflict.
func agyModelArgs(model, effort string) []string {
	var args []string
	if model != "" {
		if base, ok := trimEffortSuffix(model); ok && effort != "" {
			return []string{"--model", base + "-" + effort}
		}
		args = append(args, "--model", model)
		if _, ok := trimEffortSuffix(model); ok {
			return args
		}
	}
	if effort != "" {
		args = append(args, "--effort", effort)
	}
	return args
}

func trimEffortSuffix(model string) (string, bool) {
	for _, e := range agyEffortSuffixes {
		if base, ok := strings.CutSuffix(model, "-"+e); ok && base != "" {
			return base, true
		}
	}
	return model, false
}

// ModelsCommand lists the models agy can use.
func (a *AsgardAgy) ModelsCommand(ctx context.Context) (*exec.Cmd, error) {
	bin := a.findBinary()
	if bin == "" {
		return nil, fmt.Errorf("agy binary not found")
	}
	return exec.CommandContext(ctx, bin, "models"), nil
}
