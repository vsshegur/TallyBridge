package app

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

// Only this fixed Google endpoint is used in direct mode. Request-supplied URLs
// never determine the verifier's key source.
func keyURL(team string) string {
	if team == "accounts.google.com" {
		return "https://www.googleapis.com/oauth2/v3/certs"
	}
	return "https://" + team + "/cdn-cgi/access/certs"
}

// The Google nonce binds the ID token to the phone's public key. Every request,
// including initial identity and pairing, must prove possession of that key.
func (s *Server) verifyGoogleRequest(r *http.Request) (string, error) {
	c := s.native.config()
	if c.GoogleClientID == "" || validateCloudConfig(c) != nil {
		return "", errors.New("Google sign-in is not configured on the PC")
	}
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") || len(auth) > 16000 {
		return "", errors.New("Google sign-in required")
	}
	parts := strings.Split(strings.TrimPrefix(auth, "Bearer "), ".")
	if len(parts) != 3 {
		return "", errors.New("invalid Google identity")
	}
	dec := base64.RawURLEncoding.DecodeString
	hb, e := dec(parts[0])
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
	key, e := s.native.accessKey(r.Context(), "accounts.google.com", h.Kid)
	if e != nil {
		return "", e
	}
	sig, e := dec(parts[2])
	if e != nil {
		return "", errors.New("invalid identity signature")
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if rsa.VerifyPKCS1v15(key, crypto.SHA256, digest[:], sig) != nil {
		return "", errors.New("Google identity signature rejected")
	}
	cb, e := dec(parts[1])
	if e != nil {
		return "", errors.New("invalid identity claims")
	}
	var x struct {
		Iss      string `json:"iss"`
		Aud      string `json:"aud"`
		Exp      int64  `json:"exp"`
		Iat      int64  `json:"iat"`
		Nbf      int64  `json:"nbf"`
		Email    string `json:"email"`
		Verified bool   `json:"email_verified"`
		Sub      string `json:"sub"`
		HD       string `json:"hd"`
		Nonce    string `json:"nonce"`
	}
	if json.Unmarshal(cb, &x) != nil {
		return "", errors.New("invalid identity claims")
	}
	now := time.Now().Unix()
	if (x.Iss != "https://accounts.google.com" && x.Iss != "accounts.google.com") || x.Aud != c.GoogleClientID || x.Exp <= now || x.Iat <= 0 || x.Iat > now+30 || x.Nbf > now+30 || x.Sub == "" || !x.Verified || !strings.EqualFold(x.Email, c.AllowedEmail) {
		return "", errors.New("Google account is not allowed or sign-in expired; sign in again")
	}
	email := strings.ToLower(x.Email)
	domain := email[strings.LastIndex(email, "@")+1:]
	if domain != "gmail.com" && domain != "googlemail.com" && (x.HD == "" || !strings.EqualFold(x.HD, domain)) {
		return "", errors.New("use Gmail or a Google Workspace email verified for its domain")
	}
	encoded := r.Header.Get("X-TB-Public-Key")
	if len(encoded) > 512 {
		return "", errors.New("invalid phone key")
	}
	phone, e := parseNativeKey(encoded)
	if e != nil {
		return "", errors.New("phone key required")
	}
	raw, _ := base64.StdEncoding.DecodeString(encoded)
	binding := sha256.Sum256(raw)
	if x.Nonce != hex.EncodeToString(binding[:]) {
		return "", errors.New("Google sign-in belongs to a different phone key")
	}
	date, e := time.Parse(time.RFC3339, r.Header.Get("X-TB-Date"))
	nonce := r.Header.Get("X-TB-Nonce")
	if e != nil || absDuration(time.Since(date)) > 2*time.Minute || len(nonce) < 32 || len(nonce) > 128 {
		return "", errors.New("invalid request time or nonce")
	}
	body, e := io.ReadAll(io.LimitReader(r.Body, (2<<20)+1))
	if e != nil || len(body) > 2<<20 {
		return "", errors.New("request too large")
	}
	r.Body = io.NopCloser(strings.NewReader(string(body)))
	bh := sha256.Sum256(body)
	msg := strings.Join([]string{r.Method, r.URL.RequestURI(), r.Header.Get("X-TB-Date"), nonce, hex.EncodeToString(bh[:])}, "\n")
	hash := sha256.Sum256([]byte(msg))
	proof, e := base64.StdEncoding.DecodeString(r.Header.Get("X-TB-Signature"))
	if e != nil || !ecdsa.VerifyASN1(phone, hash[:], proof) {
		return "", errors.New("phone signature rejected")
	}
	if !s.acceptNonce("google:"+hex.EncodeToString(binding[:]), nonce) {
		return "", errors.New("replayed request")
	}
	// Configuration changes fail closed even when they happen during verification.
	if s.native.config() != c {
		return "", errors.New("owner settings changed; sign in again")
	}
	return email, nil
}
