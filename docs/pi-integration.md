# Pi Agent & Subagent Integration

Bifrost powers native AI subagents and custom model providers inside the **Pi** agent framework ([`@earendil-works/pi-coding-agent`](https://github.com/earendil-works) and `@earendil-works/pi-ai`), including environments built on top of it such as **Gentle Shell**.

Because Pi's architecture is modular and extensible, any Pi host application can integrate Bifrost to dispatch autonomous delegate tasks to **Google Antigravity (`agy`)** or **OpenCode (`opencode`)**. This grants Pi agents access to flat-rate Google AI Pro quota, deep reasoning models, and full tool execution (file editing, shell execution, web search)—all presented seamlessly through Pi's native subagent and streaming UI components.

---

## 1. How It Works (The Core Concept)

Standard Pi coding agents communicate with raw LLM APIs (OpenAI, Anthropic, Gemini) via streaming text and client-side tool calls executed by Pi itself.

However, **Google Antigravity (`agy`) is not a raw LLM—it is already a complete, autonomous agent** with its own execution environment, permission models, and native tools.

To bridge this without architectural friction:
1. **The Provider Illusion**: A Pi extension (`agy-provider.ts`) registers `agy` as a generic Pi model provider using Pi's extension API (`pi.registerProvider("agy", ...)`).
2. **Subagent Definition**: A Pi agent manifest (`subagents/agy.md`) specifies `model: agy/default`. Pi hosts treat `agy` as a native subagent with UI cards, background execution, and cancellation.
3. **Bifrost Translation**: When the subagent runs, Bifrost spawns `agy` in streaming JSON mode (`--output-format stream-json`).
4. **Tools as Thinking**: As `agy` uses its internal tools (e.g., reading files, running bash commands), Bifrost outputs JSON events that the extension intercepts and converts into Pi **"Thinking" deltas**. In Pi-based UIs (like Gentle Shell), live tool execution appears directly inside the collapsible thinking accordion!
5. **Final Output Stream**: When `agy` finishes with status `SUCCESS`, its final answer is emitted as the subagent's response text.

```mermaid
flowchart TD
    subgraph Host["Pi Agent / Host UI (e.g. Gentle Shell)"]
        SubagentWidget["Subagent Card & Widget"]
        ThinkingAccordion["Thinking Accordion<br/>(Live Tool Activity)"]
        Metrics["Token & Cost Metrics<br/>(Google AI Pro Flat Quota)"]
    end

    subgraph PiLayer["Pi Extension Layer (@earendil-works/pi-coding-agent)"]
        Ext["agy-provider.ts<br/>(pi.registerProvider)"]
        Cache["Model Cache<br/>(~/.cache/bifrost/agy-models.json)"]
    end

    subgraph BifrostBridge["Bifrost CLI"]
        BF["bifrost --json -r agy<br/>(CLI Dispatcher & Router)"]
    end

    subgraph Asgard["Google Antigravity Engine"]
        AGY["agy CLI<br/>(--dangerously-skip-permissions<br/>--output-format stream-json)"]
        Tools["Internal Tools<br/>(read_file, bash, web search, edit)"]
    end

    SubagentWidget --> Ext
    Ext -->|Spawn Process| BF
    BF -->|Execute| AGY
    AGY --> Tools

    Tools -.->|step_update: tool| AGY
    AGY -->|JSON Stream Events| BF
    BF -->|stdout stream| Ext
    Ext -->|thinking_delta| ThinkingAccordion
    Ext -->|usage| Metrics
    Ext -->|text_delta| SubagentWidget
```

---

## 2. End-to-End Sequence Diagram

The interaction lifecycle from prompt invocation to result rendering:

```mermaid
sequenceDiagram
    autonumber
    actor User as User in Pi / Gentle Shell
    participant Host as Pi Host / UI
    participant Ext as agy-provider.ts (Pi Extension)
    participant BF as Bifrost CLI
    participant AGY as Google Antigravity (agy)

    User->>Host: @agy "Refactor the database connection pool"
    activate Host
    Host->>Ext: streamSimple(model, context, options)
    activate Ext

    Note over Ext: 1. Compiles context transcript<br/>2. Reconciles model & effort args
    Ext->>BF: spawn: bifrost --json -r agy -d <cwd> -m <model> -e <effort> "<prompt>"
    activate BF

    BF->>AGY: exec: agy --dangerously-skip-permissions -p "<prompt>" --output-format stream-json
    activate AGY

    loop Streaming JSON Events
        AGY-->>BF: {"step_update": {"step_type": "tool", "state": "ACTIVE", "tool_name": "read_file", ...}}
        BF-->>Ext: stdout (unbuffered line)
        Note over Ext: Formats to: "→ read_file: path/to/file"
        Ext-->>Host: push({ type: "thinking_delta", delta: "→ read_file: ..." })
        Host-->>User: Renders in live collapsible accordion

        AGY-->>BF: {"step_update": {"state": "DONE", "usage": {"input_tokens": 1200, "output_tokens": 350}}}
        BF-->>Ext: stdout (unbuffered line)
        Note over Ext: Accumulates input, output & cache tokens
        Ext-->>Host: push({ type: "start", partial: { usage: ... } })
    end

    AGY-->>BF: {"result": {"status": "SUCCESS", "response": "Refactoring complete: updated pool.go..."}}
    deactivate AGY
    BF-->>Ext: stdout (final result JSON)
    deactivate BF

    Ext-->>Host: push({ type: "thinking_end" })
    Ext-->>Host: push({ type: "text_delta", delta: "Refactoring complete..." })
    Ext-->>Host: push({ type: "done", reason: "stop" })
    deactivate Ext
    Host-->>User: Displays completed subagent response
    deactivate Host
```

---

## 3. Key Components & Implementation Details

### 3.1. The Generic Pi Extension: `agy-provider.ts`

The extension hooks into Pi's lifecycle using standard `pi.registerProvider`:

```typescript
pi.registerProvider("agy", {
  baseUrl: "bifrost://agy",
  models: [ ... ],
  streamSimple: streamAgy,
});
```

Key responsibilities:

1. **Model Discovery & Catalog Synchronization**:
   - Runs `bifrost models agy` in the background on startup.
   - Parses the tab-separated model ID and name list.
   - Caches available models locally (e.g. `~/.cache/gentle-shell/agy-models.json` or `~/.cache/pi/agy-models.json`).
   - Exposes them to the host model selector under provider prefix `agy/`.

2. **Context Compilation (`buildPrompt`)**:
   - Pi sends the conversational history as an array of messages (`user`, `assistant`, `toolResult`).
   - If it's a follow-up conversation, the extension stitches previous turns into a clean markdown transcript:
     ```text
     Conversation so far:

     User: ...
     Assistant: ...

     Continue from the last user message.
     ```
   - This allows Antigravity to retain complete context across multiple turns without needing stateful API sessions.

3. **Tool Execution Mapping to Thinking**:
   - Standard LLM providers stream `reasoning` or `thinking` blocks.
   - Antigravity reports tool execution via JSON steps:
     ```json
     {
       "step_update": {
         "step_type": "tool",
         "state": "ACTIVE",
         "tool_name": "run_command",
         "tool_info": { "parameters": { "CommandLine": "go test ./..." } }
       }
     }
     ```
   - The provider converts this into readable thinking deltas:
     ```text
     → run_command: go test ./...
     ```
   - Any agent narration preceding the tool call is similarly emitted into the thinking stream.

4. **Lifecycle & Cancellation**:
   - If the user cancels the task in the Pi host (`Ctrl+C` or closing the subagent widget), the `AbortSignal` listener fires:
     ```typescript
     const abort = () => child.kill("SIGTERM");
     options?.signal?.addEventListener("abort", abort, { once: true });
     ```
   - This sends `SIGTERM` directly to Bifrost, which cleans up and terminates the child `agy` process group cleanly.

---

### 3.2. Subagent Manifest: `subagents/agy.md`

Pi agents support declarative subagent definitions via markdown frontmatter:

```markdown
---
name: agy
description: Google Antigravity (agy) as a subagent, dispatched through bifrost. Runs on the Google AI Pro flat quota with Gemini models and grounded web search. Use for bulk refactors, test generation, repository-wide exploration, web research, or a second-opinion review. It works directly in the current directory with its own tools and returns its final answer.
model: agy/default
---

You are a delegated worker run through the Google Antigravity CLI. Complete the task you are given end to end with your own tools, then reply with a concise report: what you did, the files you changed, and anything left open.
```

When a user in any Pi environment types `@agy <prompt>`, Pi reads this manifest, resolves `model: agy/default` against the `agy` provider registered by `agy-provider.ts`, and launches the delegated worker.

---

## 4. Why This Architecture Shines

| Challenge | Traditional Approach | Bifrost + Pi Solution |
|---|---|---|
| **API Costs** | Every subagent call consumes metered API credits (Anthropic/OpenAI). | Runs on **flat-rate Google AI Pro quota**, enabling heavy subagent delegation at zero marginal cost. |
| **Agent in an Agent** | Nested agents trigger conflicting tool execution cycles. | Tool execution happens inside Antigravity and streams outward as UI **thinking** traces. |
| **Context Window Pollution** | Long file reads and test logs fill up the primary conversation. | The subagent runs in an isolated process; only its final synthesis returns to the primary chat. |
| **Model Selection** | Hardcoded models or complex proxy routing. | Dynamic discovery via `bifrost models agy` with reasoning effort reconciliation (`low`, `medium`, `high`). |
| **Framework Portability** | Custom tightly-coupled terminal integrations. | Standard Pi extension contract (`@earendil-works/pi-coding-agent`), making it reusable in Gentle Shell and any Pi client. |
