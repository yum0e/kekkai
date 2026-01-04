package tui

import (
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bigq/dojo/internal/agent"
)

// TestEventListenerReceivesAgentEvents tests that the event listener
// properly receives events from the agent manager and forwards them.
func TestEventListenerReceivesAgentEvents(t *testing.T) {
	// Create a manager with test config
	cfg := agent.ManagerConfig{
		MaxAgents:       5,
		ShutdownTimeout: time.Second,
		RepoRoot:        t.TempDir(),
	}
	mgr := agent.NewManager(cfg, nil)

	// Directly test the event flow through the channel
	events := mgr.Events()
	eventsWritable := mgr.EventsWritable()

	// Send a test event to the channel
	go func() {
		// Small delay to ensure listener is ready
		time.Sleep(10 * time.Millisecond)

		// This simulates what Process.emit() does
		select {
		case eventsWritable <- agent.Event{
			AgentName: "test-agent",
			Type:      agent.EventOutput,
			Data:      agent.OutputData{Text: "Hello from agent"},
		}:
		default:
			// Channel might be closed or full
		}
	}()

	// Read from the events channel
	select {
	case evt := <-events:
		if evt.Type != agent.EventOutput {
			t.Errorf("Expected EventOutput, got %v", evt.Type)
		}
		data, ok := evt.Data.(agent.OutputData)
		if !ok {
			t.Fatal("Expected OutputData")
		}
		if data.Text != "Hello from agent" {
			t.Errorf("Expected 'Hello from agent', got %q", data.Text)
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for event")
	}
}

// TestChatInputMsgSendsToAgent tests that ChatInputMsg properly sends input to the agent
func TestChatInputMsgSendsToAgent(t *testing.T) {
	tmpDir := t.TempDir()

	// Create manager
	cfg := agent.ManagerConfig{
		MaxAgents:       5,
		ShutdownTimeout: time.Second,
		RepoRoot:        tmpDir,
	}
	mgr := agent.NewManager(cfg, nil)

	// Create app with manager
	app := AppModel{
		agentManager: mgr,
	}

	// Test 1: ChatInputMsg when agent doesn't exist should return error event
	msg := ChatInputMsg{Workspace: "nonexistent", Input: "hello"}
	resultModel, cmd := app.Update(msg)
	_ = resultModel // Ignore the model

	if cmd == nil {
		t.Fatal("Expected command to be returned")
	}

	result := cmd()
	agentEvt, ok := result.(AgentEventMsg)
	if !ok {
		t.Fatalf("Expected AgentEventMsg, got %T", result)
	}

	if agentEvt.Event.Type != agent.EventError {
		t.Errorf("Expected EventError, got %v", agentEvt.Event.Type)
	}

	errData, ok := agentEvt.Event.Data.(agent.ErrorData)
	if !ok {
		t.Fatal("Expected ErrorData")
	}
	if errData.Message == "" {
		t.Error("Expected non-empty error message")
	}
}

// TestChatViewHandlesAgentEventMsg tests that ChatViewModel properly handles agent events
func TestChatViewHandlesAgentEventMsg(t *testing.T) {
	chat := NewChatViewModel()
	chat.workspace = "test-agent"
	chat.SetSize(80, 24)
	chat.SetFocused(true)

	// Initially no messages
	if len(chat.messages) != 0 {
		t.Fatalf("Expected 0 messages, got %d", len(chat.messages))
	}

	// Send an output event
	evt := AgentEventMsg{
		Event: agent.Event{
			AgentName: "test-agent",
			Type:      agent.EventOutput,
			Data:      agent.OutputData{Text: "Hello, I'm Claude!"},
		},
	}

	chat, _ = chat.Update(evt)

	// Should have 1 message now
	if len(chat.messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(chat.messages))
	}

	if chat.messages[0].Role != RoleAgent {
		t.Errorf("Expected RoleAgent, got %v", chat.messages[0].Role)
	}

	if chat.messages[0].Content != "Hello, I'm Claude!" {
		t.Errorf("Expected 'Hello, I'm Claude!', got %q", chat.messages[0].Content)
	}
}

// TestChatViewAppendsToContinuousAgentOutput tests that continuous output is appended
func TestChatViewAppendsToContinuousAgentOutput(t *testing.T) {
	chat := NewChatViewModel()
	chat.workspace = "test-agent"
	chat.SetSize(80, 24)

	// Send first part of output
	chat, _ = chat.Update(AgentEventMsg{
		Event: agent.Event{
			AgentName: "test-agent",
			Type:      agent.EventOutput,
			Data:      agent.OutputData{Text: "Hello, "},
		},
	})

	// Send second part
	chat, _ = chat.Update(AgentEventMsg{
		Event: agent.Event{
			AgentName: "test-agent",
			Type:      agent.EventOutput,
			Data:      agent.OutputData{Text: "world!"},
		},
	})

	// Should still have 1 message (appended)
	if len(chat.messages) != 1 {
		t.Fatalf("Expected 1 message (appended), got %d", len(chat.messages))
	}

	if chat.messages[0].Content != "Hello, world!" {
		t.Errorf("Expected 'Hello, world!', got %q", chat.messages[0].Content)
	}
}

// TestChatViewHandlesToolUse tests that tool use events are handled
func TestChatViewHandlesToolUse(t *testing.T) {
	chat := NewChatViewModel()
	chat.workspace = "test-agent"
	chat.SetSize(80, 24)

	// Send tool use event
	chat, _ = chat.Update(AgentEventMsg{
		Event: agent.Event{
			AgentName: "test-agent",
			Type:      agent.EventToolUse,
			Data: agent.ToolUseData{
				ToolID:   "tool_123",
				ToolName: "Read",
			},
		},
	})

	// Should have 1 tool message
	if len(chat.messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(chat.messages))
	}

	if chat.messages[0].Role != RoleTool {
		t.Errorf("Expected RoleTool, got %v", chat.messages[0].Role)
	}

	if chat.messages[0].ToolID != "tool_123" {
		t.Errorf("Expected tool_123, got %q", chat.messages[0].ToolID)
	}

	// Should have tool state
	if chat.toolStates["tool_123"] == nil {
		t.Fatal("Expected tool state to be tracked")
	}

	if chat.toolStates["tool_123"].Status != ToolInProgress {
		t.Errorf("Expected ToolInProgress, got %v", chat.toolStates["tool_123"].Status)
	}
}

// TestChatViewHandlesError tests that error events are handled
func TestChatViewHandlesError(t *testing.T) {
	chat := NewChatViewModel()
	chat.workspace = "test-agent"
	chat.SetSize(80, 24)

	// Send error event
	chat, _ = chat.Update(AgentEventMsg{
		Event: agent.Event{
			AgentName: "test-agent",
			Type:      agent.EventError,
			Data:      agent.ErrorData{Message: "Something went wrong"},
		},
	})

	// Should have 1 error message
	if len(chat.messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(chat.messages))
	}

	if chat.messages[0].Role != RoleError {
		t.Errorf("Expected RoleError, got %v", chat.messages[0].Role)
	}

	if chat.messages[0].Content != "Something went wrong" {
		t.Errorf("Expected 'Something went wrong', got %q", chat.messages[0].Content)
	}
}

// TestChatInputCreatesUserMessage tests that typing and pressing enter adds user message
func TestChatInputCreatesUserMessage(t *testing.T) {
	chat := NewChatViewModel()
	chat.workspace = "test-agent"
	chat.SetSize(80, 24)
	chat.SetFocused(true)

	// Enter insert mode
	chat, _ = chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	if chat.inputMode != ModeInsert {
		t.Fatal("Expected insert mode")
	}

	// Type "hello"
	for _, r := range "hello" {
		chat, _ = chat.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	if chat.inputBuffer != "hello" {
		t.Errorf("Expected inputBuffer='hello', got %q", chat.inputBuffer)
	}

	// Press enter to send
	chat, cmd := chat.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Should have 1 user message
	if len(chat.messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(chat.messages))
	}

	if chat.messages[0].Role != RoleUser {
		t.Errorf("Expected RoleUser, got %v", chat.messages[0].Role)
	}

	if chat.messages[0].Content != "hello" {
		t.Errorf("Expected 'hello', got %q", chat.messages[0].Content)
	}

	// Should return ChatInputMsg command
	if cmd == nil {
		t.Fatal("Expected command")
	}

	result := cmd()
	chatInput, ok := result.(ChatInputMsg)
	if !ok {
		t.Fatalf("Expected ChatInputMsg, got %T", result)
	}

	if chatInput.Input != "hello" {
		t.Errorf("Expected input='hello', got %q", chatInput.Input)
	}

	if chatInput.Workspace != "test-agent" {
		t.Errorf("Expected workspace='test-agent', got %q", chatInput.Workspace)
	}

	// Input buffer should be cleared
	if chat.inputBuffer != "" {
		t.Errorf("Expected empty inputBuffer, got %q", chat.inputBuffer)
	}
}

// TestRightPaneRoutesAgentEvents tests that RightPane routes events to correct workspace
func TestRightPaneRoutesAgentEvents(t *testing.T) {
	rp := NewRightPaneModel(nil)
	rp.workspace = "agent-1"
	rp.isDefault = false
	rp.activeTab = TabChat
	rp.SetSize(80, 24)
	rp.SetFocused(true)

	// Send event for matching workspace
	rp, _ = rp.Update(AgentEventMsg{
		Event: agent.Event{
			AgentName: "agent-1",
			Type:      agent.EventOutput,
			Data:      agent.OutputData{Text: "Response for agent-1"},
		},
	})

	// Should have message in chat view
	if len(rp.chatView.messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(rp.chatView.messages))
	}

	// Send event for different workspace - should be ignored
	rp, _ = rp.Update(AgentEventMsg{
		Event: agent.Event{
			AgentName: "agent-2",
			Type:      agent.EventOutput,
			Data:      agent.OutputData{Text: "Response for agent-2"},
		},
	})

	// Should still have only 1 message
	if len(rp.chatView.messages) != 1 {
		t.Fatalf("Expected 1 message (ignored different workspace), got %d", len(rp.chatView.messages))
	}
}

// TestFullMessageFlowWithMockProcess tests the complete message flow
func TestFullMessageFlowWithMockProcess(t *testing.T) {
	tmpDir := t.TempDir()

	// Create manager
	cfg := agent.ManagerConfig{
		MaxAgents:       5,
		ShutdownTimeout: time.Second,
		RepoRoot:        tmpDir,
	}
	mgr := agent.NewManager(cfg, nil)

	// Create app model
	app := AppModel{
		agentManager: mgr,
		rightPane:    NewRightPaneModel(nil),
	}
	app.rightPane.workspace = "test-agent"
	app.rightPane.isDefault = false
	app.rightPane.activeTab = TabChat
	app.rightPane.chatView.workspace = "test-agent"
	app.rightPane.SetSize(80, 24)

	// Manually inject a mock process into the manager
	// This simulates a running agent
	mockProc := &mockProcessWithInput{
		name:    "test-agent",
		state:   agent.StateRunning,
		events:  mgr.EventsWritable(),
		inputCh: make(chan string, 10),
	}

	// Inject into manager
	mgr.InjectProcessForTest("test-agent", mockProc)

	// Start a goroutine to simulate agent responding
	go func() {
		for input := range mockProc.inputCh {
			// Simulate agent response
			mockProc.events <- agent.Event{
				AgentName: "test-agent",
				Type:      agent.EventOutput,
				Data:      agent.OutputData{Text: "You said: " + input},
			}
		}
	}()

	// Send user input
	msg := ChatInputMsg{Workspace: "test-agent", Input: "hello agent"}
	resultModel, cmd := app.Update(msg)
	app = resultModel.(AppModel)

	if cmd == nil {
		t.Fatal("Expected command")
	}

	// Execute the command (sends to agent)
	result := cmd()
	if result != nil {
		// If there's an error, it will be returned as AgentEventMsg
		if errMsg, ok := result.(AgentEventMsg); ok {
			if errMsg.Event.Type == agent.EventError {
				errData := errMsg.Event.Data.(agent.ErrorData)
				t.Fatalf("SendInput failed: %s", errData.Message)
			}
		}
	}

	// Wait for agent response
	time.Sleep(50 * time.Millisecond)

	// Read from events channel and route to app
	select {
	case evt := <-mgr.Events():
		// This is what StartEventListener does - route to Update
		resultModel, _ = app.Update(AgentEventMsg{Event: evt})
		app = resultModel.(AppModel)
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for agent response")
	}

	// Verify the response made it to the chat view
	if len(app.rightPane.chatView.messages) != 1 {
		t.Fatalf("Expected 1 message in chat, got %d", len(app.rightPane.chatView.messages))
	}

	if app.rightPane.chatView.messages[0].Content != "You said: hello agent" {
		t.Errorf("Expected 'You said: hello agent', got %q", app.rightPane.chatView.messages[0].Content)
	}
}

// mockProcessWithInput is a mock process that responds to input
type mockProcessWithInput struct {
	name    string
	state   agent.State
	events  chan<- agent.Event
	inputCh chan string
	mu      sync.RWMutex
}

func (p *mockProcessWithInput) Stop(timeout time.Duration) error {
	close(p.inputCh)
	return nil
}

func (p *mockProcessWithInput) SendInput(input string) error {
	p.inputCh <- input
	return nil
}

func (p *mockProcessWithInput) GetPID() int {
	return 99999
}

func (p *mockProcessWithInput) GetState() agent.State {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state
}
