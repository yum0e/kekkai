package agent

import (
	"bufio"
	"context"
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

	p.cmd = exec.CommandContext(ctx, "claude", "--output-format", "stream-json")
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

	// Start process waiter goroutine
	go p.waitProcess()

	return nil
}

// Stop gracefully stops the process (SIGTERM, then SIGKILL after timeout).
func (p *Process) Stop(timeout time.Duration) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.State != StateRunning || p.cmd == nil || p.cmd.Process == nil {
		return nil
	}

	// Close stdin to signal EOF
	if p.stdin != nil {
		p.stdin.Close()
	}

	// Send SIGTERM
	if err := p.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		// Process might already be dead
		return nil
	}

	// Wait for process to exit with timeout
	done := make(chan error, 1)
	go func() {
		done <- p.cmd.Wait()
	}()

	select {
	case <-done:
		// Process exited gracefully
	case <-time.After(timeout):
		// Force kill
		p.cmd.Process.Kill()
		<-done // Wait for process to actually exit
	}

	oldState := p.State
	p.State = StateStopped
	p.emitStateChange(oldState, StateStopped)

	if p.cancel != nil {
		p.cancel()
	}

	return nil
}

// SendInput writes to stdin (used by M5 for user input).
func (p *Process) SendInput(input string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.State != StateRunning || p.stdin == nil {
		return nil
	}

	_, err := p.stdin.Write([]byte(input + "\n"))
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
					Message: "failed to parse event",
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
