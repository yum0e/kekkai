package jj

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrNotJJRepo         = errors.New("not a jj repository")
	ErrWorkspaceExists   = errors.New("workspace already exists")
	ErrWorkspaceNotFound = errors.New("workspace not found")
)

// CommandError represents an error from executing a jj command.
type CommandError struct {
	Cmd    string
	Stderr string
	Err    error
}

func (e *CommandError) Error() string {
	return fmt.Sprintf("jj %s: %s", e.Cmd, e.Stderr)
}

func (e *CommandError) Unwrap() error {
	return e.Err
}

// parseError converts a command execution error into a typed error if possible.
func parseError(subcmd, stderr string, err error) error {
	if err == nil {
		return nil
	}

	cmdErr := &CommandError{
		Cmd:    subcmd,
		Stderr: stderr,
		Err:    err,
	}

	switch {
	case strings.Contains(stderr, "There is no jj repo in"):
		cmdErr.Err = ErrNotJJRepo
	case strings.Contains(stderr, "already exists"):
		cmdErr.Err = ErrWorkspaceExists
	case strings.Contains(stderr, "No such workspace"):
		cmdErr.Err = ErrWorkspaceNotFound
	}

	return cmdErr
}
