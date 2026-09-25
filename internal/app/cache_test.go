package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCacheMigratesPreviousGeneration(t *testing.T) {
	base := t.TempDir()
	old := filepath.Join(base, "cache-v2")
	if err := os.MkdirAll(old, 0700); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 24, 12, 0, 0, 0, time.Local)
	payload, _ := json.Marshal(OutstandingData{Total: 48500, Count: 2, Type: "receivable"})
	disk, _ := json.Marshal(cacheDisk{Kind: "receivable-party", Company: "DEMO TEXTILES", FetchedAt: at, Data: payload})
	if err := os.WriteFile(filepath.Join(old, "entry-old.json"), disk, 0600); err != nil {
		t.Fatal(err)
	}
	c, err := NewCacheStore(base)
	if err != nil {
		t.Fatal(err)
	}
	var x OutstandingData
	gotAt, ok := c.Get("receivable-party", "DEMO TEXTILES", "", &x)
	if !ok || x.Total != 48500 || !gotAt.Equal(at) {
		t.Fatalf("old cache not preserved: ok=%v at=%v x=%+v", ok, gotAt, x)
	}
}
