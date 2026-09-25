package app

import (
	"crypto"
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

func nativeFixture(t *testing.T) (*Server, *rsa.PrivateKey) {
	t.Helper()
	s, e := NewServer(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	s.native.Config = CloudConfig{PublicURL: "https://tally.example.com", TeamDomain: "demo.cloudflareaccess.com", Audience: "0123456789abcdef0123456789abcdef", AllowedEmail: "owner@gmail.com"}
	s.native.keys = map[string]*rsa.PublicKey{"test-key": &key.PublicKey}
	s.native.keysTeam = s.native.Config.TeamDomain
	s.native.keysUntil = time.Now().Add(time.Hour)
	return s, key
}
func accessToken(t *testing.T, s *Server, key *rsa.PrivateKey, edit func(map[string]any)) string {
	t.Helper()
	claims := map[string]any{"iss": "https://" + s.native.Config.TeamDomain, "aud": []string{s.native.Config.Audience}, "exp": time.Now().Add(time.Hour).Unix(), "nbf": time.Now().Add(-time.Minute).Unix(), "email": "owner@gmail.com", "type": "app"}
	if edit != nil {
		edit(claims)
	}
	h := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","kid":"test-key"}`))
	b, _ := json.Marshal(claims)
	msg := h + "." + base64.RawURLEncoding.EncodeToString(b)
	hash := sha256.Sum256([]byte(msg))
	sig, e := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])
	if e != nil {
		t.Fatal(e)
	}
	return msg + "." + base64.RawURLEncoding.EncodeToString(sig)
}
func nativeRequest(t *testing.T, method, path, body, token, id string, key *ecdsa.PrivateKey) *http.Request {
	t.Helper()
	r := httptest.NewRequest(method, "http://127.0.0.1:8766"+path, strings.NewReader(body))
	r.Header.Set("Cf-Access-Jwt-Assertion", token)
	if key != nil {
		date := time.Now().UTC().Format(time.RFC3339)
		nonce := randomHex(24)
		bh := sha256.Sum256([]byte(body))
		msg := strings.Join([]string{method, path, date, nonce, hex.EncodeToString(bh[:])}, "\n")
		h := sha256.Sum256([]byte(msg))
		sig, e := ecdsa.SignASN1(rand.Reader, key, h[:])
		if e != nil {
			t.Fatal(e)
		}
		r.Header.Set("X-TB-Device", id)
		r.Header.Set("X-TB-Date", date)
		r.Header.Set("X-TB-Nonce", nonce)
		r.Header.Set("X-TB-Signature", base64.StdEncoding.EncodeToString(sig))
	}
	return r
}
func serve(h http.Handler, r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestNativeIdentityFailsClosed(t *testing.T) {
	s, key := nativeFixture(t)
	h := s.NativeHandler()
	for _, mutate := range []func(map[string]any){func(c map[string]any) { c["email"] = "stranger@gmail.com" }, func(c map[string]any) { c["aud"] = []string{"wrong"} }, func(c map[string]any) { c["iss"] = "https://other.cloudflareaccess.com" }, func(c map[string]any) { c["exp"] = time.Now().Add(-time.Second).Unix() }, func(c map[string]any) { c["type"] = "service" }, func(c map[string]any) { c["nbf"] = time.Now().Add(time.Hour).Unix() }} {
		token := accessToken(t, s, key, mutate)
		w := serve(h, nativeRequest(t, "GET", "/api/v1/mobile/bootstrap", "", token, "", nil))
		if w.Code != 401 {
			t.Fatalf("invalid identity accepted: %d %s", w.Code, w.Body.String())
		}
	}
	w := serve(h, nativeRequest(t, "GET", "/api/v1/mobile/bootstrap", "", "", "", nil))
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
}
func TestNativeApprovalSignatureReplayAndRevocation(t *testing.T) {
	s, key := nativeFixture(t)
	token := accessToken(t, s, key, nil)
	phone, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	pub, _ := x509.MarshalPKIXPublicKey(&phone.PublicKey)
	code, _ := s.store.GeneratePairCode()
	body, _ := json.Marshal(map[string]string{"code": code, "name": "My Android", "publicKey": base64.StdEncoding.EncodeToString(pub)})
	h := s.NativeHandler()
	w := serve(h, nativeRequest(t, "POST", "/api/v2/native/pair", string(body), token, "", nil))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var response map[string]any
	json.Unmarshal(w.Body.Bytes(), &response)
	id := response["deviceId"].(string)
	w = serve(h, nativeRequest(t, "GET", "/api/v2/native/status", "", token, id, phone))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"approved":false`) {
		t.Fatal(w.Code, w.Body.String())
	}
	w = serve(h, nativeRequest(t, "GET", "/api/v1/mobile/auth-check", "", token, id, phone))
	if w.Code != 403 {
		t.Fatal("pending phone accessed data", w.Code)
	}
	approve := httptest.NewRequest("POST", "http://127.0.0.1:8765/api/v1/admin/native-device", strings.NewReader(`{"id":"`+id+`","action":"approve","fingerprint":"`+response["fingerprint"].(string)+`"}`))
	approve.RemoteAddr = "127.0.0.1:50505"
	approve.Header.Set("X-TB-Admin", "1")
	w = serve(s.DesktopHandler(), approve)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	req := nativeRequest(t, "GET", "/api/v1/mobile/auth-check", "", token, id, phone)
	w = serve(h, req)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = serve(h, req)
	if w.Code != 403 || !strings.Contains(w.Body.String(), "replayed") {
		t.Fatal("replay accepted", w.Code, w.Body.String())
	}
	wrong, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	w = serve(h, nativeRequest(t, "GET", "/api/v1/mobile/auth-check", "", token, id, wrong))
	if w.Code != 403 {
		t.Fatal("wrong phone key accepted")
	}
	req = nativeRequest(t, "GET", "/api/v1/mobile/auth-check", "", token, id, phone)
	req.URL.RawQuery = "changed=1"
	if w = serve(h, req); w.Code != 403 {
		t.Fatal("tampered URI accepted")
	}
	s.native.mu.Lock()
	s.native.Devices[0].Revoked = true
	s.native.saveLocked()
	s.native.mu.Unlock()
	w = serve(h, nativeRequest(t, "GET", "/api/v1/mobile/bootstrap", "", token, id, phone))
	if w.Code != 403 {
		t.Fatal("revoked phone allowed")
	}
	reloaded, e := newNative(s.native.dir)
	if e != nil || !reloaded.Devices[0].Revoked {
		t.Fatal("revocation was not persisted", e)
	}
}
func TestNativeListenerNeverExposesDesktopOrLegacyPhone(t *testing.T) {
	s, key := nativeFixture(t)
	token := accessToken(t, s, key, nil)
	for _, path := range []string{"/", "/admin", "/phone", "/assets/admin.js", "/api/v1/admin/snapshot", "/api/v1/admin/paircode", "/api/v1/pair", "/api/v1/public/live", "/api/v1/mobile/../admin/snapshot", "//api/v1/admin/snapshot"} {
		r := nativeRequest(t, "GET", path, "", token, "", nil)
		r.Host = "localhost:8765"
		r.RemoteAddr = "127.0.0.1:12345"
		w := serve(s.NativeHandler(), r)
		if w.Code != 404 {
			t.Fatalf("%s leaked with spoofed localhost host: %d", path, w.Code)
		}
	}
	for _, path := range []string{"/phone", "/api/v1/mobile/bootstrap", "/api/v1/pair"} {
		r := httptest.NewRequest("GET", "http://localhost:8765"+path, nil)
		r.RemoteAddr = "127.0.0.1:12345"
		if w := serve(s.DesktopHandler(), r); w.Code != 404 {
			t.Fatal("desktop exposes phone", path, w.Code)
		}
	}
}
func TestDesktopRejectsHostSpoofAndCrossSite(t *testing.T) {
	s, _ := nativeFixture(t)
	for _, remote := range []string{"192.168.1.20:5000", "10.0.0.2:5000"} {
		r := httptest.NewRequest("GET", "http://127.0.0.1:8765/api/v1/admin/snapshot", nil)
		r.RemoteAddr = remote
		if w := serve(s.DesktopHandler(), r); w.Code != 403 {
			t.Fatal("spoofed host accepted")
		}
	}
	for _, header := range []string{"Origin", "Sec-Fetch-Site", "X-Forwarded-For", "Cf-Access-Jwt-Assertion"} {
		r := httptest.NewRequest("GET", "http://localhost:8765/api/v1/admin/snapshot", nil)
		r.RemoteAddr = "127.0.0.1:9999"
		v := "evil"
		if header == "Sec-Fetch-Site" {
			v = "cross-site"
		}
		r.Header.Set(header, v)
		if w := serve(s.DesktopHandler(), r); w.Code != 403 {
			t.Fatal("cross site/proxy accepted", header, w.Code)
		}
	}
}
func TestNativeConfigAndPairRateLimit(t *testing.T) {
	s, key := nativeFixture(t)
	valid := s.native.Config
	for _, change := range []func(*CloudConfig){func(c *CloudConfig) { c.PublicURL = "http://example.com" }, func(c *CloudConfig) { c.PublicURL = "https://example.com/admin" }, func(c *CloudConfig) { c.TeamDomain = "attacker.test" }, func(c *CloudConfig) { c.AllowedEmail = "a@gmail.com,b@gmail.com" }} {
		c := valid
		change(&c)
		if validateCloudConfig(c) == nil {
			t.Fatal("invalid config accepted", c)
		}
	}
	token := accessToken(t, s, key, nil)
	phone, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	pub, _ := x509.MarshalPKIXPublicKey(&phone.PublicKey)
	b, _ := json.Marshal(map[string]string{"code": "wrong", "publicKey": base64.StdEncoding.EncodeToString(pub)})
	for i := 0; i < 6; i++ {
		w := serve(s.NativeHandler(), nativeRequest(t, "POST", "/api/v2/native/pair", string(b), token, "", nil))
		expected := 403
		if i == 5 {
			expected = 429
		}
		if w.Code != expected {
			t.Fatal(i, w.Code, w.Body.String())
		}
	}
}

func TestIndependentOwnersCannotAccessEachOthersBridge(t *testing.T) {
	a, key := nativeFixture(t)
	b, _ := nativeFixture(t)
	// Even the same Google email and same simulated Access application cannot make
	// an approved device on PC A an approved device on independently installed PC B.
	b.native.keys = a.native.keys
	token := accessToken(t, a, key, nil)
	phone, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	pub, _ := x509.MarshalPKIXPublicKey(&phone.PublicKey)
	a.native.Devices = []NativeDevice{{ID: "phone-A", Email: a.native.Config.AllowedEmail, PublicKey: base64.StdEncoding.EncodeToString(pub), Approved: true, Created: time.Now()}}
	if w := serve(a.NativeHandler(), nativeRequest(t, "GET", "/api/v1/mobile/auth-check", "", token, "phone-A", phone)); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := serve(b.NativeHandler(), nativeRequest(t, "GET", "/api/v1/mobile/auth-check", "", token, "phone-A", phone)); w.Code != 403 {
		t.Fatal("PC A phone accessed PC B", w.Code)
	}
	// Owner changes invalidate prior phone approval; changing back cannot reactivate it.
	config := a.native.Config
	config.AllowedEmail = "newowner@gmail.com"
	body, _ := json.Marshal(config)
	r := httptest.NewRequest("POST", "http://localhost:8765/api/v1/admin/cloudflare", strings.NewReader(string(body)))
	r.RemoteAddr = "127.0.0.1:2222"
	r.Header.Set("X-TB-Admin", "1")
	if w := serve(a.DesktopHandler(), r); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if !a.native.Devices[0].Revoked {
		t.Fatal("old owner's phone approval survived owner change")
	}
	if w := serve(a.NativeHandler(), nativeRequest(t, "GET", "/api/v2/native/identity", "", token, "", nil)); w.Code != 401 {
		t.Fatal("old owner identity accepted")
	}
}
