package app

// The native listener is deliberately separate from desktop setup. Never route
// cloudflared to the setup listener (8765); route it to 127.0.0.1:8766.
import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type nativeContextKey struct{}
type CloudConfig struct {
	PublicURL    string `json:"publicUrl"`
	TeamDomain   string `json:"teamDomain"`
	Audience     string `json:"audience"`
	AllowedEmail string `json:"allowedEmail"`
}
type NativeDevice struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Email       string    `json:"email"`
	PublicKey   string    `json:"publicKey"`
	Fingerprint string    `json:"fingerprint"`
	Created     time.Time `json:"createdAt"`
	LastSeen    time.Time `json:"lastSeen"`
	Approved    bool      `json:"approved"`
	Revoked     bool      `json:"revoked"`
}
type NativeSecurity struct {
	mu             sync.Mutex
	Config         CloudConfig    `json:"config"`
	Devices        []NativeDevice `json:"devices"`
	keys           map[string]*rsa.PublicKey
	keysUntil      time.Time
	keysTeam       string
	lastKeyAttempt time.Time
	attempts       []time.Time
	dir            string
}

func newNative(dir string) (*NativeSecurity, error) {
	n := &NativeSecurity{dir: dir, keys: map[string]*rsa.PublicKey{}}
	b, e := os.ReadFile(filepath.Join(dir, "native-security.json"))
	if e == nil {
		if e = json.Unmarshal(b, n); e != nil {
			return nil, fmt.Errorf("native security settings are damaged: %w", e)
		}
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	return n, nil
}
func (n *NativeSecurity) saveLocked() error {
	b, e := json.MarshalIndent(n, "", "  ")
	if e != nil {
		return e
	}
	return atomicWrite(filepath.Join(n.dir, "native-security.json"), b, 0600)
}
func (n *NativeSecurity) config() CloudConfig { n.mu.Lock(); defer n.mu.Unlock(); return n.Config }
func validateCloudConfig(c CloudConfig) error {
	u, e := url.Parse(c.PublicURL)
	if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || u.Port() != "" {
		return errors.New("enter an HTTPS subdomain without a path or port")
	}
	team := strings.TrimSuffix(c.TeamDomain, ".cloudflareaccess.com")
	if team == c.TeamDomain || team == "" || strings.ContainsAny(team, "/.:@ ") || strings.ContainsAny(c.TeamDomain, "?#\\") {
		return errors.New("team domain must be your-team.cloudflareaccess.com")
	}
	if len(c.Audience) < 16 || len(c.Audience) > 256 {
		return errors.New("copy the Access application audience (AUD) tag")
	}
	a, e := mail.ParseAddress(c.AllowedEmail)
	if e != nil || a.Address != c.AllowedEmail {
		return errors.New("enter exactly one allowed Google email")
	}
	return nil
}
func (n *NativeSecurity) snapshot() any {
	n.mu.Lock()
	defer n.mu.Unlock()
	return struct {
		Config  CloudConfig    `json:"config"`
		Devices []NativeDevice `json:"devices"`
		Port    int            `json:"port"`
	}{n.Config, append([]NativeDevice{}, n.Devices...), 8766}
}
func (s *Server) handleCloudConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		jsonOut(w, s.native.snapshot())
		return
	}
	if r.Method != "POST" {
		jsonErr(w, 405, errors.New("POST required"))
		return
	}
	var c CloudConfig
	if readJSON(r, &c) != nil {
		jsonErr(w, 400, errors.New("invalid settings"))
		return
	}
	c.PublicURL = strings.TrimRight(strings.TrimSpace(c.PublicURL), "/")
	c.TeamDomain = strings.ToLower(strings.TrimSpace(c.TeamDomain))
	c.AllowedEmail = strings.ToLower(strings.TrimSpace(c.AllowedEmail))
	c.Audience = strings.TrimSpace(c.Audience)
	if e := validateCloudConfig(c); e != nil {
		jsonErr(w, 400, e)
		return
	}
	n := s.native
	n.mu.Lock()
	old := n.Config
	oldDevices := append([]NativeDevice(nil), n.Devices...)
	changedOwner := old.AllowedEmail != "" && old != c
	if changedOwner {
		for i := range n.Devices {
			n.Devices[i].Revoked = true
		}
	}
	n.Config = c
	e := n.saveLocked()
	if e != nil {
		n.Config = old
		n.Devices = oldDevices
	}
	n.keysUntil = time.Time{}
	n.mu.Unlock()
	if e != nil {
		jsonErr(w, 500, e)
		return
	}
	jsonOut(w, map[string]any{"ok": true, "phonesRevoked": changedOwner})
}
func (s *Server) handleNativeApproval(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonErr(w, 405, errors.New("POST required"))
		return
	}
	var q struct {
		ID          string `json:"id"`
		Action      string `json:"action"`
		Fingerprint string `json:"fingerprint"`
	}
	if readJSON(r, &q) != nil || (q.Action != "approve" && q.Action != "revoke") {
		jsonErr(w, 400, errors.New("invalid action"))
		return
	}
	n := s.native
	n.mu.Lock()
	defer n.mu.Unlock()
	for i, d := range n.Devices {
		if d.ID == q.ID {
			old := d
			if q.Action == "approve" {
				if d.Revoked || time.Since(d.Created) > 10*time.Minute || q.Fingerprint != d.Fingerprint {
					jsonErr(w, 400, errors.New("pairing expired or fingerprint mismatch; pair again"))
					return
				}
				n.Devices[i].Approved = true
			} else {
				n.Devices[i].Revoked = true
			}
			if e := n.saveLocked(); e != nil {
				n.Devices[i] = old
				jsonErr(w, 500, e)
				return
			}
			jsonOut(w, map[string]any{"ok": true})
			return
		}
	}
	jsonErr(w, 404, errors.New("phone not found"))
}

// NativeHandler exposes an exact allowlist. No admin, assets, legacy pairing or
// web app is reachable, even when a proxy rewrites Host to localhost.
func (s *Server) NativeHandler() http.Handler {
	mobile := s.Handler()
	allowed := map[string]bool{}
	for _, p := range []string{"auth-check", "bootstrap", "status", "sync", "probe", "ledgers", "outstanding", "party-outstanding", "vouchers", "voucher-detail", "voucher-pdf", "ledger-details", "ledger-statement", "ledger-statement-pdf", "ledger-outstanding", "ledger-outstanding-pdf", "group-balances", "group-balances-pdf", "customer360", "customer360-pdf", "today-dispatch", "search-index", "ageing", "banks", "bank-details", "report"} {
		allowed["/api/v1/mobile/"+p] = true
	}
	return securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/native/identity" && r.URL.Path != "/api/v2/native/pair" && r.URL.Path != "/api/v2/native/status" && !allowed[r.URL.Path] {
			http.NotFound(w, r)
			return
		}
		expected := "GET"
		if r.URL.Path == "/api/v2/native/pair" || r.URL.Path == "/api/v1/mobile/sync" || r.URL.Path == "/api/v1/mobile/probe" {
			expected = "POST"
		}
		if r.Method != expected {
			jsonErr(w, 405, errors.New("method not allowed"))
			return
		}
		email, e := s.native.verifyAccess(r.Context(), r.Header.Get("Cf-Access-Jwt-Assertion"))
		if e != nil {
			jsonErr(w, 401, e)
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), nativeContextKey{}, email))
		if r.URL.Path == "/api/v2/native/identity" {
			jsonOut(w, map[string]any{"email": email, "server": s.native.config().PublicURL})
			return
		}
		if r.URL.Path == "/api/v2/native/pair" {
			s.handleNativePair(w, r)
			return
		}
		if r.URL.Path == "/api/v2/native/status" {
			d, e := s.verifyNativeRequest(r, false)
			if e != nil {
				jsonErr(w, 403, e)
				return
			}
			jsonOut(w, map[string]any{"approved": d.Approved, "fingerprint": d.Fingerprint, "name": d.Name, "email": d.Email})
			return
		}
		mobile.ServeHTTP(w, r)
	}))
}
func (n *NativeSecurity) verifyAccess(ctx context.Context, token string) (string, error) {
	c := n.config()
	if validateCloudConfig(c) != nil {
		return "", errors.New("Cloudflare access has not been configured on the PC")
	}
	if len(token) > 16000 {
		return "", errors.New("invalid Access identity")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", errors.New("Google sign-in required")
	}
	decode := base64.RawURLEncoding.DecodeString
	hb, e := decode(parts[0])
	if e != nil {
		return "", errors.New("invalid identity header")
	}
	var h struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if json.Unmarshal(hb, &h) != nil || h.Alg != "RS256" || h.Kid == "" {
		return "", errors.New("invalid identity algorithm")
	}
	key, e := n.accessKey(ctx, c.TeamDomain, h.Kid)
	if e != nil {
		return "", e
	}
	sig, e := decode(parts[2])
	if e != nil {
		return "", errors.New("invalid identity signature")
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], sig) != nil {
		return "", errors.New("identity signature rejected")
	}
	cb, e := decode(parts[1])
	if e != nil {
		return "", errors.New("invalid identity claims")
	}
	var x struct {
		Iss   string   `json:"iss"`
		Aud   []string `json:"aud"`
		Exp   int64    `json:"exp"`
		Nbf   int64    `json:"nbf"`
		Email string   `json:"email"`
		Type  string   `json:"type"`
	}
	if json.Unmarshal(cb, &x) != nil {
		return "", errors.New("invalid identity claims")
	}
	aud := false
	for _, a := range x.Aud {
		if a == c.Audience {
			aud = true
		}
	}
	if x.Iss != "https://"+c.TeamDomain || !aud || x.Exp <= time.Now().Unix() || x.Nbf > time.Now().Unix()+30 || x.Type != "app" || !strings.EqualFold(x.Email, c.AllowedEmail) {
		return "", errors.New("this Google identity is not allowed, or its session expired")
	}
	return strings.ToLower(x.Email), nil
}
func (n *NativeSecurity) accessKey(ctx context.Context, team, kid string) (*rsa.PublicKey, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.keysTeam == team && time.Now().Before(n.keysUntil) {
		if k := n.keys[kid]; k != nil {
			return k, nil
		}
	}
	if n.keysTeam == team && time.Since(n.lastKeyAttempt) < 30*time.Second {
		return nil, errors.New("identity signing key unavailable; retry shortly")
	}
	n.lastKeyAttempt = time.Now()
	n.keysTeam = team
	req, e := http.NewRequestWithContext(ctx, "GET", "https://"+team+"/cdn-cgi/access/certs", nil)
	if e != nil {
		return nil, e
	}
	client := &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, e := client.Do(req)
	if e != nil {
		return nil, errors.New("cannot verify Cloudflare identity while its signing keys are unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, errors.New("Cloudflare signing key request failed")
	}
	var j struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&j) != nil {
		return nil, errors.New("invalid signing keys")
	}
	keys := map[string]*rsa.PublicKey{}
	for _, k := range j.Keys {
		b, er := base64.RawURLEncoding.DecodeString(k.N)
		eb, ee := base64.RawURLEncoding.DecodeString(k.E)
		if er != nil || ee != nil || k.Kty != "RSA" || len(b) < 256 || len(eb) > 4 {
			continue
		}
		exp := new(big.Int).SetBytes(eb).Int64()
		if exp < 3 {
			continue
		}
		keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(b), E: int(exp)}
	}
	n.keys = keys
	n.keysUntil = time.Now().Add(time.Hour)
	if k := keys[kid]; k != nil {
		return k, nil
	}
	return nil, errors.New("unknown identity signing key")
}
func parseNativeKey(encoded string) (*ecdsa.PublicKey, error) {
	b, e := base64.StdEncoding.DecodeString(encoded)
	if e != nil {
		return nil, errors.New("invalid phone key")
	}
	k, e := x509.ParsePKIXPublicKey(b)
	if e != nil {
		return nil, e
	}
	p, ok := k.(*ecdsa.PublicKey)
	if !ok || p.Curve != elliptic.P256() {
		return nil, errors.New("P-256 phone key required")
	}
	return p, nil
}
func (s *Server) handleNativePair(w http.ResponseWriter, r *http.Request) {
	var q struct {
		Code      string `json:"code"`
		Name      string `json:"name"`
		PublicKey string `json:"publicKey"`
	}
	if readJSON(r, &q) != nil || len(q.Name) > 100 || len(q.PublicKey) > 512 {
		jsonErr(w, 400, errors.New("invalid pairing request"))
		return
	}
	if _, e := parseNativeKey(q.PublicKey); e != nil {
		jsonErr(w, 400, e)
		return
	}
	n := s.native
	n.mu.Lock()
	defer n.mu.Unlock()
	if email, _ := r.Context().Value(nativeContextKey{}).(string); email != n.Config.AllowedEmail {
		jsonErr(w, 401, errors.New("owner changed; sign in again"))
		return
	}
	now := time.Now()
	recent := n.attempts[:0]
	for _, at := range n.attempts {
		if now.Sub(at) < time.Minute {
			recent = append(recent, at)
		}
	}
	n.attempts = recent
	if len(n.attempts) >= 5 {
		jsonErr(w, 429, errors.New("too many pairing attempts; wait one minute"))
		return
	}
	n.attempts = append(n.attempts, now)
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	pair := s.store.pair
	if pair.Code == "" || now.After(pair.Expires) || subtle.ConstantTimeCompare([]byte(q.Code), []byte(pair.Code)) != 1 {
		jsonErr(w, 403, errors.New("invalid or expired desktop pairing code"))
		return
	}
	raw, _ := base64.StdEncoding.DecodeString(q.PublicKey)
	hash := sha256.Sum256(raw)
	d := NativeDevice{ID: randomHex(16), Name: strings.TrimSpace(q.Name), Email: r.Context().Value(nativeContextKey{}).(string), PublicKey: q.PublicKey, Fingerprint: strings.ToUpper(hex.EncodeToString(hash[:8])), Created: now}
	if d.Name == "" {
		d.Name = "Android phone"
	}
	old := append([]NativeDevice(nil), n.Devices...)
	// Expired unapproved attempts do not accumulate forever.
	kept := []NativeDevice{}
	for _, x := range n.Devices {
		if x.Approved || now.Sub(x.Created) < 10*time.Minute {
			kept = append(kept, x)
		}
	}
	n.Devices = append(kept, d)
	if e := n.saveLocked(); e != nil {
		n.Devices = old
		jsonErr(w, 500, e)
		return
	}
	s.store.pair = pairCode{}
	jsonOut(w, map[string]any{"deviceId": d.ID, "fingerprint": d.Fingerprint, "approved": false, "email": d.Email})
}
func (s *Server) verifyNativeRequest(r *http.Request, requireApproval bool) (NativeDevice, error) {
	id := r.Header.Get("X-TB-Device")
	email, _ := r.Context().Value(nativeContextKey{}).(string)
	n := s.native
	n.mu.Lock()
	var d NativeDevice
	for _, x := range n.Devices {
		if x.ID == id {
			d = x
			break
		}
	}
	n.mu.Unlock()
	if d.ID == "" || d.Revoked || d.Email != email {
		return d, errors.New("phone authorization revoked or missing")
	}
	if requireApproval && !d.Approved {
		return d, errors.New("approve this phone on the Windows PC first")
	}
	if !d.Approved && time.Since(d.Created) > 10*time.Minute {
		return d, errors.New("pairing approval expired; pair again")
	}
	ts, e := time.Parse(time.RFC3339, r.Header.Get("X-TB-Date"))
	if e != nil || absDuration(time.Since(ts)) > 2*time.Minute {
		return d, errors.New("phone time is incorrect; enable automatic date and time")
	}
	nonce := r.Header.Get("X-TB-Nonce")
	if len(nonce) < 32 || len(nonce) > 128 {
		return d, errors.New("invalid nonce")
	}
	body, e := io.ReadAll(io.LimitReader(r.Body, (2<<20)+1))
	if e != nil || len(body) > 2<<20 {
		return d, errors.New("request too large")
	}
	r.Body = io.NopCloser(strings.NewReader(string(body)))
	bh := sha256.Sum256(body)
	msg := strings.Join([]string{r.Method, r.URL.RequestURI(), r.Header.Get("X-TB-Date"), nonce, hex.EncodeToString(bh[:])}, "\n")
	hash := sha256.Sum256([]byte(msg))
	key, e := parseNativeKey(d.PublicKey)
	sig, se := base64.StdEncoding.DecodeString(r.Header.Get("X-TB-Signature"))
	if e != nil || se != nil || !ecdsa.VerifyASN1(key, hash[:], sig) {
		return d, errors.New("phone signature rejected")
	}
	if !s.acceptNonce("native:"+id, nonce) {
		return d, errors.New("replayed request")
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	for i, x := range n.Devices {
		if x.ID == id {
			if x.Revoked || (requireApproval && !x.Approved) {
				return d, errors.New("phone authorization revoked")
			}
			if time.Since(x.LastSeen) > time.Minute {
				n.Devices[i].LastSeen = time.Now()
				_ = n.saveLocked()
			}
			break
		}
	}
	return d, nil
}
