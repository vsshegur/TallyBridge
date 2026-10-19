package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func itSample(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "invoicetotally", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

const itCompany = "Shree Ganesh & Sons"

func TestValidatePostingXMLAcceptsInvoiceTotallyRequests(t *testing.T) {
	for file, want := range map[string]string{"voucher.xml": "Purchase voucher", "sales.xml": "Sales voucher", "ledger.xml": "new ledger", "stock.xml": "new stock item", "unit.xml": "new unit"} {
		got, err := validatePostingXML([]byte(itSample(t, file)), itCompany)
		if err != nil || got != want {
			t.Fatalf("%s: %q %v", file, got, err)
		}
		if _, err := validatePostingXML([]byte(itSample(t, file)), "shree ganesh & sons"); err != nil {
			t.Fatalf("%s: company match should ignore case: %v", file, err)
		}
	}
}

func TestValidatePostingXMLRejectsEverythingElse(t *testing.T) {
	v := itSample(t, "voucher.xml")
	led := itSample(t, "ledger.xml")
	cases := map[string]string{
		"other company":       v,
		"export":              itSample(t, "export_ledgers_request.xml"),
		"alter":               strings.Replace(v, `ACTION="Create"`, `ACTION="Alter"`, 1),
		"delete":              strings.Replace(led, `ACTION="Create"`, `ACTION="Delete"`, 1),
		"no action":           strings.Replace(led, ` ACTION="Create"`, ``, 1),
		"journal voucher":     strings.Replace(strings.Replace(v, `VCHTYPE="Purchase"`, `VCHTYPE="Journal"`, 1), "<VOUCHERTYPENAME>Purchase<", "<VOUCHERTYPENAME>Journal<", 1),
		"type mismatch":       strings.Replace(v, "<VOUCHERTYPENAME>Purchase<", "<VOUCHERTYPENAME>Payment<", 1),
		"no company":          strings.Replace(v, "<SVCURRENTCOMPANY>Shree Ganesh &amp; Sons</SVCURRENTCOMPANY>", "", 1),
		"tdl injection":       strings.Replace(led, "</STATICVARIABLES>", "</STATICVARIABLES><TDL><TDLMESSAGE/></TDL>", 1),
		"extra variable":      strings.Replace(led, "</SVCURRENTCOMPANY>", "</SVCURRENTCOMPANY><SVFROMDATE>20260101</SVFROMDATE>", 1),
		"two entries":         strings.Replace(led, "</LEDGER>", `</LEDGER><LEDGER NAME="X" ACTION="Create"/>`, 1),
		"voucher in masters":  strings.Replace(v, "<REPORTNAME>Vouchers<", "<REPORTNAME>All Masters<", 1),
		"group master":        strings.Replace(strings.Replace(led, "<LEDGER ", "<GROUP ", 1), "</LEDGER>", "</GROUP>", 1),
		"other report":        strings.Replace(led, "All Masters", "Day Book", 1),
		"header type":         strings.Replace(led, "</TALLYREQUEST>", "</TALLYREQUEST><TYPE>Data</TYPE>", 1),
		"malformed":           v[:len(v)-20],
		"nested object":       strings.Replace(led, "<PARENT>", `<OBJECT NAME="x"/><PARENT>`, 1),
		"second tallymessage": strings.Replace(led, "</REQUESTDATA>", "<TALLYMESSAGE><UNIT NAME=\"A\" ACTION=\"Create\"/></TALLYMESSAGE></REQUESTDATA>", 1),
		"lowercase alter":     strings.Replace(v, `ACTION="Create"`, `action="alter"`, 1),
		"duplicate action":    strings.Replace(v, `ACTION="Create"`, `ACTION="Create" ACTION="Alter"`, 1),
		"namespaced action":   strings.Replace(v, `ACTION="Create"`, `ACTION="Create" x:ACTION="Delete" xmlns:x="urn:x"`, 1),
		"namespaced element":  strings.Replace(led, "<PARENT>", `<x:ALTER xmlns:x="urn:x"/><PARENT>`, 1),
		"doctype":             `<!DOCTYPE ENVELOPE [<!ENTITY x "y">]>` + v,
		"processing instr":    `<?tally do="alter"?>` + v,
		"remote id":           strings.Replace(v, "<DATE>", "<REMOTEID>abc</REMOTEID><DATE>", 1),
		"remote id attr":      strings.Replace(v, `ACTION="Create"`, `ACTION="Create" REMOTEID="abc"`, 1),
		"guid":                strings.Replace(v, "<DATE>", "<GUID>abc</GUID><DATE>", 1),
		"rename":              strings.Replace(led, "<PARENT>", "<OLDNAME>Cash</OLDNAME><PARENT>", 1),
		"action element":      strings.Replace(led, "<PARENT>", "<ACTION>Delete</ACTION><PARENT>", 1),
	}
	for name, x := range cases {
		company := itCompany
		if name == "other company" {
			company = "Another Company"
		}
		if _, err := validatePostingXML([]byte(x), company); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := validatePostingXML([]byte(v), ""); err == nil {
		t.Error("posting without a known Tally company was accepted")
	}
}

func TestPhoneReadsStayReadOnly(t *testing.T) {
	// The read path keeps refusing anything that is not an Export.
	if err := validateExportXML([]byte(itSample(t, "voucher.xml"))); err == nil {
		t.Fatal("request() would accept a voucher import")
	}
	if err := validateExportXML([]byte(itSample(t, "export_ledgers_request.xml"))); err != nil {
		t.Fatal("master export must stay allowed:", err)
	}
}

const tallyLedgersReply = `<ENVELOPE><BODY><DATA><COLLECTION>
<LEDGER NAME="R&amp;D Traders"><PARENT>Sundry Creditors</PARENT><PARTYGSTIN>27ABCPA1234C1ZL</PARTYGSTIN><LEDSTATENAME>Maharashtra</LEDSTATENAME></LEDGER>
<LEDGER><NAME.LIST><NAME>GST Purchase</NAME></NAME.LIST><PARENT>Purchase Accounts</PARENT></LEDGER>
<LEDGER NAME="Old Party"><PARENT>Sundry Creditors</PARENT><LEDGSTREGDETAILS.LIST><GSTIN>29AAAAA0000A1Z5</GSTIN></LEDGSTREGDETAILS.LIST><STATENAME>Karnataka</STATENAME></LEDGER>
</COLLECTION></DATA></BODY></ENVELOPE>`

const tallyStockReply = `<ENVELOPE><BODY><DATA><COLLECTION>
<STOCKITEM NAME="3060 WHITE"><PARENT>Cloth</PARENT><BASEUNITS>PCS</BASEUNITS><GSTDETAILS.LIST><HSNCODE>5208</HSNCODE><STATEWISEDETAILS.LIST>
<RATEDETAILS.LIST><GSTRATEDUTYHEAD>CGST</GSTRATEDUTYHEAD><GSTRATE>2.5</GSTRATE></RATEDETAILS.LIST>
<RATEDETAILS.LIST><GSTRATEDUTYHEAD>Integrated Tax</GSTRATEDUTYHEAD><GSTRATE>5</GSTRATE></RATEDETAILS.LIST></STATEWISEDETAILS.LIST></GSTDETAILS.LIST></STOCKITEM>
<STOCKITEM NAME="Buttons"><BASEUNITS>DOZ</BASEUNITS><RATEDETAILS.LIST><GSTRATEDUTYHEAD>Central Tax</GSTRATEDUTYHEAD><GSTRATE>6</GSTRATE></RATEDETAILS.LIST></STOCKITEM>
<STOCKITEM NAME="Loose" xmlns:UDF="TallyUDF"><BASEUNITS>KG</BASEUNITS><UDF:COLOUR.LIST DESC="Colour"><UDF:COLOUR>Red</UDF:COLOUR></UDF:COLOUR.LIST></STOCKITEM>
</COLLECTION></DATA></BODY></ENVELOPE>`

func TestParseITMasters(t *testing.T) {
	led, err := parseITMasters("export_ledgers", []byte(tallyLedgersReply))
	if err != nil || len(led) != 3 {
		t.Fatal(led, err)
	}
	if led[0].Name != "R&D Traders" || led[0].GSTIN != "27ABCPA1234C1ZL" || led[0].State != "Maharashtra" || led[1].Name != "GST Purchase" || led[2].GSTIN != "29AAAAA0000A1Z5" || led[2].State != "Karnataka" {
		t.Fatalf("%+v", led)
	}
	st, err := parseITMasters("export_stock", []byte(tallyStockReply))
	if err != nil || len(st) != 3 {
		t.Fatal(st, err)
	}
	if st[0].HSN != "5208" || st[0].GSTRate == nil || *st[0].GSTRate != 5 || st[0].Unit != "PCS" || *st[1].GSTRate != 12 || st[2].GSTRate != nil {
		t.Fatalf("%+v %v %v", st, *st[0].GSTRate, st[2].GSTRate)
	}
	b, _ := json.Marshal(st[2])
	if strings.Contains(string(b), "gst_rate") {
		t.Fatal("missing rate must be omitted, not 0:", string(b))
	}
}

func TestImportOutcome(t *testing.T) {
	if ok, _ := importOutcome("<RESPONSE><CREATED>1</CREATED><ALTERED>0</ALTERED><ERRORS>0</ERRORS></RESPONSE>"); !ok {
		t.Fatal("created not ok")
	}
	ok, errs := importOutcome("<RESPONSE><LINEERROR>Ledger 'X' does not exist!</LINEERROR><CREATED>0</CREATED><ERRORS>1</ERRORS></RESPONSE>")
	if ok || errs != "Ledger 'X' does not exist!" {
		t.Fatal(ok, errs)
	}
}

// ---------------------------------------------------------------- fakes

type fakeTally struct {
	mu        sync.Mutex
	companies []string
	imports   []string
	down      bool
	srv       *httptest.Server
}

func newFakeTally(t *testing.T) *fakeTally {
	f := &fakeTally{companies: []string{itCompany, "Other Co"}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body := string(b)
		f.mu.Lock()
		defer f.mu.Unlock()
		switch {
		case strings.Contains(body, "<TALLYREQUEST>Import Data"):
			f.imports = append(f.imports, body)
			io.WriteString(w, "<RESPONSE><CREATED>1</CREATED><ALTERED>0</ALTERED><ERRORS>0</ERRORS><EXCEPTIONS>0</EXCEPTIONS></RESPONSE>")
		case strings.Contains(body, "<TYPE>Company</TYPE>"):
			out := "<ENVELOPE><BODY><DATA><COLLECTION>"
			for _, c := range f.companies {
				out += `<COMPANY NAME="` + escXML(c) + `"><NAME>` + escXML(c) + `</NAME></COMPANY>`
			}
			io.WriteString(w, out+"</COLLECTION></DATA></BODY></ENVELOPE>")
		case strings.Contains(body, "ITLedgerColl"):
			io.WriteString(w, tallyLedgersReply)
		default:
			io.WriteString(w, "<ENVELOPE></ENVELOPE>")
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeTally) importCount() int { f.mu.Lock(); defer f.mu.Unlock(); return len(f.imports) }

type fakeCloud struct {
	mu         sync.Mutex
	token      string
	pending    []itJob
	results    map[string][]map[string]any
	masters    map[string][]itMaster
	heartbeats []map[string]any
	polls      []string
	batches    map[string]map[string]any
	pages      map[string]map[int]map[string]any
	started    map[string]bool
	failPages  int
	srv        *httptest.Server
}

func newFakeCloud(t *testing.T) *fakeCloud {
	f := &fakeCloud{token: "itb_" + strings.Repeat("a", 40), results: map[string][]map[string]any{}, masters: map[string][]itMaster{},
		batches: map[string]map[string]any{}, pages: map[string]map[int]map[string]any{}, started: map[string]bool{}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+f.token {
			w.WriteHeader(401)
			io.WriteString(w, `{"error":"Unknown bridge token"}`)
			return
		}
		var in map[string]any
		_ = json.NewDecoder(r.Body).Decode(&in)
		f.mu.Lock()
		defer f.mu.Unlock()
		p := r.URL.Path
		switch {
		case p == "/api/bridge/heartbeat":
			if in["test"] != true {
				f.heartbeats = append(f.heartbeats, in)
			}
			json.NewEncoder(w).Encode(map[string]any{"company": "Ganesh Web", "tally_company": itCompany, "pending": len(f.pending)})
		case p == "/api/bridge/jobs":
			f.polls = append(f.polls, r.URL.RawQuery)
			out, keep := []itJob{}, []itJob{}
			for _, j := range f.pending {
				if r.URL.Query().Get("kinds") == "export" && !strings.HasPrefix(j.Kind, "export_") {
					keep = append(keep, j)
				} else {
					out = append(out, j)
				}
			}
			f.pending = keep
			json.NewEncoder(w).Encode(map[string]any{"jobs": out})
		case strings.HasSuffix(p, "/masters"):
			id := strings.TrimSuffix(strings.TrimPrefix(p, "/api/bridge/jobs/"), "/masters")
			b, _ := json.Marshal(in["rows"])
			var rows []itMaster
			json.Unmarshal(b, &rows)
			f.masters[id] = append(f.masters[id], rows...)
			io.WriteString(w, `{"ok":true}`)
		case strings.HasPrefix(p, "/api/bridge/jobs/"):
			id := strings.TrimPrefix(p, "/api/bridge/jobs/")
			f.results[id] = append(f.results[id], in)
			io.WriteString(w, `{"ok":true}`)
		case p == "/api/bridge/batches" && r.Method == "POST":
			id := randomHex(16)
			f.batches[id] = in
			files := []map[string]any{}
			for range in["files"].([]any) {
				fid := randomHex(16)
				f.pages[fid] = map[int]map[string]any{}
				files = append(files, map[string]any{"id": fid, "batch": id})
			}
			json.NewEncoder(w).Encode(map[string]any{"id": id, "name": "PURCHASE · 📱 " + in["uploaded_by"].(string), "files": files})
		case strings.HasPrefix(p, "/api/bridge/files/"):
			if f.failPages > 0 {
				f.failPages--
				w.WriteHeader(503)
				return
			}
			fid := strings.TrimSuffix(strings.TrimPrefix(p, "/api/bridge/files/"), "/pages")
			f.pages[fid][int(in["idx"].(float64))] = in
			io.WriteString(w, `{"ok":true}`)
		case strings.HasSuffix(p, "/start"):
			f.started[strings.TrimSuffix(strings.TrimPrefix(p, "/api/bridge/batches/"), "/start")] = true
			io.WriteString(w, `{"ok":true}`)
		case p == "/api/bridge/batches" && r.Method == "GET":
			out := []map[string]any{}
			for _, id := range strings.Split(r.URL.Query().Get("ids"), ",") {
				if f.started[id] {
					out = append(out, map[string]any{"id": id, "stage": "review", "state": "Ready to review", "synced": 0,
						"invoices": []map[string]any{{"number": "180", "party": "R&D Traders", "total": 42212, "state": "Ready to review"}}})
				}
			}
			json.NewEncoder(w).Encode(map[string]any{"batches": out})
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeCloud) push(j itJob) { f.mu.Lock(); f.pending = append(f.pending, j); f.mu.Unlock() }
func (f *fakeCloud) resultsFor(id string) []map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]map[string]any(nil), f.results[id]...)
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timed out waiting for " + what)
}

func itServer(t *testing.T) (*Server, *fakeTally, *fakeCloud) {
	t.Helper()
	s, err := NewServer(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	tally, cloud := newFakeTally(t), newFakeCloud(t)
	cfg := s.store.Settings()
	cfg.TallyURL = tally.srv.URL
	cfg.CooldownSec = 1
	if err := s.store.UpdateSettings(cfg); err != nil {
		t.Fatal(err)
	}
	s.it.heartbeatEvery, s.it.retryPause, s.it.tallyCache, s.it.downRecheck = 50*time.Millisecond, 30*time.Millisecond, 0, 3*time.Second
	t.Cleanup(s.it.Stop)
	return s, tally, cloud
}

func adminPost(t *testing.T, s *Server, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest("POST", "http://127.0.0.1:8765"+path, strings.NewReader(body))
	r.RemoteAddr = "127.0.0.1:50505"
	r.Header.Set("X-TB-Admin", "1")
	return serve(s.DesktopHandlerWithOrders(), r)
}

// ---------------------------------------------------------------- connector

func TestITConnectorPostsOnlyWhenAllowedAndNeverTwice(t *testing.T) {
	s, tally, cloud := itServer(t)
	// A wrong token is refused before anything is saved.
	w := adminPost(t, s, "/api/v1/admin/invoicetotally/save", `{"serverUrl":"`+cloud.srv.URL+`","add":["itb_`+strings.Repeat("b", 40)+`"]}`)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "Token rejected") || len(s.it.config().Connections) != 0 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = adminPost(t, s, "/api/v1/admin/invoicetotally/save", `{"serverUrl":"`+cloud.srv.URL+`/","add":["`+cloud.token+`"]}`)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	conns := s.it.config().Connections
	if len(conns) != 1 || conns[0].Company != "Ganesh Web" || conns[0].TallyCompany != itCompany || s.it.config().ServerURL != cloud.srv.URL {
		t.Fatalf("%+v", s.it.config())
	}
	waitFor(t, "heartbeat", func() bool {
		cloud.mu.Lock()
		defer cloud.mu.Unlock()
		if len(cloud.heartbeats) == 0 {
			return false
		}
		h := cloud.heartbeats[len(cloud.heartbeats)-1]
		return h["client"] == "TallyBridge" && h["posting"] == false && h["tally_ok"] == true && strings.Contains(strings.Join(func() []string {
			o := []string{}
			for _, x := range h["companies"].([]any) {
				o = append(o, x.(string))
			}
			return o
		}(), "|"), itCompany)
	})

	// Master import runs even with posting off.
	cloud.push(itJob{ID: "m1", Kind: "export_ledgers", Payload: itSample(t, "export_ledgers_request.xml"), Label: "Reading ledgers"})
	waitFor(t, "masters", func() bool { return len(cloud.resultsFor("m1")) == 1 })
	if r := cloud.resultsFor("m1")[0]; r["ok"] != true || r["response"] != "3 records" {
		t.Fatal(r)
	}
	cloud.mu.Lock()
	if len(cloud.masters["m1"]) != 3 || cloud.masters["m1"][0].GSTIN != "27ABCPA1234C1ZL" {
		t.Fatalf("%+v", cloud.masters["m1"])
	}
	cloud.mu.Unlock()

	// Posting is off by default: the voucher is not even fetched.
	cloud.push(itJob{ID: "v1", Kind: "import_voucher", Payload: itSample(t, "voucher.xml"), Label: "Purchase 180"})
	time.Sleep(300 * time.Millisecond)
	if tally.importCount() != 0 || len(cloud.resultsFor("v1")) != 0 {
		t.Fatal("posted while posting was off")
	}
	cloud.mu.Lock()
	lastPoll := cloud.polls[len(cloud.polls)-1]
	cloud.mu.Unlock()
	if !strings.Contains(lastPoll, "kinds=export") {
		t.Fatal("poll did not ask for master reads only:", lastPoll)
	}

	// Switch posting on: the voucher goes to Tally once.
	if w := adminPost(t, s, "/api/v1/admin/invoicetotally/options", `{"postingEnabled":true}`); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	waitFor(t, "voucher posted", func() bool { return len(cloud.resultsFor("v1")) == 1 })
	if r := cloud.resultsFor("v1")[0]; r["ok"] != true || !strings.Contains(r["response"].(string), "<CREATED>1</CREATED>") || tally.importCount() != 1 {
		t.Fatal(r, tally.importCount())
	}
	if !strings.Contains(tally.imports[0], "<SVCURRENTCOMPANY>Shree Ganesh &amp; Sons</SVCURRENTCOMPANY>") {
		t.Fatal("posted XML changed on the way")
	}

	// InvoiceTotally hands the same job out again (lost report): result re-sent, not re-posted.
	cloud.push(itJob{ID: "v1", Kind: "import_voucher", Payload: itSample(t, "voucher.xml"), Label: "Purchase 180"})
	waitFor(t, "re-report", func() bool { return len(cloud.resultsFor("v1")) == 2 })
	if tally.importCount() != 1 || cloud.resultsFor("v1")[1]["ok"] != true {
		t.Fatal("voucher posted twice")
	}
	// ...even after TallyBridge restarts.
	reloaded := newITConnector(s, s.it.dir)
	if d, ok := reloaded.journalGet("v1"); !ok || d.State != "done" {
		t.Fatal("posting journal not persisted", d)
	}

	// A request for another company is refused, not posted.
	cloud.push(itJob{ID: "v2", Kind: "import_master", Payload: strings.ReplaceAll(itSample(t, "ledger.xml"), "Shree Ganesh &amp; Sons", "Other Co"), Label: "New ledger"})
	waitFor(t, "refusal", func() bool { return len(cloud.resultsFor("v2")) == 1 })
	if r := cloud.resultsFor("v2")[0]; r["ok"] != false || !strings.Contains(r["error"].(string), "refused") || tally.importCount() != 1 {
		t.Fatal(r)
	}

	// The company is closed in Tally: the job waits instead of going into another company.
	tally.mu.Lock()
	tally.companies = []string{"Other Co"}
	tally.mu.Unlock()
	cloud.push(itJob{ID: "v3", Kind: "import_master", Payload: itSample(t, "unit.xml"), Label: "New unit: DOZ"})
	waitFor(t, "wait report", func() bool { return len(cloud.resultsFor("v3")) >= 1 })
	if r := cloud.resultsFor("v3")[0]; r["retry"] != true || !strings.Contains(r["error"].(string), "Waiting for “Shree Ganesh & Sons”") || tally.importCount() != 1 {
		t.Fatal(r)
	}
	tally.mu.Lock()
	tally.companies = []string{itCompany}
	tally.mu.Unlock()
	cloud.push(itJob{ID: "v3", Kind: "import_master", Payload: itSample(t, "unit.xml"), Label: "New unit: DOZ"})
	waitFor(t, "unit created", func() bool {
		rs := cloud.resultsFor("v3")
		return len(rs) > 0 && rs[len(rs)-1]["ok"] == true
	})
	if tally.importCount() != 2 {
		t.Fatal(tally.importCount())
	}

	snap := s.it.snapshot()
	c0 := snap["connections"].([]itConnView)[0]
	if c0.Stats.PostedToday != 1 || c0.Stats.MastersToday != 1 || c0.Stats.FailedToday != 1 || c0.Server.Status != "ok" || c0.Hint != "…aaaa" {
		t.Fatalf("%+v", c0)
	}
	b, _ := json.Marshal(snap)
	if strings.Contains(string(b), cloud.token) {
		t.Fatal("token leaked into the admin snapshot")
	}
}

func TestITPostingWhenTallyIsDownIsRetriedNotLost(t *testing.T) {
	s, tally, cloud := itServer(t)
	if _, err := s.it.save(context.Background(), cloud.srv.URL, []string{cloud.token}, nil); err != nil {
		t.Fatal(err)
	}
	s.it.mu.Lock()
	s.it.cfg.PostingEnabled = true
	s.it.mu.Unlock()
	waitFor(t, "connected", func() bool { return s.it.snapshot()["connections"].([]itConnView)[0].Server.Status == "ok" })
	addr := tally.srv.Listener.Addr().String()
	tally.srv.Close() // TallyPrime closed
	cloud.push(itJob{ID: "d1", Kind: "import_voucher", Payload: itSample(t, "voucher.xml"), Label: "Purchase 180"})
	waitFor(t, "retry", func() bool { return len(cloud.resultsFor("d1")) >= 1 })
	if r := cloud.resultsFor("d1")[0]; r["retry"] != true {
		t.Fatal(r)
	}
	if _, ok := s.it.journalGet("d1"); ok {
		t.Fatal("a job that never reached Tally stays blocked")
	}
	_ = addr
}

func TestNormaliseServerURL(t *testing.T) {
	for in, want := range map[string]string{"invoicetotally.me.workers.dev": "https://invoicetotally.me.workers.dev", "https://x.dev/#/c/1": "https://x.dev", "http://127.0.0.1:8787/": "http://127.0.0.1:8787"} {
		if got, err := normaliseServerURL(in); err != nil || got != want {
			t.Fatal(in, got, err)
		}
	}
	for _, bad := range []string{"", "http://example.com", "ftp://x"} {
		if _, err := normaliseServerURL(bad); err == nil {
			t.Fatal("accepted", bad)
		}
	}
}
