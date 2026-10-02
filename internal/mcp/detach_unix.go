//go:build !windows

package mcp

import (
	"os/exec"
	"syscall"
)

// startDetached puts the agent in its own session: no controlling terminal,
// and signals sent to the Claude session's process group don't reach it.
func startDetached(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd.Start()
}
