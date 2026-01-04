package jj

import (
	"context"
)

// DiffOptions configures the diff command.
type DiffOptions struct {
	Revision string // Revision to show diff for (defaults to working copy)
}

// Diff returns the raw diff output with color codes.
func (c *Client) Diff(ctx context.Context, opts *DiffOptions) (string, error) {
	args := []string{"diff", "--color=always"}

	if opts != nil && opts.Revision != "" {
		args = append(args, "-r", opts.Revision)
	}

	return c.run(ctx, args...)
}
