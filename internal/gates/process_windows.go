//go:build windows

package gates

import (
	"fmt"
	"os/exec"
	"strconv"
)

func configureProcessTree(command *exec.Cmd) {
	command.Cancel = func() error {
		if command.Process == nil {
			return nil
		}
		// taskkill /T recursively terminates descendants of the shell process.
		// Keep the OS operation out of the gate's canceled context.
		err := exec.Command("taskkill.exe", "/PID", strconv.Itoa(command.Process.Pid), "/T", "/F").Run()
		if err != nil {
			return fmt.Errorf("terminate gate process tree: %w", err)
		}
		return nil
	}
}
