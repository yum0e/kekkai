"""Custom exceptions for kekkai."""


class KekkaiError(Exception):
    """Base exception for kekkai."""

    pass


class NotJJRepoError(KekkaiError):
    """Not in a jj repository."""

    pass


class WorkspaceExistsError(KekkaiError):
    """Workspace already exists."""

    pass


class WorkspaceNotFoundError(KekkaiError):
    """Workspace not found."""

    pass


class JJCommandError(KekkaiError):
    """jj command failed."""

    def __init__(self, cmd: str, stderr: str, returncode: int):
        self.cmd = cmd
        self.stderr = stderr
        self.returncode = returncode
        super().__init__(f"jj {cmd}: {stderr}")
