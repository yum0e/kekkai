package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// Process represents a single Claude Code subprocess.
type Process struct {
	Name    string
	WorkDir string
	State   State

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	cancel context.CancelFunc
	events chan<- Event
	pid    int

	mu sync.RWMutex
}

// NewProcess creates a new Process instance.
func NewProcess(name, workDir string, events chan<- Event) *Process {
	return &Process{
		Name:    name,
		WorkDir: workDir,
		State:   StateIdle,
		events:  events,
	}
}

// Start spawns the claude process with --output-format stream-json.
func (p *Process) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.State == StateRunning {
		return nil // Already running
	}

	ctx, cancel := context.WithCancel(ctx)
	p.cancel = cancel

	p.cmd = exec.CommandContext(ctx, "claude",
		"-p",                                // Print mode (non-interactive)
		"--verbose",                         // Required for stream-json output
		"--input-format", "stream-json",     // Accept streaming JSON input
		"--output-format", "stream-json",    // Output streaming JSON
	)
	p.cmd.Dir = p.WorkDir

	stdin, err := p.cmd.StdinPipe()
	if err != nil {
		cancel()
		return err
	}
	p.stdin = stdin

	stdout, err := p.cmd.StdoutPipe()
	if err != nil {
		cancel()
		return err
	}

	stderr, err := p.cmd.StderrPipe()
	if err != nil {
		cancel()
		return err
	}

	if err := p.cmd.Start(); err != nil {
		cancel()
		return err
	}

	p.pid = p.cmd.Process.Pid
	oldState := p.State
	p.State = StateRunning

	// Emit state change event
	p.emitStateChange(oldState, StateRunning)

	// Start output reader goroutine
	go p.readOutput(stdout)

	// Start stderr reader goroutine for debugging
	go p.readStderr(stderr)

	// Start process waiter goroutine
	go p.waitProcess()

	return nil
}

// Stop gracefully stops the process (SIGTERM, then SIGKILL after timeout).
// This is non-blocking - it signals the process to stop and returns immediately.
// The waitProcess goroutine will handle state updates when the process exits.
func (p *Process) Stop(timeout time.Duration) error {
	p.mu.Lock()
	if p.State != StateRunning || p.cmd == nil || p.cmd.Process == nil {
		p.mu.Unlock()
		return nil
	}

	// Close stdin to signal EOF
	if p.stdin != nil {
		p.stdin.Close()
		p.stdin = nil
	}

	// Get process reference and cancel func while holding lock
	proc := p.cmd.Process
	cancel := p.cancel
	p.mu.Unlock()

	// Send SIGTERM
	_ = proc.Signal(syscall.SIGTERM)

	// Schedule force kill after timeout (non-blocking)
	go func() {
		time.Sleep(timeout)
		p.mu.RLock()
		state := p.State
		p.mu.RUnlock()
		if state == StateRunning {
			proc.Kill()
		}
	}()

	// Cancel context to stop readOutput goroutine
	if cancel != nil {
		cancel()
	}

	return nil
}

// userInputMessage represents the JSON format for user input in stream-json mode.
// Format matches Claude's expected input structure
type userInputMessage struct {
	Type    string      `json:"type"`
	Message userMessage `json:"message"`
}

type userMessage struct {
	Role    string         `json:"role"`
	Content []contentBlock `json:"content"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// SendInput writes to stdin (used by M5 for user input).
// Input is formatted as stream-json for Claude's --input-format stream-json mode.
func (p *Process) SendInput(input string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.State != StateRunning {
		return fmt.Errorf("process not running (state=%s)", p.State)
	}
	if p.stdin == nil {
		return fmt.Errorf("stdin is nil")
	}

	// Format as stream-json user message
	msg := userInputMessage{
		Type: "user",
		Message: userMessage{
			Role: "user",
			Content: []contentBlock{
				{Type: "text", Text: input},
			},
		},
	}

	jsonBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal input: %w", err)
	}

	// Write JSON followed by newline (NDJSON format)
	_, err = p.stdin.Write(append(jsonBytes, '\n'))
	return err
}

// GetPID returns the process ID.
func (p *Process) GetPID() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.pid
}

// GetState returns the current state.
func (p *Process) GetState() State {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.State
}

// readOutput parses stream-json from stdout and emits events.
func (p *Process) readOutput(stdout io.Reader) {
	scanner := bufio.NewScanner(stdout)
	// Increase buffer size for potentially large JSON lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024) // 1MB max line size

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		events, err := ParseEvent(line, p.Name)
		if err != nil {
			// Emit parse error
			p.emit(Event{
				AgentName: p.Name,
				Type:      EventError,
				Data: ErrorData{
					Message: fmt.Sprintf("failed to parse event: %s (line: %s)", err, string(line)),
					Err:     err,
				},
			})
			continue
		}

		for _, evt := range events {
			p.emit(evt)
		}
	}

	if err := scanner.Err(); err != nil {
		p.emit(Event{
			AgentName: p.Name,
			Type:      EventError,
			Data: ErrorData{
				Message: "error reading output",
				Err:     err,
			},
		})
	}
}

// readStderr reads stderr and emits as error events for debugging.
func (p *Process) readStderr(stderr io.Reader) {
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			p.emit(Event{
				AgentName: p.Name,
				Type:      EventError,
				Data: ErrorData{
					Message: fmt.Sprintf("[stderr] %s", line),
				},
			})
		}
	}
}

// waitProcess waits for the process to exit and updates state.
func (p *Process) waitProcess() {
	if p.cmd == nil {
		return
	}

	err := p.cmd.Wait()

	p.mu.Lock()
	oldState := p.State
	if err != nil {
		p.State = StateError
		p.emit(Event{
			AgentName: p.Name,
			Type:      EventError,
			Data: ErrorData{
				Message: "process exited with error",
				Err:     err,
			},
		})
	} else {
		p.State = StateStopped
	}
	p.emitStateChange(oldState, p.State)
	p.mu.Unlock()
}

// emit sends an event to the events channel.
func (p *Process) emit(evt Event) {
	if p.events != nil {
		select {
		case p.events <- evt:
		default:
			// Channel full, drop event to prevent blocking
		}
	}
}

// emitStateChange emits a state change event.
func (p *Process) emitStateChange(oldState, newState State) {
	p.emit(Event{
		AgentName: p.Name,
		Type:      EventStateChange,
		Data: StateChangeData{
			OldState: oldState,
			NewState: newState,
		},
	})
}
