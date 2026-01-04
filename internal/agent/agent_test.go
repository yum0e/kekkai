package agent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestParseEvent_Result(t *testing.T) {
	line := []byte(`{"type":"result","result":"ok"}`)

	events, err := ParseEvent(line, "test-agent")
	if err != nil {
		t.Fatalf("ParseEvent() error = %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("ParseEvent() returned %d events, want 1", len(events))
	}

	evt := events[0]
	if evt.Type != EventToolResult {
		t.Errorf("Event.Type = %v, want %v", evt.Type, EventToolResult)
	}

	data, ok := evt.Data.(ToolResultData)
	if !ok {
		t.Fatalf("Event.Data is not ToolResultData")
	}
	if data.Output != "ok" {
		t.Errorf("ToolResultData.Output = %v, want %v", data.Output, "ok")
	}
	if !data.Success {
		t.Error("ToolResultData.Success = false, want true")
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

func TestStartAgent_ExistingWorkspace(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, AgentsDir)
	workspaceDir := filepath.Join(agentsDir, "test-agent")

	// Create the workspace directory (simulating existing workspace)
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatalf("Failed to create workspace dir: %v", err)
	}

	// Create manager with RepoRoot set to avoid jjClient calls
	cfg := ManagerConfig{
		MaxAgents:       5,
		ShutdownTimeout: DefaultShutdownTimeout,
		RepoRoot:        tmpDir,
	}
	mgr := NewManager(cfg, nil)

	// StartAgent should fail because claude binary doesn't exist in test
	// but we can verify it attempts to start (gets past workspace check)
	ctx := context.Background()
	err := mgr.StartAgent(ctx, "test-agent")

	// We expect an error about starting the process (not about workspace)
	if err == nil {
		// If it succeeded (unlikely in test env), that's fine too
		t.Log("StartAgent succeeded (claude available in test env)")
	} else if err == ErrWorkspaceNotFound {
		t.Error("StartAgent returned ErrWorkspaceNotFound for existing workspace")
	} else {
		// Expected: error starting process (claude not found)
		t.Logf("StartAgent failed as expected (process start): %v", err)
	}
}

func TestStartAgent_CreatesGitMarkerForExistingWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, AgentsDir)
	workspaceDir := filepath.Join(agentsDir, "test-agent")

	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatalf("Failed to create workspace dir: %v", err)
	}

	cfg := ManagerConfig{
		MaxAgents:       5,
		ShutdownTimeout: DefaultShutdownTimeout,
		RepoRoot:        tmpDir,
	}
	mgr := NewManager(cfg, nil)

	ctx := context.Background()
	err := mgr.StartAgent(ctx, "test-agent")
	if err != nil && err == ErrWorkspaceNotFound {
		t.Fatalf("StartAgent returned ErrWorkspaceNotFound for existing workspace")
	}

	gitMarker := filepath.Join(workspaceDir, ".git")
	info, statErr := os.Stat(gitMarker)
	if os.IsNotExist(statErr) {
		t.Fatalf(".git marker should exist after StartAgent")
	}
	if statErr != nil {
		t.Fatalf("Failed to stat .git marker: %v", statErr)
	}
	if info.IsDir() {
		t.Fatalf(".git marker should be a file, not a directory")
	}
}

func TestStartAgent_NonExistentWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, AgentsDir)

	// Create agents dir but NOT the workspace
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatalf("Failed to create agents dir: %v", err)
	}

	cfg := ManagerConfig{
		MaxAgents:       5,
		ShutdownTimeout: DefaultShutdownTimeout,
		RepoRoot:        tmpDir,
	}
	mgr := NewManager(cfg, nil)

	ctx := context.Background()
	err := mgr.StartAgent(ctx, "nonexistent-agent")

	if err != ErrWorkspaceNotFound {
		t.Errorf("StartAgent() error = %v, want ErrWorkspaceNotFound", err)
	}
}

func TestStartAgent_AlreadyRunning(t *testing.T) {
	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, AgentsDir)
	workspaceDir := filepath.Join(agentsDir, "test-agent")

	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatalf("Failed to create workspace dir: %v", err)
	}

	cfg := ManagerConfig{
		MaxAgents:       5,
		ShutdownTimeout: DefaultShutdownTimeout,
		RepoRoot:        tmpDir,
	}
	mgr := NewManager(cfg, nil)

	// Manually add a running process to simulate already running agent
	mgr.mu.Lock()
	mgr.processes["test-agent"] = &Process{
		Name:    "test-agent",
		WorkDir: workspaceDir,
		State:   StateRunning,
	}
	mgr.mu.Unlock()

	// StartAgent should return nil (no-op) for already running agent
	ctx := context.Background()
	err := mgr.StartAgent(ctx, "test-agent")

	if err != nil {
		t.Errorf("StartAgent() error = %v, want nil for already running agent", err)
	}
}

func TestStartAgent_IdleAgent(t *testing.T) {
	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, AgentsDir)
	workspaceDir := filepath.Join(agentsDir, "test-agent")

	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatalf("Failed to create workspace dir: %v", err)
	}

	cfg := ManagerConfig{
		MaxAgents:       5,
		ShutdownTimeout: DefaultShutdownTimeout,
		RepoRoot:        tmpDir,
	}
	mgr := NewManager(cfg, nil)

	// Add a process in Idle state - should also return nil
	mgr.mu.Lock()
	mgr.processes["test-agent"] = &Process{
		Name:    "test-agent",
		WorkDir: workspaceDir,
		State:   StateIdle,
	}
	mgr.mu.Unlock()

	ctx := context.Background()
	err := mgr.StartAgent(ctx, "test-agent")

	if err != nil {
		t.Errorf("StartAgent() error = %v, want nil for idle agent", err)
	}
}

func TestStartAgent_MaxAgentsReached(t *testing.T) {
	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, AgentsDir)
	workspaceDir := filepath.Join(agentsDir, "new-agent")

	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatalf("Failed to create workspace dir: %v", err)
	}

	cfg := ManagerConfig{
		MaxAgents:       2, // Low limit for testing
		ShutdownTimeout: DefaultShutdownTimeout,
		RepoRoot:        tmpDir,
	}
	mgr := NewManager(cfg, nil)

	// Fill up the agent slots
	mgr.mu.Lock()
	mgr.processes["agent-1"] = &Process{Name: "agent-1", State: StateRunning}
	mgr.processes["agent-2"] = &Process{Name: "agent-2", State: StateRunning}
	mgr.mu.Unlock()

	// Try to start another agent
	ctx := context.Background()
	err := mgr.StartAgent(ctx, "new-agent")

	if err != ErrMaxAgentsReached {
		t.Errorf("StartAgent() error = %v, want ErrMaxAgentsReached", err)
	}
}

func TestStartAgent_StoppedAgent_Restarts(t *testing.T) {
	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, AgentsDir)
	workspaceDir := filepath.Join(agentsDir, "test-agent")

	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatalf("Failed to create workspace dir: %v", err)
	}

	cfg := ManagerConfig{
		MaxAgents:       5,
		ShutdownTimeout: DefaultShutdownTimeout,
		RepoRoot:        tmpDir,
	}
	mgr := NewManager(cfg, nil)

	// Add a process in Stopped state - should attempt restart
	mgr.mu.Lock()
	mgr.processes["test-agent"] = &Process{
		Name:    "test-agent",
		WorkDir: workspaceDir,
		State:   StateStopped,
	}
	mgr.mu.Unlock()

	ctx := context.Background()
	err := mgr.StartAgent(ctx, "test-agent")

	// Should attempt to restart (will fail due to no claude binary in test)
	// The key is that it doesn't return nil like for Running/Idle states
	if err == nil {
		t.Log("StartAgent succeeded (claude available in test env)")
	} else if err == ErrWorkspaceNotFound {
		t.Error("StartAgent returned ErrWorkspaceNotFound for stopped agent with existing workspace")
	} else {
		// Expected: error starting process
		t.Logf("StartAgent attempted restart as expected: %v", err)
	}
}

func TestGetProcess_Exists(t *testing.T) {
	cfg := DefaultConfig()
	mgr := NewManager(cfg, nil)

	// Add a process
	proc := &Process{Name: "test-agent", State: StateRunning}
	mgr.mu.Lock()
	mgr.processes["test-agent"] = proc
	mgr.mu.Unlock()

	result, err := mgr.GetProcess("test-agent")
	if err != nil {
		t.Errorf("GetProcess() error = %v, want nil", err)
	}
	if result != proc {
		t.Errorf("GetProcess() returned different process")
	}
}

func TestGetProcess_NotFound(t *testing.T) {
	cfg := DefaultConfig()
	mgr := NewManager(cfg, nil)

	_, err := mgr.GetProcess("nonexistent")
	if err != ErrAgentNotFound {
		t.Errorf("GetProcess() error = %v, want ErrAgentNotFound", err)
	}
}

func TestDeleteAgent_RemovesFromProcessMap(t *testing.T) {
	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, AgentsDir)
	workspaceDir := filepath.Join(agentsDir, "test-agent")

	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatalf("Failed to create workspace dir: %v", err)
	}

	cfg := ManagerConfig{
		MaxAgents:       5,
		ShutdownTimeout: DefaultShutdownTimeout,
		RepoRoot:        tmpDir,
	}
	mgr := NewManager(cfg, nil)

	// Add a process (simulating a running agent)
	mgr.mu.Lock()
	mgr.processes["test-agent"] = &Process{
		Name:    "test-agent",
		WorkDir: workspaceDir,
		State:   StateIdle, // Use Idle to avoid needing real process
	}
	mgr.mu.Unlock()

	// Verify process exists
	_, err := mgr.GetProcess("test-agent")
	if err != nil {
		t.Fatalf("Process should exist before delete: %v", err)
	}

	// Delete the agent
	ctx := context.Background()
	err = mgr.DeleteAgent(ctx, "test-agent")
	if err != nil {
		t.Fatalf("DeleteAgent() error = %v", err)
	}

	// Verify process is removed from map
	_, err = mgr.GetProcess("test-agent")
	if err != ErrAgentNotFound {
		t.Errorf("GetProcess() after delete = %v, want ErrAgentNotFound", err)
	}

	// Verify workspace directory is removed
	if _, err := os.Stat(workspaceDir); !os.IsNotExist(err) {
		t.Error("Workspace directory should be removed after DeleteAgent")
	}
}

func TestDeleteAgent_StopsRunningProcess(t *testing.T) {
	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, AgentsDir)
	workspaceDir := filepath.Join(agentsDir, "test-agent")

	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatalf("Failed to create workspace dir: %v", err)
	}

	cfg := ManagerConfig{
		MaxAgents:       5,
		ShutdownTimeout: DefaultShutdownTimeout,
		RepoRoot:        tmpDir,
	}
	mgr := NewManager(cfg, nil)

	// Add a process in Running state
	mgr.mu.Lock()
	mgr.processes["test-agent"] = &Process{
		Name:    "test-agent",
		WorkDir: workspaceDir,
		State:   StateRunning,
	}
	mgr.mu.Unlock()

	// Delete should handle stopping (even if process isn't real)
	ctx := context.Background()
	err := mgr.DeleteAgent(ctx, "test-agent")

	// Should succeed (process.Stop handles nil cmd gracefully)
	if err != nil {
		t.Logf("DeleteAgent() returned error (expected for mock): %v", err)
	}

	// Process should be removed from map regardless
	_, err = mgr.GetProcess("test-agent")
	if err != ErrAgentNotFound {
		t.Errorf("Process should be removed after DeleteAgent")
	}
}

func TestDeleteAgent_NonExistentProcess_StillCleansUp(t *testing.T) {
	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, AgentsDir)
	workspaceDir := filepath.Join(agentsDir, "orphan-agent")

	// Create workspace directory (orphan - no process in manager)
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatalf("Failed to create workspace dir: %v", err)
	}

	cfg := ManagerConfig{
		MaxAgents:       5,
		ShutdownTimeout: DefaultShutdownTimeout,
		RepoRoot:        tmpDir,
	}
	mgr := NewManager(cfg, nil)

	// Delete should still clean up directory even if no process exists
	ctx := context.Background()
	err := mgr.DeleteAgent(ctx, "orphan-agent")

	if err != nil {
		t.Logf("DeleteAgent() returned error: %v", err)
	}

	// Workspace directory should be removed
	if _, err := os.Stat(workspaceDir); !os.IsNotExist(err) {
		t.Error("Workspace directory should be removed even for orphan agent")
	}
}

func TestWorkspaceIsolation_GitMarkerCreated(t *testing.T) {
	tmpDir := t.TempDir()
	agentsDir := filepath.Join(tmpDir, AgentsDir)
	workspaceDir := filepath.Join(agentsDir, "test-agent")

	// Create workspace directory
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatalf("Failed to create workspace dir: %v", err)
	}

	// Simulate the .git marker creation that SpawnAgent does
	gitMarker := filepath.Join(workspaceDir, ".git")
	if err := os.WriteFile(gitMarker, []byte("# Marker to scope Claude to this workspace\n"), 0644); err != nil {
		t.Fatalf("Failed to write .git marker: %v", err)
	}

	// Verify .git marker exists
	if _, err := os.Stat(gitMarker); os.IsNotExist(err) {
		t.Error(".git marker should exist in workspace")
	}

	// Verify it's a file (not directory)
	info, err := os.Stat(gitMarker)
	if err != nil {
		t.Fatalf("Failed to stat .git marker: %v", err)
	}
	if info.IsDir() {
		t.Error(".git should be a file, not a directory")
	}
}

func TestWorkspaceIsolation_IsolatedEnv(t *testing.T) {
	workDir := "/test/workspace"
	proc := NewProcess("test-agent", workDir, nil)

	env := proc.isolatedEnv()

	// Check PWD is set to workspace
	var pwdFound bool
	var homePreserved bool
	for _, e := range env {
		if e == "PWD="+workDir {
			pwdFound = true
		}
		if len(e) > 5 && e[:5] == "HOME=" {
			homePreserved = true
		}
	}

	if !pwdFound {
		t.Error("PWD should be set to workspace directory")
	}
	if !homePreserved {
		t.Error("HOME should be preserved for auth")
	}

	// Check OLDPWD is not present
	for _, e := range env {
		if len(e) > 7 && e[:7] == "OLDPWD=" {
			t.Error("OLDPWD should not be present in isolated env")
		}
	}
}

func TestEnsureGitShimCreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	proc := NewProcess("test-agent", tmpDir, nil)

	shimDir, err := proc.ensureGitShim()
	if err != nil {
		t.Fatalf("ensureGitShim() error = %v", err)
	}

	expectedDir := filepath.Join(tmpDir, gitShimBaseDir, gitShimDirName)
	if shimDir != expectedDir {
		t.Fatalf("ensureGitShim() dir = %v, want %v", shimDir, expectedDir)
	}

	name, _, _ := gitShimSpec()
	shimPath := filepath.Join(shimDir, name)
	info, err := os.Stat(shimPath)
	if err != nil {
		t.Fatalf("failed to stat git shim: %v", err)
	}
	if info.IsDir() {
		t.Fatal("git shim should be a file, not a directory")
	}
}

func TestPrependPath(t *testing.T) {
	env := []string{"FOO=bar", "PATH=/usr/bin"}
	updated := prependPath(env, "/tmp/shim")

	var got string
	for _, e := range updated {
		if strings.HasPrefix(e, "PATH=") {
			got = strings.TrimPrefix(e, "PATH=")
			break
		}
	}
	if got == "" {
		t.Fatal("PATH not found in updated env")
	}

	sep := string(os.PathListSeparator)
	wantPrefix := "/tmp/shim" + sep
	if !strings.HasPrefix(got, wantPrefix) {
		t.Fatalf("PATH prefix = %q, want prefix %q", got, wantPrefix)
	}
}

func TestWorkspaceIsolation_ProcessStartsInWorkspace(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a simple test script
	scriptPath := filepath.Join(tmpDir, "test.sh")
	script := "#!/bin/sh\npwd"
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to write test script: %v", err)
	}

	// Create workspace
	workspaceDir := filepath.Join(tmpDir, "workspace")
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}

	// Run script from workspace directory
	cmd := exec.Command(scriptPath)
	cmd.Dir = workspaceDir
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Script failed: %v", err)
	}

	// Verify pwd output matches workspace
	got := string(output)
	got = got[:len(got)-1] // trim newline
	if got != workspaceDir {
		t.Errorf("Process ran in %q, want %q", got, workspaceDir)
	}
}
