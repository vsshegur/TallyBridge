//go:build windows

package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

const hiddenFlag = 0x08000000

func hiddenCmd(name string, args ...string) *exec.Cmd {
	c := exec.Command(name, args...)
	c.SysProcAttr = &syscall.SysProcAttr{CreationFlags: hiddenFlag, HideWindow: true}
	return c
}
func tailscalePath() string {
	cands := []string{}
	if p, e := exec.LookPath("tailscale.exe"); e == nil {
		cands = append(cands, p)
	}
	for _, env := range []string{"ProgramW6432", "ProgramFiles", "ProgramFiles(x86)", "LOCALAPPDATA"} {
		if v := os.Getenv(env); v != "" {
			cands = append(cands, filepath.Join(v, "Tailscale", "tailscale.exe"))
		}
	}
	// Some normal Tailscale Windows installations do not expose tailscale.exe
	// through PATH. Ask the existing Windows service where tailscaled.exe lives
	// and look for the CLI beside it. The command is hidden, so this cannot flash
	// a console window during normal GUI use.
	if out, e := hiddenCmd("sc.exe", "qc", "Tailscale").CombinedOutput(); e == nil {
		if svc := serviceExecutablePath(string(out)); svc != "" {
			cands = append(cands, filepath.Join(filepath.Dir(svc), "tailscale.exe"))
		}
	}
	seen := map[string]bool{}
	for _, p := range cands {
		p = strings.TrimSpace(p)
		if p == "" || seen[strings.ToLower(p)] {
			continue
		}
		seen[strings.ToLower(p)] = true
		if st, e := os.Stat(p); e == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

func serviceExecutablePath(out string) string {
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(strings.ToUpper(line), "BINARY_PATH_NAME") {
			continue
		}
		i := strings.Index(line, ":")
		if i < 0 {
			continue
		}
		v := strings.TrimSpace(line[i+1:])
		if strings.HasPrefix(v, `"`) {
			v = strings.TrimPrefix(v, `"`)
			if j := strings.Index(v, `"`); j >= 0 {
				v = v[:j]
			}
		} else if j := strings.Index(strings.ToLower(v), ".exe"); j >= 0 {
			v = v[:j+4]
		}
		return strings.TrimSpace(v)
	}
	return ""
}

type tsStatus struct {
	Self struct {
		Online  bool   `json:"Online"`
		DNSName string `json:"DNSName"`
	} `json:"Self"`
	BackendState string `json:"BackendState"`
}

func checkTailscale(port int) (bool, string, string, error) {
	p := tailscalePath()
	if p == "" {
		return false, "Tailscale CLI not found", "", errors.New("Tailscale is installed but its CLI could not be located")
	}
	out, e := hiddenCmd(p, "status", "--json").Output()
	if e != nil {
		return false, "Tailscale status failed", "", e
	}
	var x tsStatus
	if json.Unmarshal(out, &x) != nil {
		return false, "Tailscale returned invalid status", "", errors.New("invalid Tailscale status")
	}
	name := strings.TrimSuffix(x.Self.DNSName, ".")
	on := x.Self.Online || strings.EqualFold(x.BackendState, "Running")
	u := ""
	if name != "" {
		u = "https://" + name + "/phone"
	}
	return on, x.BackendState, u, nil
}
func configureTailscale(port int) (bool, string, string, error) {
	p := tailscalePath()
	if p == "" {
		return false, "Tailscale CLI not found", "", errors.New("Tailscale CLI not found")
	}
	target := fmt.Sprintf("http://127.0.0.1:%d", port)
	cmd := hiddenCmd(p, "serve", "--bg", "--yes", target)
	if out, e := cmd.CombinedOutput(); e != nil {
		return false, strings.TrimSpace(string(out)), "", e
	}
	return checkTailscale(port)
}
func openDesktop(url string) error {
	cands := []string{filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft", "Edge", "Application", "msedge.exe"), filepath.Join(os.Getenv("ProgramFiles"), "Microsoft", "Edge", "Application", "msedge.exe")}
	if p, e := exec.LookPath("msedge.exe"); e == nil {
		cands = append([]string{p}, cands...)
	}
	for _, p := range cands {
		if p == "" {
			continue
		}
		if _, e := os.Stat(p); e == nil {
			cmd := hiddenCmd(p, "--app="+url, "--start-maximized")
			return cmd.Start()
		}
	}
	return hiddenCmd("rundll32.exe", "url.dll,FileProtocolHandler", url).Start()
}
