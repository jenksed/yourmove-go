//go:build windows

package playwright

import "os/exec"

// isolateProcessGroup has no group semantics on Windows; the bridge close
// path still terminates the child process directly.
func isolateProcessGroup(cmd *exec.Cmd) {}

// killProcessGroup falls back to terminating the bridge child directly.
func killProcessGroup(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
