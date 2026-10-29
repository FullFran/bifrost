# The Realms System (Agent Drivers)

Bifrost uses the **Realms** pattern to decouple high-level orchestration (routing, worktrees, process supervision, streaming) from the concrete mechanics of each AI agent CLI.

---

## 1. The Realm Interface

Every agent supported by Bifrost implements the `Realm` interface defined in [`internal/realm/realm.go`](file:///home/fran/fullfran/bifrost/internal/realm/realm.go):

```go
type Realm interface {
    ID() string
    Name() string
    Description() string
    IsAvailable(ctx context.Context) (bool, string)
    BuildCommand(ctx context.Context, task Task, targetDir string) (*exec.Cmd, error)
    ModelsCommand(ctx context.Context) (*exec.Cmd, error)
}
```

### Self-Registration
Realms register themselves during package initialization (`init()`):

```go
func init() {
    Register(&AsgardAgy{})
}
```

---

## 2. Supported Realms

### 2.1. Asgard (`agy`) — Google Antigravity
- **ID**: `agy`
- **Binary**: Looks for `agy` in `$PATH` or falls back to `~/.local/bin/agy`.
- **Capabilities**: Flat-rate Google AI Pro subscription, Gemini 3.8 / 4 series models, deep reasoning effort, grounded web search, and headless execution.
- **Flags Injected**:
  - `--dangerously-skip-permissions`: Runs unattended in headless/background mode.
  - `-p "<prompt>"`: The prompt string.
  - `--output-format stream-json`: Injected when `task.StreamJSON` is enabled (for `--json` / Pi extensions).
- **Model & Effort Reconciliation**:
  Antigravity encodes reasoning effort directly into model IDs (e.g. `gemini-3.8-flash-high`). If a user passes both `--model gemini-3.8-flash-high` and `--effort low`, `agy` would reject the command due to conflict. Bifrost intelligently inspects the suffix and swaps it to `gemini-3.8-flash-low`:
  ```go
  func agyModelArgs(model, effort string) []string {
      if model != "" {
          if base, ok := trimEffortSuffix(model); ok && effort != "" {
              return []string{"--model", base + "-" + effort}
          }
          ...
      }
  }
  ```

### 2.2. Midgard (`opencode`) — OpenCode CLI
- **ID**: `opencode`
- **Binary**: Looks for `opencode` in `$PATH` or `~/.local/bin/opencode`.
- **Capabilities**: Local weights, open-source models, Ollama/vLLM backends, custom provider endpoints.
- **Flags Injected**:
  - `run --auto`: Unattended autonomous mode.
  - `-m <model>`: When an explicit model override is provided.

---

## 3. Realm Resolution & Fallback Flow

When a user runs `bifrost "<prompt>"` without specifying `-r <realm>`, Bifrost invokes the resolution algorithm:

```mermaid
flowchart TD
    Start["User dispatches task"] --> CheckFlag{"-r flag specified?"}
    
    CheckFlag -->|Yes| TargetRealm["Use specified realm"]
    CheckFlag -->|No / auto| LoadPriority["Read priority from config.json<br/>(e.g., ['agy', 'opencode'])"]
    
    LoadPriority --> IterateRealms["Inspect first candidate realm"]
    IterateRealms --> IsAvail{"Realm.IsAvailable()?"}
    
    IsAvail -->|No| HasNext{"More realms in priority?"}
    HasNext -->|Yes| NextRealm["Check next realm"] --> IsAvail
    HasNext -->|No| FailAvail["Error: No available realms found"]
    
    IsAvail -->|Yes| LaunchTask["Launch Task with Realm"]
    TargetRealm --> LaunchTask
    
    LaunchTask --> RunTask{"Execution succeeds?"}
    RunTask -->|Yes| Success["Task Completed"]
    RunTask -->|No| CheckFallback{"fallback_on_error == true?"}
    
    CheckFallback -->|Yes| TryNext["Fallback to next available realm"] --> LaunchTask
    CheckFallback -->|No| Error["Exit with error"]
```

---

## 4. Realm Doctor & Model Inspection

You can inspect all configured realms and verify environment readiness using CLI commands:

### Check availability:
```bash
$ bifrost realms

REALM     NAME                           STATUS      PATH
agy       Asgard (Google Antigravity)    AVAILABLE   /home/fran/.local/bin/agy
opencode  Midgard (OpenCode CLI)         AVAILABLE   /usr/local/bin/opencode
```

### Inspect models supported by a realm:
```bash
$ bifrost models agy

gemini-3.8-flash-low      Gemini 3.8 Flash (Low Reasoning)
gemini-3.8-flash-medium   Gemini 3.8 Flash (Medium Reasoning)
gemini-3.8-flash-high     Gemini 3.8 Flash (High Reasoning)
gemini-4-argon-low        Gemini 4 Argon (Low Reasoning)
gemini-4-argon-high       Gemini 4 Argon (High Reasoning)
```

---

## 5. Adding a New Realm

To add a new backend (e.g. `Valhalla / claude-code` or `Vanaheim / aider`):

1. Create a new file in `internal/realm/<name>.go`.
2. Implement the `Realm` interface:
   ```go
   package realm

   type ValhallaClaude struct{}

   func init() {
       Register(&ValhallaClaude{})
   }

   func (v *ValhallaClaude) ID() string { return "claude" }
   func (v *ValhallaClaude) Name() string { return "Valhalla (Claude Code)" }
   func (v *ValhallaClaude) Description() string { return "Anthropic Claude Code CLI" }
   ...
   ```
3. Register the binary path checks in `IsAvailable()`.
4. Build the execution command in `BuildCommand()`.
5. Run tests: `go test ./internal/realm/...`.
