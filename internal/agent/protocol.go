package agent

import (
	"encoding/json"
	"fmt"
)

// StreamEvent is the base type for all Claude Code stream-json events.
type StreamEvent struct {
	Type string `json:"type"`
}

// AssistantMessage represents the message field in assistant events.
type AssistantMessage struct {
	Content []ContentBlock `json:"content"`
}

// AssistantEvent is emitted when the assistant produces output.
type AssistantEvent struct {
	Type    string           `json:"type"`
	Message AssistantMessage `json:"message"`
}

// ContentBlock represents a single content item in an assistant message.
type ContentBlock struct {
	Type  string `json:"type"`            // "text" or "tool_use"
	Text  string `json:"text,omitempty"`  // for type="text"
	ID    string `json:"id,omitempty"`    // for type="tool_use"
	Name  string `json:"name,omitempty"`  // tool name for type="tool_use"
	Input any    `json:"input,omitempty"` // tool input for type="tool_use"
}

// ResultEvent is emitted when a tool execution completes or the agent finishes.
type ResultEvent struct {
	Type      string `json:"type"` // "result"
	Subtype   string `json:"subtype,omitempty"`
	Result    string `json:"result,omitempty"`
	Duration  int    `json:"duration_ms,omitempty"`
	CostUSD   float64 `json:"cost_usd,omitempty"`
}

// ErrorEvent is emitted when an error occurs.
type ErrorEvent struct {
	Type  string `json:"type"` // "error"
	Error string `json:"error"`
}

// SystemEvent is emitted for system messages.
type SystemEvent struct {
	Type    string `json:"type"` // "system"
	Message string `json:"message"`
}

// ParseEvent parses a single JSON line from the stream and converts it to an Event.
// The agentName is used to populate the Event.AgentName field.
func ParseEvent(line []byte, agentName string) ([]Event, error) {
	if len(line) == 0 {
		return nil, nil
	}

	// First, determine the event type
	var base StreamEvent
	if err := json.Unmarshal(line, &base); err != nil {
		return nil, fmt.Errorf("failed to parse event type: %w", err)
	}

	var events []Event

	switch base.Type {
	case "assistant":
		var evt AssistantEvent
		if err := json.Unmarshal(line, &evt); err != nil {
			return nil, fmt.Errorf("failed to parse assistant event: %w", err)
		}

		for _, block := range evt.Message.Content {
			switch block.Type {
			case "text":
				if block.Text != "" {
					events = append(events, Event{
						AgentName: agentName,
						Type:      EventOutput,
						Data: OutputData{
							Text: block.Text,
						},
					})
				}
			case "tool_use":
				inputStr := ""
				if block.Input != nil {
					if b, err := json.Marshal(block.Input); err == nil {
						inputStr = string(b)
					}
				}
				events = append(events, Event{
					AgentName: agentName,
					Type:      EventToolUse,
					Data: ToolUseData{
						ToolID:   block.ID,
						ToolName: block.Name,
						Input:    inputStr,
					},
				})
			}
		}

	case "result":
		var evt ResultEvent
		if err := json.Unmarshal(line, &evt); err != nil {
			return nil, fmt.Errorf("failed to parse result event: %w", err)
		}
		// Result events indicate completion, but we don't emit them as tool results
		// since tool results come from separate events in the stream

	case "error":
		var evt ErrorEvent
		if err := json.Unmarshal(line, &evt); err != nil {
			return nil, fmt.Errorf("failed to parse error event: %w", err)
		}
		events = append(events, Event{
			AgentName: agentName,
			Type:      EventError,
			Data: ErrorData{
				Message: evt.Error,
			},
		})

	case "system":
		// System events are informational, we can ignore or log them
		// They contain setup info like "Initializing..."

	default:
		// Unknown event types are silently ignored for forward compatibility
	}

	return events, nil
}
