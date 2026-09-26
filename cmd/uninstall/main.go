//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

const hiddenFlag = 0x08000000

func hidden(name string, args ...string) *exec.Cmd {
	c := exec.Command(name, args...)
	c.SysProcAttr = &syscall.SysProcAttr{CreationFlags: hiddenFlag, HideWindow: true}
	return c
}
func ask() int {
	u := syscall.NewLazyDLL("user32.dll")
	p := u.NewProc("MessageBoxW")
	text, _ := syscall.UTF16PtrFromString("Uninstall TallyBridge?\n\nYes = remove the app but KEEP pairing/settings/saved data.\nNo = remove the app AND all TallyBridge local data.\nCancel = do nothing.")
	title, _ := syscall.UTF16PtrFromString("TallyBridge Uninstall")
	r, _, _ := p.Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0x23|0x30)
	return int(r)
}
func main() {
	r := ask()
	if r == 2 {
		return
	}
	_ = hidden("reg.exe", "delete", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", "TallyBridge", "/f").Run()
	removeData := r == 7
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base, _ = os.UserHomeDir()
	}
	dir := filepath.Join(base, "TallyBridge")
	for _, n := range []string{"TallyBridge.exe", "TallyBridge Auto.exe", "TallyBridge Live.exe"} {
		_ = hidden("taskkill.exe", "/F", "/IM", n).Run()
	}
	for _, p := range []string{filepath.Join(os.Getenv("USERPROFILE"), "Desktop", "TallyBridge.lnk"), filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs", "TallyBridge.lnk"), filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs", "TallyBridge Uninstall.lnk")} {
		_ = os.Remove(p)
	}
	for _, n := range []string{"TallyBridge.exe", "TallyBridge.ico"} {
		_ = os.Remove(filepath.Join(dir, n))
	}
	self, _ := os.Executable()
	if removeData {
		cmd := fmt.Sprintf(`ping 127.0.0.1 -n 2 >nul & rmdir /s /q "%s"`, strings.ReplaceAll(dir, `"`, `""`))
		_ = hidden("cmd.exe", "/C", cmd).Start()
	} else {
		cmd := fmt.Sprintf(`ping 127.0.0.1 -n 2 >nul & del /f /q "%s"`, strings.ReplaceAll(self, `"`, `""`))
		_ = hidden("cmd.exe", "/C", cmd).Start()
	}
}
