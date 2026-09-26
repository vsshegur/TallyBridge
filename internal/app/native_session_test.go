package app

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"testing"
	"time"
)

func TestApprovedSessionSurvivesGoogleExpiryButRequiresDeviceProof(t *testing.T) {
	s, k, p, pub := googleFixture(t)
	s.native.Devices = []NativeDevice{{ID: "phone", PublicKey: pub, Email: s.native.Config.AllowedEmail, Approved: true, Created: time.Now()}}
	token := googleToken(t, s, k, pub, nil)
	w := serve(s.NativeHandler(), googleRequest(t, "GET", "/api/v2/native/status", "", token, "phone", pub, p))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var result struct {
		SessionToken string `json:"sessionToken"`
	}
	json.Unmarshal(w.Body.Bytes(), &result)
	if len(result.SessionToken) != 64 {
		t.Fatal("session missing")
	}
	request := func(key *ecdsa.PrivateKey) int {
		r := nativeRequest(t, "GET", "/api/v1/mobile/auth-check", "", "", "phone", key)
		r.Header.Set("Authorization", "TallyBridge "+result.SessionToken)
		return serve(s.NativeHandler(), r).Code
	}
	// No Google token is needed after approval, even after the original has expired.
	s.native.Devices[0].LastSeen = time.Now().Add(-3 * time.Hour)
	if code := request(p); code != 200 {
		t.Fatal("session did not survive idle", code)
	}
	other, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if code := request(other); code != 403 {
		t.Fatal("stolen session without private key accepted", code)
	}
	r := nativeRequest(t, "GET", "/api/v1/mobile/auth-check", "", "", "phone", p)
	r.Header.Set("Authorization", "TallyBridge "+result.SessionToken)
	if w := serve(s.NativeHandler(), r); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w := serve(s.NativeHandler(), r); w.Code != 403 {
		t.Fatal("replay accepted", w.Code)
	}
	s.native.Devices[0].LastSeen = time.Now().Add(-31 * 24 * time.Hour)
	if code := request(p); code != 401 {
		t.Fatal("inactive expired session accepted", code)
	}
	s.native.Devices[0].LastSeen = time.Now()
	s.native.Devices[0].Revoked = true
	if code := request(p); code != 401 {
		t.Fatal("revoked session accepted", code)
	}
	s.native.Devices[0].Revoked = false
	s.native.Config.AllowedEmail = "newowner@gmail.com"
	if code := request(p); code != 401 {
		t.Fatal("changed owner accepted", code)
	}
	s.native.Config.AllowedEmail = "owner@gmail.com"
	s.native.Devices[0].Approved = false
	if code := request(p); code != 401 {
		t.Fatal("unapproved phone accepted", code)
	}
}
