package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/bigq/dojo/internal/jj"
)

// Default configuration values
const (
	DefaultMaxAgents       = 5
	DefaultShutdownTimeout = 30 * time.Second
	// AgentsDir is the directory inside .jj where agent workspaces live
	AgentsDir = ".jj/agents"
	// PIDSubDir is the subdirectory for PID files within AgentsDir
	PIDSubDir = ".pids"
)

// Errors
var (
	ErrMaxAgentsReached   = errors.New("maximum number of agents reached")
	ErrAgentExists        = errors.New("agent with this name already exists")
	ErrAgentNotFound      = errors.New("agent not found")
	ErrWorkspaceNotFound  = errors.New("workspace not found")
)

// ManagerConfig configures the agent manager.
type ManagerConfig struct {
	MaxAgents       int           // Maximum number of concurrent agents
	ShutdownTimeout time.Duration // Timeout before SIGKILL on shutdown
	RepoRoot        string        // Root of the jj repository (auto-detected if empty)
}

// DefaultConfig returns the default manager configuration.
func DefaultConfig() ManagerConfig {
	return ManagerConfig{
		MaxAgents:       DefaultMaxAgents,
		ShutdownTimeout: DefaultShutdownTimeout,
	}
}

// Manager manages agent processes.
type Manager struct {
	config    ManagerConfig
	processes map[string]*Process
	events    chan Event
	jjClient  *jj.Client
	mu        sync.RWMutex
}

// NewManager creates a new agent manager.
func NewManager(cfg ManagerConfig, jjClient *jj.Client) *Manager {
	if cfg.MaxAgents <= 0 {
		cfg.MaxAgents = DefaultMaxAgents
	}
	if cfg.ShutdownTimeout <= 0 {
		cfg.ShutdownTimeout = DefaultShutdownTimeout
	}

	return &Manager{
		config:    cfg,
		processes: make(map[string]*Process),
		events:    make(chan Event, 100),
		jjClient:  jjClient,
	}
}

// agentsPath returns the path to the agents directory.
func (m *Manager) agentsPath(repoRoot string) string {
	return filepath.Join(repoRoot, AgentsDir)
}

// pidPath returns the path to the PID files directory.
func (m *Manager) pidPath(repoRoot string) string {
	return filepath.Join(repoRoot, AgentsDir, PIDSubDir)
}

// agentWorkspacePath returns the path for an agent's workspace.
func (m *Manager) agentWorkspacePath(repoRoot, name string) string {
	return filepath.Join(repoRoot, AgentsDir, name)
}

// Events returns a read-only channel for consuming agent events.
func (m *Manager) Events() <-chan Event {
	return m.events
}

// SpawnAgent creates a workspace and starts a claude process.
// Steps:
// 1. Validate agent limit not exceeded
// 2. Create new revision on top of @ (jj new)
// 3. Create workspace at .jj/.workspaces/<name> (jj workspace add)
// 4. Spawn claude --output-format stream-json in workspace dir
// 5. Start output reader goroutine
func (m *Manager) SpawnAgent(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if agent already exists
	if _, exists := m.processes[name]; exists {
		return ErrAgentExists
	}

	// Check agent limit
	if len(m.processes) >= m.config.MaxAgents {
		return ErrMaxAgentsReached
	}

	// Create new revision on top of current working copy
	if err := m.jjClient.New(ctx); err != nil {
		return fmt.Errorf("failed to create new revision: %w", err)
	}

	// Get repo root for workspace path
	repoRoot := m.config.RepoRoot
	if repoRoot == "" {
		var err error
		repoRoot, err = m.jjClient.WorkspaceRoot(ctx)
		if err != nil {
			return fmt.Errorf("failed to get workspace root: %w", err)
		}
	}

	// Workspace lives inside .jj/.workspaces/<name>
	workDir := m.agentWorkspacePath(repoRoot, name)

	// Create workspace at the new revision using absolute path
	if err := m.jjClient.WorkspaceAdd(ctx, workDir, "@"); err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	// Create process
	proc := NewProcess(name, workDir, m.events)

	// Start the process
	if err := proc.Start(ctx); err != nil {
		// Clean up workspace on failure
		_ = m.jjClient.WorkspaceForget(ctx, name)
		_ = os.RemoveAll(workDir)
		return fmt.Errorf("failed to start agent process: %w", err)
	}

	// Write PID file
	if err := WritePIDFile(m.pidPath(repoRoot), name, proc.GetPID()); err != nil {
		// Non-fatal, just log
		m.events <- Event{
			AgentName: name,
			Type:      EventError,
			Data: ErrorData{
				Message: "failed to write PID file",
				Err:     err,
			},
		}
	}

	m.processes[name] = proc
	return nil
}

// StopAgent gracefully stops an agent (keeps workspace).
func (m *Manager) StopAgent(name string) error {
	m.mu.Lock()
	proc, exists := m.processes[name]
	m.mu.Unlock()

	if !exists {
		return ErrAgentNotFound
	}

	if err := proc.Stop(m.config.ShutdownTimeout); err != nil {
		return fmt.Errorf("failed to stop agent: %w", err)
	}

	// Remove PID file
	repoRoot := m.config.RepoRoot
	if repoRoot == "" {
		if root, err := m.jjClient.WorkspaceRoot(context.Background()); err == nil {
			repoRoot = root
		}
	}
	if repoRoot != "" {
		_ = RemovePIDFile(m.pidPath(repoRoot), name)
	}

	m.mu.Lock()
	delete(m.processes, name)
	m.mu.Unlock()

	return nil
}

// RestartAgent stops and restarts an agent with the same context.
func (m *Manager) RestartAgent(ctx context.Context, name string) error {
	m.mu.RLock()
	proc, exists := m.processes[name]
	m.mu.RUnlock()

	if !exists {
		return ErrAgentNotFound
	}

	workDir := proc.WorkDir

	// Stop the existing process
	if err := proc.Stop(m.config.ShutdownTimeout); err != nil {
		return fmt.Errorf("failed to stop agent for restart: %w", err)
	}

	// Create new process with same workdir
	newProc := NewProcess(name, workDir, m.events)

	// Start the new process
	if err := newProc.Start(ctx); err != nil {
		return fmt.Errorf("failed to restart agent: %w", err)
	}

	// Update PID file
	repoRoot := m.config.RepoRoot
	if repoRoot == "" {
		if root, err := m.jjClient.WorkspaceRoot(ctx); err == nil {
			repoRoot = root
		}
	}
	if repoRoot != "" {
		_ = WritePIDFile(m.pidPath(repoRoot), name, newProc.GetPID())
	}

	m.mu.Lock()
	m.processes[name] = newProc
	m.mu.Unlock()

	return nil
}

// DeleteAgent stops the agent, forgets the jj workspace, and removes the directory.
func (m *Manager) DeleteAgent(ctx context.Context, name string) error {
	m.mu.Lock()
	proc, exists := m.processes[name]
	m.mu.Unlock()

	// Get repo root for paths
	repoRoot := m.config.RepoRoot
	if repoRoot == "" {
		var err error
		repoRoot, err = m.jjClient.WorkspaceRoot(ctx)
		if err != nil {
			return fmt.Errorf("failed to get workspace root: %w", err)
		}
	}

	// Stop the process if running
	if exists {
		if err := proc.Stop(m.config.ShutdownTimeout); err != nil {
			return fmt.Errorf("failed to stop agent: %w", err)
		}

		m.mu.Lock()
		delete(m.processes, name)
		m.mu.Unlock()
	}

	// Remove PID file
	_ = RemovePIDFile(m.pidPath(repoRoot), name)

	// Forget the jj workspace
	if err := m.jjClient.WorkspaceForget(ctx, name); err != nil {
		// Non-fatal - workspace might not exist
	}

	// Remove the workspace directory
	workDir := m.agentWorkspacePath(repoRoot, name)
	if err := os.RemoveAll(workDir); err != nil {
		return fmt.Errorf("failed to remove workspace directory: %w", err)
	}

	return nil
}

// GetState returns the current state of an agent.
func (m *Manager) GetState(name string) State {
	m.mu.RLock()
	defer m.mu.RUnlock()

	proc, exists := m.processes[name]
	if !exists {
		return StateStopped
	}
	return proc.GetState()
}

// ListAgents returns all agent names and states.
func (m *Manager) ListAgents() map[string]State {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]State, len(m.processes))
	for name, proc := range m.processes {
		result[name] = proc.GetState()
	}
	return result
}

// Shutdown stops all agents gracefully.
func (m *Manager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	names := make([]string, 0, len(m.processes))
	for name := range m.processes {
		names = append(names, name)
	}
	m.mu.Unlock()

	var lastErr error
	for _, name := range names {
		if err := m.StopAgent(name); err != nil {
			lastErr = err
		}
	}

	// Close events channel
	close(m.events)

	return lastErr
}

// DetectOrphans checks PID files for orphaned processes.
func (m *Manager) DetectOrphans() ([]OrphanInfo, error) {
	repoRoot := m.config.RepoRoot
	if repoRoot == "" {
		var err error
		repoRoot, err = m.jjClient.WorkspaceRoot(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to get workspace root: %w", err)
		}
	}

	pidDir := m.pidPath(repoRoot)
	names, err := ListPIDFiles(pidDir)
	if err != nil {
		return nil, fmt.Errorf("failed to list PID files: %w", err)
	}

	var orphans []OrphanInfo
	for _, name := range names {
		// Skip if we're already managing this agent
		m.mu.RLock()
		_, managed := m.processes[name]
		m.mu.RUnlock()
		if managed {
			continue
		}

		pid, err := ReadPIDFile(pidDir, name)
		if err != nil {
			continue
		}

		if IsProcessRunning(pid) {
			orphans = append(orphans, OrphanInfo{
				Name: name,
				PID:  pid,
			})
		} else {
			// Clean up stale PID file
			_ = RemovePIDFile(pidDir, name)
		}
	}

	return orphans, nil
}

// KillOrphan terminates an orphaned process.
func (m *Manager) KillOrphan(pid int) error {
	if !IsProcessRunning(pid) {
		return nil
	}

	proc, err := findProcess(pid)
	if err != nil {
		return err
	}

	return proc.Kill()
}

// findProcess finds a process by PID (platform-specific).
func findProcess(pid int) (interface{ Kill() error }, error) {
	return findProcessByPID(pid)
}
