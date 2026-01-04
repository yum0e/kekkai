package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bigq/dojo/internal/agent"
	"github.com/bigq/dojo/internal/jj"
)

// AgentState represents the state of an agent in a workspace.
type AgentState int

const (
	AgentStateNone AgentState = iota
	AgentStateIdle
	AgentStateRunning
)

// WorkspaceItem represents a workspace with its agent state.
type WorkspaceItem struct {
	jj.Workspace
	State AgentState
}

// WorkspaceListModel is the model for the workspace list component.
type WorkspaceListModel struct {
	items    []WorkspaceItem
	cursor   int
	focused  bool
	jjClient *jj.Client
	width    int
	height   int
}

// NewWorkspaceListModel creates a new workspace list model.
func NewWorkspaceListModel(client *jj.Client) WorkspaceListModel {
	return WorkspaceListModel{
		jjClient: client,
		focused:  true,
	}
}

// Init initializes the workspace list.
func (m WorkspaceListModel) Init() tea.Cmd {
	return m.loadWorkspaces()
}

// loadWorkspaces fetches the workspace list from jj.
func (m WorkspaceListModel) loadWorkspaces() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		workspaces, err := m.jjClient.WorkspaceList(ctx)
		return WorkspacesLoadedMsg{Workspaces: workspaces, Err: err}
	}
}

// Update handles messages for the workspace list.
func (m WorkspaceListModel) Update(msg tea.Msg) (WorkspaceListModel, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.items)-1 {
				m.cursor++
				return m, m.emitSelected()
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
				return m, m.emitSelected()
			}
		case "enter":
			return m, m.emitSelected()
		case "a":
			return m, m.addWorkspace()
		case "d":
			if m.cursor < len(m.items) && m.items[m.cursor].Name != "default" {
				return m, func() tea.Msg {
					return ConfirmDeleteMsg{WorkspaceName: m.items[m.cursor].Name}
				}
			}
		}

	case WorkspacesLoadedMsg:
		if msg.Err != nil {
			return m, nil
		}
		m.items = mockAgentStates(msg.Workspaces)
		if m.cursor >= len(m.items) {
			m.cursor = max(0, len(m.items)-1)
		}
		return m, m.emitSelected()

	case WorkspaceAddedMsg:
		if msg.Err == nil {
			return m, m.loadWorkspaces()
		}

	case WorkspaceDeletedMsg:
		if msg.Err == nil {
			return m, m.loadWorkspaces()
		}
	}

	return m, nil
}

// emitSelected emits a WorkspaceSelectedMsg for the current cursor position.
func (m WorkspaceListModel) emitSelected() tea.Cmd {
	if len(m.items) == 0 {
		return nil
	}
	return func() tea.Msg {
		return WorkspaceSelectedMsg{Workspace: m.items[m.cursor].Workspace}
	}
}

// addWorkspace creates a new agent workspace.
func (m WorkspaceListModel) addWorkspace() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		// Generate unique name
		name := m.generateWorkspaceName()

		// Get the repo root for workspace path
		root, err := m.jjClient.WorkspaceRoot(ctx)
		if err != nil {
			return WorkspaceAddedMsg{Err: err}
		}

		// Ensure agents directory exists
		agentsDir := filepath.Join(root, agent.AgentsDir)
		if err := os.MkdirAll(agentsDir, 0755); err != nil {
			return WorkspaceAddedMsg{Err: err}
		}

		// Create workspace at .jj/agents/<name>, based on default's working copy
		path := filepath.Join(agentsDir, name)
		err = m.jjClient.WorkspaceAdd(ctx, path, "@")
		return WorkspaceAddedMsg{Name: name, Err: err}
	}
}

// generateWorkspaceName generates a unique agent workspace name.
func (m WorkspaceListModel) generateWorkspaceName() string {
	existing := make(map[string]bool)
	for _, item := range m.items {
		existing[item.Name] = true
	}

	for i := 1; ; i++ {
		name := fmt.Sprintf("agent-%d", i)
		if !existing[name] {
			return name
		}
	}
}

// View renders the workspace list.
func (m WorkspaceListModel) View() string {
	if len(m.items) == 0 {
		return "Loading workspaces..."
	}

	var b strings.Builder
	for i, item := range m.items {
		// Indicator
		indicator := m.renderIndicator(item)

		// Workspace name
		name := item.Name

		// Build line
		line := fmt.Sprintf("%s %s", indicator, name)

		// Apply selection style
		if i == m.cursor {
			line = WorkspaceItemSelected.Width(m.width - 2).Render(line)
		} else {
			line = WorkspaceItemNormal.Width(m.width - 2).Render(line)
		}

		b.WriteString(line)
		if i < len(m.items)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// renderIndicator renders the state indicator for a workspace item.
func (m WorkspaceListModel) renderIndicator(item WorkspaceItem) string {
	switch {
	case item.Name == "default":
		return IndicatorDefaultStyle.Render(IndicatorDefault)
	case item.State == AgentStateRunning:
		return IndicatorRunningStyle.Render(IndicatorRunning)
	default:
		return IndicatorIdleStyle.Render(IndicatorIdle)
	}
}

// MinWidth calculates the minimum width needed for the workspace list.
func (m WorkspaceListModel) MinWidth() int {
	maxLen := 0
	for _, item := range m.items {
		if len(item.Name) > maxLen {
			maxLen = len(item.Name)
		}
	}
	// indicator (2) + space (1) + name + padding (1)
	return maxLen + 4
}

// SetSize sets the dimensions of the workspace list.
func (m *WorkspaceListModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetFocused sets whether the workspace list is focused.
func (m *WorkspaceListModel) SetFocused(focused bool) {
	m.focused = focused
}

// Focused returns whether the workspace list is focused.
func (m WorkspaceListModel) Focused() bool {
	return m.focused
}

// SelectedWorkspace returns the currently selected workspace.
func (m WorkspaceListModel) SelectedWorkspace() *jj.Workspace {
	if len(m.items) == 0 || m.cursor >= len(m.items) {
		return nil
	}
	return &m.items[m.cursor].Workspace
}

// mockAgentStates adds mock agent states for visual testing in M3.
func mockAgentStates(workspaces []jj.Workspace) []WorkspaceItem {
	items := make([]WorkspaceItem, len(workspaces))
	for i, ws := range workspaces {
		items[i] = WorkspaceItem{Workspace: ws, State: AgentStateNone}
		if strings.HasPrefix(ws.Name, "agent-") {
			// Alternate between running/idle for visual testing
			if i%2 == 0 {
				items[i].State = AgentStateRunning
			} else {
				items[i].State = AgentStateIdle
			}
		}
	}
	return items
}

// DeleteWorkspace deletes a workspace by name.
func (m WorkspaceListModel) DeleteWorkspace(name string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		// Get repo root to compute workspace path
		root, err := m.jjClient.WorkspaceRoot(ctx)
		if err != nil {
			return WorkspaceDeletedMsg{Name: name, Err: err}
		}

		// Forget the workspace from jj
		if err := m.jjClient.WorkspaceForget(ctx, name); err != nil {
			return WorkspaceDeletedMsg{Name: name, Err: err}
		}

		// Remove the workspace directory at .jj/agents/<name>
		workspacePath := filepath.Join(root, agent.AgentsDir, name)
		if err := os.RemoveAll(workspacePath); err != nil {
			return WorkspaceDeletedMsg{Name: name, Err: err}
		}

		return WorkspaceDeletedMsg{Name: name, Err: nil}
	}
}

// borderStyle returns the appropriate border style based on focus.
func (m WorkspaceListModel) borderStyle() lipgloss.Style {
	if m.focused {
		return PaneBorderFocused
	}
	return PaneBorderUnfocused
}
