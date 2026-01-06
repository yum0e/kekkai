"""Main CLI entry point for kekkai."""

import argparse
import difflib
import json
import os
import shutil
import subprocess
import sys
from dataclasses import asdict, dataclass
from datetime import datetime, timezone
from pathlib import Path

from rich.console import Console

from . import __version__
from .errors import NotJJRepoError, NotRootWorkspaceError, WorkspaceExistsError
from .jj import JJClient

SHIM_DIR = ".jj/.kekkai-bin"
AGENT_MARKER_FILE = ".jj/kekkai-agent"

SHIM_CONTENT = """\
#!/bin/sh
echo "git disabled for agents; use jj" >&2
exit 1
"""


@dataclass(frozen=True)
class Agent:
    """Agent configuration."""

    name: str
    executable: str


AGENTS: dict[str, Agent] = {
    "codex": Agent("codex", "codex"),
    "claude": Agent("claude", "claude"),
}
DEFAULT_AGENT = "codex"


@dataclass
class AgentMarker:
    """Metadata stored in the agent marker file."""

    root_workspace: str
    name: str
    created_at: str
    agent: str = "claude"  # default for backward compatibility


def find_root_workspace(client: JJClient) -> str:
    """Find the original root workspace.

    If we're in an agent workspace, follows the marker to find the root.
    Otherwise, returns the current jj workspace root.
    """
    current_root = client.workspace_root()
    marker_path = Path(current_root) / AGENT_MARKER_FILE

    if marker_path.exists():
        try:
            data = json.loads(marker_path.read_text())
            if root := data.get("root_workspace"):
                return root
        except (json.JSONDecodeError, OSError):
            pass

    return current_root


def ensure_root_workspace(root: str) -> None:
    """Ensure the command is run from the root workspace."""
    marker_path = Path(root) / AGENT_MARKER_FILE
    if marker_path.exists():
        raise NotRootWorkspaceError("look must be run from the root workspace")


def suggest_agent_names(query: str, candidates: list[str]) -> list[str]:
    """Return a short list of suggested agent names."""
    if not candidates:
        return []
    lookup = {name.lower(): name for name in candidates}
    matches = difflib.get_close_matches(
        query.lower(), lookup.keys(), n=3, cutoff=0.5
    )
    suggestions = [lookup[name] for name in matches]
    if suggestions:
        return suggestions
    return [name for name in candidates if query.lower() in name.lower()]


def compute_agent_path(root_path: str, agent_name: str) -> str:
    """Compute the sibling workspace path."""
    root = Path(root_path)
    return str(root.parent / f"{root.name}-{agent_name}")


def compute_jj_workspace_name(root_path: str, agent_name: str) -> str:
    """Return the jj workspace name for an agent."""
    return f"{Path(root_path).name}-{agent_name}"


def create_agent_marker(
    workspace_path: str, root_path: str, name: str, agent: str
) -> None:
    """Write the agent marker file."""
    marker = AgentMarker(
        root_workspace=root_path,
        name=name,
        created_at=datetime.now(timezone.utc).isoformat(),
        agent=agent,
    )
    marker_path = Path(workspace_path) / AGENT_MARKER_FILE
    marker_path.write_text(json.dumps(asdict(marker), indent=2))


def check_parent_writable(root_path: str) -> None:
    """Verify we can write to the parent directory."""
    parent = Path(root_path).parent
    test_file = parent / ".kekkai-write-test"

    try:
        test_file.write_text("test")
        test_file.unlink()
    except OSError as e:
        raise PermissionError(f"parent directory {parent} is not writable: {e}")


def has_uncommitted_changes(client: JJClient, workspace_path: str) -> bool:
    """Check if workspace has uncommitted changes."""
    try:
        output = client.status(cwd=workspace_path)
        return "Working copy changes:" in output
    except Exception:
        return False


def cleanup(
    client: JJClient, jj_workspace_name: str, workspace_path: str, root_path: str
) -> None:
    """Clean up workspace resources."""
    ws_path = Path(workspace_path)

    # Remove .git directory first so jj can work properly
    git_dir = ws_path / ".git"
    if git_dir.exists():
        shutil.rmtree(git_dir)

    # Remove marker file
    marker = ws_path / AGENT_MARKER_FILE
    if marker.exists():
        marker.unlink()

    # Forget workspace in jj
    try:
        client.workspace_forget(jj_workspace_name, cwd=root_path)
    except Exception as e:
        print(f"Warning: failed to forget workspace: {e}", file=sys.stderr)

    # Remove directory
    try:
        shutil.rmtree(workspace_path)
    except Exception as e:
        print(f"Warning: failed to remove workspace directory: {e}", file=sys.stderr)


def run_agent(name: str, agent: Agent) -> None:
    """Create workspace and run agent."""
    client = JJClient()
    console = Console()

    with console.status("Summoning...", spinner="dots"):
        # 1. Find root workspace
        try:
            root = find_root_workspace(client)
        except NotJJRepoError:
            console.print("Error: not in a jj repository", style="red")
            sys.exit(1)

        # 2. Check parent directory is writable
        try:
            check_parent_writable(root)
        except PermissionError as e:
            console.print(f"Error: {e}", style="red")
            sys.exit(1)

        # 3. Compute sibling workspace path
        workspace_path = compute_agent_path(root, name)
        shim_path = Path(workspace_path) / SHIM_DIR
        jj_workspace_name = compute_jj_workspace_name(root, name)

        # 4. Create workspace via jj workspace add
        try:
            client.workspace_add(workspace_path, cwd=root)
        except WorkspaceExistsError:
            console.print(f"Error: workspace '{name}' already exists", style="red")
            console.print("Use 'kekkai list' to see existing workspaces")
            sys.exit(1)
        except Exception as e:
            console.print(f"Error creating workspace: {e}", style="red")
            sys.exit(1)

        # 5. Configure jj to auto-update stale working copies
        try:
            subprocess.run(
                ["jj", "config", "set", "--repo", "snapshot.auto-update-stale", "true"],
                cwd=workspace_path,
                capture_output=True,
                check=True,
            )
        except Exception:
            pass  # Non-fatal if this fails

        # 6. Register watchman trigger by running jj in the new workspace
        try:
            client.status(cwd=workspace_path)
        except Exception:
            pass  # Non-fatal if this fails

        # 7. Create .git directory (scopes Claude to workspace)
        git_dir = Path(workspace_path) / ".git"
        try:
            git_dir.mkdir(parents=True, exist_ok=True)
        except OSError as e:
            console.print(f"Error creating .git directory: {e}", style="red")
            cleanup(client, jj_workspace_name, workspace_path, root)
            sys.exit(1)

        # 8. Create agent marker file
        try:
            create_agent_marker(workspace_path, root, name, agent.name)
        except OSError as e:
            console.print(f"Error creating agent marker: {e}", style="red")
            cleanup(client, jj_workspace_name, workspace_path, root)
            sys.exit(1)

        # 9. Create git shim
        try:
            shim_path.mkdir(parents=True, exist_ok=True)
            shim_script = shim_path / "git"
            shim_script.write_text(SHIM_CONTENT)
            shim_script.chmod(0o755)
        except OSError as e:
            console.print(f"Error creating git shim: {e}", style="red")
            cleanup(client, jj_workspace_name, workspace_path, root)
            sys.exit(1)

        # 10. Build env with shim in PATH
        env = os.environ.copy()
        env["PATH"] = f"{shim_path}:{env.get('PATH', '')}"

    # 11. Run agent with terminal passthrough (outside spinner)
    result = subprocess.run([agent.executable], cwd=workspace_path, env=env)

    if result.returncode != 0:
        print(f"\n{agent.name.capitalize()} exited with code {result.returncode}", file=sys.stderr)

    # 11. Check for uncommitted changes
    if has_uncommitted_changes(client, workspace_path):
        print("\nWarning: This workspace has uncommitted changes!")

    # 12. Prompt for cleanup
    try:
        answer = input("\nKeep workspace for inspection? [y/N] ").strip().lower()
    except (EOFError, KeyboardInterrupt):
        answer = ""

    # 13. Cleanup or keep
    if answer not in ("y", "yes"):
        cleanup(client, jj_workspace_name, workspace_path, root)
        print(f"Workspace '{name}' removed")
    else:
        print(f"Workspace kept at: {workspace_path}")


def list_workspaces() -> None:
    """List existing agent workspaces."""
    client = JJClient()

    try:
        root = find_root_workspace(client)
    except NotJJRepoError:
        print("Error: not in a jj repository", file=sys.stderr)
        sys.exit(1)

    try:
        workspaces = client.workspace_list(cwd=root)
    except Exception as e:
        print(f"Error listing workspaces: {e}", file=sys.stderr)
        sys.exit(1)

    repo_name = Path(root).name
    prefix = f"{repo_name}-"

    found = False
    for ws in workspaces:
        # Skip default workspace
        if ws.name == "default":
            continue

        # Check if this is a kekkai agent workspace
        if ws.name.startswith(prefix):
            agent_name = ws.name[len(prefix) :]

            # Verify it has the agent marker
            agent_path = Path(root).parent / ws.name
            marker_path = agent_path / AGENT_MARKER_FILE
            if marker_path.exists():
                try:
                    data = json.loads(marker_path.read_text())
                    agent_type = data.get("agent", "claude")
                except (json.JSONDecodeError, OSError):
                    agent_type = "unknown"
                print(f"{agent_name} [{agent_type}]: {ws.change_id} {ws.commit_id} {ws.summary}")
                found = True

    if not found:
        print("No workspaces")


def look_workspace(agent_name: str) -> None:
    """Create a new revision based on an agent workspace."""
    client = JJClient()

    try:
        root = client.workspace_root()
    except NotJJRepoError:
        print("Error: not in a jj repository", file=sys.stderr)
        sys.exit(1)

    try:
        ensure_root_workspace(root)
    except NotRootWorkspaceError as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)

    try:
        workspaces = client.workspace_list(cwd=root)
    except Exception as e:
        print(f"Error listing workspaces: {e}", file=sys.stderr)
        sys.exit(1)

    repo_name = Path(root).name
    prefix = f"{repo_name}-"
    agents: dict[str, str] = {}

    for ws in workspaces:
        if ws.name == "default":
            continue
        if not ws.name.startswith(prefix):
            continue
        agent = ws.name[len(prefix) :]
        agent_path = Path(root).parent / ws.name
        marker_path = agent_path / AGENT_MARKER_FILE
        if marker_path.exists():
            agents[agent] = ws.name

    if agent_name not in agents:
        print(f"Error: agent workspace '{agent_name}' not found", file=sys.stderr)
        suggestions = suggest_agent_names(agent_name, sorted(agents.keys()))
        if suggestions:
            print(f"Did you mean: {', '.join(suggestions)}", file=sys.stderr)
        sys.exit(1)

    try:
        client.new(revision=f"\"{agents[agent_name]}\"@", cwd=root)
    except Exception as e:
        print(f"Error creating new revision: {e}", file=sys.stderr)
        sys.exit(1)

    print(f"Created new revision from '{agent_name}'")


def main() -> None:
    """Main entry point."""
    parser = argparse.ArgumentParser(
        prog="kekkai",
        description=f"kekkai {__version__} - Launch AI agents in isolated jj workspaces",
    )
    parser.add_argument(
        "--version",
        action="version",
        version=f"kekkai {__version__}",
        help="Show version and exit",
    )
    parser.add_argument(
        "name",
        nargs="?",
        help="Workspace name (or 'list'/'look' commands)",
    )
    parser.add_argument(
        "agent_name",
        nargs="?",
        help="Agent workspace name for 'look'",
    )
    parser.add_argument(
        "--agent",
        "-a",
        choices=list(AGENTS.keys()),
        default=DEFAULT_AGENT,
        help=f"Agent to use (default: {DEFAULT_AGENT})",
    )
    args = parser.parse_args()

    if args.name is None:
        parser.print_help()
        sys.exit(1)
    elif args.name == "list":
        list_workspaces()
    elif args.name == "look":
        if not args.agent_name:
            print("Error: look requires an agent name", file=sys.stderr)
            sys.exit(1)
        look_workspace(args.agent_name)
    else:
        run_agent(args.name, AGENTS[args.agent])


if __name__ == "__main__":
    main()
