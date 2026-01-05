# Kekkai

Minimal CLI wrapper that launches Claude Code in isolated jj workspaces.

## Architecture

```
kekkai <name>
  → jj workspace add ../<repo>-<name>/   (sibling directory)
  → create .git directory (scope isolation, auto-ignored by jj)
  → create .jj/kekkai-agent marker (auto-ignored by jj)
  → create git shim (blocks git)
  → exec claude (full terminal passthrough)
  → prompt cleanup on exit
```

## Key Files

| File                    | Purpose                                           |
| ----------------------- | ------------------------------------------------- |
| `src/kekkai/cli.py`     | CLI entry point, workspace setup, claude launcher |
| `src/kekkai/jj.py`      | jj CLI wrapper + Workspace dataclass              |
| `src/kekkai/errors.py`  | Custom exception classes                          |

## Commands

- `kekkai <name>` - Create workspace and launch Claude interactively
- `kekkai list` - List existing agent workspaces

## Running

```bash
# Local development
uv run kekkai <name>
uv run kekkai list

# After publishing to PyPI
uvx kekkai <name>
uvx kekkai list
```

## Workspace Isolation Mechanisms

1. **jj workspace**: Each agent gets its own jj workspace/revision as sibling directory
2. **.git directory**: Empty directory at workspace root scopes Claude (auto-ignored by jj)
3. **.jj/kekkai-agent**: Marker file with metadata (auto-ignored, inside .jj/)
4. **git shim**: Script in PATH that blocks git commands, forces jj usage
5. **PWD**: Claude runs with workspace as working directory

## Code Patterns

### Launching Claude

```python
env = os.environ.copy()
env["PATH"] = f"{shim_path}:{env.get('PATH', '')}"

subprocess.run(["claude"], cwd=workspace_path, env=env)
```

### Cleanup

```python
shutil.rmtree(Path(workspace_path) / ".git")           # Remove .git directory
(Path(workspace_path) / AGENT_MARKER_FILE).unlink()    # Remove marker
client.workspace_forget(jj_workspace_name, cwd=root)   # Unregister from jj
shutil.rmtree(workspace_path)                          # Delete directory
```

## Testing

```bash
uv run --with pytest pytest tests/ -v
```

## When to Look Here

- Adding new CLI commands
- Modifying workspace isolation behavior
- Changing cleanup behavior
- jj integration issues
