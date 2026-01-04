package jj

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Commit represents a jj commit.
type Commit struct {
	ChangeID      string
	ChangeIDShort string
	Description   string
	Author        string
	Timestamp     time.Time
	IsWorkingCopy bool
}

// LogOptions configures the log command.
type LogOptions struct {
	Revisions string // Revision set expression (e.g., "::@" or "main..@")
	Limit     int    // Maximum number of commits to return
}

// logTemplate is the jj template for parsing commit info.
const logTemplate = `change_id.short() ++ "|" ++ description.first_line() ++ "|" ++ author.email() ++ "|" ++ author.timestamp() ++ "\n"`

// Log returns commits matching the given options.
func (c *Client) Log(ctx context.Context, opts *LogOptions) ([]Commit, error) {
	args := []string{"log", "--no-graph", "-T", logTemplate}

	if opts != nil {
		if opts.Revisions != "" {
			args = append(args, "-r", opts.Revisions)
		}
		if opts.Limit > 0 {
			args = append(args, "-n", fmt.Sprintf("%d", opts.Limit))
		}
	}

	output, err := c.run(ctx, args...)
	if err != nil {
		return nil, err
	}

	var commits []Commit
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}

		commit := Commit{
			ChangeIDShort: parts[0],
			ChangeID:      parts[0], // Short ID is what we get from template
			Description:   parts[1],
			Author:        parts[2],
		}

		// Parse timestamp (jj format: "2024-01-15 10:30:00.000 -08:00")
		if ts, err := parseTimestamp(parts[3]); err == nil {
			commit.Timestamp = ts
		}

		commits = append(commits, commit)
	}

	return commits, nil
}

// parseTimestamp parses jj's timestamp format.
func parseTimestamp(s string) (time.Time, error) {
	// jj format: "2024-01-15 10:30:00.000 -08:00"
	s = strings.TrimSpace(s)

	// Try common formats
	formats := []string{
		"2006-01-02 15:04:05.000 -07:00",
		"2006-01-02 15:04:05 -07:00",
		"2006-01-02 15:04:05.000 MST",
		"2006-01-02 15:04:05 MST",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse timestamp: %s", s)
}
