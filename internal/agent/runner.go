package agent

import (
	"bufio"
	"context"
	"io"
	"strings"
	"sync"
	"time"
)

// ProcessRunner interface for dependency injection in testing.
type ProcessRunner interface {
	Start(ctx context.Context, name, workDir string, events chan<- Event) (RunningProcess, error)
}

// RunningProcess represents a running process that can be controlled.
type RunningProcess interface {
	Stop(timeout time.Duration) error
	SendInput(input string) error
	GetPID() int
	GetState() State
}

// RealRunner uses exec.CommandContext to run real claude processes.
type RealRunner struct{}

// Start spawns a real claude process.
func (r *RealRunner) Start(ctx context.Context, name, workDir string, events chan<- Event) (RunningProcess, error) {
	proc := NewProcess(name, workDir, events)
	if err := proc.Start(ctx); err != nil {
		return nil, err
	}
	return proc, nil
}

// MockRunner reads from predefined JSON streams for testing.
type MockRunner struct {
	Events      []string      // JSON lines to emit
	Delay       time.Duration // Delay between events
	ShouldError bool          // If true, simulate an error
	ErrorMsg    string        // Error message to emit
}

// mockProcess implements RunningProcess for testing.
type mockProcess struct {
	name     string
	state    State
	events   chan<- Event
	stopChan chan struct{}
	mu       sync.RWMutex
}

// Start creates a mock process that emits predefined events.
func (r *MockRunner) Start(ctx context.Context, name, workDir string, events chan<- Event) (RunningProcess, error) {
	proc := &mockProcess{
		name:     name,
		state:    StateRunning,
		events:   events,
		stopChan: make(chan struct{}),
	}

	// Emit state change
	events <- Event{
		AgentName: name,
		Type:      EventStateChange,
		Data: StateChangeData{
			OldState: StateIdle,
			NewState: StateRunning,
		},
	}

	// Start goroutine to emit events
	go func() {
		reader := strings.NewReader(strings.Join(r.Events, "\n"))
		scanner := bufio.NewScanner(reader)

		for scanner.Scan() {
			select {
			case <-proc.stopChan:
				return
			default:
			}

			// Check context if provided
			if ctx != nil {
				select {
				case <-ctx.Done():
					return
				default:
				}
			}

			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			parsedEvents, err := ParseEvent(line, name)
			if err != nil {
				events <- Event{
					AgentName: name,
					Type:      EventError,
					Data: ErrorData{
						Message: "failed to parse event",
						Err:     err,
					},
				}
				continue
			}

			for _, evt := range parsedEvents {
				events <- evt
			}

			if r.Delay > 0 {
				time.Sleep(r.Delay)
			}
		}

		// Emit error if configured
		if r.ShouldError {
			events <- Event{
				AgentName: name,
				Type:      EventError,
				Data: ErrorData{
					Message: r.ErrorMsg,
				},
			}
			proc.mu.Lock()
			proc.state = StateError
			proc.mu.Unlock()
		} else {
			proc.mu.Lock()
			proc.state = StateStopped
			proc.mu.Unlock()
		}

		// Emit final state change
		proc.mu.RLock()
		finalState := proc.state
		proc.mu.RUnlock()

		events <- Event{
			AgentName: name,
			Type:      EventStateChange,
			Data: StateChangeData{
				OldState: StateRunning,
				NewState: finalState,
			},
		}
	}()

	return proc, nil
}

func (p *mockProcess) Stop(timeout time.Duration) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.state != StateRunning {
		return nil
	}

	close(p.stopChan)
	p.state = StateStopped

	p.events <- Event{
		AgentName: p.name,
		Type:      EventStateChange,
		Data: StateChangeData{
			OldState: StateRunning,
			NewState: StateStopped,
		},
	}

	return nil
}

func (p *mockProcess) SendInput(input string) error {
	// Mock implementation - do nothing
	return nil
}

func (p *mockProcess) GetPID() int {
	return 12345 // Fake PID for testing
}

func (p *mockProcess) GetState() State {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state
}

// StreamReader wraps an io.Reader to simulate streaming output.
type StreamReader struct {
	reader  io.Reader
	scanner *bufio.Scanner
}

// NewStreamReader creates a new StreamReader from an io.Reader.
func NewStreamReader(r io.Reader) *StreamReader {
	return &StreamReader{
		reader:  r,
		scanner: bufio.NewScanner(r),
	}
}

// Next returns the next JSON line from the stream.
func (s *StreamReader) Next() ([]byte, error) {
	if s.scanner.Scan() {
		return s.scanner.Bytes(), nil
	}
	if err := s.scanner.Err(); err != nil {
		return nil, err
	}
	return nil, io.EOF
}
