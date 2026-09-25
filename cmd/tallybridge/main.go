package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"tallybridge-live/internal/app"
	"time"
)

func main() {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base, _ = os.UserHomeDir()
	}
	base = filepath.Join(base, "TallyBridge")
	srv, e := app.NewServer(base)
	if e != nil {
		return
	}
	cfg := srv.Settings()
	addr := fmt.Sprintf("127.0.0.1:%d", cfg.Port)
	ln, e := net.Listen("tcp", addr)
	if e != nil {
		_ = app.OpenDesktop(fmt.Sprintf("http://127.0.0.1:%d/admin", cfg.Port))
		return
	}
	h := &http.Server{Handler: srv.DesktopHandler(), ReadHeaderTimeout: 5 * time.Second}
	nativeListener, err := net.Listen("tcp", "127.0.0.1:8766")
	if err != nil {
		ln.Close()
		return
	}
	nativeHTTP := &http.Server{Handler: srv.NativeHandler(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32768}
	go nativeHTTP.Serve(nativeListener)
	srv.StartBackground()
	go h.Serve(ln)
	time.Sleep(250 * time.Millisecond)
	_ = app.OpenDesktop(fmt.Sprintf("http://127.0.0.1:%d/admin", cfg.Port))
	select {}
}
