# Plan: Rewrite Dojo CLI from Go to Python (renamed to "kekkai")

## Goal
Rewrite the ~557 lines of Go into ~400 lines of Python, packaged for `uvx` distribution.
Rename from "dojo" to "kekkai" (結界 - barrier, fitting for workspace isolation).

## Project Structure

```
dojo/                         # Keep existing directory name
  pyproject.toml              # hatchling backend, [project.scripts] kekkai = "kekkai.cli:main"
  specs/
    0-start.md                # Existing
    1-python.md               # This plan (to be saved)
  src/
    kekkai/
      __init__.py             # __version__ = "0.1.0"
      __main__.py             # from .cli import main; main()
      cli.py                  # Main logic (~200 lines)
      jj.py                   # JJClient + Workspace dataclass (~100 lines)
      errors.py               # Custom exceptions (~30 lines)
  tests/
    conftest.py               # pytest fixtures (temp_jj_repo)
    test_cli.py               # CLI integration tests
    test_jj.py                # JJClient tests
```

## Implementation Steps

### 1. Create package scaffolding
- `pyproject.toml` with hatchling, requires-python >= 3.10, no runtime deps
- `src/kekkai/__init__.py` with version
- `src/kekkai/__main__.py` entry point

### 2. Create `src/kekkai/errors.py`
```python
class KekkaiError(Exception): pass
class NotJJRepoError(KekkaiError): pass
class WorkspaceExistsError(KekkaiError): pass
class WorkspaceNotFoundError(KekkaiError): pass
class JJCommandError(KekkaiError):
    def __init__(self, cmd, stderr, returncode): ...
```

### 3. Create `src/kekkai/jj.py`
- `Workspace` dataclass (name, change_id, commit_id, summary)
- `JJClient` class with methods:
  - `_run(*args, cwd=None)` - subprocess wrapper
  - `workspace_root(cwd=None)`
  - `workspace_add(path, revision="", cwd=None)`
  - `workspace_forget(name, cwd=None)`
  - `workspace_list(cwd=None)` - regex parsing
  - `status(cwd=None)`

### 4. Create `src/kekkai/cli.py`
Port from `cmd/dojo/main.go`:
- Constants: `SHIM_DIR = ".kekkai-shim"`, `AGENT_MARKER_FILE = ".jj/kekkai-agent"`
- `AgentMarker` dataclass for JSON metadata
- `main()` - sys.argv dispatch (match statement)
- `run_agent(name)` - workspace setup, claude launch, cleanup prompt
- `list_workspaces()` - find and display agent workspaces
- `cleanup(workspace_path, name, jj)` - remove .git, marker, forget, rmdir
- Helper: `find_root_workspace()` - traverse up looking for marker

Key patterns:
- `subprocess.run(["claude"], cwd=workspace_path, env=modified_env)` for terminal passthrough
- `input()` for cleanup prompt
- `pathlib.Path` for all path operations

### 5. Create tests
- `tests/conftest.py` - `temp_jj_repo` fixture using `tmp_path`
- `tests/test_jj.py` - port from `internal/jj/jj_test.go`
- `tests/test_cli.py` - port from `cmd/dojo/main_test.go`

### 6. Delete Go files
- Remove `cmd/`, `internal/`, `go.mod`, `go.sum`
- Update `CLAUDE.md` for Python + kekkai naming
- Keep directory as `dojo/` (package name is `kekkai`)

## Key Files to Reference During Implementation

| Source (Go) | Target (Python) |
|-------------|-----------------|
| `cmd/dojo/main.go` | `src/kekkai/cli.py` |
| `internal/jj/client.go` | `src/kekkai/jj.py` |
| `internal/jj/workspace.go` | `src/kekkai/jj.py` |
| `internal/jj/errors.go` | `src/kekkai/errors.py` |
| `cmd/dojo/main_test.go` | `tests/test_cli.py` |
| `internal/jj/jj_test.go` | `tests/test_jj.py` |

## Design Decisions

- **CLI args**: Simple `sys.argv` + match statement (no argparse/click)
- **Dependencies**: Stdlib only (subprocess, pathlib, json, re, dataclasses)
- **Python version**: 3.10+ (for match statement)
- **Build backend**: hatchling (modern, minimal)
- **Testing**: pytest as dev dependency only

## Usage After Implementation

```bash
# Local development
uv run kekkai <name>
uv run kekkai list

# After publishing to PyPI
uvx kekkai <name>
uvx kekkai list
```
