# Kekkai

A minimal CLI that launches Claude Code in isolated jj workspaces.

## Installation

```bash
# Install globally with uv (recommended)
uv tool install kekkai

# Or use uvx for one-off usage
uvx kekkai
```

## Requirements

- [Claude Code](https://claude.ai/code) installed and in PATH
- [Jujutsu (jj)](https://github.com/martinvonz/jj) installed and in PATH
- Must be run from inside a jj repository
- [Watchman](https://facebook.github.io/watchman/) (**highly recommended** for real-time snapshotting and a full experience, see [Watchman Setup](#watchman-setup))

## Quick Start

```bash
# Launch Claude in a new isolated workspace
kekkai feature-auth

# List existing agent workspaces
kekkai list
```

When you run `kekkai <name>`:

1. Creates an isolated jj workspace as a sibling directory (`<repo>-<name>/`)
2. Launches Claude Code with full terminal experience
3. On exit, prompts whether to keep or delete the workspace

## Multi-Agent Workflow

Run multiple agents in parallel by opening multiple terminals:

```bash
# Terminal 1
kekkai feature-auth

# Terminal 2
kekkai bugfix-login

# Terminal 3
kekkai refactor-api
```

Monitor all agents from your main workspace:

```bash
jj log
```

## Managing Agent Changes

From your main workspace, use jj to integrate agent work:

```bash
# See all changes across workspaces
jj log

# Squash an agent's changes into current revision
jj squash --from <agent-revision>

# Rebase agent work onto latest
jj rebase -s <agent-revision> -d @

# Cherry-pick specific changes
jj new <agent-revision>
```

> see [jjui](https://github.com/idursun/jjui) for a more user-friendly interface.

## Watchman Setup

Without Watchman, jj only captures changes when you run a `jj` command. With Watchman, snapshotting is automatic and you can monitor agent progress in real-time.

1. [Install Watchman](https://facebook.github.io/watchman/docs/install)
2. Enable in `~/.config/jj/config.toml`:

```toml
# ~/.config/jj/config.toml
[fsmonitor]
backend = "watchman"
watchman.register-snapshot-trigger = true

# highly recommend auto updating stale snapshots as well
[snapshot]
auto-update-stale = true
```

3. Verify: `jj debug watchman status`

## Development

```bash
git clone https://github.com/yum0e/kekkai
cd kekkai
uv run kekkai --help
uv run --with pytest pytest tests/ -v
```
