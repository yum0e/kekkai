# internal/tui

Bubbletea TUI components for the Dojo application.

## Purpose

This package contains all UI components using the Elm architecture (Model-Update-View). It handles user input, rendering, and coordinates child components.

## Key Files

| File                | Purpose                                                 |
| ------------------- | ------------------------------------------------------- |
| `app.go`            | Root model, orchestrates layout and child components    |
| `workspace_list.go` | Left pane: lists workspaces with state indicators       |
| `right_pane.go`     | Right pane: tabbed container (Chat/Diff)                |
| `chat_view.go`      | Chat tab: vim-style input, streaming output, tool states|
| `diff_view.go`      | Diff tab: displays jj diff output with scrolling        |
| `confirm.go`        | Modal Y/N confirmation dialog                           |
| `styles.go`         | Lipgloss style definitions (colors, borders, tabs)      |
| `messages.go`       | Custom tea.Msg types for component communication        |

## Architecture

```
AppModel
├── WorkspaceListModel  (left pane, focused by default)
├── RightPaneModel      (right pane, tabbed)
│   ├── ChatViewModel   (Chat tab - agent interaction)
│   └── DiffViewModel   (Diff tab - jj diff output)
├── ConfirmModel        (overlay dialog)
└── agentManager        (eager init in NewApp, manages agent processes)
```

## Chat View Features

- Vim-style input: Normal mode (j/k scroll, i insert) and Insert mode (Enter submit, Shift+Enter newline)
- Smart scroll: auto-scrolls only when at bottom
- Tool states: compact display for success, auto-expand on error
- Message queue: input queued while agent is busy, delivered when idle
- Auto-spawn: agents spawned automatically when entering Chat tab

## Tab Navigation

- `Shift+Tab` or `Ctrl+Tab`: cycle between Chat/Diff tabs
- Default workspace has no Chat tab (user-only workspace)
- Tab preference remembered per workspace

## App Initialization Sequence

The correct initialization order in `main()` is critical:

```go
app, _ := tui.NewApp()           // 1. Create app (creates agentManager eagerly)
p := tea.NewProgram(app, ...)    // 2. Create tea.Program
app.SetProgram(p)                // 3. Set program reference
app.StartEventListener()         // 4. Start event listener goroutine
p.Run()                          // 5. Run the TUI
app.Shutdown()                   // 6. Cleanup on exit
```

### Why This Order Matters

- **agentManager is created eagerly in `NewApp()`** - not lazily, because the event listener needs a stable reference
- **`StartEventListener()` MUST be called from `main()`** - NOT from `Update()`. Bubbletea's Update method operates on model copies, so goroutines started from Update hold stale pointers to copied structs
- **`SetProgram()` before `StartEventListener()`** - the listener needs the program reference to send events

## Important Notes

- Agent workspaces are created at `.jj/agents/<name>/` (not repo root)
- `DeleteWorkspace()` removes both jj workspace and the directory
- Agent events flow: `manager.Events()` → `StartEventListener` goroutine → `tea.Program.Send()`
- Error messages from agent are shown in chat (stderr captured, SendInput errors displayed)

## Key Messages

| Message               | Purpose                                    |
| --------------------- | ------------------------------------------ |
| `AgentEventMsg`       | Wraps agent.Event from manager             |
| `SpawnAgentMsg`       | Request to spawn agent for workspace       |
| `SpawnAgentResultMsg` | Result of spawn attempt                    |
| `ChatInputMsg`        | Send user input to agent                   |
| `RestartAgentMsg`     | Restart crashed agent                      |
| `StatusFlashClearMsg` | Clear temporary error flash                |

## When to Look Here

- UI bugs or styling issues
- Keybinding changes
- Chat/agent interaction issues
- Tab switching behavior
- Layout/responsive design issues
