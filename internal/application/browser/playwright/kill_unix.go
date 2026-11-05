//go:build unix

package playwright

import (
	"os/exec"
	"syscall"
)

// isolateProcessGroup starts the bridge as a group leader so closing the
// session can reach the browser even when the bridge itself is already dead.
func isolateProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup SIGKILLs the bridge and every browser it launched.
func killProcessGroup(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		return err
	}
	return syscall.Kill(-pgid, syscall.SIGKILL)
}
