# internal/jj

Go wrapper for the Jujutsu (jj) CLI.

## Purpose

This package provides a typed Go interface to jj commands. It executes jj as a subprocess and parses output into Go structs.

## Key Files

| File           | Purpose                                                  |
| -------------- | -------------------------------------------------------- |
| `client.go`    | Client struct, `run()` and `runInDir()` command execution |
| `workspace.go` | Workspace operations: list, add, forget, root            |
| `diff.go`      | Diff command with color output                           |
| `status.go`    | Repository status                                        |
| `log.go`       | Commit log queries                                       |
| `ops.go`       | VCS operations: new, commit, squash, rebase, describe    |
| `errors.go`    | Error types: `ErrNotJJRepo`, `ErrWorkspaceExists`, etc.  |

## Usage Pattern

```go
client := jj.NewClient()
workspaces, err := client.WorkspaceList(ctx)
diff, err := client.Diff(ctx, &jj.DiffOptions{WorkDir: "/path"})
```

## Important Notes

- `WorkspaceForget()` only unregisters from jj - caller must delete the directory
- `New()` creates an empty revision on top of the current working copy

## When to Look Here

- Adding new jj command wrappers
- Fixing jj output parsing
- Error handling for jj failures
- Supporting new jj features
