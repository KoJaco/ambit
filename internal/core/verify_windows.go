//go:build windows

package core

import "os/exec"

// prepareProcessGroup is a no-op on Windows: cancel kills the shell process only.
func prepareProcessGroup(cmd *exec.Cmd) {}
