//go:build !windows

package main

import (
	"os/exec"
	"syscall"
)

func configureSetupWorkerProcess(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }
