package tui

import (
	"context"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/bigq/dojo/internal/agent"
	"github.com/bigq/dojo/internal/jj"
)

// DiffViewModel is the model for the diff view component.
type DiffViewModel struct {
	content   string
	workspace string
	workDir   string
	scrollY   int
	focused   bool
	loading   bool
	jjClient  *jj.Client
	width     int
	height    int
}

// NewDiffViewModel creates a new diff view model.
func NewDiffViewModel(client *jj.Client) DiffViewModel {
	return DiffViewModel{
		jjClient: client,
		loading:  true,
	}
}

// Init initializes the diff view.
func (m DiffViewModel) Init() tea.Cmd {
	return nil
}

// loadDiff fetches the diff for a workspace.
func (m DiffViewModel) loadDiff(workspaceName, workDir string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		opts := &jj.DiffOptions{
			WorkDir: workDir,
		}
		content, err := m.jjClient.Diff(ctx, opts)
		return DiffLoadedMsg{
			Workspace: workspaceName,
			Content:   content,
			Err:       err,
		}
	}
}

// Update handles messages for the diff view.
func (m DiffViewModel) Update(msg tea.Msg) (DiffViewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if !m.focused {
			return m, nil
		}
		switch msg.String() {
		case "j", "down":
			m.scrollY++
			m.clampScroll()
		case "k", "up":
			if m.scrollY > 0 {
				m.scrollY--
			}
		case "g":
			m.scrollY = 0
		case "G":
			m.scrollY = m.maxScroll()
		}

	case DiffLoadedMsg:
		if msg.Err != nil {
			m.content = ErrorStyle.Render("Error loading diff: " + msg.Err.Error())
		} else {
			m.content = msg.Content
		}
		m.workspace = msg.Workspace
		m.loading = false
		m.scrollY = 0

	case WorkspaceSelectedMsg:
		m.loading = true
		m.workspace = msg.Workspace.Name
		// Compute work directory for the workspace
		workDir := m.computeWorkDir(msg.Workspace.Name)
		return m, m.loadDiff(msg.Workspace.Name, workDir)

	case RefreshDiffMsg:
		if m.workspace != "" {
			m.loading = true
			workDir := m.computeWorkDir(m.workspace)
			return m, m.loadDiff(m.workspace, workDir)
		}
	}

	return m, nil
}

// computeWorkDir computes the working directory for a workspace.
func (m DiffViewModel) computeWorkDir(workspaceName string) string {
	if workspaceName == "default" {
		return "" // Use current directory
	}
	// For non-default workspaces, get root and use .jj/agents/<name>
	ctx := context.Background()
	root, err := m.jjClient.WorkspaceRoot(ctx)
	if err != nil {
		return ""
	}
	return filepath.Join(root, agent.AgentsDir, workspaceName)
}

// View renders the diff view.
func (m DiffViewModel) View() string {
	if m.loading {
		return "Loading..."
	}

	if strings.TrimSpace(m.content) == "" {
		return EmptyDiffStyle.Render("No changes in this workspace")
	}

	// Split content into lines and apply scrolling
	lines := strings.Split(m.content, "\n")
	m.clampScroll()

	// Get visible lines
	visibleHeight := m.height - 2 // Account for borders
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	endY := m.scrollY + visibleHeight
	if endY > len(lines) {
		endY = len(lines)
	}

	var visibleLines []string
	if m.scrollY < len(lines) {
		visibleLines = lines[m.scrollY:endY]
	}

	return strings.Join(visibleLines, "\n")
}

// clampScroll ensures scrollY is within valid bounds.
func (m *DiffViewModel) clampScroll() {
	maxScroll := m.maxScroll()
	if m.scrollY > maxScroll {
		m.scrollY = maxScroll
	}
	if m.scrollY < 0 {
		m.scrollY = 0
	}
}

// maxScroll returns the maximum scroll position.
func (m DiffViewModel) maxScroll() int {
	lines := strings.Split(m.content, "\n")
	visibleHeight := m.height - 2
	if visibleHeight < 1 {
		visibleHeight = 1
	}
	max := len(lines) - visibleHeight
	if max < 0 {
		return 0
	}
	return max
}

// SetSize sets the dimensions of the diff view.
func (m *DiffViewModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetFocused sets whether the diff view is focused.
func (m *DiffViewModel) SetFocused(focused bool) {
	m.focused = focused
}

// Focused returns whether the diff view is focused.
func (m DiffViewModel) Focused() bool {
	return m.focused
}

// borderStyle returns the appropriate border style based on focus.
func (m DiffViewModel) borderStyle() lipgloss.Style {
	if m.focused {
		return PaneBorderFocused
	}
	return PaneBorderUnfocused
}

// SetWorkDir sets the working directory for the diff view.
func (m *DiffViewModel) SetWorkDir(dir string) {
	m.workDir = dir
}
