package tui

import (
	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bigq/dojo/internal/agent"
	"github.com/bigq/dojo/internal/jj"
)

// Tab represents the active tab in the right pane.
type Tab int

const (
	TabChat Tab = iota
	TabDiff
)

// workspaceTabState stores per-workspace tab memory.
type workspaceTabState struct {
	lastTab     Tab
	chatHistory []ChatMessage
	scrollPos   int
}

// RightPaneModel manages the tabbed right pane (Chat/Diff).
type RightPaneModel struct {
	activeTab     Tab
	chatView      ChatViewModel
	diffView      DiffViewModel
	tabMemory     map[string]*workspaceTabState
	workspace     string
	isDefault     bool // No chat for default workspace
	focused       bool
	width, height int
	statusFlash   string // Temporary status message
}

// NewRightPaneModel creates a new right pane model.
func NewRightPaneModel(client *jj.Client) RightPaneModel {
	return RightPaneModel{
		activeTab: TabDiff, // Start with diff (chat not available for default)
		chatView:  NewChatViewModel(),
		diffView:  NewDiffViewModel(client),
		tabMemory: make(map[string]*workspaceTabState),
		isDefault: true, // Start assuming default workspace
	}
}

// Init initializes the right pane.
func (m RightPaneModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the right pane.
func (m RightPaneModel) Update(msg tea.Msg) (RightPaneModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if !m.focused {
			return m, nil
		}

		switch msg.String() {
		case "ctrl+tab", "shift+tab":
			// Cycle tabs (only if not default workspace)
			if !m.isDefault {
				cmd := m.cycleTab()
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
			return m, tea.Batch(cmds...)
		}

		// Route to active tab
		if m.activeTab == TabChat && !m.isDefault {
			var cmd tea.Cmd
			m.chatView, cmd = m.chatView.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		} else {
			var cmd tea.Cmd
			m.diffView, cmd = m.diffView.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

	case WorkspaceSelectedMsg:
		m.workspace = msg.Workspace.Name
		m.isDefault = msg.Workspace.Name == "default"

		// Restore tab state for this workspace
		if state, ok := m.tabMemory[m.workspace]; ok {
			if !m.isDefault {
				m.activeTab = state.lastTab
				m.chatView.messages = state.chatHistory
				m.chatView.scrollY = state.scrollPos
			}
		} else {
			// New workspace, default to diff
			m.activeTab = TabDiff
			m.tabMemory[m.workspace] = &workspaceTabState{lastTab: TabDiff}
		}

		// Force diff tab for default workspace
		if m.isDefault {
			m.activeTab = TabDiff
		}

		// Update child views
		m.chatView.workspace = m.workspace

		// Update focus on child views based on active tab
		if m.focused {
			if m.activeTab == TabChat && !m.isDefault {
				m.chatView.SetFocused(true)
				m.diffView.SetFocused(false)
			} else {
				m.chatView.SetFocused(false)
				m.diffView.SetFocused(true)
			}
		}

		var cmd tea.Cmd
		m.diffView, cmd = m.diffView.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case AgentEventMsg:
		// Route to chat view if it matches this workspace
		if msg.Event.AgentName == m.workspace {
			var cmd tea.Cmd
			m.chatView, cmd = m.chatView.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

	case SpawnAgentResultMsg:
		if !msg.Success && msg.WorkspaceName == m.workspace {
			// Spawn failed - show flash and revert to diff tab
			m.statusFlash = msg.Error.Error()
			m.activeTab = TabDiff
		} else if msg.Success && msg.WorkspaceName == m.workspace {
			// Spawn succeeded - now safe to show chat
			m.statusFlash = ""
		}

	case StatusFlashClearMsg:
		m.statusFlash = ""

	default:
		// Pass through to both views for other messages
		var cmd tea.Cmd
		m.chatView, cmd = m.chatView.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		m.diffView, cmd = m.diffView.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// cycleTab switches between Chat and Diff tabs.
func (m *RightPaneModel) cycleTab() tea.Cmd {
	if m.isDefault {
		return nil
	}

	var cmd tea.Cmd
	if m.activeTab == TabChat {
		m.activeTab = TabDiff
	} else {
		m.activeTab = TabChat
		// Request agent spawn if entering chat
		cmd = func() tea.Msg {
			return SpawnAgentMsg{WorkspaceName: m.workspace}
		}
	}

	// Update focus on child views
	if m.focused {
		if m.activeTab == TabChat {
			m.chatView.SetFocused(true)
			m.diffView.SetFocused(false)
		} else {
			m.chatView.SetFocused(false)
			m.diffView.SetFocused(true)
		}
	}

	// Save tab preference
	if state, ok := m.tabMemory[m.workspace]; ok {
		state.lastTab = m.activeTab
	}

	return cmd
}

// View renders the right pane.
func (m RightPaneModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	// Tab bar (only show for non-default workspaces)
	var tabBar string
	tabBarHeight := 0
	if !m.isDefault {
		tabBar = m.renderTabBar()
		tabBarHeight = 1
	}

	// Content area
	contentHeight := m.height - tabBarHeight
	if contentHeight < 1 {
		contentHeight = 1
	}

	var content string
	if m.activeTab == TabChat && !m.isDefault {
		content = m.chatView.View()
	} else {
		content = m.diffView.View()
	}

	// Add status flash if present
	if m.statusFlash != "" {
		flash := ErrorStyle.Render("Error: " + m.statusFlash)
		content = flash + "\n" + content
	}

	if tabBar != "" {
		return lipgloss.JoinVertical(lipgloss.Left, tabBar, content)
	}
	return content
}

// renderTabBar renders the tab bar.
func (m RightPaneModel) renderTabBar() string {
	chatTab := " Chat "
	diffTab := " Diff "

	if m.activeTab == TabChat {
		chatTab = TabActiveStyle.Render(chatTab)
		diffTab = TabInactiveStyle.Render(diffTab)
	} else {
		chatTab = TabInactiveStyle.Render(chatTab)
		diffTab = TabActiveStyle.Render(diffTab)
	}

	tabs := lipgloss.JoinHorizontal(lipgloss.Top, chatTab, diffTab)
	return TabBarStyle.Width(m.width).Render(tabs)
}

// SetSize sets the dimensions of the right pane.
func (m *RightPaneModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	// Account for tab bar
	tabBarHeight := 0
	if !m.isDefault {
		tabBarHeight = 1
	}
	contentHeight := height - tabBarHeight
	if contentHeight < 1 {
		contentHeight = 1
	}

	m.chatView.SetSize(width, contentHeight)
	m.diffView.SetSize(width, contentHeight)
}

// SetFocused sets whether the right pane is focused.
func (m *RightPaneModel) SetFocused(focused bool) {
	m.focused = focused
	// Pass focus to active tab
	if m.activeTab == TabChat && !m.isDefault {
		m.chatView.SetFocused(focused)
		m.diffView.SetFocused(false)
	} else {
		m.chatView.SetFocused(false)
		m.diffView.SetFocused(focused)
	}
}

// Focused returns whether the right pane is focused.
func (m RightPaneModel) Focused() bool {
	return m.focused
}

// borderStyle returns the appropriate border style based on focus.
func (m RightPaneModel) borderStyle() lipgloss.Style {
	if m.focused {
		return PaneBorderFocused
	}
	return PaneBorderUnfocused
}

// SaveState saves the current chat state for workspace switching.
func (m *RightPaneModel) SaveState() {
	if m.workspace == "" || m.isDefault {
		return
	}

	if state, ok := m.tabMemory[m.workspace]; ok {
		state.lastTab = m.activeTab
		state.chatHistory = m.chatView.messages
		state.scrollPos = m.chatView.scrollY
	}
}

// SetAgentState updates the agent state for the chat view.
func (m *RightPaneModel) SetAgentState(name string, state agent.State) {
	if name == m.workspace {
		m.chatView.agentState = state
	}
}
