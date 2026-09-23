# bifrost

A fast, compiled Go CLI to route, dispatch, and isolate AI coding agents (`agy`, `opencode`, and upcoming backends) on your machine.

Inspired by Google AX's declarative workspaces, but built for local development: instead of running heavy cluster infrastructure, it uses native Git worktrees so background agents can refactor and test code without touching your active editor or uncommitted files.

[![CI](https://github.com/FullFran/bifrost/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/FullFran/bifrost/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/FullFran/bifrost)](https://github.com/FullFran/bifrost/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

---

## Why this exists

1. **Context and token efficiency**: Running bulk refactoring or secondary reviews directly inside your primary interactive agent eats up the context window fast. Delegating self-contained tasks to an external CLI keeps the primary session lean.
2. **Workspace safety**: Running autonomous agents in your current directory while you are coding leads to dirty git states and race conditions. Bifrost isolates each task into an ephemeral git worktree on its own branch.
3. **Multi-agent flexibility**: Different tasks suit different tools. Google Antigravity (`agy`) gives you flat-rate Google AI Pro quota and grounded search; OpenCode (`opencode`) gives you local and open-source models. Bifrost provides a single interface with fallback routing across all of them.

---

## How it works

```text
               ┌── [ -r agy ] ──────> Google Antigravity (Asgard)
               │                      (AI Pro flat quota, web search)
User / Script ─┼── [ -r opencode ] ──> OpenCode CLI (Midgard)
               │                      (local models, open weights)
               └── [ -w worktree ] ──> Ephemeral Git Worktree
                                      (~/.cache/bifrost/worktrees/...)
```

- **Realms**: Drivers that know how to invoke each agent headless with proper permissions (`agy -p ... --dangerously-skip-permissions` or `opencode run --auto ...`).
- **Workspace Isolation (`-w`)**: Checks your git root and creates a temporary worktree (`bifrost/<task-id>`). The agent works there in isolation. You can review the diff and merge when ready.
- **Detached Execution (`-b`)**: Detaches the process cleanly, captures all stdout/stderr to disk, tracks PIDs, and lets you follow the logs live in another terminal.

---

## Install

### Quick install (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/FullFran/bifrost/main/scripts/install.sh | bash
```

### From source

```bash
git clone https://github.com/FullFran/bifrost.git
cd bifrost
make install
```

Verify your setup:

```bash
bifrost realms
```

---

## Usage

### 1. Basic dispatch

```bash
# Auto-route using configured priority (default: agy -> opencode)
bifrost "Audit internal/router for concurrency issues"

# Target a specific agent
bifrost -r agy "Write unit tests for the supervisor package"
bifrost -r opencode "Refactor this helper function"
```

### 2. Isolated execution (Git Worktree)

To prevent the agent from touching your current workspace or uncommitted changes:

```bash
bifrost -w "Refactor database migrations to support postgres 16"
```

When it finishes, review what changed:

```bash
bifrost diff <task-id>
bifrost merge <task-id>
```

### 3. Background jobs and live streaming

For long tasks (test generation, deep audits, large refactors):

```bash
# Launch detached in background with worktree isolation
bifrost -w -b "Generate full test coverage for internal/supervisor"
# => 🚀 Agent dispatched in background! Task ID: bf_20260923_150000_a1b2 (PID: 45210)

# Check active and completed tasks
bifrost list
bifrost status bf_20260923_150000_a1b2

# Stream output in real time from another terminal
bifrost logs -f bf_20260923_150000_a1b2

# Review and merge
bifrost diff bf_20260923_150000_a1b2
bifrost merge bf_20260923_150000_a1b2

# Or discard changes and delete the worktree
bifrost clean bf_20260923_150000_a1b2
```

---

## Configuration

Settings live in `~/.config/bifrost/config.json`:

```json
{
  "default_realm": "auto",
  "priority": [
    "agy",
    "opencode"
  ],
  "fallback_on_error": true,
  "worktree": {
    "auto_cleanup_on_merge": true
  },
  "realms": {
    "agy": {
      "default_model": "gemini-3.8-flash-high",
      "default_effort": "medium"
    },
    "opencode": {}
  }
}
```

Check current active config at any time:

```bash
bifrost config
```

---

## Command Reference

| Command | Description |
|---|---|
| `bifrost "<prompt>"` | Dispatches a task to an agent realm |
| `bifrost realms` | Doctor check: probes installed binaries and status |
| `bifrost list` | Lists active and historical jobs |
| `bifrost status <id>` | Shows detailed task state, PID, and duration |
| `bifrost logs [-f] <id>` | Prints or streams (`-f`) execution logs |
| `bifrost diff <id>` | Shows git diff produced by a worktree task |
| `bifrost merge <id>` | Merges the isolated branch into your active branch |
| `bifrost clean <id>` | Removes worktree directory and temporary branch |
| `bifrost prune` | Cleans all finished jobs, logs, and orphan worktrees |
| `bifrost abort <id>` | Gracefully terminates a running background task |
| `bifrost config` | Shows current declarative configuration |

---

## License

MIT
