package app

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

// InvoiceTotally signs in with the phone app's Google account and asks this PC to approve each new browser.
func TestITBrowserSignInIsApprovedOnThePC(t *testing.T) {
	s, _, cloud := itServer(t)
	s.native.mu.Lock()
	s.native.Config = CloudConfig{PublicURL: "https://tb.example.com", GoogleClientID: "1234567890-abcdefg.apps.googleusercontent.com", AllowedEmail: "owner@example.com"}
	s.native.mu.Unlock()
	if w := adminPost(t, s, "/api/v1/admin/invoicetotally/save", `{"serverUrl":"`+cloud.srv.URL+`","add":["`+cloud.token+`"]}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	// The heartbeat carries the Google client ID and owner email (nothing secret).
	waitFor(t, "heartbeat with sign-in settings", func() bool {
		cloud.mu.Lock()
		defer cloud.mu.Unlock()
		for _, h := range cloud.heartbeats {
			if a, ok := h["auth"].(map[string]any); ok && a["google_client_id"] == "1234567890-abcdefg.apps.googleusercontent.com" && a["owner_email"] == "owner@example.com" && len(a) == 2 {
				return true
			}
		}
		return false
	})

	// A browser asks; the PC hears about it from the next heartbeat.
	cloud.mu.Lock()
	cloud.webPending = []map[string]any{{"id": strings.Repeat("ab", 16), "email": "owner@example.com", "name": "Chrome on Windows", "code": "482913", "status": "pending"}}
	cloud.mu.Unlock()
	waitFor(t, "pending count", func() bool { return s.it.webPendingCount() == 1 })
	if !strings.Contains(strings.Join(func() []string {
		o := []string{}
		for _, a := range s.it.snapshot()["activity"].([]ITActivity) {
			o = append(o, a.Text)
		}
		return o
	}(), "|"), "1 browser is waiting for approval") {
		t.Fatal("waiting browser not logged")
	}

	get := httptest.NewRequest("GET", "http://127.0.0.1:8765/api/v1/admin/invoicetotally/browsers", nil)
	get.RemoteAddr = "127.0.0.1:50505"
	get.Header.Set("X-TB-Admin", "1")
	w := serve(s.DesktopHandlerWithOrders(), get)
	var list struct {
		Pending   []itBrowser `json:"pending"`
		WebSignIn bool        `json:"webSignIn"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &list) != nil || len(list.Pending) != 1 || list.Pending[0].Code != "482913" || !list.WebSignIn {
		t.Fatal(w.Code, w.Body.String())
	}

	// Bad requests never reach InvoiceTotally.
	for _, body := range []string{`{"id":"x","action":"approve"}`, `{"id":"` + strings.Repeat("ab", 16) + `","action":"delete"}`} {
		if w := adminPost(t, s, "/api/v1/admin/invoicetotally/browsers", body); w.Code != 400 {
			t.Fatal(body, w.Code)
		}
	}
	if w := adminPost(t, s, "/api/v1/admin/invoicetotally/browsers", `{"id":"`+strings.Repeat("ab", 16)+`","action":"approve","name":"Chrome on Windows","email":"owner@example.com"}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	cloud.mu.Lock()
	acts := append([]map[string]any(nil), cloud.webActions...)
	cloud.mu.Unlock()
	if len(acts) != 1 || acts[0]["action"] != "approve" || acts[0]["id"] != strings.Repeat("ab", 16) || !strings.HasPrefix(acts[0]["by"].(string), "TallyBridge on ") {
		t.Fatalf("%v", acts)
	}
	waitFor(t, "pending cleared", func() bool { return s.it.webPendingCount() == 0 })

	// Only this PC can approve: the same request from elsewhere is refused.
	r := httptest.NewRequest("POST", "http://127.0.0.1:8765/api/v1/admin/invoicetotally/browsers", strings.NewReader(`{"id":"`+strings.Repeat("ab", 16)+`","action":"approve"}`))
	r.RemoteAddr = "10.0.0.9:50505"
	r.Header.Set("X-TB-Admin", "1")
	if w := serve(s.DesktopHandlerWithOrders(), r); w.Code == 200 {
		t.Fatal("approval from another machine was accepted")
	}
}
