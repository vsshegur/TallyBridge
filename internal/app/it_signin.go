package app

// Signing in to InvoiceTotally with Google, approved on this PC.
//
// InvoiceTotally uses the same Google Web client ID and owner email that the
// phone app signs in with (Connection settings). TallyBridge sends both with
// its heartbeat. When a new browser signs in, InvoiceTotally shows a six-digit
// code there and asks this PC to approve it — the same idea as approving a new
// phone. Approving happens only on this PC (local admin page), never from the
// phone or the browser itself.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

var itDeviceID = regexp.MustCompile(`^[0-9a-f]{32}$`)

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

// webSignIn is what InvoiceTotally needs to check a Google sign-in, or nil when Google sign-in is not set up here.
func (c *ITConnector) webSignIn() map[string]string {
	if c.s == nil || c.s.native == nil {
		return nil
	}
	nc := c.s.native.config()
	if nc.GoogleClientID == "" || nc.AllowedEmail == "" || validateCloudConfig(nc) != nil {
		return nil
	}
	return map[string]string{"google_client_id": nc.GoogleClientID, "owner_email": nc.AllowedEmail}
}

// webPendingLocked: browsers waiting for approval (every company link reports the same number). Caller holds c.mu.
func (c *ITConnector) webPendingLocked() int {
	n := 0
	for _, l := range c.links {
		if l.server.Status == "ok" && l.webPending > n {
			n = l.webPending
		}
	}
	return n
}

func (c *ITConnector) webPendingCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.webPendingLocked()
}

type itBrowser struct {
	ID        string   `json:"id"`
	Email     string   `json:"email"`
	Name      string   `json:"name"`
	Code      string   `json:"code"`
	Status    string   `json:"status"`
	CreatedAt float64  `json:"created_at"`
	ExpiresAt float64  `json:"expires_at"`
	DecidedAt *float64 `json:"decided_at"`
	DecidedBy string   `json:"decided_by"`
	LastSeen  *float64 `json:"last_seen"`
}

type itBrowsers struct {
	Pending  []itBrowser `json:"pending"`
	Approved []itBrowser `json:"approved"`
}

// cloudCall tries each company token until one is accepted (any of them may manage sign-ins).
func (c *ITConnector) cloudCall(ctx context.Context, method, path string, payload, out any) error {
	cfg := c.config()
	if cfg.ServerURL == "" || len(cfg.Connections) == 0 {
		return errors.New("connect InvoiceTotally on this page first")
	}
	var last error
	for _, conn := range cfg.Connections {
		err := c.call(ctx, cfg.ServerURL, conn.Token, method, path, payload, out, 20*time.Second)
		if err == nil {
			return nil
		}
		last = err
		if !strings.Contains(err.Error(), "Token rejected") {
			break // the web app answered (or is unreachable): another token will not help
		}
	}
	return last
}

func (c *ITConnector) heartbeatNow() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, l := range c.links {
		select {
		case l.beat <- struct{}{}:
		default:
		}
	}
}

func (s *Server) handleITBrowsers(w http.ResponseWriter, r *http.Request) {
	c := s.it
	switch r.Method {
	case http.MethodGet:
		var out itBrowsers
		if err := c.cloudCall(r.Context(), "GET", "/api/bridge/web-devices", nil, &out); err != nil {
			jsonErr(w, 502, errors.New(friendlyNetErr(err)))
			return
		}
		if out.Pending == nil {
			out.Pending = []itBrowser{}
		}
		if out.Approved == nil {
			out.Approved = []itBrowser{}
		}
		jsonOut(w, map[string]any{"pending": out.Pending, "approved": out.Approved, "webSignIn": c.webSignIn() != nil})
	case http.MethodPost:
		var q struct {
			ID     string `json:"id"`
			Action string `json:"action"`
			Name   string `json:"name"`
			Email  string `json:"email"`
		}
		if readJSON(r, &q) != nil || !itDeviceID.MatchString(q.ID) || (q.Action != "approve" && q.Action != "deny" && q.Action != "revoke") {
			jsonErr(w, 400, errors.New("invalid request"))
			return
		}
		by := "TallyBridge on this PC"
		if h, err := os.Hostname(); err == nil && strings.TrimSpace(h) != "" {
			by = "TallyBridge on " + clip(h, 40)
		}
		var res map[string]any
		if err := c.cloudCall(r.Context(), "POST", "/api/bridge/web-devices/"+q.ID, map[string]any{"action": q.Action, "by": by}, &res); err != nil {
			jsonErr(w, 502, errors.New(friendlyNetErr(err)))
			return
		}
		who := strings.TrimSpace(clip(q.Name, 80) + " " + func() string {
			if q.Email == "" {
				return ""
			}
			return "(" + clip(q.Email, 120) + ")"
		}())
		msg := map[string]string{"approve": "Approved InvoiceTotally sign-in", "deny": "Denied InvoiceTotally sign-in", "revoke": "Signed out of InvoiceTotally"}[q.Action]
		if who != "" {
			msg += ": " + who
		}
		c.log(msg, "signin", q.Action != "deny", nil)
		c.heartbeatNow()
		jsonOut(w, map[string]any{"ok": true})
	default:
		jsonErr(w, 405, errors.New("method not allowed"))
	}
}

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}
