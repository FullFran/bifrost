# 🌈 Bifrost

<div align="center">

```
   ____  _  __               _   
  / __ )(_)/ /_ _____ ____  / /_ 
 / __  / // __// ___// __ \/ __/ 
/ /_/ / // /_ / /   / /_/ / /_   
\____/_/ \__//_/    \____/\__/   
```

**The Universal AI Coding Agent Bridge & Worktree Orchestrator**  
*Cross the realms of Google Antigravity, OpenCode, and beyond.*

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://golang.org)
[![Google Antigravity](https://img.shields.io/badge/Google-Antigravity%20(agy)-4285F4?logo=google&logoColor=white)](https://antigravity.google)
[![OpenCode](https://img.shields.io/badge/OpenCode-Compatible-000000)](https://opencode.ai)

</div>

---

## ⚡ The Myth & Architecture

In Norse mythology, **Bifröst** is the burning rainbow bridge connecting Asgard (the realm of the gods) with Midgard (the realm of mortals). 

In modern software development, **Bifrost** is a fast, compiled Go CLI that bridges your workflow across different AI coding agent backends ("Realms"), inspired by **Google AX**'s declarative workspace sandboxing:

| Realm | Agent Backend | Role & Superpower |
|---|---|---|
| 🛡️ **Asgard** | **Google Antigravity (`agy`)** | Flat-rate Google AI Pro quota, grounded search, and deep reasoning models. |
| 🌿 **Midgard** | **OpenCode (`opencode`)** | Flexible local & open-source runner for offline or specialized models. |
| ⚡ *Alfheim* *(coming soon)* | **Claude Code (`claude`)** | Surgical refactors and complex multi-file codebase operations. |
| 🔥 *Muspelheim* *(coming soon)* | **Codex CLI (`codex`)** | Raw OpenAI reasoning engines. |

---

## 🚀 Key Features

* **🛡️ Multi-Agent Routing & Fallback**: Run tasks using `auto` routing or target a specific realm (`-r agy`, `-r opencode`). If a realm is unavailable, Bifrost can intelligently fall back.
* **🌐 Google AX-Inspired Workspace Isolation (`--worktree`)**: Never let an autonomous agent modify your working directory while you are editing. Bifrost can mount an ephemeral `git worktree` on a dedicated branch (`bifrost/<task-id>`), allowing the agent to test and refactor in total isolation.
* **⚡ Background Asynchronous Jobs (`--bg`)**: Detach long-running refactors or audits. Bifrost manages PIDs, status checkpoints, and full stdout/stderr logging in `~/.cache/bifrost/`.
* **🔍 Review & Merge Flow**: Review changes made in isolated worktrees using `bifrost diff <id>` and merge them into your branch with `bifrost merge <id>`.
* **🏎️ Zero Dependencies**: Built in Go standard library. Compiles to a single lightweight binary with instantaneous startup.

---

## 📦 Quickstart

### 1. Build and Install

```bash
cd bifrost
make install
```

This compiles the static binary and installs it to `~/.local/bin/bifrost`.

### 2. Verify Your Realms (Doctor Check)

```bash
bifrost realms
```

Output:
```text
🌈 Bifrost Realms & Doctor Check:
--------------------------------------------------------------------------------
REALM      NAME                          STATUS          DETAILS
agy        Asgard (Google Antigravity)   ✅ Operational   /home/.../.local/bin/agy
opencode   Midgard (OpenCode)            ✅ Operational   /home/.../.opencode/bin/opencode
--------------------------------------------------------------------------------
```

---

## 💻 CLI Usage

### A. Direct Execution

```bash
# Automatic routing across available realms
bifrost "Audit src/auth.go for potential concurrency races"

# Explicitly cross to Asgard (Google Antigravity)
bifrost -r agy "Write unit tests for the router package"

# Explicitly cross to Midgard (OpenCode)
bifrost -r opencode "Refactor this utility function"
```

### B. Workspace Isolation (Git Worktree)

To prevent the agent from touching your active uncommitted code:

```bash
bifrost -w "Refactor the database layer to use sqlx"
```

Bifrost will:
1. Detect your git repository root.
2. Create an isolated worktree at `~/.cache/bifrost/worktrees/bf_...` on branch `bifrost/bf_...`.
3. Run the agent strictly inside that worktree.
4. Leave your current editor and files 100% clean.

### C. Background Jobs & Supervision

```bash
# Launch a background task inside an isolated worktree
bifrost -w -b "Generate full test coverage for internal/supervisor"
# => 🚀 Agent dispatched in background! Task ID: bf_20260923_150000_a1b2 (PID: 45210)

# Check all active and finished jobs
bifrost list

# Inspect detailed status
bifrost status bf_20260923_150000_a1b2

# View live execution logs
bifrost logs bf_20260923_150000_a1b2

# Inspect git diff created by the worktree
bifrost diff bf_20260923_150000_a1b2

# Merge the changes into your branch when satisfied
bifrost merge bf_20260923_150000_a1b2
```

---

## 🛠️ Subcommands Reference

| Subcommand | Description |
|---|---|
| `bifrost "<prompt>"` | Dispatches a task to an agent realm |
| `bifrost realms` | Checks status and binaries of all agent backends |
| `bifrost list` | Lists all background and historical jobs |
| `bifrost status <id>` | Shows detailed task state, execution duration, and PID |
| `bifrost logs <id>` | Dumps output and error stream logs |
| `bifrost diff <id>` | Shows `git diff` produced by a worktree session |
| `bifrost merge <id>` | Merges the isolated branch into your current branch |
| `bifrost abort <id>` | Gracefully terminates a running background task |

---

## 📜 License

MIT License. Designed and crafted with pair programming.
