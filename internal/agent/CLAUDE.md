# internal/agent

Claude Code subprocess management for Dojo.

## Purpose

This package spawns and manages Claude Code CLI processes. Each agent runs in its own jj workspace at `.jj/agents/<name>/`. The manager is TUI-agnostic, communicating via Go channels.

## Key Files

| File              | Purpose                                           |
| ----------------- | ------------------------------------------------- |
| `types.go`        | State (`Idle/Running/Stopped/Error`), Event types |
| `protocol.go`     | Parse Claude Code `--output-format stream-json`   |
| `process.go`      | Process struct wrapping claude subprocess         |
| `manager.go`      | Manager: spawn, stop, restart, shutdown agents    |
| `pidfile.go`      | PID file tracking for orphan detection            |
| `runner.go`       | ProcessRunner interface + MockRunner for testing  |
| `process_unix.go` | Unix-specific process operations                  |

## Architecture

```
Manager
├── Process (agent-1)  →  claude subprocess  →  Events channel
├── Process (agent-2)  →  claude subprocess  →  Events channel
└── ...
```

## Claude CLI Invocation

The claude subprocess is started with these flags:

```bash
cd <workspace> && claude -p --verbose --input-format stream-json --output-format stream-json --add-dir <workspace>
```

| Flag                           | Required | Purpose                                      |
| ------------------------------ | -------- | -------------------------------------------- |
| `-p`                           | Yes      | Print mode (non-interactive)                 |
| `--verbose`                    | Yes      | Required when using `--output-format stream-json` |
| `--input-format stream-json`   | Yes      | Accept streaming JSON input on stdin         |
| `--output-format stream-json`  | Yes      | Output streaming JSON on stdout              |
| `--add-dir <workspace>`        | Yes      | Allow editing files in the agent's workspace |

Note: The `cd` is done via `cmd.Dir` in Go, which sets the process working directory.

### Input Message Format

User messages must be sent as NDJSON (newline-delimited JSON):

```json
{"type":"user","message":{"role":"user","content":[{"type":"text","text":"user message here"}]}}
```

### Output Message Format

Claude outputs events as NDJSON. Key event types:
- `assistant` - Contains text or tool_use content blocks
- `result` - Completion indicator
- `error` - Error occurred
- `system` - Informational (ignored)

## Key Constants

```go
AgentsDir = ".jj/agents"    // Workspace location
PIDSubDir = ".pids"         // PID files at .jj/agents/.pids/
```

## Usage Pattern

```go
mgr := agent.NewManager(agent.DefaultConfig(), jjClient)

// Spawn agent (creates NEW workspace + starts claude)
err := mgr.SpawnAgent(ctx, "agent-1")

// Start agent in EXISTING workspace (use when switching to Chat tab)
err = mgr.StartAgent(ctx, "agent-1") // Returns nil if already running

// Listen for events
go func() {
    for evt := range mgr.Events() {
        switch evt.Type {
        case agent.EventOutput:
            // Handle text output
        case agent.EventToolUse:
            // Handle tool invocation
        }
    }
}()

// Stop agent (keeps workspace)
mgr.StopAgent("agent-1")

// Delete agent (removes workspace)
mgr.DeleteAgent(ctx, "agent-1")

// Get process to send input (M5 chat integration)
proc, err := mgr.GetProcess("agent-1")
if err == nil {
    proc.SendInput("user message")
}
```

## Event Types

| Event              | Data Type         | When                      |
| ------------------ | ----------------- | ------------------------- |
| `EventOutput`      | `OutputData`      | Agent emits text          |
| `EventToolUse`     | `ToolUseData`     | Agent starts using a tool |
| `EventToolResult`  | `ToolResultData`  | Tool execution completed  |
| `EventError`       | `ErrorData`       | Error occurred            |
| `EventStateChange` | `StateChangeData` | Agent state changed       |

## Important Implementation Details

### Shutdown Safety
- `Manager.Shutdown()` uses `sync.Once` to prevent double-close of the events channel
- Always call `Shutdown()` when the app exits to clean up processes

### Intentional Stop vs Crash
- `Process.Stop()` sets a `stopping` flag before sending SIGTERM
- `waitProcess()` checks this flag and only emits `EventError` for unexpected crashes
- Intentional stops (via `Stop()`, `RestartAgent()`, etc.) result in `StateStopped`, not `StateError`

### Stale Workspace Handling
- `StartAgent()` automatically calls `jj workspace update-stale` before starting
- This prevents "stale working copy" errors when default workspace has changed
- The jj `Diff()` function also auto-recovers from stale errors

### Error Visibility
- Stderr from claude is captured and emitted as `EventError` with `[stderr]` prefix
- Parse errors include the raw line that failed to parse
- `SendInput` returns proper errors (not silent failures)

### Workspace Isolation
- Each agent's working directory is set to its workspace via `cmd.Dir`
- `--add-dir` restricts file editing to the workspace only
- Agents see full project context (jj provides revision-level isolation)

### Testing
- Use `InjectProcessForTest()` to inject mock processes
- Use `EventsWritable()` to get writable channel for testing
- Use `GetRunningProcess()` which checks mock processes first

## When to Look Here

- Agent lifecycle issues (spawn, stop, crash)
- Claude Code output parsing
- Adding new event types
- PID file / orphan detection
- Process signal handling
- Claude CLI flag changes (check `process.go:Start()`)
