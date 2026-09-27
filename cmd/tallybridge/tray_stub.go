//go:build !windows

package main

func runSystemTray(adminURL, iconPath string) error {
	select {}
}
