package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type pairCode struct {
	Code    string
	Expires time.Time
}

type Store struct {
	mu       sync.Mutex
	dir      string
	settings Settings
	devices  []Device
	pair     pairCode
}

func defaultSettings() Settings {
	return Settings{TallyURL: "http://127.0.0.1:9000", TimeoutSec: 6, CooldownSec: 12, Port: 8765}
}

func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	s := &Store{dir: dir, settings: defaultSettings()}
	s.load()
	return s, nil
}
func (s *Store) load() {
	b, err := os.ReadFile(filepath.Join(s.dir, "settings.json"))
	if err == nil {
		var x Settings
		if json.Unmarshal(b, &x) == nil {
			if x.TallyURL != "" {
				s.settings.TallyURL = x.TallyURL
			}
			if x.TimeoutSec > 0 {
				s.settings.TimeoutSec = x.TimeoutSec
			}
			if x.CooldownSec > 0 {
				s.settings.CooldownSec = x.CooldownSec
			}
			if x.Port > 0 {
				s.settings.Port = x.Port
			}
		}
	}
	// v1 compatibility: phone.json may be []Device or {devices:[...]}
	b, err = os.ReadFile(filepath.Join(s.dir, "phone.json"))
	if err == nil {
		var xs []Device
		if json.Unmarshal(b, &xs) == nil {
			s.devices = xs
		} else {
			var w struct {
				Devices []Device `json:"devices"`
			}
			if json.Unmarshal(b, &w) == nil {
				s.devices = w.Devices
			}
		}
	}
	if len(s.devices) == 0 { // some older builds placed devices in settings.json
		if b, err := os.ReadFile(filepath.Join(s.dir, "settings.json")); err == nil {
			var w struct {
				Devices []Device `json:"devices"`
			}
			if json.Unmarshal(b, &w) == nil && len(w.Devices) > 0 {
				s.devices = w.Devices
			}
		}
	}
}
func (s *Store) saveSettingsLocked() error {
	b, _ := json.MarshalIndent(s.settings, "", "  ")
	return atomicWrite(filepath.Join(s.dir, "settings.json"), b, 0600)
}
func (s *Store) saveDevicesLocked() error {
	b, _ := json.MarshalIndent(struct {
		Devices []Device `json:"devices"`
	}{s.devices}, "", "  ")
	return atomicWrite(filepath.Join(s.dir, "phone.json"), b, 0600)
}
func atomicWrite(path string, b []byte, mode os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, mode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
func (s *Store) Snapshot() (Settings, []Device) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings, append([]Device(nil), s.devices...)
}
func (s *Store) Settings() Settings { s.mu.Lock(); defer s.mu.Unlock(); return s.settings }
func (s *Store) UpdateSettings(x Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings = x
	return s.saveSettingsLocked()
}
func randomHex(n int) string { b := make([]byte, n); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func (s *Store) GeneratePairCode() (string, time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	n := int(b[0])<<16 | int(b[1])<<8 | int(b[2])
	code := fmt.Sprintf("%06d", n%1000000)
	exp := time.Now().Add(5 * time.Minute)
	s.pair = pairCode{code, exp}
	return code, exp
}
func (s *Store) Pair(code, name string) (Device, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pair.Code == "" || time.Now().After(s.pair.Expires) || code != s.pair.Code {
		return Device{}, errors.New("invalid or expired pairing code")
	}
	d := Device{ID: randomHex(12), Name: strings.TrimSpace(name), Key: randomHex(32), Created: time.Now(), LastSeen: time.Now()}
	if d.Name == "" {
		d.Name = "Phone"
	}
	s.devices = append(s.devices, d)
	s.pair = pairCode{}
	if err := s.saveDevicesLocked(); err != nil {
		return Device{}, err
	}
	return d, nil
}
func (s *Store) Revoke(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.devices {
		if s.devices[i].ID == id {
			s.devices[i].Revoked = true
			return s.saveDevicesLocked()
		}
	}
	return errors.New("device not found")
}
func (s *Store) Device(id string) (Device, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, d := range s.devices {
		if d.ID == id && !d.Revoked {
			return d, true
		}
	}
	return Device{}, false
}
func (s *Store) Touch(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.devices {
		if s.devices[i].ID == id && !s.devices[i].Revoked {
			s.devices[i].LastSeen = time.Now()
			_ = s.saveDevicesLocked()
			return
		}
	}
}
func appDataDir() string {
	if v := os.Getenv("LOCALAPPDATA"); v != "" {
		return filepath.Join(v, "TallyBridge")
	}
	if h, err := os.UserHomeDir(); err == nil {
		return filepath.Join(h, ".tallybridge")
	}
	return filepath.Join(os.TempDir(), "TallyBridge")
}
func parseIntDefault(s string, d int) int {
	n, e := strconv.Atoi(s)
	if e != nil {
		return d
	}
	return n
}
