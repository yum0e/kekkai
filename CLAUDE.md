# Dojo

A TUI for orchestrating AI agents (like Claude Code) across jj workspaces.

## What is this?

Dojo enables parallel AI agent workflows using Jujutsu's workspace feature. Each agent runs in its own isolated workspace, allowing:

- Multiple agents working on uncorrelated features simultaneously
- Easy rollback of agent changes via jj's native undo
- Clear separation between user edits (default workspace) and agent work
- Merge agent results selectively using jj operations

## Tech Stack

- **Language**: Go
- **TUI Framework**: Bubbletea (Elm architecture)
- **Styling**: Lipgloss
- **VCS**: Jujutsu (jj)
- **Agent**: Claude Code CLI (subprocess)

## Project Structure

```
dojo/
├── cmd/dojo/
│   └── main.go                 # Entry point
├── internal/
│   ├── tui/                    # Bubbletea UI components
│   ├── jj/                     # Jujutsu CLI wrapper
│   ├── agent/                  # Claude Code process management
│   └── session/                # State persistence
├── specs/                      # Architecture specs and decisions
├── go.mod
└── go.sum
```

## Key Commands

```bash
# Run the TUI
go run ./cmd/dojo

# Build
go build -o dojo ./cmd/dojo

# Run in a specific repo
./dojo /path/to/repo
```

## Architecture Decisions

See `specs/0-start.md` for detailed architecture decisions and milestones.

## Core Concepts

- **Workspaces**: User works in `default`, agents get `agent-*` workspaces at `.jj/agents/`
- **Session Restore**: Full state persistence is mandatory for usability
- **External Editor**: User edits via `$EDITOR`, TUI refreshes on return
- **Agent Subprocess**: Claude Code CLI spawned per workspace

## Claude Code Integration

Agents are spawned using Claude Code CLI with streaming JSON:

```bash
claude -p --verbose --input-format stream-json --output-format stream-json
```

Key details:
- `-p` (print mode) is required for non-interactive use
- `--verbose` is required when using `--output-format stream-json`
- Input messages are NDJSON: `{"type":"user","message":{"role":"user","content":[{"type":"text","text":"..."}]}}`
- See `internal/agent/CLAUDE.md` for full protocol details

## Documentation

Each internal package has its own `CLAUDE.md` with package-specific details:

- `internal/jj/CLAUDE.md` - jj CLI wrapper
- `internal/tui/CLAUDE.md` - TUI components
- `internal/agent/CLAUDE.md` - Agent process management

**Important**: When modifying behavior in an internal package, update its `CLAUDE.md` to reflect the changes. This includes:

- New or removed functions
- Changed behavior (e.g., who is responsible for cleanup)
- New constants or configuration
- Important usage notes

## Bug Fixing Workflow

When a bug is discovered, follow this process:

1. **Write a failing test first** - Create a test that reproduces the broken behavior. The test should fail, confirming the bug exists.
2. **Verify the test fails** - Run the test to ensure it actually catches the bug.
3. **Fix the bug** - Implement the fix.
4. **Verify the test passes** - Run the test again to confirm the fix works.
5. **Run full test suite** - Ensure no regressions.

This approach ensures:
- The bug is properly understood before fixing
- The fix actually addresses the issue
- The bug won't silently regress in the future

Example:
```go
// Test written BEFORE fix - should fail
func TestStartAgent_AlreadyRunning(t *testing.T) {
    // ... setup ...
    err := mgr.StartAgent(ctx, "test-agent")
    if err != nil {
        t.Errorf("StartAgent() should return nil for already running agent")
    }
}
// Run test -> FAILS (bug confirmed)
// Fix the code
// Run test -> PASSES (bug fixed)
```
