package jj

import (
	"bytes"
	"context"
	"os/exec"
)

// Client wraps jj CLI commands.
type Client struct {
	jjPath string
}

// Option configures a Client.
type Option func(*Client)

// NewClient creates a new jj client with the given options.
func NewClient(opts ...Option) *Client {
	c := &Client{
		jjPath: "jj",
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// WithJJPath sets a custom path to the jj binary.
func WithJJPath(path string) Option {
	return func(c *Client) {
		c.jjPath = path
	}
}

// run executes a jj command and returns the stdout output.
// The caller is responsible for setting the working directory.
func (c *Client) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, c.jjPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		subcmd := ""
		if len(args) > 0 {
			subcmd = args[0]
		}
		return "", parseError(subcmd, stderr.String(), err)
	}

	return stdout.String(), nil
}
