package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/bigq/dojo/internal/jj"
)

const (
	shimDir         = ".jj/.dojo-bin"
	agentMarkerFile = ".dojo-agent"
)

// AgentMarker contains metadata stored in .dojo-agent file.
type AgentMarker struct {
	RootWorkspace string `json:"root_workspace"` // Absolute path to root
	Name          string `json:"name"`           // Agent name
	CreatedAt     string `json:"created_at"`     // ISO timestamp
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "list":
		listWorkspaces()
	case "-h", "--help", "help":
		printUsage()
	default:
		runAgent(os.Args[1])
	}
}

func printUsage() {
	fmt.Println(`Usage: dojo <name>    Create workspace and launch Claude
       dojo list      List existing workspaces`)
}

// findRootWorkspace finds the original root workspace.
// If we're in an agent workspace, it follows the marker to find the root.
// Otherwise, returns the current jj workspace root.
func findRootWorkspace(ctx context.Context, client *jj.Client) (string, error) {
	// Get current workspace root
	currentRoot, err := client.WorkspaceRoot(ctx)
	if err != nil {
		return "", err
	}

	// Check if we're in an agent workspace
	markerPath := filepath.Join(currentRoot, agentMarkerFile)
	if data, err := os.ReadFile(markerPath); err == nil {
		var marker AgentMarker
		if json.Unmarshal(data, &marker) == nil && marker.RootWorkspace != "" {
			return marker.RootWorkspace, nil
		}
	}

	return currentRoot, nil
}

// computeAgentPath computes the sibling workspace path.
// Returns path like /Users/dev/myproject-feature-auth
func computeAgentPath(rootPath, agentName string) string {
	parentDir := filepath.Dir(rootPath)
	repoName := filepath.Base(rootPath)
	return filepath.Join(parentDir, repoName+"-"+agentName)
}

// computeJJWorkspaceName returns the jj workspace name for an agent.
func computeJJWorkspaceName(rootPath, agentName string) string {
	repoName := filepath.Base(rootPath)
	return repoName + "-" + agentName
}

// createAgentMarker writes the .dojo-agent marker file.
func createAgentMarker(workspacePath, rootPath, name string) error {
	marker := AgentMarker{
		RootWorkspace: rootPath,
		Name:          name,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(workspacePath, agentMarkerFile), data, 0644)
}

// setupClaudeSymlink creates .claude symlink pointing to root's .claude.
func setupClaudeSymlink(workspacePath, rootPath string) error {
	rootClaude := filepath.Join(rootPath, ".claude")
	agentClaude := filepath.Join(workspacePath, ".claude")

	// Only create if root has .claude directory
	if _, err := os.Stat(rootClaude); os.IsNotExist(err) {
		return nil // No .claude to symlink
	}

	// Remove existing .claude if present (jj workspace add copies it)
	if _, err := os.Lstat(agentClaude); err == nil {
		if err := os.RemoveAll(agentClaude); err != nil {
			return fmt.Errorf("failed to remove existing .claude: %w", err)
		}
	}

	// Create relative symlink (more portable)
	relPath, err := filepath.Rel(workspacePath, rootClaude)
	if err != nil {
		return err
	}

	return os.Symlink(relPath, agentClaude)
}

// checkParentWritable verifies we can write to the parent directory.
func checkParentWritable(rootPath string) error {
	parentDir := filepath.Dir(rootPath)
	testFile := filepath.Join(parentDir, ".dojo-write-test")

	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return fmt.Errorf("parent directory %s is not writable: %w", parentDir, err)
	}
	os.Remove(testFile)
	return nil
}

// hasUncommittedChanges checks if workspace has uncommitted changes.
func hasUncommittedChanges(ctx context.Context, client *jj.Client, workspacePath string) bool {
	output, err := client.StatusFromDir(ctx, workspacePath)
	if err != nil {
		return false // Assume no changes on error
	}
	// jj status shows "Working copy changes:" if there are changes
	return strings.Contains(output, "Working copy changes:")
}

func runAgent(name string) {
	ctx := context.Background()
	client := jj.NewClient()

	// 1. Find root workspace (handles nesting case)
	root, err := findRootWorkspace(ctx, client)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: not in a jj repository\n")
		os.Exit(1)
	}

	// 2. Check parent directory is writable
	if err := checkParentWritable(root); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// 3. Compute sibling workspace path
	workspacePath := computeAgentPath(root, name)
	shimPath := filepath.Join(workspacePath, shimDir)
	jjWorkspaceName := computeJJWorkspaceName(root, name)

	// 4. Create workspace via jj workspace add (with relative path from parent dir)
	// jj workspace add expects a path relative to current dir, so we use the full path
	if err := client.WorkspaceAddFromDir(ctx, root, workspacePath, ""); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			fmt.Fprintf(os.Stderr, "Error: workspace '%s' already exists\n", name)
			fmt.Fprintf(os.Stderr, "Use 'dojo list' to see existing workspaces\n")
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Error creating workspace: %v\n", err)
		os.Exit(1)
	}

	// 5. Create .dojo-agent marker file
	if err := createAgentMarker(workspacePath, root, name); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating agent marker: %v\n", err)
		cleanup(ctx, client, jjWorkspaceName, workspacePath, root)
		os.Exit(1)
	}

	// 6. Create .claude symlink for permission inheritance
	if err := setupClaudeSymlink(workspacePath, root); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating .claude symlink: %v\n", err)
		cleanup(ctx, client, jjWorkspaceName, workspacePath, root)
		os.Exit(1)
	}

	// 7. Create .git marker file (scopes Claude to workspace)
	gitMarker := filepath.Join(workspacePath, ".git")
	if err := os.WriteFile(gitMarker, []byte{}, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating .git marker: %v\n", err)
		cleanup(ctx, client, jjWorkspaceName, workspacePath, root)
		os.Exit(1)
	}

	// 8. Create git shim
	if err := os.MkdirAll(shimPath, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating shim directory: %v\n", err)
		cleanup(ctx, client, jjWorkspaceName, workspacePath, root)
		os.Exit(1)
	}

	shimScript := filepath.Join(shimPath, "git")
	shimContent := `#!/bin/sh
echo "git disabled for agents; use jj" >&2
exit 1
`
	if err := os.WriteFile(shimScript, []byte(shimContent), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating git shim: %v\n", err)
		cleanup(ctx, client, jjWorkspaceName, workspacePath, root)
		os.Exit(1)
	}

	// 9. Build env with shim in PATH
	env := os.Environ()
	newPath := shimPath + ":" + os.Getenv("PATH")
	for i, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			env[i] = "PATH=" + newPath
			break
		}
	}

	// 10. Fork claude with Stdin/Stdout/Stderr passthrough
	cmd := exec.Command("claude")
	cmd.Dir = workspacePath
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// Claude exited with error - still prompt for cleanup
		if exitErr, ok := err.(*exec.ExitError); ok {
			fmt.Fprintf(os.Stderr, "\nClaude exited with code %d\n", exitErr.ExitCode())
		} else {
			fmt.Fprintf(os.Stderr, "\nError running claude: %v\n", err)
		}
	}

	// 11. Check for uncommitted changes and warn
	if hasUncommittedChanges(ctx, client, workspacePath) {
		fmt.Println("\nWarning: This workspace has uncommitted changes!")
	}

	// 12. Prompt for cleanup
	fmt.Print("\nKeep workspace for inspection? [y/N] ")
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))

	// 13. If no: cleanup
	if answer != "y" && answer != "yes" {
		cleanup(ctx, client, jjWorkspaceName, workspacePath, root)
		fmt.Printf("Workspace '%s' removed\n", name)
	} else {
		fmt.Printf("Workspace kept at: %s\n", workspacePath)
	}
}

func cleanup(ctx context.Context, client *jj.Client, jjWorkspaceName, workspacePath, rootPath string) {
	// Remove .git marker first so jj can work properly
	os.Remove(filepath.Join(workspacePath, ".git"))

	// Remove .claude symlink
	os.Remove(filepath.Join(workspacePath, ".claude"))

	// Remove .dojo-agent marker
	os.Remove(filepath.Join(workspacePath, agentMarkerFile))

	// Forget workspace in jj (run from root workspace context)
	if err := client.WorkspaceForgetFromDir(ctx, rootPath, jjWorkspaceName); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to forget workspace: %v\n", err)
	}

	// Remove directory
	if err := os.RemoveAll(workspacePath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to remove workspace directory: %v\n", err)
	}
}

func listWorkspaces() {
	ctx := context.Background()
	client := jj.NewClient()

	// Find root workspace
	root, err := findRootWorkspace(ctx, client)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: not in a jj repository\n")
		os.Exit(1)
	}

	// Use jj workspace list for accurate info
	workspaces, err := client.WorkspaceListFromDir(ctx, root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing workspaces: %v\n", err)
		os.Exit(1)
	}

	repoName := filepath.Base(root)
	prefix := repoName + "-"

	var found bool
	for _, ws := range workspaces {
		// Skip default workspace
		if ws.Name == "default" {
			continue
		}

		// Check if this is a dojo agent workspace (has our prefix)
		if strings.HasPrefix(ws.Name, prefix) {
			agentName := strings.TrimPrefix(ws.Name, prefix)

			// Verify it has the .dojo-agent marker
			agentPath := filepath.Join(filepath.Dir(root), ws.Name)
			markerPath := filepath.Join(agentPath, agentMarkerFile)
			if _, err := os.Stat(markerPath); err == nil {
				// Format: name: change-id commit-id summary
				fmt.Printf("%s: %s %s %s\n", agentName, ws.ChangeID, ws.CommitID, ws.Summary)
				found = true
			}
		}
	}
	if !found {
		fmt.Println("No workspaces")
	}
}
