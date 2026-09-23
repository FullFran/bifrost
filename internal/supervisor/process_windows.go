//go:build windows

package supervisor

import (
	"os"
	"os/exec"
	"strconv"
)

func setSysProcAttr(cmd *exec.Cmd) {
	// Windows process detached attribute handled if necessary
}

func killProcessGroup(pid int) {
	if pid > 0 {
		_ = exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid)).Run()
	}
}

func isProcessAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	return err == nil && p != nil
}
