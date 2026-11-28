//go:build !windows

package core

import (
	"os/exec"
	"syscall"
)

// prepareProcessGroup puts the command in its own process group and kills the whole group on
// cancel, so a timed-out shell does not leave children running.
func prepareProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

// killProcessGroup SIGKILLs the command's process group (best effort; the group may already
// be gone).
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
