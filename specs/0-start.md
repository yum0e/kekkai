# Dojo - Architecture & Milestones

## Overview

Dojo is a TUI that leverages Jujutsu (jj) workspaces to manage AI agents like Claude Code. The core insight: jj workspaces provide isolated working directories sharing the same repo, perfect for parallel agent work with easy merging and rollback.

## Key Decisions

### Language & Framework

| Decision      | Choice    | Rationale                                                                |
| ------------- | --------- | ------------------------------------------------------------------------ |
| Language      | Go        | Single binary distribution, fast compilation, easy cross-platform builds |
| TUI Framework | Bubbletea | Elm architecture fits stateful multi-pane UI, excellent ecosystem        |
| Styling       | Lipgloss  | Pairs with bubbletea, declarative styling                                |
| Components    | Bubbles   | Pre-built viewport, text input, list components                          |

Alternatives considered:

- **Rust + Ratatui**: Rejected due to slow compilation impacting dev feedback loop
- **Python + Textual**: Rejected due to painful distribution (no single binary)
- **TypeScript + Ink**: Rejected due to Node runtime requirement

### Agent Execution

**Decision**: Spawn Claude Code CLI as subprocesses (not direct API calls)

Rationale:

- Leverage existing Claude Code agent logic
- Simpler implementation for MVP
- Claude Code handles tool use, context, etc.

Each agent runs in its own jj workspace via:

```bash
jj workspace add agent-1
cd $(jj workspace root --workspace agent-1)
claude
```

### TUI Layout

```
┌─────────────────────────────────────────────────────────────┐
│                         DOJO                                │
├─────────────────┬───────────────────────────────────────────┤
│  WORKSPACES     │  [Chat] [Diff] [History]                  │
│  (always visible)│                                          │
│                 │  Tab-based main view                      │
│  ● default      │                                           │
│  ◐ agent-1      │  Content changes based on selected tab    │
│  ◐ agent-2      │  and selected workspace                   │
│                 │                                           │
└─────────────────┴───────────────────────────────────────────┘
```

- **Left pane**: Workspace list, always visible (like chat app sidebar)
- **Right pane**: Tabbed view (Chat | Diff | History)
- **Coupling**: Selected workspace determines what chat/diff/history shows

### Workspace Mental Model

| Workspace | Owner | Behavior                                 |
| --------- | ----- | ---------------------------------------- |
| `default` | User  | User's editing space, external `$EDITOR` |
| `agent-*` | Agent | Managed by dojo, Claude Code subprocess  |

- User edits happen in default workspace via `$EDITOR`
- TUI refreshes when editor returns
- Agent workspaces are created/deleted automatically by dojo
- Visual distinction between user diffs and agent diffs

### File Editing

**Decision**: External `$EDITOR`, not built-in editor

Rationale:

- Users have muscle memory with their editor
- No need to know about workspaces (always editing in default)
- Simpler MVP implementation
- TUI detects changes on editor return and refreshes

### Session Persistence

**Decision**: Full session restore (mandatory)

Stored in `~/.config/dojo/` (XDG compliant):

- Active workspaces and their state
- Conversation history per workspace
- UI state (selected workspace, active tab, scroll positions)
- Running agent processes (reconnect or show status)

Without session restore, the tool is unusable for real workflows.

### jj Operations (MVP)

**Basic** (required):

- `jj workspace add <name>` - Create agent workspace
- `jj workspace delete <name>` - Cleanup agent workspace
- `jj workspace list` - List all workspaces
- `jj diff` - Show changes in workspace
- `jj commit -m <msg>` - Commit changes
- `jj git push` - Push to GitHub

**Intermediate** (required for MVP):

- `jj squash` - Squash changes into parent
- `jj rebase -d <dest>` - Rebase onto destination
- `jj describe -m <msg>` - Update commit message

**Deferred**:

- `jj split` - Split commits
- `jj absorb` - Auto-absorb fixups
- Conflict resolution UI

### Scope

- **Single repo**: One repo at a time, opened like `dojo ~/myproject`
- **MVP agents**: 2 concurrent agents in 2 workspaces
- **Bootstrap goal**: Build dojo using dojo

---

## Milestones

### M1: Scaffold

- [x] Initialize Go module (`go mod init github.com/user/dojo`)
- [x] Basic bubbletea app structure
- [x] Renders "Hello Dojo" with lipgloss styling
- [x] Project directory structure in place

### M2: jj Client ✓

- [x] `internal/jj/errors.go` - Typed errors (ErrWorkspaceExists, ErrWorkspaceNotFound, ErrNotJJRepo)
- [x] `internal/jj/client.go` - Execute jj commands, parse output (CWD-based)
- [x] `internal/jj/workspace.go` - add (at change ID), delete, list workspaces
- [x] `internal/jj/diff.go` - Get diff as raw string
- [x] `internal/jj/log.go` - Get and parse log (change ID, message, author, date)
- [x] `internal/jj/status.go` - Get working copy status
- [x] `internal/jj/ops.go` - commit, squash, rebase, describe, git push
- [x] Integration tests with real jj + temp repos

### M3: Workspace List Pane ✓

- [x] Left pane component with workspace list
- [x] Keyboard navigation (j/k or arrows)
- [x] Visual indicators: ● default, ◐ agent (running), ○ agent (idle)
- [x] Selection state management

### M4: Agent Spawning

- [ ] `internal/agent/manager.go` - Track multiple agents
- [ ] `internal/agent/process.go` - Spawn Claude Code subprocess
- [ ] `internal/agent/protocol.go` - Parse Claude Code output stream
- [ ] Create workspace before spawning agent
- [ ] Route agent output to correct workspace state

### M5: Chat View

- [ ] Chat tab component
- [ ] Display messages (user + agent) with styling
- [ ] Text input for user messages
- [ ] Send to agent subprocess stdin
- [ ] Auto-scroll, viewport for history

### M6: Diff View

- [ ] Diff tab component
- [ ] Syntax-highlighted diff display
- [ ] Auto-refresh on file changes (fsnotify or polling)
- [ ] Visual distinction: user changes vs agent changes
- [ ] Open in `$EDITOR` action (press 'e')

### M7: Session Persistence

- [ ] `internal/session/state.go` - App state struct
- [ ] `internal/session/store.go` - Save/load JSON to XDG config
- [ ] Save on exit, load on start
- [ ] Handle stale agent processes (died while closed)

### M8: jj Operations UI

- [ ] Command palette or keybindings for jj ops
- [ ] Squash workflow (select commits)
- [ ] Rebase workflow (select destination)
- [ ] Describe (edit commit message)
- [ ] Git push with status feedback

---

## Dependencies

```go
require (
    github.com/charmbracelet/bubbletea   v1.x
    github.com/charmbracelet/lipgloss    v1.x
    github.com/charmbracelet/bubbles     v0.x
    github.com/adrg/xdg                  v0.x
)
```

Optional:

- `github.com/fsnotify/fsnotify` - File watching for diff refresh
- `github.com/alecthomas/chroma` - Syntax highlighting for diffs

---

## Open Questions (for future specs)

1. **Agent protocol**: How to structure Claude Code communication? Raw stdio? JSON messages?
2. **Workspace naming**: Auto-generated (`agent-1`) or user-named?
3. **History view**: ASCII DAG like `jj log` or simplified list?
4. **Keybindings**: Vim-style? Configurable?
5. **Theming**: Dark/light? Configurable colors?

---

## Bootstrap Strategy

The goal is to build dojo using dojo itself. Bootstrap sequence:

1. Build M1-M3 manually (basic TUI without agents)
2. Use early dojo to manage agents for M4-M5
3. Full dogfooding from M6 onwards

---

## Interview Findings (2026-01-04)

### M2: jj Client

**Design Decisions**:

| Topic | Decision |
|-------|----------|
| Error handling | Typed errors: `ErrWorkspaceExists`, `ErrWorkspaceNotFound`, `ErrNotJJRepo` |
| Output parsing | Default text format (regex/string parsing, not JSON) |
| Diff format | Raw string for display (no structured parsing) |
| Client context | CWD-based (caller manages directory) |
| Revision spec | jj Change IDs (e.g., `kpqxywon`) |
| Testing | Real jj + temp repos (integration-style tests) |
| Validation | Lazy (fail on first command, no eager check) |
| Concurrency | Support concurrent calls (mutex for cached state) |
| Git auth | Assume SSH/credential helper pre-configured |
| Undo tracking | Deferred to later milestone |

**Scope Additions** (beyond original spec):
- `jj log` → parsed into commits (change ID, message, author, date)
- `jj status` → working copy state

**Workspace Model Clarification**:
- User chooses revision (change ID) and workspace name
- Path is internally created and tracked by TUI
- Goal: abstract workspace complexity from user

**Concurrency Note**:
jj uses file locks per workspace. Concurrent reads are safe. Concurrent writes to same workspace could race, but jj handles gracefully. Consider mutex if caching workspace lists in Go client.
