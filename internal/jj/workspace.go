package jj

import (
	"context"
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

// WorkspaceForget removes a workspace from jj tracking.
// It does NOT delete the workspace directory - caller must handle that.
func (c *Client) WorkspaceForget(ctx context.Context, name string) error {
	_, err := c.run(ctx, "workspace", "forget", name)
	return err
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

// WorkspaceUpdateStale updates a stale working copy in a specific directory.
// This is needed when the working copy has diverged from the operation log.
func (c *Client) WorkspaceUpdateStale(ctx context.Context, dir string) error {
	_, err := c.runInDir(ctx, dir, "workspace", "update-stale")
	return err
}

// WorkspaceAddFromDir creates a new workspace at the given path, running from a specific directory.
// If revision is non-empty, the workspace starts at that revision.
func (c *Client) WorkspaceAddFromDir(ctx context.Context, dir, path, revision string) error {
	args := []string{"workspace", "add", path}
	if revision != "" {
		args = append(args, "-r", revision)
	}
	_, err := c.runInDir(ctx, dir, args...)
	return err
}

// WorkspaceForgetFromDir removes a workspace from jj tracking, running from a specific directory.
// It does NOT delete the workspace directory - caller must handle that.
func (c *Client) WorkspaceForgetFromDir(ctx context.Context, dir, name string) error {
	_, err := c.runInDir(ctx, dir, "workspace", "forget", name)
	return err
}

// WorkspaceListFromDir returns all workspaces in the repository, running from a specific directory.
func (c *Client) WorkspaceListFromDir(ctx context.Context, dir string) ([]Workspace, error) {
	output, err := c.runInDir(ctx, dir, "workspace", "list")
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

// WorkspaceRootFromDir returns the root directory of the workspace, running from a specific directory.
func (c *Client) WorkspaceRootFromDir(ctx context.Context, dir string) (string, error) {
	output, err := c.runInDir(ctx, dir, "workspace", "root")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
}

// StatusFromDir returns the jj status output for a workspace.
func (c *Client) StatusFromDir(ctx context.Context, dir string) (string, error) {
	return c.runInDir(ctx, dir, "status")
}
