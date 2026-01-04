//go:build !windows

package agent

import "os"

// findProcessByPID finds a process by PID on Unix systems.
func findProcessByPID(pid int) (interface{ Kill() error }, error) {
	return os.FindProcess(pid)
}
