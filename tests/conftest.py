"""Pytest fixtures for kekkai tests."""

import subprocess
import tempfile
from pathlib import Path

import pytest


@pytest.fixture
def temp_jj_repo(tmp_path):
    """Create a temporary jj repository for testing.

    Creates a parent directory to hold both the repo and sibling workspaces.
    """
    repo_dir = tmp_path / "testrepo"
    repo_dir.mkdir()

    subprocess.run(["jj", "git", "init"], cwd=repo_dir, check=True, capture_output=True)

    return repo_dir


@pytest.fixture
def temp_non_jj_dir(tmp_path):
    """Create a temporary directory that is NOT a jj repo."""
    non_jj_dir = tmp_path / "notjj"
    non_jj_dir.mkdir()
    return non_jj_dir
