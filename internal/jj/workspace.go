package jj

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Workspace represents a jj workspace.
type Workspace struct {
	Name     string // "default", "agent-1"
	ChangeID string // Short change ID
	CommitID string // Short commit ID
	Summary  string // Description
}

// workspaceLineRegex parses lines like:
// default: wpxqlmox f3c3a79d (no description set)
var workspaceLineRegex = regexp.MustCompile(`^(\S+): (\S+) (\S+) (.*)$`)

// WorkspaceAdd creates a new workspace at the given path.
// If revision is non-empty, the workspace starts at that revision.
func (c *Client) WorkspaceAdd(ctx context.Context, path, revision string) error {
	args := []string{"workspace", "add", path}
	if revision != "" {
		args = append(args, "-r", revision)
	}
	_, err := c.run(ctx, args...)
	return err
}

// WorkspaceForget removes a workspace and deletes its directory.
// The workspace directory is assumed to be at <repo_root>/<name>.
func (c *Client) WorkspaceForget(ctx context.Context, name string) error {
	// Get repo root before forgetting (need it to compute workspace path)
	root, err := c.WorkspaceRoot(ctx)
	if err != nil {
		return err
	}

	_, err = c.run(ctx, "workspace", "forget", name)
	if err != nil {
		return err
	}

	// Remove workspace directory
	path := filepath.Join(root, name)
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("failed to remove workspace directory: %w", err)
	}

	return nil
}

// WorkspaceList returns all workspaces in the repository.
func (c *Client) WorkspaceList(ctx context.Context) ([]Workspace, error) {
	output, err := c.run(ctx, "workspace", "list")
	if err != nil {
		return nil, err
	}

	var workspaces []Workspace
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}

		matches := workspaceLineRegex.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		workspaces = append(workspaces, Workspace{
			Name:     matches[1],
			ChangeID: matches[2],
			CommitID: matches[3],
			Summary:  matches[4],
		})
	}

	return workspaces, nil
}

// WorkspaceRoot returns the root directory of the current workspace.
func (c *Client) WorkspaceRoot(ctx context.Context) (string, error) {
	output, err := c.run(ctx, "workspace", "root")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}
