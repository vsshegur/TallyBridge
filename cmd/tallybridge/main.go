package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"tallybridge-live/internal/app"
	"time"
)

func main() {
	background := flag.Bool("background", false, "Run without opening desktop setup")
	flag.Parse()

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
	adminURL := fmt.Sprintf("http://127.0.0.1:%d/admin", cfg.Port)
	addr := fmt.Sprintf("127.0.0.1:%d", cfg.Port)

	ln, e := net.Listen("tcp", addr)
	if e != nil {
		// A running tray instance owns the port. Opening the desktop shortcut
		// should simply bring that existing instance's admin UI forward.
		if !*background {
			_ = app.OpenDesktop(adminURL)
		}
		return
	}

	nativeListener, err := net.Listen("tcp", "127.0.0.1:8766")
	if err != nil {
		_ = ln.Close()
		return
	}

	h := &http.Server{
		Handler:           srv.DesktopHandler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	nativeHTTP := &http.Server{
		Handler:           srv.NativeHandler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    32768,
	}

	go nativeHTTP.Serve(nativeListener)
	srv.StartBackground()
	go h.Serve(ln)

	time.Sleep(250 * time.Millisecond)
	if !*background {
		_ = app.OpenDesktop(adminURL)
	}

	// The primary Windows instance lives in the notification area. At Windows
	// sign-in --background keeps the browser closed while the tray icon stays
	// available. Double-click/Open launches the local admin UI; Exit ends the
	// bridge process and both local listeners.
	exe, _ := os.Executable()
	iconPath := filepath.Join(filepath.Dir(exe), "TallyBridge.ico")
	if err := runSystemTray(adminURL, iconPath); err != nil {
		// A tray failure must not kill a working read-only bridge. Keep serving
		// so the desktop shortcut and phone can still reach it.
		select {}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = h.Shutdown(ctx)
	_ = nativeHTTP.Shutdown(ctx)
	_ = ln.Close()
	_ = nativeListener.Close()
}
