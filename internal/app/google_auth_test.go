package app

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func googleFixture(t *testing.T) (*Server, *rsa.PrivateKey, *ecdsa.PrivateKey, string) {
	s, k := nativeFixture(t)
	s.native.Config.GoogleClientID = "123-owner.apps.googleusercontent.com"
	s.native.keysTeam = "accounts.google.com"
	p, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	raw, _ := x509.MarshalPKIXPublicKey(&p.PublicKey)
	return s, k, p, base64.StdEncoding.EncodeToString(raw)
}
func googleToken(t *testing.T, s *Server, k *rsa.PrivateKey, pub string, edit func(map[string]any)) string {
	raw, _ := base64.StdEncoding.DecodeString(pub)
	hash := sha256.Sum256(raw)
	return accessToken(t, s, k, func(c map[string]any) {
		c["iss"] = "https://accounts.google.com"
		c["aud"] = s.native.Config.GoogleClientID
		c["iat"] = time.Now().Unix()
		c["sub"] = "google-account-123"
		c["email_verified"] = true
		c["nonce"] = hex.EncodeToString(hash[:])
		if edit != nil {
			edit(c)
		}
	})
}
func googleRequest(t *testing.T, method, path, body, token, id, pub string, p *ecdsa.PrivateKey) *http.Request {
	r := nativeRequest(t, method, path, body, "", id, p)
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("X-TB-Public-Key", pub)
	return r
}
func TestDirectGoogleClaimsAndKeyProof(t *testing.T) {
	s, k, p, pub := googleFixture(t)
	h := s.NativeHandler()
	for name, edit := range map[string]func(map[string]any){
		"email":      func(c map[string]any) { c["email"] = "other@gmail.com" },
		"aud":        func(c map[string]any) { c["aud"] = "123-other.apps.googleusercontent.com" },
		"issuer":     func(c map[string]any) { c["iss"] = "https://evil.example" },
		"expired":    func(c map[string]any) { c["exp"] = time.Now().Add(-time.Minute).Unix() },
		"unverified": func(c map[string]any) { c["email_verified"] = false },
		"subject":    func(c map[string]any) { c["sub"] = "" },
		"nonce":      func(c map[string]any) { c["nonce"] = "another-phone" },
		"future":     func(c map[string]any) { c["iat"] = time.Now().Add(time.Hour).Unix() },
	} {
		t.Run(name, func(t *testing.T) {
			token := googleToken(t, s, k, pub, edit)
			w := serve(h, googleRequest(t, "GET", "/api/v2/native/identity", "", token, "", pub, p))
			if w.Code != 401 {
				t.Fatalf("accepted %s: %d", name, w.Code)
			}
		})
	}
	token := googleToken(t, s, k, pub, nil)
	for _, mutation := range []func(*http.Request){func(r *http.Request) { r.Header.Del("X-TB-Signature") }, func(r *http.Request) { r.Header.Set("X-TB-Signature", "AAAA") }, func(r *http.Request) { r.URL.RawQuery = "tampered=1" }, func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+token[:len(token)-8]+"AAAAAAAA") }} {
		r := googleRequest(t, "GET", "/api/v2/native/identity", "", token, "", pub, p)
		mutation(r)
		if w := serve(h, r); w.Code != 401 {
			t.Fatal("invalid proof accepted", w.Code)
		}
	}
	r := googleRequest(t, "GET", "/api/v2/native/identity", "", token, "", pub, p)
	if w := serve(h, r); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := serve(h, r); w.Code != 401 {
		t.Fatal("replay accepted")
	}
	// Cloudflare's header must not bypass direct Google mode.
	r = httptest.NewRequest("GET", "http://localhost/api/v2/native/identity", nil)
	r.Header.Set("Cf-Access-Jwt-Assertion", accessToken(t, s, k, nil))
	if w := serve(h, r); w.Code != 401 {
		t.Fatal("accepted Access identity in direct mode")
	}
}
func TestDirectGooglePairApprovalRevocation(t *testing.T) {
	s, k, p, pub := googleFixture(t)
	h := s.NativeHandler()
	token := googleToken(t, s, k, pub, nil)
	if w := serve(h, googleRequest(t, "GET", "/api/v1/mobile/auth-check", "", token, "", pub, p)); w.Code != 403 {
		t.Fatal("unpaired phone reached reports", w.Code)
	}
	code, _ := s.store.GeneratePairCode()
	other, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	raw, _ := x509.MarshalPKIXPublicKey(&other.PublicKey)
	bad, _ := json.Marshal(map[string]string{"code": code, "name": "phone", "publicKey": base64.StdEncoding.EncodeToString(raw)})
	if w := serve(h, googleRequest(t, "POST", "/api/v2/native/pair", string(bad), token, "", pub, p)); w.Code != 403 {
		t.Fatal("different pairing key accepted")
	}
	body, _ := json.Marshal(map[string]string{"code": code, "name": "phone", "publicKey": pub})
	w := serve(h, googleRequest(t, "POST", "/api/v2/native/pair", string(body), token, "", pub, p))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var d struct {
		ID          string `json:"deviceId"`
		Fingerprint string `json:"fingerprint"`
	}
	json.Unmarshal(w.Body.Bytes(), &d)
	if w := serve(h, googleRequest(t, "GET", "/api/v1/mobile/auth-check", "", token, d.ID, pub, p)); w.Code != 403 {
		t.Fatal("pending phone allowed")
	}
	r := httptest.NewRequest("POST", "http://127.0.0.1:8765/api/v1/admin/native-device", strings.NewReader(`{"id":"`+d.ID+`","action":"approve","fingerprint":"`+d.Fingerprint+`"}`))
	r.RemoteAddr = "127.0.0.1:5050"
	r.Header.Set("X-TB-Admin", "1")
	if w := serve(s.DesktopHandler(), r); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w := serve(h, googleRequest(t, "GET", "/api/v1/mobile/auth-check", "", token, d.ID, pub, p)); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	s.native.mu.Lock()
	s.native.Devices[0].Revoked = true
	s.native.mu.Unlock()
	if w := serve(h, googleRequest(t, "GET", "/api/v1/mobile/auth-check", "", token, d.ID, pub, p)); w.Code != 403 {
		t.Fatal("revoked phone allowed")
	}
}
func TestDirectGoogleConfigAndWorkspace(t *testing.T) {
	s, k, p, pub := googleFixture(t)
	w := serve(s.NativeHandler(), httptest.NewRequest("GET", "http://localhost/api/v2/native/config", nil))
	if w.Code != 200 || strings.Contains(w.Body.String(), "owner@gmail.com") {
		t.Fatal("public config leaked owner or is unavailable")
	}
	s.native.Config.AllowedEmail = "owner@business.example"
	for _, hd := range []string{"", "other.example", "business.example"} {
		tok := googleToken(t, s, k, pub, func(c map[string]any) { c["email"] = "owner@business.example"; c["hd"] = hd })
		w := serve(s.NativeHandler(), googleRequest(t, "GET", "/api/v2/native/identity", "", tok, "", pub, p))
		if (w.Code == 200) != (hd == "business.example") {
			t.Fatal(hd, w.Code, w.Body.String())
		}
	}
	if keyURL("accounts.google.com") != "https://www.googleapis.com/oauth2/v3/certs" {
		t.Fatal("Google key URL")
	}
}
