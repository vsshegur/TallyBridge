//go:build windows

package app

import (
	"context"
	"os/exec"
	"strconv"
	"syscall"
)

func pdfCommand(ctx context.Context, path string, args ...string) *exec.Cmd {
	c := exec.CommandContext(ctx, path, args...)
	c.SysProcAttr = &syscall.SysProcAttr{CreationFlags: hiddenFlag, HideWindow: true}
	c.Cancel = func() error {
		if c.Process == nil {
			return nil
		}
		if e := hiddenCmd("taskkill.exe", "/PID", strconv.Itoa(c.Process.Pid), "/T", "/F").Run(); e != nil {
			return c.Process.Kill()
		}
		return nil
	}
	return c
}
