package workspace

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// WorktreeSession represents an isolated git worktree environment inspired by AX Workspace.
type WorktreeSession struct {
	TaskID    string
	RepoRoot  string
	Branch    string
	Path      string
	IsActive  bool
}

// PrepareWorktree creates a dedicated ephemeral git worktree for the task.
func PrepareWorktree(ctx context.Context, taskID string, currentDir string) (*WorktreeSession, error) {
	// Check if in git repository
	cmd := exec.CommandContext(ctx, "git", "-C", currentDir, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("directory '%s' is not inside a git repository: %w", currentDir, err)
	}
	repoRoot := strings.TrimSpace(string(out))

	branch := fmt.Sprintf("bifrost/%s", taskID)
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	worktreeBase := filepath.Join(home, ".cache", "bifrost", "worktrees")
	if err := os.MkdirAll(worktreeBase, 0755); err != nil {
		return nil, fmt.Errorf("failed to create worktree cache directory: %w", err)
	}

	worktreePath := filepath.Join(worktreeBase, taskID)

	// Add worktree branching from current HEAD
	addCmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "worktree", "add", "-b", branch, worktreePath, "HEAD")
	var errBuf bytes.Buffer
	addCmd.Stderr = &errBuf
	if err := addCmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to create git worktree: %s (%w)", errBuf.String(), err)
	}

	return &WorktreeSession{
		TaskID:   taskID,
		RepoRoot: repoRoot,
		Branch:   branch,
		Path:     worktreePath,
		IsActive: true,
	}, nil
}

// Cleanup removes the worktree filesystem allocation while preserving or deleting the branch.
func Cleanup(ctx context.Context, repoRoot, worktreePath string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "worktree", "remove", "--force", worktreePath)
	return cmd.Run()
}

// Diff returns git diff between the original HEAD and the worktree branch.
func Diff(ctx context.Context, repoRoot, branch string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "diff", fmt.Sprintf("HEAD...%s", branch))
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// Merge merges the worktree branch into current branch.
func Merge(ctx context.Context, repoRoot, branch string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "merge", "--no-ff", "-m", fmt.Sprintf("merge: bifrost task branch %s", branch), branch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("merge failed: %s (%w)", string(out), err)
	}
	return string(out), nil
}
