//go:build windows

package main

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

//go:embed payload/TallyBridge.exe payload/TallyBridge_Uninstall.exe payload/TallyBridge.ico
var payload embed.FS

const hiddenFlag = 0x08000000

func hidden(name string, args ...string) *exec.Cmd {
	c := exec.Command(name, args...)
	c.SysProcAttr = &syscall.SysProcAttr{CreationFlags: hiddenFlag, HideWindow: true}
	return c
}
func psQuote(s string) string { return strings.ReplaceAll(s, "'", "''") }
func message(title, text string) {
	u := syscall.NewLazyDLL("user32.dll")
	p := u.NewProc("MessageBoxW")
	t, _ := syscall.UTF16PtrFromString(text)
	h, _ := syscall.UTF16PtrFromString(title)
	p.Call(0, uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(h)), 0x40)
}
func main() {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base, _ = os.UserHomeDir()
	}
	dir := filepath.Join(base, "TallyBridge")
	_ = os.MkdirAll(dir, 0700)
	for _, n := range []string{"TallyBridge.exe", "TallyBridge Auto.exe", "TallyBridge Live.exe"} {
		_ = hidden("taskkill.exe", "/F", "/IM", n).Run()
	}
	time.Sleep(350 * time.Millisecond)
	files := map[string]string{"payload/TallyBridge.exe": "TallyBridge.exe", "payload/TallyBridge_Uninstall.exe": "TallyBridge_Uninstall.exe", "payload/TallyBridge.ico": "TallyBridge.ico"}
	for src, dst := range files {
		b, e := payload.ReadFile(src)
		if e != nil {
			message("TallyBridge Setup", "Installer payload is incomplete: "+e.Error())
			return
		}
		p := filepath.Join(dir, dst)
		tmp := p + ".new"
		if e = os.WriteFile(tmp, b, 0700); e != nil {
			message("TallyBridge Setup", "Could not install: "+e.Error())
			return
		}
		_ = os.Remove(p)
		if e = os.Rename(tmp, p); e != nil {
			message("TallyBridge Setup", "Could not replace application: "+e.Error())
			return
		}
	}
	exe := filepath.Join(dir, "TallyBridge.exe")
	ico := filepath.Join(dir, "TallyBridge.ico")
	un := filepath.Join(dir, "TallyBridge_Uninstall.exe")
	desktop := filepath.Join(os.Getenv("USERPROFILE"), "Desktop", "TallyBridge.lnk")
	start := filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs", "TallyBridge.lnk")
	for _, pair := range [][2]string{{desktop, exe}, {start, exe}} {
		script := fmt.Sprintf(`$ws=New-Object -ComObject WScript.Shell;$s=$ws.CreateShortcut('%s');$s.TargetPath='%s';$s.WorkingDirectory='%s';$s.IconLocation='%s,0';$s.Description='TallyBridge Hybrid Auto - read-only Tally companion';$s.Save()`, psQuote(pair[0]), psQuote(pair[1]), psQuote(dir), psQuote(ico))
		_ = hidden("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script).Run()
	}
	// Uninstall shortcut
	ustart := filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs", "TallyBridge Uninstall.lnk")
	script := fmt.Sprintf(`$ws=New-Object -ComObject WScript.Shell;$s=$ws.CreateShortcut('%s');$s.TargetPath='%s';$s.WorkingDirectory='%s';$s.IconLocation='%s,0';$s.Save()`, psQuote(ustart), psQuote(un), psQuote(dir), psQuote(ico))
	_ = hidden("powershell.exe", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script).Run()
	cmd := hidden(exe)
	_ = cmd.Start()
	message("TallyBridge Setup", "TallyBridge Native v2.0 PREVIEW has been installed.\n\nYour Tally settings, bank defaults and saved reports were kept. Android phones require new Google sign-in and code approval.\n\nOpen Cloudflare to configure your own connection, then Android Phones to pair. The old browser phone interface is disabled in this build.")
}
