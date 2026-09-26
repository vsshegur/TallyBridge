package app

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestAutoSyncContinuesAfterUnsupportedReport(t *testing.T) {
	count := 0
	s, ts := testServerWithTally(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		count++
		if strings.Contains(string(b), "TBReceivableParties") {
			io.WriteString(w, `<ENVELOPE><COLLECTION><LEDGER NAME="ABC"><CLOSINGBALANCE>-100</CLOSINGBALANCE></LEDGER></COLLECTION></ENVELOPE>`)
			return
		}
		io.WriteString(w, `<ENVELOPE><LINEERROR>Report unavailable</LINEERROR></ENVELOPE>`)
	})
	defer ts.Close()
	s.runAutoSync("Demo", "test")
	if count != 4 {
		t.Fatalf("stopped at failed report: %d requests", count)
	}
	var saved OutstandingData
	if _, ok := s.cache.Get("receivable-party", "Demo", "", &saved); !ok || saved.Total != 100 {
		t.Fatalf("valid report was not saved: %+v", saved)
	}
	if s.autoSyncSnapshot().LastError == "" || !s.autoSyncSnapshot().LastOK.IsZero() {
		t.Fatal("partial sync marked successful")
	}
}
func TestAutoSyncRespectsCooldownAndFreshCycle(t *testing.T) {
	s, ts := testServerWithTally(t, func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected Tally read") })
	defer ts.Close()
	s.setAuto(func(a *AutoSyncState) { a.StartedAt = time.Now(); a.LastError = "temporary failure" })
	s.ensureAutoSync()
	if s.autoSyncSnapshot().Running {
		t.Fatal("retried too early")
	}
	s.setAuto(func(a *AutoSyncState) { a.StartedAt = time.Now().Add(-time.Hour) })
	s.tally.mu.Lock()
	s.tally.cooldownUntil = time.Now().Add(time.Minute)
	s.tally.mu.Unlock()
	s.ensureAutoSync()
	if s.autoSyncSnapshot().Running {
		t.Fatal("ignored cooldown")
	}
}
