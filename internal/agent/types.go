package agent

// State represents the current state of an agent process.
type State int

const (
	StateIdle State = iota
	StateRunning
	StateStopped
	StateError
)

func (s State) String() string {
	switch s {
	case StateIdle:
		return "idle"
	case StateRunning:
		return "running"
	case StateStopped:
		return "stopped"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

// EventType identifies the kind of event emitted by an agent.
type EventType int

const (
	EventOutput      EventType = iota // Text output from agent
	EventToolUse                      // Agent started using a tool
	EventToolResult                   // Tool execution completed
	EventError                        // Error occurred
	EventStateChange                  // Agent state changed
)

func (e EventType) String() string {
	switch e {
	case EventOutput:
		return "output"
	case EventToolUse:
		return "tool_use"
	case EventToolResult:
		return "tool_result"
	case EventError:
		return "error"
	case EventStateChange:
		return "state_change"
	default:
		return "unknown"
	}
}

// Event represents a notification from an agent.
type Event struct {
	AgentName string
	Type      EventType
	Data      any // Type-specific payload
}

// OutputData contains text output from the agent.
type OutputData struct {
	Text string
}

// ToolUseData contains information about a tool being invoked.
type ToolUseData struct {
	ToolID   string
	ToolName string
	Input    string
}

// ToolResultData contains the result of a tool execution.
type ToolResultData struct {
	ToolID   string
	ToolName string
	Output   string
	Success  bool
}

// ErrorData contains error information.
type ErrorData struct {
	Message string
	Err     error
}

// StateChangeData contains state transition information.
type StateChangeData struct {
	OldState State
	NewState State
}

// OrphanInfo describes an orphaned agent process.
type OrphanInfo struct {
	Name string
	PID  int
}
