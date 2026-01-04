# M5: Chat View - Implementation Spec

## Overview

Add tabbed interface (Chat/Diff) to right pane with vim-style chat for agent interaction.

## Scope

- Tab system: [Chat] [Diff] with Ctrl+Tab cycling
- Chat display: streaming output, collapsible tools
- Vim input: Normal/Insert modes
- Agent integration: auto-spawn, error handling, message queue

## New Files

### `internal/tui/right_pane.go`

Tab container managing Chat/Diff:

```go
type RightPaneModel struct {
    activeTab    Tab         // Chat or Diff
    chatView     ChatViewModel
    diffView     DiffViewModel
    tabMemory    map[string]*workspaceTabState
    workspace    string
    isDefault    bool        // No chat for default workspace
    focused      bool
    width, height int
}

type Tab int
const (
    TabChat Tab = iota
    TabDiff
)

type workspaceTabState struct {
    lastTab     Tab
    chatHistory []ChatMessage
    scrollPos   int
}
```

### `internal/tui/chat_view.go`

Chat component:

```go
type ChatViewModel struct {
    messages      []ChatMessage
    scrollY       int
    inputMode     InputMode  // Normal or Insert
    inputBuffer   string
    inputCursor   int
    pendingQueue  []string
    agentState    agent.State
    workspace     string
    toolStates    map[string]*ToolState
    focused       bool
    width, height int
}

type InputMode int
const (
    ModeNormal InputMode = iota
    ModeInsert
)

type ChatMessage struct {
    Role      MessageRole
    Content   string
    Timestamp time.Time
    ToolID    string
    Expanded  bool
}

type MessageRole int
const (
    RoleUser MessageRole = iota
    RoleAgent
    RoleTool
    RoleError
)

type ToolState struct {
    Name     string
    Status   ToolStatus  // InProgress, Success, Error
    Output   string
    Expanded bool
}
```

## Files to Modify

### `internal/tui/app.go`
- Replace `diffView` with `rightPane RightPaneModel`
- Add `agentManager *agent.Manager`
- Wire event listener goroutine
- Route AgentEventMsg to chat by workspace
- Handle SpawnAgentMsg, ChatInputMsg

### `internal/tui/messages.go`
```go
type TabSwitchMsg struct{ Tab Tab }
type ChatInputMsg struct{ Workspace, Input string }
type SpawnAgentMsg struct{ WorkspaceName string }
type AgentCrashedMsg struct{ WorkspaceName string; Error error }
```

### `internal/tui/styles.go`
- Tab styles (active/inactive)
- Chat message styles (user/agent/tool/error)
- Mode indicator styles

### `internal/agent/manager.go`
```go
func (m *Manager) GetProcess(name string) (*Process, error)
```

## Keybindings

### Tab Navigation
| Key | Action |
|-----|--------|
| Ctrl+Tab | Cycle tabs |

### Chat Normal Mode
| Key | Action |
|-----|--------|
| j/k | Scroll viewport |
| g | Go to top |
| G | Go to bottom |
| i | Enter insert mode |
| Enter | Toggle tool expand (on tool line) |
| r | Retry (after crash) |

### Chat Insert Mode
| Key | Action |
|-----|--------|
| chars | Type into buffer |
| Backspace | Delete char |
| Enter | Submit message |
| Esc | Exit to normal mode |

## Behavior

### Tool Display
- Success: compact one-liner `◐ Read file.go ✓`
- Error: auto-expanded with output
- Enter toggles expand/collapse

### Agent States
- No agent: auto-spawn on chat tab entry
- Running: show streaming output
- Busy: queue user messages, dequeue on idle
- Crashed: show error + `[r] to retry`

### Workspace Switching
- Preserve chat history per workspace
- Remember last-used tab per workspace
- Default workspace: no chat tab

---

## Interview Findings (2026-01-04)

### Decisions

| Topic | Decision |
|-------|----------|
| Scope | Chat + Tab system together |
| Tool display | Compact for success, auto-expand on errors |
| Input | Vim-style: i/Esc, standard vim navigation |
| History | In-memory only (defer to M7) |
| No agent | Auto-spawn when entering chat tab |
| Streaming | Show tokens as they arrive |
| Error | Show error in chat + retry option |
| Default tab | Remember last-used per workspace |
| Tab keys | Ctrl+Tab cycles |
| Default workspace | No chat tab (user-only) |
| Markdown | Plain text for now |
| Busy state | Queue messages |

### Implementation Phases

1. **Tab Container**: right_pane.go, tab switching, workspace memory
2. **Chat Display**: chat_view.go, message rendering, tool states
3. **Vim Input**: Normal/Insert modes, keybindings
4. **Agent Integration**: SendInput, queue, auto-spawn, crash handling
5. **Polish**: auto-scroll, help bar, state preservation

---

## Interview Findings (2026-01-04) - Round 2

### Implementation Decisions

| Topic | Decision |
|-------|----------|
| Event bridging | tea.Sub goroutine: App spawns goroutine reading manager.Events(), sends via tea program |
| Busy detection | Running = busy, Idle = ready. No new state needed |
| Input queue | Always queue input, deliver when agent becomes Idle |
| Tab keybinding | Add Shift+Tab as fallback (Ctrl+Tab may fail in terminals) |
| Input mode | Multiline: Enter submits, Shift+Enter for newlines |
| Auto-scroll | Smart scroll: only if already at bottom, preserve position if user scrolled up |
| Workspace switch | View switch only - background agents keep running |
| Normal mode nav | Viewport scroll only (j/k), no message cursor |
| Retry behavior | Restart process via manager.RestartAgent() (same workspace) |
| Manager init | Lazy init: create manager on first agent spawn, shutdown on app quit |
| Spawn failure | Block tab switch + status line flash error message |

### Updated Keybindings

#### Tab Navigation
| Key | Action |
|-----|--------|
| Ctrl+Tab | Cycle tabs |
| Shift+Tab | Cycle tabs (fallback) |

#### Chat Insert Mode
| Key | Action |
|-----|--------|
| chars | Type into buffer |
| Backspace | Delete char |
| Enter | Submit message |
| Shift+Enter | Insert newline |
| Esc | Exit to normal mode |
