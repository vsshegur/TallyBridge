from pathlib import Path
root=Path('.')
# Android/backend versions and production shrink are already present on base; only bump version.
p=root/'android/app/build.gradle'
s=p.read_text();s=s.replace("versionCode 20401\n        versionName '2.4.1'","versionCode 20402\n        versionName '2.4.2'")
assert "versionCode 20402" in s and "versionName '2.4.2'" in s
p.write_text(s)

p=root/'internal/app/types.go'
s=p.read_text();s=s.replace('const Version = "2.4.1"','const Version = "2.4.2"')
old='''type AutoSyncState struct {\n\tRunning   bool      `json:"running"`\n\tCompany   string    `json:"company"`\n\tStep      string    `json:"step"`\n\tStartedAt time.Time `json:"startedAt"`\n\tLastOK    time.Time `json:"lastOk"`\n\tLastError string    `json:"lastError"`\n}'''
new='''type AutoSyncState struct {\n\tRunning      bool      `json:"running"`\n\tCompany      string    `json:"company"`\n\tStep         string    `json:"step"`\n\tStartedAt    time.Time `json:"startedAt"`\n\tLastOK       time.Time `json:"lastOk"`\n\tLastProgress time.Time `json:"lastProgress,omitempty"`\n\tLastError    string    `json:"lastError"`\n\tLastErrorAt  time.Time `json:"lastErrorAt,omitempty"`\n\tRetrySoon    bool      `json:"retrySoon,omitempty"`\n}'''
assert old in s;s=s.replace(old,new);p.write_text(s)

p=root/'internal/app/autosync.go';s=p.read_text()
s=s.replace('''\ts.auto.StartedAt = time.Now()\n\ts.auto.LastError = ""''','''\ts.auto.StartedAt = time.Now()\n\ts.auto.LastError = ""\n\ts.auto.LastErrorAt = time.Time{}\n\ts.auto.RetrySoon = false''')
s=s.replace('''\t\t\ts.setAuto(func(a *AutoSyncState) { a.LastError = "Tally connection: " + e.Error() })''','''\t\t\ts.setAuto(func(a *AutoSyncState) { a.LastError = "Tally connection: " + e.Error(); a.LastErrorAt = time.Now(); a.RetrySoon = true })''')
s=s.replace('''\t\ts.setAuto(func(a *AutoSyncState) { a.LastError = "No open Tally company detected yet" })''','''\t\ts.setAuto(func(a *AutoSyncState) { a.LastError = "No open Tally company detected yet"; a.LastErrorAt = time.Now(); a.RetrySoon = true })''')
old='''\t\tfor _, st := range steps {\n\t\t\ts.setAuto(func(a *AutoSyncState) { a.Step = st.name })\n\t\t\tif e := st.fn(); e != nil {\n\t\t\t\tfailures = append(failures, fmt.Sprintf("%s / %s: %v", c, st.name, e))\n\t\t\t\ts.setAuto(func(a *AutoSyncState) { a.LastError = strings.Join(failures, "; ") })\n\t\t\t\t// Stop on transport/cooldown failures; continue past a single unsupported report.\n\t\t\t\tif _, _, pending := s.tally.Pending(); pending {\n\t\t\t\t\treturn\n\t\t\t\t}\n\t\t\t\tcontinue\n\t\t\t}\n\t\t}\n'''
new='''\t\tfor _, st := range steps {\n\t\t\ts.setAuto(func(a *AutoSyncState) { a.Step = st.name })\n\t\t\tif e := st.fn(); e != nil {\n\t\t\t\tfailures = append(failures, fmt.Sprintf("%s / %s: %v", c, st.name, e))\n\t\t\t\t_, _, pending := s.tally.Pending()\n\t\t\t\ts.setAuto(func(a *AutoSyncState) { a.LastError = strings.Join(failures, "; "); a.LastErrorAt = time.Now(); a.RetrySoon = pending })\n\t\t\t\t// Stop on transport/cooldown failures; continue past a single unsupported report.\n\t\t\t\tif pending { return }\n\t\t\t\tcontinue\n\t\t\t}\n\t\t\ts.setAuto(func(a *AutoSyncState) { a.LastProgress = time.Now() })\n\t\t}\n'''
assert old in s;s=s.replace(old,new)
s=s.replace('''\tif len(failures) == 0 {\n\t\ts.setAuto(func(a *AutoSyncState) { a.LastOK = time.Now(); a.LastError = "" })\n\t}''','''\tif len(failures) == 0 {\n\t\ts.setAuto(func(a *AutoSyncState) { now := time.Now(); a.LastOK = now; a.LastProgress = now; a.LastError = ""; a.LastErrorAt = time.Time{}; a.RetrySoon = false })\n\t}''')
s=s.replace('''\tinterval := 3 * time.Minute\n\tif a.LastError != "" {\n\t\tinterval = 30 * time.Second\n\t}''','''\tinterval := 3 * time.Minute\n\tif a.RetrySoon { interval = 30 * time.Second }''')
p.write_text(s)

p=root/'internal/app/workspace.go';s=p.read_text()
old='''\tsync := s.autoSyncSnapshot()\n\tif strings.TrimSpace(sync.LastError) != "" {\n\t\talerts = append(alerts, SmartAlert{ID: "sync-error", Severity: "critical", Title: "Sync needs attention", Detail: sync.LastError, Action: "diagnostics"})\n\t}'''
new='''\tsync := s.autoSyncSnapshot()\n\tif strings.TrimSpace(sync.LastError) != "" && (!st.TallyOnline || sync.RetrySoon) {\n\t\talerts = append(alerts, SmartAlert{ID: "sync-error", Severity: "warning", Title: "Sync retrying", Detail: sync.LastError, Action: "diagnostics"})\n\t}'''
assert old in s;s=s.replace(old,new);p.write_text(s)

p=root/'internal/app/operations_240_test.go';s=p.read_text()
s=s.replace('''import (\n\t"strings"''','''import (\n\t"net/http"\n\t"net/http/httptest"\n\t"strings"''')
s += '''\nfunc TestMobileOperationsRoutesAreRegistered(t *testing.T) {\n\ts := newOrderTestServer(t)\n\th := s.Handler()\n\tfor _, tc := range []struct{ method, path string }{\n\t\t{http.MethodGet, "/api/v1/mobile/workspace?company=Firm"},\n\t\t{http.MethodGet, "/api/v1/mobile/global-search?company=Firm"},\n\t\t{http.MethodGet, "/api/v1/mobile/customer-workspace?company=Firm&party=ABC"},\n\t\t{http.MethodGet, "/api/v1/mobile/followups?company=Firm"},\n\t\t{http.MethodPost, "/api/v1/mobile/sync"},\n\t} {\n\t\tr := httptest.NewRequest(tc.method, tc.path, nil)\n\t\tw := httptest.NewRecorder()\n\t\th.ServeHTTP(w, r)\n\t\tif w.Code == http.StatusNotFound { t.Fatalf("mobile route returned 404: %s %s", tc.method, tc.path) }\n\t}\n}\n\nfunc TestConnectedTallyDoesNotRaiseStaleSyncAttentionAlert(t *testing.T) {\n\ts := newOrderTestServer(t)\n\ts.tally.mu.Lock(); s.tally.tallyOnline = true; s.tally.mu.Unlock()\n\ts.setAuto(func(a *AutoSyncState) { a.LastError = "previous unsupported report"; a.LastErrorAt = time.Now().Add(-time.Minute); a.RetrySoon = false })\n\tfor _, a := range s.smartAlerts("Firm") {\n\t\tif a.ID == "sync-error" { t.Fatalf("connected Tally should not show stale sync attention alert: %#v", a) }\n\t}\n}\n'''
p.write_text(s)
