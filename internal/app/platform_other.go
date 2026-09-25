//go:build !windows

package app

import (
	"errors"
	"fmt"
)

func checkTailscale(port int) (bool, string, string, error) {
	return false, "Tailscale check is available on Windows", "", errors.New("Tailscale CLI is Windows-only in this build")
}
func configureTailscale(port int) (bool, string, string, error) {
	return false, "Tailscale configure is available on Windows", "", errors.New("Tailscale CLI is Windows-only in this build")
}
func openDesktop(url string) error { fmt.Println("TallyBridge:", url); return nil }
func hideProcess(any)              {}
