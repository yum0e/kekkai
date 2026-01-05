"""jj CLI wrapper."""

import re
import subprocess
from dataclasses import dataclass

from .errors import (
    JJCommandError,
    KekkaiError,
    NotJJRepoError,
    WorkspaceExistsError,
    WorkspaceNotFoundError,
)


@dataclass
class Workspace:
    """Represents a jj workspace."""

    name: str
    change_id: str
    commit_id: str
    summary: str


# Parses lines like: default: wpxqlmox f3c3a79d (no description set)
WORKSPACE_LINE_RE = re.compile(r"^(\S+): (\S+) (\S+) (.*)$")


def _parse_error(cmd: str, stderr: str, returncode: int) -> KekkaiError:
    """Convert subprocess error to typed exception."""
    if "There is no jj repo in" in stderr:
        return NotJJRepoError(stderr)
    if "already exists" in stderr:
        return WorkspaceExistsError(stderr)
    if "No such workspace" in stderr:
        return WorkspaceNotFoundError(stderr)
    return JJCommandError(cmd, stderr, returncode)


class JJClient:
    """Wrapper for jj CLI commands."""

    def __init__(self, jj_path: str = "jj"):
        self.jj_path = jj_path

    def _run(self, *args: str, cwd: str | None = None) -> str:
        """Execute jj command and return stdout."""
        result = subprocess.run(
            [self.jj_path, *args],
            capture_output=True,
            text=True,
            cwd=cwd,
        )
        if result.returncode != 0:
            cmd = args[0] if args else ""
            raise _parse_error(cmd, result.stderr.strip(), result.returncode)
        return result.stdout

    def workspace_root(self, cwd: str | None = None) -> str:
        """Return the root directory of the current workspace."""
        return self._run("workspace", "root", cwd=cwd).strip()

    def workspace_add(
        self, path: str, revision: str = "", cwd: str | None = None
    ) -> None:
        """Create a new workspace at the given path."""
        args = ["workspace", "add", path]
        if revision:
            args.extend(["-r", revision])
        self._run(*args, cwd=cwd)

    def workspace_forget(self, name: str, cwd: str | None = None) -> None:
        """Remove a workspace from jj tracking (does NOT delete directory)."""
        self._run("workspace", "forget", name, cwd=cwd)

    def workspace_list(self, cwd: str | None = None) -> list[Workspace]:
        """Return all workspaces in the repository."""
        output = self._run("workspace", "list", cwd=cwd)
        workspaces = []
        for line in output.strip().split("\n"):
            if not line:
                continue
            match = WORKSPACE_LINE_RE.match(line)
            if match:
                workspaces.append(
                    Workspace(
                        name=match.group(1),
                        change_id=match.group(2),
                        commit_id=match.group(3),
                        summary=match.group(4),
                    )
                )
        return workspaces

    def status(self, cwd: str | None = None) -> str:
        """Return jj status output."""
        return self._run("status", cwd=cwd)
