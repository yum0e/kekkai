package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

const pidFileSuffix = ".pid"

// WritePIDFile writes a PID file for the given agent.
func WritePIDFile(dir, agentName string, pid int) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create PID directory: %w", err)
	}

	path := filepath.Join(dir, agentName+pidFileSuffix)
	content := strconv.Itoa(pid)

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	return nil
}

// ReadPIDFile reads a PID file for the given agent.
func ReadPIDFile(dir, agentName string) (int, error) {
	path := filepath.Join(dir, agentName+pidFileSuffix)

	content, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(content)))
	if err != nil {
		return 0, fmt.Errorf("invalid PID file content: %w", err)
	}

	return pid, nil
}

// RemovePIDFile removes a PID file for the given agent.
func RemovePIDFile(dir, agentName string) error {
	path := filepath.Join(dir, agentName+pidFileSuffix)
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// ListPIDFiles returns all agent names that have PID files.
func ListPIDFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, pidFileSuffix) {
			agentName := strings.TrimSuffix(name, pidFileSuffix)
			names = append(names, agentName)
		}
	}

	return names, nil
}

// IsProcessRunning checks if a process with the given PID is still running.
func IsProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Send signal 0 to check if process exists
	err = process.Signal(syscall.Signal(0))
	return err == nil
}
