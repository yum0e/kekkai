package tui

import (
	"context"
	"errors"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bigq/dojo/internal/agent"
	"github.com/bigq/dojo/internal/jj"
)

// FocusedPane indicates which pane is focused.
type FocusedPane int

const (
	FocusWorkspaceList FocusedPane = iota
	FocusRightPane
)

// AppModel is the root model for the TUI application.
type AppModel struct {
	workspaceList WorkspaceListModel
	rightPane     RightPaneModel
	confirm       ConfirmModel
	jjClient      *jj.Client
	agentManager  *agent.Manager // Lazy initialized
	focusedPane   FocusedPane
	width         int
	height        int
	err           error
	program       *tea.Program // For event subscription
}

// NewApp creates a new TUI application.
func NewApp() (*AppModel, error) {
	client := jj.NewClient()

	// Validate we're in a jj repo
	ctx := context.Background()
	_, err := client.WorkspaceRoot(ctx)
	if err != nil {
		if errors.Is(err, jj.ErrNotJJRepo) {
			return nil, fmt.Errorf("not a jj repository (or any parent up to mount point /)")
		}
		return nil, fmt.Errorf("failed to detect jj repository: %w", err)
	}

	// Create agent manager eagerly so the event listener can be started
	// from main() with a stable pointer (not from Update which uses copies)
	agentManager := agent.NewManager(agent.DefaultConfig(), client)

	app := &AppModel{
		workspaceList: NewWorkspaceListModel(client),
		rightPane:     NewRightPaneModel(client),
		confirm:       NewConfirmModel(),
		jjClient:      client,
		agentManager:  agentManager,
		focusedPane:   FocusWorkspaceList,
	}

	// Set initial focus
	app.workspaceList.SetFocused(true)
	app.rightPane.SetFocused(false)

	return app, nil
}

// SetProgram sets the tea.Program for event subscription.
func (m *AppModel) SetProgram(p *tea.Program) {
	m.program = p
}

// StartEventListener starts the goroutine that reads agent events and sends them to the TUI.
// This MUST be called from main() after SetProgram(), not from Update(), because Update
// operates on model copies and goroutines started there would hold stale pointers.
func (m *AppModel) StartEventListener() {
	go func() {
		for evt := range m.agentManager.Events() {
			if m.program != nil {
				m.program.Send(AgentEventMsg{Event: evt})
			}
		}
	}()
}

// Shutdown cleans up resources.
func (m *AppModel) Shutdown() {
	if m.agentManager != nil {
		m.agentManager.Shutdown(context.Background())
	}
}

// Init initializes the application.
func (m AppModel) Init() tea.Cmd {
	return m.workspaceList.Init()
}

// Update handles messages for the application.
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalculateLayout()
		return m, nil

	case tea.KeyMsg:
		// Handle confirm dialog first if visible
		if m.confirm.Visible() {
			var cmd tea.Cmd
			m.confirm, cmd = m.confirm.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}

		switch msg.String() {
		case "q", "ctrl+c":
			m.Shutdown()
			return m, tea.Quit
		case "tab":
			m.toggleFocus()
			return m, nil
		case "r":
			// Only refresh if in diff view, not chat
			if m.focusedPane == FocusRightPane && m.rightPane.activeTab == TabDiff {
				return m, func() tea.Msg { return RefreshDiffMsg{} }
			}
		}

	case ConfirmDeleteMsg:
		m.confirm.Show(
			fmt.Sprintf("Delete workspace '%s'?", msg.WorkspaceName),
			"delete",
			msg.WorkspaceName,
		)
		return m, nil

	case ConfirmResultMsg:
		if msg.Confirmed && msg.Action == "delete" {
			if name, ok := msg.Data.(string); ok {
				// Use agent manager to properly stop agent before deleting
				if m.agentManager != nil {
					return m, func() tea.Msg {
						ctx := context.Background()
						err := m.agentManager.DeleteAgent(ctx, name)
						return WorkspaceDeletedMsg{Name: name, Err: err}
					}
				}
				// Fallback to direct deletion if no manager
				return m, m.workspaceList.DeleteWorkspace(name)
			}
		}
		return m, nil

	case SpawnAgentMsg:
		// Start agent in existing workspace
		return m, func() tea.Msg {
			ctx := context.Background()
			// Use StartAgent since workspace already exists
			err := m.agentManager.StartAgent(ctx, msg.WorkspaceName)
			return SpawnAgentResultMsg{
				WorkspaceName: msg.WorkspaceName,
				Success:       err == nil,
				Error:         err,
			}
		}

	case SpawnAgentResultMsg:
		// Route to right pane
		var cmd tea.Cmd
		m.rightPane, cmd = m.rightPane.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		// Start flash clear timer if there was an error
		if !msg.Success {
			cmds = append(cmds, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
				return StatusFlashClearMsg{}
			}))
		}
		return m, tea.Batch(cmds...)

	case ChatInputMsg:
		// Send input to agent asynchronously to avoid blocking TUI
		proc, err := m.agentManager.GetRunningProcess(msg.Workspace)
		if err != nil {
			// Agent not found - emit error event so user knows message wasn't sent
			return m, func() tea.Msg {
				return AgentEventMsg{Event: agent.Event{
					AgentName: msg.Workspace,
					Type:      agent.EventError,
					Data:      agent.ErrorData{Message: fmt.Sprintf("Agent not found for workspace '%s': %v", msg.Workspace, err)},
				}}
			}
		}
		return m, func() tea.Msg {
			if err := proc.SendInput(msg.Input); err != nil {
				return AgentEventMsg{Event: agent.Event{
					AgentName: msg.Workspace,
					Type:      agent.EventError,
					Data:      agent.ErrorData{Message: fmt.Sprintf("Failed to send to '%s': %v", msg.Workspace, err)},
				}}
			}
			return nil
		}

	case RestartAgentMsg:
		return m, func() tea.Msg {
			ctx := context.Background()
			err := m.agentManager.RestartAgent(ctx, msg.WorkspaceName)
			if err != nil {
				return AgentCrashedMsg{WorkspaceName: msg.WorkspaceName, Error: err}
			}
			return SpawnAgentResultMsg{WorkspaceName: msg.WorkspaceName, Success: true}
		}

	case AgentEventMsg:
		// Route to right pane
		var cmd tea.Cmd
		m.rightPane, cmd = m.rightPane.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	case StatusFlashClearMsg:
		var cmd tea.Cmd
		m.rightPane, cmd = m.rightPane.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)
	}

	// Route to child components
	var cmd tea.Cmd

	// Update workspace list
	m.workspaceList, cmd = m.workspaceList.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	// Update right pane
	m.rightPane, cmd = m.rightPane.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// toggleFocus switches focus between panes.
func (m *AppModel) toggleFocus() {
	if m.focusedPane == FocusWorkspaceList {
		m.focusedPane = FocusRightPane
		m.workspaceList.SetFocused(false)
		m.rightPane.SetFocused(true)
	} else {
		m.focusedPane = FocusWorkspaceList
		m.workspaceList.SetFocused(true)
		m.rightPane.SetFocused(false)
	}
}

// recalculateLayout recalculates the layout based on terminal size.
func (m *AppModel) recalculateLayout() {
	// Calculate left pane width (adaptive based on workspace names)
	leftWidth := m.workspaceList.MinWidth()
	if leftWidth < 15 {
		leftWidth = 15 // Minimum width
	}
	if leftWidth > m.width/3 {
		leftWidth = m.width / 3 // Max 1/3 of screen
	}

	// Right pane gets remaining width
	rightWidth := m.width - leftWidth - 3 // 3 for borders/gap

	// Height for content (minus title and help bar)
	contentHeight := m.height - 4 // title (1) + help (1) + borders (2)

	m.workspaceList.SetSize(leftWidth, contentHeight)
	m.rightPane.SetSize(rightWidth, contentHeight)
	m.confirm.SetSize(m.width, m.height)
}

// View renders the application.
func (m AppModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	// Title bar
	title := TitleStyle.Render("DOJO")
	titleBar := lipgloss.NewStyle().Width(m.width).Render(title)

	// Calculate pane dimensions
	leftWidth := m.workspaceList.MinWidth()
	if leftWidth < 15 {
		leftWidth = 15
	}
	if leftWidth > m.width/3 {
		leftWidth = m.width / 3
	}
	rightWidth := m.width - leftWidth - 1 // 1 for gap

	contentHeight := m.height - 4

	// Left pane (workspace list)
	leftBorder := m.workspaceList.borderStyle().
		Width(leftWidth - 2).
		Height(contentHeight)
	leftPane := leftBorder.Render(m.workspaceList.View())

	// Right pane (tabbed view)
	rightBorder := m.rightPane.borderStyle().
		Width(rightWidth - 2).
		Height(contentHeight)
	rightPane := rightBorder.Render(m.rightPane.View())

	// Join panes horizontally
	content := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)

	// Help bar - context-aware
	var helpText string
	if m.focusedPane == FocusRightPane && m.rightPane.activeTab == TabChat && !m.rightPane.isDefault {
		if m.rightPane.chatView.inputMode == ModeInsert {
			helpText = "Enter: send | Shift+Enter: newline | Esc: normal mode"
		} else {
			helpText = "i: insert | j/k: scroll | g/G: top/bottom | Shift+Tab: switch tab"
		}
	} else {
		helpText = "j/k: navigate | Enter: select | a: add | d: delete | r: refresh | Tab: pane | Shift+Tab: tab"
	}
	helpBar := HelpStyle.Width(m.width).Render(helpText)

	// Combine all
	view := lipgloss.JoinVertical(lipgloss.Left, titleBar, content, helpBar)

	// Overlay confirm dialog if visible
	if m.confirm.Visible() {
		// Create overlay
		overlay := m.confirm.CenteredView()
		return overlay
	}

	return view
}
