# Dojo

Minimal CLI wrapper that launches Claude Code in isolated jj workspaces.

## Architecture

```
dojo <name>
  → jj workspace add .jj/agents/<name>/
  → create .git marker (scope isolation)
  → create git shim (blocks git)
  → exec claude (full terminal passthrough)
  → prompt cleanup on exit
```

## Key Files

| File                       | Purpose                                           |
| -------------------------- | ------------------------------------------------- |
| `cmd/dojo/main.go`         | CLI entry point, workspace setup, claude launcher |
| `internal/jj/client.go`    | jj CLI wrapper                                    |
| `internal/jj/workspace.go` | Workspace operations (add, forget, list)          |
| `internal/jj/errors.go`    | Error types                                       |

## Commands

- `dojo <name>` - Create workspace and launch Claude interactively
- `dojo list` - List existing agent workspaces

## Workspace Isolation Mechanisms

1. **jj workspace**: Each agent gets its own jj workspace/revision
2. **.git marker**: Empty file at workspace root scopes Claude to that directory
3. **git shim**: Script in PATH that blocks git commands, forces jj usage
4. **PWD**: Claude runs with workspace as working directory

## Code Patterns

### Launching Claude

```go
cmd := exec.Command("claude")
cmd.Dir = workspacePath
cmd.Env = envWithShimInPath
cmd.Stdin = os.Stdin
cmd.Stdout = os.Stdout
cmd.Stderr = os.Stderr
cmd.Run()
```

### Cleanup

```go
os.Remove(filepath.Join(workspacePath, ".git"))  // Remove marker first
client.WorkspaceForget(ctx, name)                 // Unregister from jj
os.RemoveAll(workspacePath)                       // Delete directory
```

## When to Look Here

- Adding new CLI commands
- Modifying workspace isolation behavior
- Changing cleanup behavior
- jj integration issues
