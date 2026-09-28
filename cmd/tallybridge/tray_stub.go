//go:build !windows

package main

func runSystemTray(adminURL, diagnosticsURL, iconPath string, status func() string) error {
	select {}
}
