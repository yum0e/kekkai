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
