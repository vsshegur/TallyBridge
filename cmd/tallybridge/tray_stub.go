//go:build !windows

package main

func runSystemTray(adminURL, diagnosticsURL, iconPath string, status func() string, level func() string) error {
	select {}
}
