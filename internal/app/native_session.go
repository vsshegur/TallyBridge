package app

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"
)

// The session is usable only with this approved device's per-request P-256
// signature. Google identity remains mandatory for issuing it and pairing.
func (n *NativeSecurity) issueSession(id string) (string, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	for i, d := range n.Devices {
		if d.ID != id {
			continue
		}
		if d.Revoked || !d.Approved || d.Email != n.Config.AllowedEmail {
			break
		}
		token := randomHex(32)
		hash := sha256.Sum256([]byte(token))
		n.Devices[i].SessionHash = hex.EncodeToString(hash[:])
		n.Devices[i].SessionIssued = time.Now()
		n.Devices[i].LastSeen = time.Now()
		if e := n.saveLocked(); e != nil {
			n.Devices[i] = d
			return "", e
		}
		return token, nil
	}
	return "", errors.New("phone approval is missing or revoked")
}
func (n *NativeSecurity) verifySession(r *http.Request) (string, error) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "TallyBridge ")
	if len(token) != 64 {
		return "", errors.New("phone session invalid; sign in again")
	}
	hash := sha256.Sum256([]byte(token))
	encoded := hex.EncodeToString(hash[:])
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, d := range n.Devices {
		if d.ID != r.Header.Get("X-TB-Device") {
			continue
		}
		if d.Revoked || !d.Approved || d.Email != n.Config.AllowedEmail || d.SessionIssued.IsZero() || time.Since(d.LastSeen) > 30*24*time.Hour || subtle.ConstantTimeCompare([]byte(d.SessionHash), []byte(encoded)) != 1 {
			break
		}
		return d.Email, nil
	}
	return "", errors.New("phone session expired or revoked; sign in again")
}
