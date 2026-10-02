//go:build !windows

package gates

import (
	"os/exec"
	"syscall"
)

func configureProcessTree(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		if command.Process == nil {
			return nil
		}
		// Every process started by the gate shell inherits its isolated group.
		return syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	}
}
