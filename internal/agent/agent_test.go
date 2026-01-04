package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStateString(t *testing.T) {
	tests := []struct {
		state    State
		expected string
	}{
		{StateIdle, "idle"},
		{StateRunning, "running"},
		{StateStopped, "stopped"},
		{StateError, "error"},
		{State(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.state.String(); got != tt.expected {
				t.Errorf("State.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestEventTypeString(t *testing.T) {
	tests := []struct {
		eventType EventType
		expected  string
	}{
		{EventOutput, "output"},
		{EventToolUse, "tool_use"},
		{EventToolResult, "tool_result"},
		{EventError, "error"},
		{EventStateChange, "state_change"},
		{EventType(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.eventType.String(); got != tt.expected {
				t.Errorf("EventType.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestParseEvent_Assistant_Text(t *testing.T) {
	line := []byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"Hello, world!"}]}}`)

	events, err := ParseEvent(line, "test-agent")
	if err != nil {
		t.Fatalf("ParseEvent() error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("ParseEvent() returned %d events, want 1", len(events))
	}

	evt := events[0]
	if evt.AgentName != "test-agent" {
		t.Errorf("Event.AgentName = %v, want %v", evt.AgentName, "test-agent")
	}
	if evt.Type != EventOutput {
		t.Errorf("Event.Type = %v, want %v", evt.Type, EventOutput)
	}

	data, ok := evt.Data.(OutputData)
	if !ok {
		t.Fatalf("Event.Data is not OutputData")
	}
	if data.Text != "Hello, world!" {
		t.Errorf("OutputData.Text = %v, want %v", data.Text, "Hello, world!")
	}
}

func TestParseEvent_Assistant_ToolUse(t *testing.T) {
	line := []byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tool_123","name":"Read","input":{"path":"/test"}}]}}`)

	events, err := ParseEvent(line, "test-agent")
	if err != nil {
		t.Fatalf("ParseEvent() error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("ParseEvent() returned %d events, want 1", len(events))
	}

	evt := events[0]
	if evt.Type != EventToolUse {
		t.Errorf("Event.Type = %v, want %v", evt.Type, EventToolUse)
	}

	data, ok := evt.Data.(ToolUseData)
	if !ok {
		t.Fatalf("Event.Data is not ToolUseData")
	}
	if data.ToolID != "tool_123" {
		t.Errorf("ToolUseData.ToolID = %v, want %v", data.ToolID, "tool_123")
	}
	if data.ToolName != "Read" {
		t.Errorf("ToolUseData.ToolName = %v, want %v", data.ToolName, "Read")
	}
}

func TestParseEvent_Error(t *testing.T) {
	line := []byte(`{"type":"error","error":"Something went wrong"}`)

	events, err := ParseEvent(line, "test-agent")
	if err != nil {
		t.Fatalf("ParseEvent() error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("ParseEvent() returned %d events, want 1", len(events))
	}

	evt := events[0]
	if evt.Type != EventError {
		t.Errorf("Event.Type = %v, want %v", evt.Type, EventError)
	}

	data, ok := evt.Data.(ErrorData)
	if !ok {
		t.Fatalf("Event.Data is not ErrorData")
	}
	if data.Message != "Something went wrong" {
		t.Errorf("ErrorData.Message = %v, want %v", data.Message, "Something went wrong")
	}
}

func TestParseEvent_System_Ignored(t *testing.T) {
	line := []byte(`{"type":"system","message":"Initializing..."}`)

	events, err := ParseEvent(line, "test-agent")
	if err != nil {
		t.Fatalf("ParseEvent() error = %v", err)
	}

	if len(events) != 0 {
		t.Errorf("ParseEvent() returned %d events, want 0 (system events should be ignored)", len(events))
	}
}

func TestParseEvent_EmptyLine(t *testing.T) {
	events, err := ParseEvent([]byte{}, "test-agent")
	if err != nil {
		t.Fatalf("ParseEvent() error = %v", err)
	}

	if events != nil {
		t.Errorf("ParseEvent() returned events for empty line, want nil")
	}
}

func TestParseEvent_InvalidJSON(t *testing.T) {
	line := []byte(`not valid json`)

	_, err := ParseEvent(line, "test-agent")
	if err == nil {
		t.Error("ParseEvent() expected error for invalid JSON")
	}
}

func TestPIDFile_WriteReadRemove(t *testing.T) {
	tmpDir := t.TempDir()
	agentName := "test-agent"
	pid := 12345

	// Write PID file
	if err := WritePIDFile(tmpDir, agentName, pid); err != nil {
		t.Fatalf("WritePIDFile() error = %v", err)
	}

	// Check file exists
	pidPath := filepath.Join(tmpDir, agentName+".pid")
	if _, err := os.Stat(pidPath); os.IsNotExist(err) {
		t.Fatal("PID file was not created")
	}

	// Read PID file
	readPID, err := ReadPIDFile(tmpDir, agentName)
	if err != nil {
		t.Fatalf("ReadPIDFile() error = %v", err)
	}
	if readPID != pid {
		t.Errorf("ReadPIDFile() = %v, want %v", readPID, pid)
	}

	// Remove PID file
	if err := RemovePIDFile(tmpDir, agentName); err != nil {
		t.Fatalf("RemovePIDFile() error = %v", err)
	}

	// Check file is removed
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("PID file was not removed")
	}
}

func TestListPIDFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Write a few PID files
	agents := []string{"agent-1", "agent-2", "agent-3"}
	for i, name := range agents {
		if err := WritePIDFile(tmpDir, name, 1000+i); err != nil {
			t.Fatalf("WritePIDFile() error = %v", err)
		}
	}

	// List PID files
	names, err := ListPIDFiles(tmpDir)
	if err != nil {
		t.Fatalf("ListPIDFiles() error = %v", err)
	}

	if len(names) != len(agents) {
		t.Errorf("ListPIDFiles() returned %d names, want %d", len(names), len(agents))
	}

	// Check all names are present
	nameSet := make(map[string]bool)
	for _, name := range names {
		nameSet[name] = true
	}
	for _, agent := range agents {
		if !nameSet[agent] {
			t.Errorf("ListPIDFiles() missing agent %v", agent)
		}
	}
}

func TestListPIDFiles_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	names, err := ListPIDFiles(tmpDir)
	if err != nil {
		t.Fatalf("ListPIDFiles() error = %v", err)
	}

	if names != nil && len(names) != 0 {
		t.Errorf("ListPIDFiles() returned %d names, want 0", len(names))
	}
}

func TestListPIDFiles_NonExistentDir(t *testing.T) {
	names, err := ListPIDFiles("/nonexistent/directory")
	if err != nil {
		t.Fatalf("ListPIDFiles() error = %v", err)
	}

	if names != nil && len(names) != 0 {
		t.Errorf("ListPIDFiles() returned %d names, want 0", len(names))
	}
}

func TestIsProcessRunning_InvalidPID(t *testing.T) {
	if IsProcessRunning(0) {
		t.Error("IsProcessRunning(0) = true, want false")
	}
	if IsProcessRunning(-1) {
		t.Error("IsProcessRunning(-1) = true, want false")
	}
}

func TestIsProcessRunning_CurrentProcess(t *testing.T) {
	// Current process should be running
	if !IsProcessRunning(os.Getpid()) {
		t.Error("IsProcessRunning(current pid) = false, want true")
	}
}

func TestMockRunner(t *testing.T) {
	runner := &MockRunner{
		Events: []string{
			`{"type":"assistant","message":{"content":[{"type":"text","text":"Hello"}]}}`,
			`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"1","name":"Read","input":{}}]}}`,
		},
	}

	events := make(chan Event, 100)

	proc, err := runner.Start(nil, "test-agent", "/tmp", events)
	if err != nil {
		t.Fatalf("MockRunner.Start() error = %v", err)
	}

	if proc.GetPID() != 12345 {
		t.Errorf("MockProcess.GetPID() = %v, want 12345", proc.GetPID())
	}

	// Wait for events
	var receivedEvents []Event
	for evt := range events {
		receivedEvents = append(receivedEvents, evt)
		// Stop reading after we get state change to stopped
		if evt.Type == EventStateChange {
			if data, ok := evt.Data.(StateChangeData); ok {
				if data.NewState == StateStopped {
					break
				}
			}
		}
	}

	// Should have received: state change to running, output event, tool use event, state change to stopped
	if len(receivedEvents) < 4 {
		t.Errorf("MockRunner emitted %d events, want at least 4", len(receivedEvents))
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxAgents != DefaultMaxAgents {
		t.Errorf("DefaultConfig().MaxAgents = %v, want %v", cfg.MaxAgents, DefaultMaxAgents)
	}
	if cfg.ShutdownTimeout != DefaultShutdownTimeout {
		t.Errorf("DefaultConfig().ShutdownTimeout = %v, want %v", cfg.ShutdownTimeout, DefaultShutdownTimeout)
	}
}

func TestAgentsDir(t *testing.T) {
	if AgentsDir != ".jj/agents" {
		t.Errorf("AgentsDir = %v, want .jj/agents", AgentsDir)
	}
}
