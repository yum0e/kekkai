"""Tests for kekkai.jj module."""

import os
from pathlib import Path

import pytest

from kekkai.errors import NotJJRepoError, WorkspaceExistsError
from kekkai.jj import JJClient


def test_workspace_root(temp_jj_repo):
    """Test getting workspace root."""
    client = JJClient()
    root = client.workspace_root(cwd=str(temp_jj_repo))

    # Resolve symlinks for comparison (macOS /var -> /private/var)
    expected = temp_jj_repo.resolve()
    actual = Path(root).resolve()

    assert actual == expected


def test_workspace_add(temp_jj_repo):
    """Test adding a workspace."""
    client = JJClient()
    workspace_path = temp_jj_repo / "test-workspace"

    # Add workspace
    client.workspace_add(str(workspace_path), cwd=str(temp_jj_repo))

    # Verify workspace directory exists
    assert workspace_path.exists()

    # Adding same workspace again should fail
    with pytest.raises(WorkspaceExistsError):
        client.workspace_add(str(workspace_path), cwd=str(temp_jj_repo))


def test_workspace_list(temp_jj_repo):
    """Test listing workspaces."""
    client = JJClient()

    # Initially should have just "default" workspace
    workspaces = client.workspace_list(cwd=str(temp_jj_repo))
    assert len(workspaces) == 1
    assert workspaces[0].name == "default"

    # Add a workspace
    workspace_path = temp_jj_repo / "agent-1"
    client.workspace_add(str(workspace_path), cwd=str(temp_jj_repo))

    # Should now have 2 workspaces
    workspaces = client.workspace_list(cwd=str(temp_jj_repo))
    assert len(workspaces) == 2

    # Find agent-1
    names = [ws.name for ws in workspaces]
    assert "agent-1" in names


def test_workspace_forget(temp_jj_repo):
    """Test forgetting a workspace."""
    client = JJClient()

    # Add a workspace
    workspace_path = temp_jj_repo / "to-forget"
    client.workspace_add(str(workspace_path), cwd=str(temp_jj_repo))

    # Forget it
    client.workspace_forget("to-forget", cwd=str(temp_jj_repo))

    # Should only have default workspace now
    workspaces = client.workspace_list(cwd=str(temp_jj_repo))
    names = [ws.name for ws in workspaces]
    assert "to-forget" not in names

    # Note: workspace_forget does NOT delete the directory
    assert workspace_path.exists()


def test_not_jj_repo(temp_non_jj_dir):
    """Test error when not in a jj repo."""
    client = JJClient()

    with pytest.raises(NotJJRepoError):
        client.workspace_root(cwd=str(temp_non_jj_dir))
