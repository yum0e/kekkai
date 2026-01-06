"""Tests for kekkai.cli module."""

import json
import os
import subprocess
import sys
from pathlib import Path

import pytest

from kekkai.cli import (
    AGENT_MARKER_FILE,
    AGENTS,
    SHIM_DIR,
    Agent,
    AgentMarker,
    check_parent_writable,
    cleanup,
    compute_agent_path,
    compute_jj_workspace_name,
    create_agent_marker,
    find_root_workspace,
    look_workspace,
    main,
    run_agent,
)
from kekkai.jj import JJClient


def test_compute_agent_path():
    """Test agent path computation."""
    cases = [
        ("/Users/dev/myproject", "feature-auth", "/Users/dev/myproject-feature-auth"),
        ("/home/user/repo", "test", "/home/user/repo-test"),
        ("/tmp/dojo", "agent1", "/tmp/dojo-agent1"),
    ]

    for root, name, expected in cases:
        assert compute_agent_path(root, name) == expected


def test_compute_jj_workspace_name():
    """Test jj workspace name computation."""
    cases = [
        ("/Users/dev/myproject", "feature-auth", "myproject-feature-auth"),
        ("/home/user/repo", "test", "repo-test"),
    ]

    for root, name, expected in cases:
        assert compute_jj_workspace_name(root, name) == expected


def test_find_root_workspace_from_root(temp_jj_repo):
    """Test finding root workspace when in root."""
    client = JJClient()
    root = find_root_workspace(client)

    # We need to be in the temp_jj_repo for this to work
    old_cwd = os.getcwd()
    os.chdir(temp_jj_repo)
    try:
        root = find_root_workspace(client)
        expected = temp_jj_repo.resolve()
        actual = Path(root).resolve()
        assert actual == expected
    finally:
        os.chdir(old_cwd)


def test_find_root_workspace_from_agent(temp_jj_repo):
    """Test finding root workspace from an agent workspace."""
    client = JJClient()

    # Create a sibling agent workspace
    agent_name = "test-agent"
    agent_path = compute_agent_path(str(temp_jj_repo), agent_name)

    client.workspace_add(agent_path, cwd=str(temp_jj_repo))

    # Create the agent marker pointing to root
    create_agent_marker(agent_path, str(temp_jj_repo), agent_name, "codex")

    # Change to agent directory
    old_cwd = os.getcwd()
    os.chdir(agent_path)
    try:
        root = find_root_workspace(client)
        assert root == str(temp_jj_repo)
    finally:
        os.chdir(old_cwd)


def test_create_agent_marker(temp_jj_repo):
    """Test agent marker creation."""
    client = JJClient()

    # Create sibling workspace
    agent_name = "marker-test"
    agent_path = compute_agent_path(str(temp_jj_repo), agent_name)

    client.workspace_add(agent_path, cwd=str(temp_jj_repo))

    # Create marker
    create_agent_marker(agent_path, str(temp_jj_repo), agent_name, "codex")

    # Verify marker exists and has correct content
    marker_path = Path(agent_path) / AGENT_MARKER_FILE
    data = json.loads(marker_path.read_text())

    assert data["root_workspace"] == str(temp_jj_repo)
    assert data["name"] == agent_name
    assert data["created_at"]  # Should not be empty
    assert data["agent"] == "codex"


def test_git_shim_creation(temp_jj_repo):
    """Test git shim creation and behavior."""
    client = JJClient()

    # Create sibling workspace
    agent_name = "shim-test"
    agent_path = compute_agent_path(str(temp_jj_repo), agent_name)

    client.workspace_add(agent_path, cwd=str(temp_jj_repo))

    # Create git shim
    from kekkai.cli import SHIM_CONTENT

    shim_path = Path(agent_path) / SHIM_DIR
    shim_path.mkdir(parents=True, exist_ok=True)
    shim_script = shim_path / "git"
    shim_script.write_text(SHIM_CONTENT)
    shim_script.chmod(0o755)

    # Verify shim exists and is executable
    assert shim_script.exists()
    assert os.access(shim_script, os.X_OK)

    # Test that shim blocks git
    result = subprocess.run(
        [str(shim_script), "status"], capture_output=True, text=True
    )
    assert result.returncode != 0
    assert "git disabled" in result.stderr


def test_git_dir_creation(temp_jj_repo):
    """Test .git directory creation."""
    client = JJClient()

    # Create sibling workspace
    agent_name = "git-dir-test"
    agent_path = compute_agent_path(str(temp_jj_repo), agent_name)

    client.workspace_add(agent_path, cwd=str(temp_jj_repo))

    # Create .git directory
    git_dir = Path(agent_path) / ".git"
    git_dir.mkdir(parents=True, exist_ok=True)

    # Verify .git directory exists
    assert git_dir.exists()
    assert git_dir.is_dir()


def test_cleanup(temp_jj_repo):
    """Test workspace cleanup."""
    client = JJClient()

    # Create sibling workspace with all fixtures
    agent_name = "cleanup-test"
    agent_path = compute_agent_path(str(temp_jj_repo), agent_name)
    jj_workspace_name = compute_jj_workspace_name(str(temp_jj_repo), agent_name)

    client.workspace_add(agent_path, cwd=str(temp_jj_repo))

    # Create agent marker
    create_agent_marker(agent_path, str(temp_jj_repo), agent_name, "codex")

    # Create .git directory
    git_dir = Path(agent_path) / ".git"
    git_dir.mkdir(parents=True, exist_ok=True)

    # Create shim
    from kekkai.cli import SHIM_CONTENT

    shim_path = Path(agent_path) / SHIM_DIR
    shim_path.mkdir(parents=True, exist_ok=True)
    (shim_path / "git").write_text(SHIM_CONTENT)

    # Verify workspace exists
    assert Path(agent_path).exists()

    # Run cleanup
    cleanup(client, jj_workspace_name, agent_path, str(temp_jj_repo))

    # Verify workspace is gone
    assert not Path(agent_path).exists()

    # Verify workspace is forgotten from jj
    workspaces = client.workspace_list(cwd=str(temp_jj_repo))
    names = [ws.name for ws in workspaces]
    assert jj_workspace_name not in names


def test_check_parent_writable(temp_jj_repo):
    """Test parent directory writability check."""
    # Parent should be writable (it's a temp dir)
    check_parent_writable(str(temp_jj_repo))  # Should not raise


def test_markers_hidden_from_jj_status(temp_jj_repo):
    """Test that markers don't appear in jj status."""
    client = JJClient()

    # Create sibling workspace
    agent_name = "status-test"
    agent_path = compute_agent_path(str(temp_jj_repo), agent_name)

    client.workspace_add(agent_path, cwd=str(temp_jj_repo))

    # Create .git directory (auto-ignored by jj)
    git_dir = Path(agent_path) / ".git"
    git_dir.mkdir(parents=True, exist_ok=True)

    # Create agent marker (inside .jj so auto-ignored)
    create_agent_marker(agent_path, str(temp_jj_repo), agent_name, "codex")

    # Get jj status from the agent workspace
    status = client.status(cwd=agent_path)

    # Verify markers don't appear in status
    assert "kekkai-agent" not in status


def test_nested_agent_creation(temp_jj_repo):
    """Test creating agent from within another agent workspace."""
    client = JJClient()

    # Create first agent workspace
    agent1_name = "agent1"
    agent1_path = compute_agent_path(str(temp_jj_repo), agent1_name)

    client.workspace_add(agent1_path, cwd=str(temp_jj_repo))
    create_agent_marker(agent1_path, str(temp_jj_repo), agent1_name, "codex")

    # Change to agent1 directory
    old_cwd = os.getcwd()
    os.chdir(agent1_path)
    try:
        # find_root_workspace from agent1 should return original root
        root = find_root_workspace(client)
        assert root == str(temp_jj_repo)

        # Computing a new agent path from root should give sibling to original root
        agent2_name = "agent2"
        agent2_path = compute_agent_path(root, agent2_name)

        expected = str(temp_jj_repo.parent / f"{temp_jj_repo.name}-{agent2_name}")
        assert agent2_path == expected
    finally:
        os.chdir(old_cwd)


def test_list_workspaces_empty(temp_jj_repo):
    """Test listing workspaces when none exist."""
    client = JJClient()

    workspaces = client.workspace_list(cwd=str(temp_jj_repo))
    repo_name = temp_jj_repo.name
    prefix = f"{repo_name}-"

    agent_count = sum(
        1
        for ws in workspaces
        if ws.name != "default" and ws.name.startswith(prefix)
    )

    assert agent_count == 0


def test_list_workspaces_with_agents(temp_jj_repo):
    """Test listing workspaces with agents."""
    client = JJClient()

    # Create two agent workspaces
    agents = ["agent1", "agent2"]
    for name in agents:
        agent_path = compute_agent_path(str(temp_jj_repo), name)
        client.workspace_add(agent_path, cwd=str(temp_jj_repo))
        create_agent_marker(agent_path, str(temp_jj_repo), name, "codex")

    # List workspaces
    workspaces = client.workspace_list(cwd=str(temp_jj_repo))
    repo_name = temp_jj_repo.name
    prefix = f"{repo_name}-"

    found_agents = []
    for ws in workspaces:
        if ws.name != "default" and ws.name.startswith(prefix):
            agent_name = ws.name[len(prefix) :]
            # Verify marker exists
            agent_path = temp_jj_repo.parent / ws.name
            marker_path = agent_path / AGENT_MARKER_FILE
            if marker_path.exists():
                found_agents.append(agent_name)

    assert len(found_agents) == 2


def test_spinner_shown_during_setup(temp_jj_repo, monkeypatch):
    """Test that spinner is shown during workspace setup before Claude launches."""
    from io import StringIO

    from rich.console import Console

    # Create mock claude binary that records when it was called
    mock_bin = temp_jj_repo / "mock-bin"
    mock_bin.mkdir()
    mock_claude = mock_bin / "claude"
    call_marker = temp_jj_repo / "claude-called"
    mock_claude.write_text(f"#!/bin/sh\ntouch {call_marker}\nexit 0\n")
    mock_claude.chmod(0o755)

    # Prepend mock bin to PATH
    original_path = os.environ.get("PATH", "")
    monkeypatch.setenv("PATH", f"{mock_bin}:{original_path}")

    # Mock input() to auto-answer cleanup prompt with 'n'
    monkeypatch.setattr("builtins.input", lambda _: "n")

    # Capture console output by forcing terminal mode with a file output
    output_buffer = StringIO()

    # Patch Console to capture spinner output
    class TestConsole(Console):
        def __init__(self, *args, **kwargs):
            kwargs["force_terminal"] = True
            kwargs["file"] = output_buffer
            super().__init__(*args, **kwargs)

    monkeypatch.setattr("kekkai.cli.Console", TestConsole)

    # Change to temp repo directory
    old_cwd = os.getcwd()
    os.chdir(temp_jj_repo)
    try:
        run_agent("spinner-test", AGENTS["claude"])
    finally:
        os.chdir(old_cwd)

    # Verify mock claude was called (workspace setup completed)
    assert call_marker.exists(), "Mock claude should have been called"

    # Verify spinner message was shown
    output = output_buffer.getvalue().lower()
    assert "summoning" in output, f"Spinner message not found in output: {output}"


def test_help_shows_version(capsys, monkeypatch):
    """Help output should include the package version."""
    from kekkai import __version__

    monkeypatch.setattr(sys, "argv", ["kekkai", "--help"])
    with pytest.raises(SystemExit) as excinfo:
        main()

    assert excinfo.value.code == 0
    output = capsys.readouterr().out
    assert f"kekkai {__version__}" in output


def test_version_flag_outputs_version(capsys, monkeypatch):
    """--version should print the package version."""
    from kekkai import __version__

    monkeypatch.setattr(sys, "argv", ["kekkai", "--version"])
    with pytest.raises(SystemExit) as excinfo:
        main()

    assert excinfo.value.code == 0
    output = capsys.readouterr().out
    assert f"kekkai {__version__}" in output


def test_look_workspace_creates_new_revision(temp_jj_repo, monkeypatch):
    """look should create a new revision based on the agent workspace."""
    client = JJClient()
    agent_name = "look-agent"
    agent_path = compute_agent_path(str(temp_jj_repo), agent_name)

    client.workspace_add(agent_path, cwd=str(temp_jj_repo))
    create_agent_marker(agent_path, str(temp_jj_repo), agent_name, "codex")

    def current_change_id() -> str:
        result = subprocess.run(
            [
                "jj",
                "log",
                "-r",
                "@",
                "--no-graph",
                "--template",
                "change_id",
            ],
            cwd=temp_jj_repo,
            check=True,
            capture_output=True,
            text=True,
        )
        return result.stdout.strip()

    before = current_change_id()
    monkeypatch.chdir(temp_jj_repo)
    look_workspace(agent_name)
    after = current_change_id()

    assert before != after


def test_look_workspace_requires_root(temp_jj_repo, monkeypatch, capsys):
    """look should fail when run from an agent workspace."""
    client = JJClient()
    agent_name = "agent-root-check"
    agent_path = compute_agent_path(str(temp_jj_repo), agent_name)

    client.workspace_add(agent_path, cwd=str(temp_jj_repo))
    create_agent_marker(agent_path, str(temp_jj_repo), agent_name, "codex")

    monkeypatch.chdir(agent_path)
    with pytest.raises(SystemExit) as excinfo:
        look_workspace(agent_name)

    assert excinfo.value.code == 1
    err = capsys.readouterr().err.lower()
    assert "root workspace" in err


def test_look_workspace_suggests_similar_names(temp_jj_repo, monkeypatch, capsys):
    """look should suggest similar agent names when not found."""
    client = JJClient()
    agent_name = "feature-preview"
    agent_path = compute_agent_path(str(temp_jj_repo), agent_name)

    client.workspace_add(agent_path, cwd=str(temp_jj_repo))
    create_agent_marker(agent_path, str(temp_jj_repo), agent_name, "codex")

    monkeypatch.chdir(temp_jj_repo)
    with pytest.raises(SystemExit) as excinfo:
        look_workspace("feature-prevew")

    assert excinfo.value.code == 1
    err = capsys.readouterr().err.lower()
    assert "did you mean" in err
    assert agent_name in err
