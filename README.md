# Kekkai

A minimal CLI that launches Claude Code in isolated jj workspaces.

## Usage

```bash
# Launch Claude in a new workspace
kekkai feature-auth

# List existing workspaces
kekkai list
```

When you run `kekkai <name>`:

1. Creates an isolated jj workspace as a sibling directory (`<repo>-<name>/`)
2. Launches Claude Code with full terminal experience
3. On exit, warns about uncommitted changes if any
4. Prompts whether to keep or delete the workspace

## Requirements

- Python 3.10+
- [Claude Code](https://claude.ai/code) installed and in PATH
- [Jujutsu (jj)](https://github.com/martinvonz/jj) installed and in PATH
- Must be run from inside a jj repository
- [Watchman](https://facebook.github.io/watchman/) (**highly recommended**) - enables automatic snapshotting across workspaces

### Watchman Setup

**Without Watchman**: Snapshotting becomes a **manual step**. jj only captures file changes when you explicitly run a `jj` command (like `jj status`). Changes made by Claude in agent workspaces remain invisible until you manually trigger a snapshot. With multiple agents working in parallel, this makes it impossible to monitor their progress in real-time.

**With Watchman**: Snapshotting is **automatic**. jj captures every file change instantly. You can monitor all agent workspaces from your main workspace via `jj log` and see their work as it happens.

1. [Install Watchman](https://facebook.github.io/watchman/docs/install)
2. Enable in jj config (`~/.config/jj/config.toml`):
   ```toml
   [fsmonitor]
   backend = "watchman"
   watchman.register-snapshot-trigger = true
   ```
3. Verify: `jj debug watchman status`

## Installation

```bash
# Using uvx (recommended)
uvx kekkai

# Or install with pip
pip install kekkai
```

For development:

```bash
git clone https://github.com/yum0e/kekkai
cd kekkai
uv run kekkai --help
```

## How It Works

### Workspace Location

Agent workspaces are created as siblings to your repository:

```
/Users/dev/
├── myproject/               <- Your repository
└── myproject-feature-auth/  <- Agent workspace
```

This structure enables:

- **Full copy**: Agent workspaces get a complete copy of the repository including `.claude/`
- **Better visibility**: Workspaces are easily accessible, not hidden in `.jj/`
- **Clean jj status**: Kekkai markers are auto-ignored by jj

### Workspace Isolation

Each agent runs in its own jj workspace with:

- **Separate revision**: Changes don't affect your main workspace
- **Git shim**: Blocks `git` commands, forcing `jj` usage
- **Scoped root**: Claude sees only the workspace as project root
- **Marker file**: `.jj/kekkai-agent` identifies agent workspaces (auto-ignored)

### Multi-Agent Workflow

Run multiple agents by opening multiple terminals:

```bash
# Terminal 1
kekkai feature-auth

# Terminal 2
kekkai bugfix-login

# Terminal 3
kekkai refactor-api
```

You can even run `kekkai` from inside an agent workspace - it will create a new sibling to the original root.

### Version Control

Use jj directly in your default workspace to manage agent changes:

```bash
# See what agents have changed
jj log

# Squash an agent's changes
jj squash --from <agent-revision>

# Rebase agents on latest
jj rebase -s <agent-revision> -d @
```

## Development

```bash
# Run tests
uv run --with pytest pytest tests/ -v

# Run the CLI
uv run kekkai --help
```
