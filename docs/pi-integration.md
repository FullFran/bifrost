# Bifrost in Pi

Bifrost owns this Pi package. It adds `/bifrost` realm controls, keeps
`agy/default`, and adds `bifrost-claude/default` for native Claude Code dispatch.
These are **full-agent providers intended for delegated workers**, not ordinary
chat models: native agents execute their own tools in the current directory.
Pi receives tool activity as thinking, never executable Pi tool calls.

## Load the extension

Prerequisites: Node 24+, a Pi host providing `@earendil-works/pi-ai` and
`@earendil-works/pi-coding-agent`, and a current Bifrost CLI on `PATH` with
`config --json`, `config set REALM enabled|auto true|false`, and the Claude realm.
Install and authenticate the native agent CLIs separately. The extension neither
reads nor manages credentials. `BIFROST_BIN` can select a different Bifrost binary.

Try the package from the Bifrost checkout without changing settings:

```sh
pi -e ./pi/index.ts
```

For persistent installation, run this yourself from the checkout:

```sh
pi install .
```

This registers the local package in your Pi agent settings; it does not copy the
checkout. Keep that checkout available. Use `pi install --local .` instead for
project-scoped registration (subject to Pi project trust). Reload/restart the host
after changing loaded resources.

The root `package.json` declares **only the extension entry**. Host packages are
wildcard peers, not installed/bundled dependencies. There is no npm install step,
postinstall script, or automatic dotfiles migration.

### Replace the old agy extension manually

If your host already loads `agy-provider.ts`, disable that old extension in its
resource settings before enabling this package: both register `agy`. Do not load
both. This package preserves the `agy/default` model ID, cached catalog at
`~/.cache/gentle-shell/agy-models.json`, and agy model/effort overrides. Catalog
refresh runs on `session_start` using `bifrost models agy`, not during extension
factory loading; refreshed models appear on the next extension load. Failed
refreshes retain the previous cache.

No installer overwrites existing extensions or agent definitions. In particular,
this repository does not modify `gentle-shell/extensions/agy-provider.ts` or
`gentle-shell/subagents/agy.md` in your dotfiles.

## Install subagent definitions separately (host-specific)

**Pi itself has no subagent installation API or built-in subagent directory.**
The files in `pi/subagents/` are optional definitions for hosts/extensions that
understand this Markdown frontmatter, such as Gentle Shell. They are not Pi
package resources and are not installed by `pi install`.

For a compatible host, explicitly inspect `pi/subagents/agy.md` and
`pi/subagents/claude.md`, then manually copy or link the selected definition into
that host's documented subagent directory. For the Gentle Shell dotfiles layout,
these destinations are `gentle-shell/subagents/agy.md` and
`gentle-shell/subagents/claude.md`. Inspect existing destinations first; preserve
or deliberately reconcile them instead of blindly overwriting. Restart/reload
that host using its own instructions. Other hosts may require different paths or
frontmatter; consult their documentation rather than inventing a Pi setting.

The model selectors are:

| Definition | Pi provider/model | Bifrost realm |
|---|---|---|
| `agy.md` | `agy/default` | `agy` |
| `claude.md` | `bifrost-claude/default` | `claude` |

`bifrost-claude` is deliberately distinct from any host-native Claude provider.
`default` omits the model flag so Bifrost's configured defaults remain authoritative.

## Control realm policy

Run `/bifrost` in a dialog-capable TUI or RPC client. Select a realm, then toggle
`enabled` or `auto`. Escape cancels; after a successful write the selector rereads
CLI policy. Other modes receive an error notification and perform no CLI call.

- `enabled: false` prevents dispatch to that realm, including explicit selection.
- `auto: false` excludes it from automatic routing but allows explicit dispatch
  when enabled.
- Omitted values mean `true`, matching the CLI.

Controls call `bifrost config --json` and
`bifrost config set REALM enabled|auto true|false` in the session directory. The CLI
owns validation, persistence, and routing: the extension never writes config
files or maintains a second policy store. CLI/config errors are shown as errors,
not reported as successful changes. This changes future dispatches, not running
agents. Other CLI defaults remain configurable through the CLI itself.

## Streaming and safety boundaries

Each request launches foreground `bifrost --json -r REALM -d CWD -- PROMPT`
without a shell. A non-default model adds `-m`; reasoning adds `-e` (`minimal`
becomes `low`, `off` leaves Bifrost defaults alone). The host's transcript helper
preserves current system/subagent instructions, including named prompt sections;
follow-ups replay text conversation into a fresh native dispatch. Pi tool
schemas are not forwarded. Only text input is advertised; there is no native
session-resume or image handoff.

Native Claude `stream-json` tool/narration events become thinking. Text and thinking deltas (Claude and agy) stream live, and snapshots only add what the deltas did not deliver. Tool lines wait for their complete arguments: the assistant snapshot, the next block, next message, or terminal result. A final answer cannot be told from narration until `result`, so it shows live in thinking and then again as text. The terminal
`result` supplies final text and usage (input, output, cache read/write); native
errors, malformed JSON, missing/invalid final results, and **any nonzero or
signal exit even after a success result** fail closed. Empty successful text is
valid. Partial narration is not substituted for a final answer. agy retains its
step-update tool/usage protocol with the same fail-closed completion boundary.
Cancellation sends SIGTERM to the Bifrost child, whose CLI cancellation forwards
to the native agent, and reports an aborted result.

Claude's reported `total_cost_usd`, when present, is retained as the usage cost
total; component costs are not inferred. Model catalog costs are placeholders,
not billing guarantees. No HTTP response hook is fabricated for CLI transport;
payload inspection/replacement and parsed provider-event hooks are supported.
The extension does not choose subscription versus API billing, disable extra
usage, or inspect account telemetry. Native CLI authentication/settings and terms
remain the user's responsibility. Bifrost's native Claude invocation currently
uses `--dangerously-skip-permissions`: run only trusted tasks/workspaces with
appropriate external isolation. Realm controls are not a sandbox.

## Verify without real agents

```sh
node --test pi/bifrost.test.mjs
```

These deterministic Node tests use fake CLI/processes and a fake Pi stream/UI;
they do not install packages, authenticate, dispatch real Claude/agy, or modify
host settings. They cover registration, policy calls, streaming, usage, Unicode,
failures, cancellation, and agy compatibility. They do **not** establish that a
particular host loads the TypeScript entry or renders live subagent cards.

Before adopting in a host, manually smoke-test loading the entry above, model
selection for `agy/default` and `bifrost-claude/default`, `/bifrost` dialog behavior,
and the host-specific subagent definition discovery. An authorized real-agent
smoke test should then check final/error/usage rendering and cancellation through
Bifrost. No real-agent or installed-host smoke test is claimed by the Node suite.
