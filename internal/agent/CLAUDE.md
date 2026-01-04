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

## Key Constants

```go
AgentsDir = ".jj/agents"    // Workspace location
PIDSubDir = ".pids"         // PID files at .jj/agents/.pids/
```

## Usage Pattern

```go
mgr := agent.NewManager(agent.DefaultConfig(), jjClient)

// Spawn agent (creates workspace + starts claude)
err := mgr.SpawnAgent(ctx, "agent-1")

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
```

## Event Types

| Event              | Data Type         | When                      |
| ------------------ | ----------------- | ------------------------- |
| `EventOutput`      | `OutputData`      | Agent emits text          |
| `EventToolUse`     | `ToolUseData`     | Agent starts using a tool |
| `EventToolResult`  | `ToolResultData`  | Tool execution completed  |
| `EventError`       | `ErrorData`       | Error occurred            |
| `EventStateChange` | `StateChangeData` | Agent state changed       |

## When to Look Here

- Agent lifecycle issues (spawn, stop, crash)
- Claude Code output parsing
- Adding new event types
- PID file / orphan detection
- Process signal handling
