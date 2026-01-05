# Dojo Spec: Minimal CLI Wrapper

## Overview

A minimal CLI that launches Claude Code directly in an isolated jj workspace. Users get the full Claude terminal experience without recreation.

## Motivation

- No need to recreate Claude's UI (syntax highlighting, markdown, tool visualization)
- Focus on core value: workspace isolation and version control orchestration
- Simple codebase (~250 LOC)

## Commands

### `dojo <name>`

Creates an isolated workspace and launches Claude interactively.

```
$ dojo feature-auth
[Creates workspace, launches Claude]
[User interacts with full Claude UI]
[On exit]
Warning: This workspace has uncommitted changes!
Keep workspace for inspection? [y/N] _
```

**Flow:**

1. Find root workspace (follows `.dojo-agent` marker if run from agent)
2. Check parent directory is writable
3. Create jj workspace as sibling: `../<repo>-<name>/`
4. Create `.dojo-agent` marker file (JSON with root path, agent name, timestamp)
5. Create `.claude` symlink pointing to root's `.claude/` (permission inheritance)
6. Create `.git` marker file (scopes Claude to workspace)
7. Set up git shim in PATH (blocks git, forces jj)
8. Fork Claude process with full terminal passthrough
9. Wait for Claude to exit
10. Warn if uncommitted changes detected
11. Prompt: "Keep workspace for inspection? [y/N]"
12. If no: cleanup (remove markers, forget workspace, delete directory)

### `dojo list`

Shows existing agent workspaces with jj revision info.

```
$ dojo list
feature-auth: wpxqlmox f3c3a79d Add OAuth2 support
bugfix-login: rstuvwxy a1b2c3d4 Fix login redirect
```

## Workspace Isolation

### Directory Structure

```
/Users/dev/
├── myproject/                    <- Root workspace
│   ├── .claude/
│   │   └── settings.local.json   <- Permissions (shared)
│   ├── .jj/
│   │   └── [jj metadata]
│   └── [project files]
└── myproject-feature-auth/       <- Agent workspace (sibling)
    ├── .dojo-agent               <- Marker file (JSON)
    ├── .claude -> ../myproject/.claude/  <- Symlink (permission inheritance)
    ├── .git                      <- Marker file (scope isolation)
    ├── .jj/
    │   └── .dojo-bin/
    │       └── git               <- Shim script
    └── [project files]
```

### .dojo-agent Marker

JSON file identifying agent workspaces:

```json
{
  "root_workspace": "/Users/dev/myproject",
  "name": "feature-auth",
  "created_at": "2025-01-05T10:30:00Z"
}
```

Used for:
- Discovering agent workspaces
- Enabling nested `dojo` calls (from agent, creates sibling to original root)
- Cleanup tracking

### .claude Symlink

- Symlink at `<workspace>/.claude` pointing to root's `.claude/`
- Enables permission inheritance (no re-approval needed)
- Uses relative path for portability

### .git Marker

- Empty file at `<workspace>/.git`
- Prevents Claude from detecting parent jj repo
- Makes Claude treat workspace as standalone project root

### Git Shim

- Script at `<workspace>/.jj/.dojo-bin/git`
- Returns exit 1 with message "git disabled for agents; use jj"
- PATH prepended so shim shadows real git

## Multi-Agent Model

- User opens multiple terminals for multiple agents
- Each `dojo <name>` is independent
- No centralized orchestration
- Version control via jj directly in default workspace
- Can run `dojo` from agent workspace - creates sibling to original root

## Design Decisions

| Question       | Decision                                                        |
| -------------- | --------------------------------------------------------------- |
| TTY approach   | Fork with terminal passthrough (not exec, not PTY multiplexing) |
| Workspace path | Sibling: `../<repo>-<name>/` for visibility + permission inheritance |
| Permissions    | Symlink `.claude/` to root for full inheritance                 |
| Discovery      | `.dojo-agent` marker file (JSON)                                |
| List output    | Name + jj change-id + commit-id + summary                       |
| Nesting        | Works - always creates siblings to original root                |
| Cleanup        | Prompt on exit + warn about uncommitted changes                 |
| Workspace UI   | None - CLI only                                                 |
| Diff view      | None - Claude can run jj commands                               |
| Multi-agent    | Separate terminals                                              |
| CLI commands   | `dojo <name>`, `dojo list`                                      |
| Git shim       | Keep (forces jj usage)                                          |
| Claude args    | None - user interacts directly                                  |

## Dependencies

- `os/exec` (stdlib)
- `encoding/json` (stdlib)
- `internal/jj` (workspace operations)
- No external libraries
