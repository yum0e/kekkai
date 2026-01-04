# internal/tui

Bubbletea TUI components for the Dojo application.

## Purpose

This package contains all UI components using the Elm architecture (Model-Update-View). It handles user input, rendering, and coordinates child components.

## Key Files

| File                | Purpose                                              |
| ------------------- | ---------------------------------------------------- |
| `app.go`            | Root model, orchestrates layout and child components |
| `workspace_list.go` | Left pane: lists workspaces with state indicators    |
| `diff_view.go`      | Right pane: displays jj diff output with scrolling   |
| `confirm.go`        | Modal Y/N confirmation dialog                        |
| `styles.go`         | Lipgloss style definitions (colors, borders)         |
| `messages.go`       | Custom tea.Msg types for component communication     |

## Architecture

```
AppModel
├── WorkspaceListModel  (left pane, focused by default)
├── DiffViewModel       (right pane)
└── ConfirmModel        (overlay dialog)
```

## Important Notes

- Agent workspaces are created at `.jj/agents/<name>/` (not repo root)
- `DeleteWorkspace()` removes both jj workspace and the directory
- Agent-related messages: `AgentEventMsg`, `AgentSpawnedMsg`, `AgentStoppedMsg`

## When to Look Here

- UI bugs or styling issues
- Keybinding changes
- Adding new panes or dialogs
- Layout/responsive design issues
