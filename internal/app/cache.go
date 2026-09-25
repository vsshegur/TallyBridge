package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type cacheDisk struct {
	Kind      string          `json:"kind"`
	Company   string          `json:"company"`
	Variant   string          `json:"variant"`
	FetchedAt time.Time       `json:"fetchedAt"`
	Data      json.RawMessage `json:"data"`
}
type CacheStats struct {
	Entries  int       `json:"entries"`
	LatestAt time.Time `json:"latestAt"`
}
type CacheStore struct {
	mu      sync.RWMutex
	dir     string
	entries map[string]cacheDisk
}

func NewCacheStore(base string) (*CacheStore, error) {
	d := filepath.Join(base, "cache-v3")
	if err := os.MkdirAll(d, 0700); err != nil {
		return nil, err
	}
	c := &CacheStore{dir: d, entries: map[string]cacheDisk{}}
	// Load the previous cache generation first so an upgrade never throws away
	// the user's last-known-good accounting data. cache-v3 then overrides any
	// matching entries with newer data. Incompatible old files are ignored.
	c.loadDir(filepath.Join(base, "cache-v2"))
	c.loadDir(d)
	return c, nil
}
func ckey(kind, company, variant string) string {
	return strings.ToLower(strings.TrimSpace(kind)) + "\x1f" + strings.ToLower(strings.TrimSpace(company)) + "\x1f" + strings.ToLower(strings.TrimSpace(variant))
}
func (c *CacheStore) loadDir(dir string) {
	xs, _ := filepath.Glob(filepath.Join(dir, "entry-*.json"))
	for _, p := range xs {
		b, e := os.ReadFile(p)
		if e != nil {
			continue
		}
		var d cacheDisk
		if json.Unmarshal(b, &d) == nil && d.Kind != "" {
			c.entries[ckey(d.Kind, d.Company, d.Variant)] = d
		}
	}
}
func (c *CacheStore) Put(kind, company, variant string, v any, at time.Time) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	d := cacheDisk{kind, company, variant, at, b}
	db, _ := json.Marshal(d)
	sum := sha256.Sum256([]byte(ckey(kind, company, variant)))
	p := filepath.Join(c.dir, "entry-"+hex.EncodeToString(sum[:8])+".json")
	if e = atomicWrite(p, db, 0600); e != nil {
		return e
	}
	c.mu.Lock()
	c.entries[ckey(kind, company, variant)] = d
	c.mu.Unlock()
	return nil
}
func (c *CacheStore) Get(kind, company, variant string, out any) (time.Time, bool) {
	c.mu.RLock()
	d, ok := c.entries[ckey(kind, company, variant)]
	c.mu.RUnlock()
	if !ok {
		return time.Time{}, false
	}
	if json.Unmarshal(d.Data, out) != nil {
		return time.Time{}, false
	}
	return d.FetchedAt, true
}
func (c *CacheStore) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	s := CacheStats{Entries: len(c.entries)}
	for _, d := range c.entries {
		if d.FetchedAt.After(s.LatestAt) {
			s.LatestAt = d.FetchedAt
		}
	}
	return s
}
func (c *CacheStore) Companies() ([]Company, time.Time) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	m := map[string]Company{}
	var latest time.Time
	for _, d := range c.entries {
		if d.Company != "" {
			m[strings.ToLower(d.Company)] = Company{Name: d.Company}
		}
		if d.FetchedAt.After(latest) {
			latest = d.FetchedAt
		}
	}
	xs := make([]Company, 0, len(m))
	for _, x := range m {
		xs = append(xs, x)
	}
	sort.Slice(xs, func(i, j int) bool { return xs[i].Name < xs[j].Name })
	return xs, latest
}
func (c *CacheStore) VoucherHint(company, masterID string) (VoucherRow, bool) {
	var vd VoucherData
	for _, kind := range []string{"sales", "purchase"} {
		for _, period := range []string{"fy", "month"} {
			if _, ok := c.Get(kind, company, period, &vd); !ok {
				continue
			}
			for _, v := range vd.Vouchers {
				if v.MasterID == masterID {
					return v, true
				}
			}
		}
	}
	return VoucherRow{}, false
}

func (c *CacheStore) Matching(kind, company string) []cacheDisk {
	c.mu.RLock()
	defer c.mu.RUnlock()
	kind = strings.ToLower(strings.TrimSpace(kind))
	company = strings.ToLower(strings.TrimSpace(company))
	out := []cacheDisk{}
	for _, d := range c.entries {
		if strings.ToLower(strings.TrimSpace(d.Kind)) != kind {
			continue
		}
		if company != "" && strings.ToLower(strings.TrimSpace(d.Company)) != company {
			continue
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Company == out[j].Company {
			return out[i].Variant < out[j].Variant
		}
		return out[i].Company < out[j].Company
	})
	return out
}
