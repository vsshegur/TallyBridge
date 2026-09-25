//go:build !windows

package app

import (
	"context"
	"os/exec"
)

func pdfCommand(ctx context.Context, path string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, path, args...)
}
