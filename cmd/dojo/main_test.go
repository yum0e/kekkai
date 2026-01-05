package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bigq/dojo/internal/jj"
)

// setupTestRepo creates a temporary jj repository for testing.
// Returns the repo path and a cleanup function that removes the parent temp directory.
func setupTestRepo(t *testing.T) (string, func()) {
	t.Helper()

	// Create a parent temp directory to hold both the repo and any sibling workspaces
	parentDir, err := os.MkdirTemp("", "dojo-test-parent-*")
	if err != nil {
		t.Fatalf("failed to create temp parent dir: %v", err)
	}

	// Create the repo directory inside the parent
	repoDir := filepath.Join(parentDir, "testrepo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		os.RemoveAll(parentDir)
		t.Fatalf("failed to create repo dir: %v", err)
	}

	cmd := exec.Command("jj", "git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		os.RemoveAll(parentDir)
		t.Fatalf("failed to init jj repo: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(parentDir)
	}

	return repoDir, cleanup
}

func TestComputeAgentPath(t *testing.T) {
	cases := []struct {
		root, name, expected string
	}{
		{"/Users/dev/myproject", "feature-auth", "/Users/dev/myproject-feature-auth"},
		{"/home/user/repo", "test", "/home/user/repo-test"},
		{"/tmp/dojo", "agent1", "/tmp/dojo-agent1"},
	}

	for _, c := range cases {
		got := computeAgentPath(c.root, c.name)
		if got != c.expected {
			t.Errorf("computeAgentPath(%q, %q) = %q, want %q", c.root, c.name, got, c.expected)
		}
	}
}

func TestComputeJJWorkspaceName(t *testing.T) {
	cases := []struct {
		root, name, expected string
	}{
		{"/Users/dev/myproject", "feature-auth", "myproject-feature-auth"},
		{"/home/user/repo", "test", "repo-test"},
	}

	for _, c := range cases {
		got := computeJJWorkspaceName(c.root, c.name)
		if got != c.expected {
			t.Errorf("computeJJWorkspaceName(%q, %q) = %q, want %q", c.root, c.name, got, c.expected)
		}
	}
}

func TestFindRootWorkspaceFromRoot(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	ctx := context.Background()
	client := jj.NewClient()

	root, err := findRootWorkspace(ctx, client)
	if err != nil {
		t.Fatalf("findRootWorkspace failed: %v", err)
	}

	// Normalize paths to handle macOS /var -> /private/var symlinks
	expectedDir, _ := filepath.EvalSymlinks(dir)
	actualRoot, _ := filepath.EvalSymlinks(root)

	if actualRoot != expectedDir {
		t.Errorf("expected root %q, got %q", expectedDir, actualRoot)
	}
}

func TestFindRootWorkspaceFromAgent(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()
	client := jj.NewClient()

	// Create a sibling agent workspace
	agentName := "test-agent"
	agentPath := computeAgentPath(dir, agentName)

	if err := client.WorkspaceAddFromDir(ctx, dir, agentPath, ""); err != nil {
		t.Fatalf("WorkspaceAddFromDir failed: %v", err)
	}

	// Create the agent marker pointing to root
	if err := createAgentMarker(agentPath, dir, agentName); err != nil {
		t.Fatalf("createAgentMarker failed: %v", err)
	}

	// Change to agent directory
	oldWd, _ := os.Getwd()
	os.Chdir(agentPath)
	defer os.Chdir(oldWd)

	// Find root should return original root, not agent path
	root, err := findRootWorkspace(ctx, client)
	if err != nil {
		t.Fatalf("findRootWorkspace failed: %v", err)
	}

	if root != dir {
		t.Errorf("expected root %q, got %q", dir, root)
	}
}

func TestCreateAgentMarker(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()
	client := jj.NewClient()

	// Create sibling workspace
	agentName := "marker-test"
	agentPath := computeAgentPath(dir, agentName)

	if err := client.WorkspaceAddFromDir(ctx, dir, agentPath, ""); err != nil {
		t.Fatalf("WorkspaceAddFromDir failed: %v", err)
	}

	// Create marker
	if err := createAgentMarker(agentPath, dir, agentName); err != nil {
		t.Fatalf("createAgentMarker failed: %v", err)
	}

	// Verify marker exists and has correct content
	markerPath := filepath.Join(agentPath, agentMarkerFile)
	data, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("failed to read marker file: %v", err)
	}

	var marker AgentMarker
	if err := json.Unmarshal(data, &marker); err != nil {
		t.Fatalf("failed to parse marker JSON: %v", err)
	}

	if marker.RootWorkspace != dir {
		t.Errorf("marker.RootWorkspace = %q, want %q", marker.RootWorkspace, dir)
	}

	if marker.Name != agentName {
		t.Errorf("marker.Name = %q, want %q", marker.Name, agentName)
	}

	if marker.CreatedAt == "" {
		t.Error("marker.CreatedAt should not be empty")
	}
}

func TestSetupClaudeSymlink(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()
	client := jj.NewClient()

	// Create .claude directory in root
	rootClaude := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(rootClaude, 0755); err != nil {
		t.Fatalf("failed to create .claude dir: %v", err)
	}

	// Create sibling workspace
	agentName := "symlink-test"
	agentPath := computeAgentPath(dir, agentName)

	if err := client.WorkspaceAddFromDir(ctx, dir, agentPath, ""); err != nil {
		t.Fatalf("WorkspaceAddFromDir failed: %v", err)
	}

	// Setup symlink
	if err := setupClaudeSymlink(agentPath, dir); err != nil {
		t.Fatalf("setupClaudeSymlink failed: %v", err)
	}

	// Verify symlink exists and points to correct location
	agentClaude := filepath.Join(agentPath, ".claude")
	info, err := os.Lstat(agentClaude)
	if err != nil {
		t.Fatalf("failed to stat .claude in agent: %v", err)
	}

	if info.Mode()&os.ModeSymlink == 0 {
		t.Error(".claude should be a symlink")
	}

	// Verify symlink target
	target, err := os.Readlink(agentClaude)
	if err != nil {
		t.Fatalf("failed to read symlink: %v", err)
	}

	// Target should be relative and point to root's .claude
	expectedRel := "../testrepo/.claude"
	if target != expectedRel {
		t.Errorf("symlink target = %q, want %q", target, expectedRel)
	}
}

func TestSetupClaudeSymlinkNoRootClaude(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()
	client := jj.NewClient()

	// Don't create .claude directory in root
	agentName := "no-claude-test"
	agentPath := computeAgentPath(dir, agentName)

	if err := client.WorkspaceAddFromDir(ctx, dir, agentPath, ""); err != nil {
		t.Fatalf("WorkspaceAddFromDir failed: %v", err)
	}

	// Setup symlink should succeed but not create anything
	if err := setupClaudeSymlink(agentPath, dir); err != nil {
		t.Fatalf("setupClaudeSymlink failed: %v", err)
	}

	// Verify no symlink was created
	agentClaude := filepath.Join(agentPath, ".claude")
	if _, err := os.Lstat(agentClaude); !os.IsNotExist(err) {
		t.Error(".claude should not exist when root has no .claude")
	}
}

func TestSetupClaudeSymlinkReplacesExistingDir(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()
	client := jj.NewClient()

	// Create .claude directory in root
	rootClaude := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(rootClaude, 0755); err != nil {
		t.Fatalf("failed to create .claude dir: %v", err)
	}

	// Create sibling workspace
	agentName := "replace-dir-test"
	agentPath := computeAgentPath(dir, agentName)

	if err := client.WorkspaceAddFromDir(ctx, dir, agentPath, ""); err != nil {
		t.Fatalf("WorkspaceAddFromDir failed: %v", err)
	}

	// Manually create .claude directory in agent workspace to simulate
	// the scenario where it already exists (e.g., from previous run or jj tracking it)
	agentClaude := filepath.Join(agentPath, ".claude")
	if err := os.MkdirAll(agentClaude, 0755); err != nil {
		t.Fatalf("failed to create .claude dir in agent: %v", err)
	}
	// Add a file inside to make it non-empty
	if err := os.WriteFile(filepath.Join(agentClaude, "settings.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to create settings file: %v", err)
	}

	// Verify .claude exists as a directory (not a symlink)
	info, err := os.Lstat(agentClaude)
	if err != nil {
		t.Fatalf("expected .claude to exist: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("expected .claude to be a directory, not a symlink")
	}

	// Setup symlink should replace the directory with a symlink
	if err := setupClaudeSymlink(agentPath, dir); err != nil {
		t.Fatalf("setupClaudeSymlink failed: %v", err)
	}

	// Verify it's now a symlink
	info, err = os.Lstat(agentClaude)
	if err != nil {
		t.Fatalf("failed to stat .claude after symlink setup: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error(".claude should be a symlink after setupClaudeSymlink")
	}

	// Verify symlink target is correct
	target, err := os.Readlink(agentClaude)
	if err != nil {
		t.Fatalf("failed to read symlink: %v", err)
	}
	expectedRel := "../testrepo/.claude"
	if target != expectedRel {
		t.Errorf("symlink target = %q, want %q", target, expectedRel)
	}
}

func TestGitShimCreation(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	ctx := context.Background()
	client := jj.NewClient()

	// Create sibling workspace
	agentName := "shim-test"
	agentPath := computeAgentPath(dir, agentName)

	if err := client.WorkspaceAddFromDir(ctx, dir, agentPath, ""); err != nil {
		t.Fatalf("WorkspaceAddFromDir failed: %v", err)
	}

	// Create git shim (mimicking what runAgent does)
	shimPath := filepath.Join(agentPath, shimDir)
	if err := os.MkdirAll(shimPath, 0755); err != nil {
		t.Fatalf("failed to create shim directory: %v", err)
	}

	shimScript := filepath.Join(shimPath, "git")
	shimContent := `#!/bin/sh
echo "git disabled for agents; use jj" >&2
exit 1
`
	if err := os.WriteFile(shimScript, []byte(shimContent), 0755); err != nil {
		t.Fatalf("failed to write git shim: %v", err)
	}

	// Verify shim exists and is executable
	info, err := os.Stat(shimScript)
	if err != nil {
		t.Fatalf("git shim not found: %v", err)
	}

	if info.Mode().Perm()&0111 == 0 {
		t.Error("git shim is not executable")
	}

	// Test that shim blocks git
	cmd := exec.Command(shimScript, "status")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Error("expected git shim to return error")
	}

	if !strings.Contains(string(output), "git disabled") {
		t.Errorf("unexpected shim output: %s", output)
	}
}

func TestGitMarkerCreation(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	ctx := context.Background()
	client := jj.NewClient()

	// Create sibling workspace
	agentName := "marker-git-test"
	agentPath := computeAgentPath(dir, agentName)

	if err := client.WorkspaceAddFromDir(ctx, dir, agentPath, ""); err != nil {
		t.Fatalf("WorkspaceAddFromDir failed: %v", err)
	}

	// Create .git marker (mimicking what runAgent does)
	gitMarker := filepath.Join(agentPath, ".git")
	if err := os.WriteFile(gitMarker, []byte{}, 0644); err != nil {
		t.Fatalf("failed to create .git marker: %v", err)
	}

	// Verify marker exists
	if _, err := os.Stat(gitMarker); os.IsNotExist(err) {
		t.Error(".git marker was not created")
	}
}

func TestCleanup(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	ctx := context.Background()
	client := jj.NewClient()

	// Create sibling workspace with all fixtures
	agentName := "cleanup-test"
	agentPath := computeAgentPath(dir, agentName)
	jjWorkspaceName := computeJJWorkspaceName(dir, agentName)

	if err := client.WorkspaceAddFromDir(ctx, dir, agentPath, ""); err != nil {
		t.Fatalf("WorkspaceAddFromDir failed: %v", err)
	}

	// Create .dojo-agent marker
	if err := createAgentMarker(agentPath, dir, agentName); err != nil {
		t.Fatalf("createAgentMarker failed: %v", err)
	}

	// Create .claude symlink (first create root .claude)
	rootClaude := filepath.Join(dir, ".claude")
	os.MkdirAll(rootClaude, 0755)
	setupClaudeSymlink(agentPath, dir)

	// Create .git marker
	gitMarker := filepath.Join(agentPath, ".git")
	if err := os.WriteFile(gitMarker, []byte{}, 0644); err != nil {
		t.Fatalf("failed to create .git marker: %v", err)
	}

	// Create shim
	shimPath := filepath.Join(agentPath, shimDir)
	os.MkdirAll(shimPath, 0755)
	os.WriteFile(filepath.Join(shimPath, "git"), []byte("#!/bin/sh\nexit 1"), 0755)

	// Verify workspace exists
	if _, err := os.Stat(agentPath); os.IsNotExist(err) {
		t.Fatal("workspace should exist before cleanup")
	}

	// Run cleanup (using the actual cleanup function signature)
	cleanupTest(ctx, client, jjWorkspaceName, agentPath, dir)

	// Verify workspace is gone
	if _, err := os.Stat(agentPath); !os.IsNotExist(err) {
		t.Error("workspace directory should be removed after cleanup")
	}

	// Verify workspace is forgotten from jj
	workspaces, _ := client.WorkspaceListFromDir(ctx, dir)
	for _, ws := range workspaces {
		if ws.Name == jjWorkspaceName {
			t.Error("workspace should be forgotten from jj")
		}
	}
}

// cleanupTest mirrors the cleanup function for testing
func cleanupTest(ctx context.Context, client *jj.Client, jjWorkspaceName, workspacePath, rootPath string) {
	os.Remove(filepath.Join(workspacePath, ".git"))
	os.Remove(filepath.Join(workspacePath, ".claude"))
	os.Remove(filepath.Join(workspacePath, agentMarkerFile))
	client.WorkspaceForgetFromDir(ctx, rootPath, jjWorkspaceName)
	os.RemoveAll(workspacePath)
}

func TestListWorkspacesEmpty(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	ctx := context.Background()
	client := jj.NewClient()

	// List workspaces - should only have default
	workspaces, err := client.WorkspaceListFromDir(ctx, dir)
	if err != nil {
		t.Fatalf("WorkspaceListFromDir failed: %v", err)
	}

	repoName := filepath.Base(dir)
	prefix := repoName + "-"

	var agentCount int
	for _, ws := range workspaces {
		if ws.Name != "default" && strings.HasPrefix(ws.Name, prefix) {
			agentCount++
		}
	}

	if agentCount != 0 {
		t.Errorf("expected 0 agent workspaces, got %d", agentCount)
	}
}

func TestListWorkspacesWithAgents(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	oldWd, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldWd)

	ctx := context.Background()
	client := jj.NewClient()

	// Create two agent workspaces
	agents := []string{"agent1", "agent2"}
	for _, name := range agents {
		agentPath := computeAgentPath(dir, name)
		if err := client.WorkspaceAddFromDir(ctx, dir, agentPath, ""); err != nil {
			t.Fatalf("WorkspaceAddFromDir failed: %v", err)
		}
		if err := createAgentMarker(agentPath, dir, name); err != nil {
			t.Fatalf("createAgentMarker failed: %v", err)
		}
	}

	// List workspaces
	workspaces, err := client.WorkspaceListFromDir(ctx, dir)
	if err != nil {
		t.Fatalf("WorkspaceListFromDir failed: %v", err)
	}

	repoName := filepath.Base(dir)
	prefix := repoName + "-"

	var foundAgents []string
	for _, ws := range workspaces {
		if ws.Name != "default" && strings.HasPrefix(ws.Name, prefix) {
			agentName := strings.TrimPrefix(ws.Name, prefix)
			// Verify marker exists
			agentPath := filepath.Join(filepath.Dir(dir), ws.Name)
			markerPath := filepath.Join(agentPath, agentMarkerFile)
			if _, err := os.Stat(markerPath); err == nil {
				foundAgents = append(foundAgents, agentName)
			}
		}
	}

	if len(foundAgents) != 2 {
		t.Errorf("expected 2 agent workspaces, got %d: %v", len(foundAgents), foundAgents)
	}
}

func TestPathWithShim(t *testing.T) {
	shimPath := "/fake/shim/path"
	originalPath := "/usr/bin:/bin"

	env := []string{
		"HOME=/home/user",
		"PATH=" + originalPath,
		"TERM=xterm",
	}

	newPath := shimPath + ":" + originalPath
	for i, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			env[i] = "PATH=" + newPath
			break
		}
	}

	// Find PATH in modified env
	var foundPath string
	for _, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			foundPath = strings.TrimPrefix(e, "PATH=")
			break
		}
	}

	if !strings.HasPrefix(foundPath, shimPath) {
		t.Errorf("shim path should be first in PATH, got: %s", foundPath)
	}
}

func TestCheckParentWritable(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	// Parent should be writable (it's a temp dir we created)
	if err := checkParentWritable(dir); err != nil {
		t.Errorf("checkParentWritable failed for writable dir: %v", err)
	}
}

func TestNestedAgentCreation(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	ctx := context.Background()
	client := jj.NewClient()

	// Create first agent workspace
	agent1Name := "agent1"
	agent1Path := computeAgentPath(dir, agent1Name)

	if err := client.WorkspaceAddFromDir(ctx, dir, agent1Path, ""); err != nil {
		t.Fatalf("WorkspaceAddFromDir failed: %v", err)
	}
	if err := createAgentMarker(agent1Path, dir, agent1Name); err != nil {
		t.Fatalf("createAgentMarker failed: %v", err)
	}

	// Change to agent1 directory
	oldWd, _ := os.Getwd()
	os.Chdir(agent1Path)
	defer os.Chdir(oldWd)

	// findRootWorkspace from agent1 should return original root
	root, err := findRootWorkspace(ctx, client)
	if err != nil {
		t.Fatalf("findRootWorkspace failed: %v", err)
	}

	if root != dir {
		t.Errorf("expected root %q, got %q", dir, root)
	}

	// Computing a new agent path from root should give sibling to original root
	agent2Name := "agent2"
	agent2Path := computeAgentPath(root, agent2Name)

	expectedAgent2Path := filepath.Join(filepath.Dir(dir), filepath.Base(dir)+"-"+agent2Name)
	if agent2Path != expectedAgent2Path {
		t.Errorf("expected agent2 path %q, got %q", expectedAgent2Path, agent2Path)
	}
}
