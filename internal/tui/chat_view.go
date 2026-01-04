package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bigq/dojo/internal/agent"
)

// InputMode represents the vim-style input mode.
type InputMode int

const (
	ModeNormal InputMode = iota
	ModeInsert
)

// MessageRole identifies the type of chat message.
type MessageRole int

const (
	RoleUser MessageRole = iota
	RoleAgent
	RoleTool
	RoleError
)

// ToolStatus represents the status of a tool execution.
type ToolStatus int

const (
	ToolInProgress ToolStatus = iota
	ToolSuccess
	ToolError
)

// ChatMessage represents a single message in the chat.
type ChatMessage struct {
	Role      MessageRole
	Content   string
	Timestamp time.Time
	ToolID    string   // For tool messages
	Expanded  bool     // For collapsible tool output
}

// ToolState tracks the state of an active tool.
type ToolState struct {
	Name     string
	Status   ToolStatus
	Output   string
	Expanded bool
}

// ChatViewModel manages the chat interface.
type ChatViewModel struct {
	messages      []ChatMessage
	scrollY       int
	inputMode     InputMode
	inputBuffer   string
	inputCursor   int
	agentState    agent.State
	workspace     string
	toolStates    map[string]*ToolState
	focused       bool
	width, height int
	atBottom      bool // For smart scroll
}

// NewChatViewModel creates a new chat view model.
func NewChatViewModel() ChatViewModel {
	return ChatViewModel{
		messages:   make([]ChatMessage, 0),
		inputMode:  ModeNormal,
		toolStates: make(map[string]*ToolState),
		atBottom:   true,
	}
}

// Init initializes the chat view.
func (m ChatViewModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the chat view.
func (m ChatViewModel) Update(msg tea.Msg) (ChatViewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if !m.focused {
			return m, nil
		}

		if m.inputMode == ModeInsert {
			return m.handleInsertMode(msg)
		}
		return m.handleNormalMode(msg)

	case AgentEventMsg:
		return m.handleAgentEvent(msg.Event)
	}

	return m, nil
}

// handleNormalMode handles keys in normal mode.
func (m ChatViewModel) handleNormalMode(msg tea.KeyMsg) (ChatViewModel, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		m.scrollY++
		m.clampScroll()
		m.atBottom = m.scrollY >= m.maxScroll()
	case "k", "up":
		if m.scrollY > 0 {
			m.scrollY--
			m.atBottom = false
		}
	case "g":
		m.scrollY = 0
		m.atBottom = false
	case "G":
		m.scrollY = m.maxScroll()
		m.atBottom = true
	case "i":
		m.inputMode = ModeInsert
	case "r":
		// Retry after crash
		if m.agentState == agent.StateError {
			return m, func() tea.Msg {
				return RestartAgentMsg{WorkspaceName: m.workspace}
			}
		}
	}
	return m, nil
}

// handleInsertMode handles keys in insert mode.
func (m ChatViewModel) handleInsertMode(msg tea.KeyMsg) (ChatViewModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.inputMode = ModeNormal
	case "enter":
		// Submit message
		if strings.TrimSpace(m.inputBuffer) != "" {
			input := m.inputBuffer
			m.inputBuffer = ""
			m.inputCursor = 0

			// Add user message to chat
			m.messages = append(m.messages, ChatMessage{
				Role:      RoleUser,
				Content:   input,
				Timestamp: time.Now(),
			})

			// Auto-scroll to bottom
			m.atBottom = true
			m.scrollY = m.maxScroll()

			// Always send immediately - Claude handles concurrent messages
			return m, func() tea.Msg {
				return ChatInputMsg{Workspace: m.workspace, Input: input}
			}
		}
	case "shift+enter":
		// Insert newline
		m.inputBuffer = m.inputBuffer[:m.inputCursor] + "\n" + m.inputBuffer[m.inputCursor:]
		m.inputCursor++
	case "backspace":
		if m.inputCursor > 0 {
			m.inputBuffer = m.inputBuffer[:m.inputCursor-1] + m.inputBuffer[m.inputCursor:]
			m.inputCursor--
		}
	case "left":
		if m.inputCursor > 0 {
			m.inputCursor--
		}
	case "right":
		if m.inputCursor < len(m.inputBuffer) {
			m.inputCursor++
		}
	case "ctrl+a", "home":
		m.inputCursor = 0
	case "ctrl+e", "end":
		m.inputCursor = len(m.inputBuffer)
	case "ctrl+u":
		// Clear line
		m.inputBuffer = ""
		m.inputCursor = 0
	case " ":
		// Space key
		m.inputBuffer = m.inputBuffer[:m.inputCursor] + " " + m.inputBuffer[m.inputCursor:]
		m.inputCursor++
	default:
		// Insert character - use Runes for proper character handling
		if msg.Type == tea.KeyRunes && len(msg.Runes) > 0 {
			char := string(msg.Runes)
			m.inputBuffer = m.inputBuffer[:m.inputCursor] + char + m.inputBuffer[m.inputCursor:]
			m.inputCursor += len(char)
		}
	}
	return m, nil
}

// handleAgentEvent processes events from the agent.
func (m ChatViewModel) handleAgentEvent(evt agent.Event) (ChatViewModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch evt.Type {
	case agent.EventOutput:
		if data, ok := evt.Data.(agent.OutputData); ok {
			// Append to last agent message or create new one
			if len(m.messages) > 0 && m.messages[len(m.messages)-1].Role == RoleAgent {
				m.messages[len(m.messages)-1].Content += data.Text
			} else {
				m.messages = append(m.messages, ChatMessage{
					Role:      RoleAgent,
					Content:   data.Text,
					Timestamp: time.Now(),
				})
			}

			// Smart scroll
			if m.atBottom {
				m.scrollY = m.maxScroll()
			}
		}

	case agent.EventToolUse:
		if data, ok := evt.Data.(agent.ToolUseData); ok {
			m.toolStates[data.ToolID] = &ToolState{
				Name:   data.ToolName,
				Status: ToolInProgress,
			}
			m.messages = append(m.messages, ChatMessage{
				Role:      RoleTool,
				Content:   data.ToolName,
				Timestamp: time.Now(),
				ToolID:    data.ToolID,
			})

			if m.atBottom {
				m.scrollY = m.maxScroll()
			}
		}

	case agent.EventToolResult:
		if data, ok := evt.Data.(agent.ToolResultData); ok {
			if ts, ok := m.toolStates[data.ToolID]; ok {
				ts.Output = data.Output
				if data.Success {
					ts.Status = ToolSuccess
				} else {
					ts.Status = ToolError
					ts.Expanded = true // Auto-expand on error
				}
			}
		}

	case agent.EventError:
		if data, ok := evt.Data.(agent.ErrorData); ok {
			m.messages = append(m.messages, ChatMessage{
				Role:      RoleError,
				Content:   data.Message,
				Timestamp: time.Now(),
			})

			if m.atBottom {
				m.scrollY = m.maxScroll()
			}
		}

	case agent.EventStateChange:
		if data, ok := evt.Data.(agent.StateChangeData); ok {
			m.agentState = data.NewState
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the chat view.
func (m ChatViewModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	// Reserve space for input area (3 lines: separator + mode indicator + input)
	inputHeight := 3
	chatHeight := m.height - inputHeight
	if chatHeight < 1 {
		chatHeight = 1
	}

	// Render messages
	var lines []string
	for _, msg := range m.messages {
		lines = append(lines, m.renderMessage(msg)...)
	}

	// Apply scrolling
	m.clampScroll()
	startLine := m.scrollY
	endLine := startLine + chatHeight
	if endLine > len(lines) {
		endLine = len(lines)
	}

	var visibleLines []string
	if startLine < len(lines) {
		visibleLines = lines[startLine:endLine]
	}

	// Pad to fill chat area
	for len(visibleLines) < chatHeight {
		visibleLines = append(visibleLines, "")
	}

	chatContent := strings.Join(visibleLines, "\n")

	// Render input area
	inputArea := m.renderInputArea()

	return chatContent + "\n" + inputArea
}

// renderMessage renders a single chat message.
func (m ChatViewModel) renderMessage(msg ChatMessage) []string {
	switch msg.Role {
	case RoleUser:
		prefix := ChatUserStyle.Render("You: ")
		content := msg.Content
		return wrapLines(prefix+content, m.width)

	case RoleAgent:
		prefix := ChatAgentStyle.Render("Agent: ")
		content := msg.Content
		return wrapLines(prefix+content, m.width)

	case RoleTool:
		ts := m.toolStates[msg.ToolID]
		if ts == nil {
			return []string{ChatToolStyle.Render("  " + IndicatorRunning + " " + msg.Content)}
		}

		var indicator string
		switch ts.Status {
		case ToolInProgress:
			indicator = IndicatorRunningStyle.Render(IndicatorRunning)
		case ToolSuccess:
			indicator = ChatToolSuccessStyle.Render("✓")
		case ToolError:
			indicator = ChatToolErrorStyle.Render("✗")
		}

		line := ChatToolStyle.Render("  " + indicator + " " + ts.Name)
		lines := []string{line}

		// Show expanded output
		if ts.Expanded && ts.Output != "" {
			for _, l := range strings.Split(ts.Output, "\n") {
				lines = append(lines, "    "+l)
			}
		}
		return lines

	case RoleError:
		prefix := ErrorStyle.Render("Error: ")
		lines := wrapLines(prefix+msg.Content, m.width)
		if m.agentState == agent.StateError {
			lines = append(lines, HelpStyle.Render("  Press 'r' to retry"))
		}
		return lines
	}

	return []string{msg.Content}
}

// renderInputArea renders the input section.
func (m ChatViewModel) renderInputArea() string {
	separator := strings.Repeat("─", m.width)

	// Mode indicator
	var modeStr string
	if m.inputMode == ModeInsert {
		modeStr = ChatModeInsertStyle.Render("-- INSERT --")
	} else {
		modeStr = ChatModeNormalStyle.Render("-- NORMAL --")
	}

	// Agent state indicator
	var stateStr string
	switch m.agentState {
	case agent.StateRunning:
		stateStr = IndicatorRunningStyle.Render(" [running]")
	case agent.StateError:
		stateStr = ErrorStyle.Render(" [crashed]")
	case agent.StateIdle:
		stateStr = IndicatorIdleStyle.Render(" [ready]")
	}

	statusLine := modeStr + stateStr

	// Input line with cursor
	var inputLine string
	if m.inputMode == ModeInsert {
		// Show cursor
		before := m.inputBuffer[:m.inputCursor]
		after := m.inputBuffer[m.inputCursor:]
		cursor := "▌"
		inputLine = "> " + before + cursor + after
	} else {
		if m.inputBuffer == "" {
			inputLine = HelpStyle.Render("> Press 'i' to type")
		} else {
			inputLine = "> " + m.inputBuffer
		}
	}

	return separator + "\n" + statusLine + "\n" + inputLine
}

// wrapLines wraps text to fit within width.
func wrapLines(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}

	var lines []string
	for _, line := range strings.Split(text, "\n") {
		for len(line) > width {
			lines = append(lines, line[:width])
			line = line[width:]
		}
		lines = append(lines, line)
	}
	return lines
}

// clampScroll ensures scrollY is within valid bounds.
func (m *ChatViewModel) clampScroll() {
	maxScroll := m.maxScroll()
	if m.scrollY > maxScroll {
		m.scrollY = maxScroll
	}
	if m.scrollY < 0 {
		m.scrollY = 0
	}
}

// maxScroll returns the maximum scroll position.
func (m ChatViewModel) maxScroll() int {
	// Count total lines
	var totalLines int
	for _, msg := range m.messages {
		totalLines += len(m.renderMessage(msg))
	}

	inputHeight := 3
	visibleHeight := m.height - inputHeight
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	max := totalLines - visibleHeight
	if max < 0 {
		return 0
	}
	return max
}

// SetSize sets the dimensions of the chat view.
func (m *ChatViewModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetFocused sets whether the chat view is focused.
func (m *ChatViewModel) SetFocused(focused bool) {
	m.focused = focused
}

// Focused returns whether the chat view is focused.
func (m ChatViewModel) Focused() bool {
	return m.focused
}
