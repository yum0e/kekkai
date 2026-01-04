package tui

import (
	"github.com/bigq/dojo/internal/agent"
	"github.com/bigq/dojo/internal/jj"
)

// WorkspacesLoadedMsg is sent when the workspace list has been fetched.
type WorkspacesLoadedMsg struct {
	Workspaces []jj.Workspace
	Err        error
}

// DiffLoadedMsg is sent when diff content has been fetched.
type DiffLoadedMsg struct {
	Workspace string
	Content   string
	Err       error
}

// WorkspaceSelectedMsg is sent when a workspace is selected.
type WorkspaceSelectedMsg struct {
	Workspace jj.Workspace
}

// WorkspaceAddedMsg is sent when a new workspace has been created.
type WorkspaceAddedMsg struct {
	Name string
	Err  error
}

// WorkspaceDeletedMsg is sent when a workspace has been deleted.
type WorkspaceDeletedMsg struct {
	Name string
	Err  error
}

// ConfirmDeleteMsg triggers the delete confirmation dialog.
type ConfirmDeleteMsg struct {
	WorkspaceName string
}

// ConfirmResultMsg carries the result of a confirmation dialog.
type ConfirmResultMsg struct {
	Confirmed bool
	Action    string
	Data      interface{}
}

// RefreshDiffMsg triggers a diff refresh.
type RefreshDiffMsg struct{}

// AgentEventMsg wraps an agent.Event for the TUI.
type AgentEventMsg struct {
	Event agent.Event
}

// AgentSpawnedMsg is sent when an agent has been spawned.
type AgentSpawnedMsg struct {
	Name string
	Err  error
}

// AgentStoppedMsg is sent when an agent has been stopped.
type AgentStoppedMsg struct {
	Name string
	Err  error
}
