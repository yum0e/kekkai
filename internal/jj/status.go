package jj

import (
	"context"
	"regexp"
	"strings"
)

// FileStatus represents the status of a single file.
type FileStatus struct {
	Status string // M (modified), A (added), D (deleted), C (conflict)
	Path   string
}

// Status represents the working copy status.
type Status struct {
	WorkingCopy  string       // Current change ID
	ParentCommit string       // Parent commit ID
	Changes      []FileStatus // File changes
	HasConflicts bool         // Whether there are conflicts
}

// statusFileRegex parses lines like "M path/to/file" or "A new_file.go"
var statusFileRegex = regexp.MustCompile(`^([MADC])\s+(.+)$`)

// workingCopyRegex parses "Working copy : xxxxxxxx xxxxxxxx"
var workingCopyRegex = regexp.MustCompile(`Working copy\s*:\s*(\S+)\s+(\S+)`)

// parentRegex parses "Parent commit: xxxxxxxx xxxxxxxx"
var parentRegex = regexp.MustCompile(`Parent commit:\s*(\S+)\s+(\S+)`)

// Status returns the current working copy status.
func (c *Client) Status(ctx context.Context) (*Status, error) {
	output, err := c.run(ctx, "status")
	if err != nil {
		return nil, err
	}

	status := &Status{}

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check for working copy line
		if matches := workingCopyRegex.FindStringSubmatch(line); matches != nil {
			status.WorkingCopy = matches[1]
			continue
		}

		// Check for parent commit line
		if matches := parentRegex.FindStringSubmatch(line); matches != nil {
			status.ParentCommit = matches[1]
			continue
		}

		// Check for file status
		if matches := statusFileRegex.FindStringSubmatch(line); matches != nil {
			fs := FileStatus{
				Status: matches[1],
				Path:   matches[2],
			}
			status.Changes = append(status.Changes, fs)

			if matches[1] == "C" {
				status.HasConflicts = true
			}
		}
	}

	return status, nil
}
