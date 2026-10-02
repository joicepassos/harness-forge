//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

func configureSetupWorkerProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x00000008 | 0x00000200}
}
