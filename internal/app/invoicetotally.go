package app

// InvoiceTotally connector: TallyBridge on the PC doubles as the desktop bridge
// for InvoiceTotally (bills read by AI in the cloud, reviewed in the browser).
//
// For each company token it keeps a heartbeat and a long-poll for jobs:
//   - export_* jobs read masters from Tally (read-only, through request()).
//   - import_* jobs post a reviewed voucher or a new master. They run only when
//     the user has switched on posting on this PC, and only through post()
//     after validatePostingXML (tally_write.go).
// It also relays bills photographed in the phone app (bills.go).

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	itConfigFile    = "invoicetotally.json"
	itJournalFile   = "invoicetotally-posted.json"
	itHeartbeatSecs = 10
	itLegacyUIPort  = 47521
)

type ITConnection struct {
	ID           string `json:"id"`
	Token        string `json:"token"`
	Company      string `json:"company,omitempty"`
	TallyCompany string `json:"tallyCompany,omitempty"`
}

type ITConfig struct {
	ServerURL      string         `json:"serverUrl"`
	Connections    []ITConnection `json:"connections"`
	PostingEnabled bool           `json:"postingEnabled"`
	PhoneOff       bool           `json:"phoneUploadsOff,omitempty"`
}

type itStatus struct {
	Status string `json:"status"` // ok | down | idle
	Detail string `json:"detail"`
}

type itStats struct {
	PostedToday  int       `json:"postedToday"`
	FailedToday  int       `json:"failedToday"`
	MastersToday int       `json:"mastersToday"`
	Day          string    `json:"day"`
	LastSync     time.Time `json:"lastSync"`
}

type itLink struct {
	conn          ITConnection
	wake          chan struct{}
	beat          chan struct{} // send a heartbeat now (settings changed)
	server        itStatus
	pending       int
	companyOpen   *bool
	stats         itStats
	waitingLogged time.Time
	serverURL     string
	cancel        context.CancelFunc
}

// itDone remembers every posting job this PC has run. A job is sent to Tally at
// most once: if InvoiceTotally hands the same job out again (for example after
// the internet dropped before TallyBridge could report back), the saved result
// is reported instead of posting a second voucher.
type itDone struct {
	At     time.Time      `json:"at"`
	State  string         `json:"state"` // running | done
	Result map[string]any `json:"result,omitempty"`
}

type ITActivity struct {
	At      time.Time `json:"at"`
	Text    string    `json:"text"`
	Kind    string    `json:"kind"`
	OK      bool      `json:"ok"`
	Company string    `json:"company,omitempty"`
}

type itTally struct {
	Status    string   `json:"status"`
	Detail    string   `json:"detail"`
	Companies []string `json:"companies"`
}

type ITConnector struct {
	s        *Server
	dir      string
	mu       sync.Mutex
	cfg      ITConfig
	links    []*itLink
	activity []ITActivity
	tally    itTally
	tallyAt  time.Time
	fails    int
	journal  map[string]itDone
	jmu      sync.Mutex
	http     *http.Client
	// test hooks
	heartbeatEvery time.Duration
	retryPause     time.Duration
	tallyCache     time.Duration
	downRecheck    time.Duration
}

func newITConnector(s *Server, dir string) *ITConnector {
	c := &ITConnector{s: s, dir: dir, tally: itTally{Status: "idle", Detail: "Waiting", Companies: []string{}},
		http: &http.Client{}, heartbeatEvery: itHeartbeatSecs * time.Second, retryPause: 10 * time.Second, tallyCache: 8 * time.Second, downRecheck: 15 * time.Second}
	if b, err := os.ReadFile(filepath.Join(dir, itConfigFile)); err == nil {
		_ = json.Unmarshal(b, &c.cfg)
	}
	c.journal = map[string]itDone{}
	if b, err := os.ReadFile(filepath.Join(dir, itJournalFile)); err == nil {
		_ = json.Unmarshal(b, &c.journal)
	}
	return c
}

func (c *ITConnector) journalGet(id string) (itDone, bool) {
	c.jmu.Lock()
	defer c.jmu.Unlock()
	d, ok := c.journal[id]
	return d, ok
}

func (c *ITConnector) journalDelete(id string) {
	c.jmu.Lock()
	defer c.jmu.Unlock()
	delete(c.journal, id)
	b, _ := json.Marshal(c.journal)
	_ = atomicWrite(filepath.Join(c.dir, itJournalFile), b, 0600)
}

func (c *ITConnector) journalSet(id string, d itDone) error {
	c.jmu.Lock()
	defer c.jmu.Unlock()
	if r, ok := d.Result["response"].(string); ok && len(r) > 64<<10 {
		d.Result["response"] = r[:64<<10]
	}
	c.journal[id] = d
	for k, v := range c.journal {
		if time.Since(v.At) > 30*24*time.Hour {
			delete(c.journal, k)
		}
	}
	b, _ := json.Marshal(c.journal)
	return atomicWrite(filepath.Join(c.dir, itJournalFile), b, 0600)
}

func (c *ITConnector) config() ITConfig {
	c.mu.Lock()
	defer c.mu.Unlock()
	x := c.cfg
	x.Connections = append([]ITConnection(nil), c.cfg.Connections...)
	return x
}

func (c *ITConnector) saveLocked() error {
	b, _ := json.MarshalIndent(c.cfg, "", "  ")
	return atomicWrite(filepath.Join(c.dir, itConfigFile), b, 0600)
}

func (c *ITConnector) log(text, kind string, ok bool, l *itLink) {
	c.mu.Lock()
	defer c.mu.Unlock()
	a := ITActivity{At: time.Now(), Text: text, Kind: kind, OK: ok}
	if l != nil {
		a.Company = l.conn.Company
	}
	c.activity = append([]ITActivity{a}, c.activity...)
	if len(c.activity) > 300 {
		c.activity = c.activity[:300]
	}
}

// normaliseServerURL accepts the web app address as copied from the browser.
func normaliseServerURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("enter the InvoiceTotally web address")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", errors.New("the InvoiceTotally web address is not valid")
	}
	h := strings.ToLower(u.Hostname())
	local := h == "127.0.0.1" || h == "localhost" || h == "::1"
	if u.Scheme != "https" && !(u.Scheme == "http" && local) {
		return "", errors.New("the InvoiceTotally web address must start with https://")
	}
	return u.Scheme + "://" + u.Host, nil
}

func validITToken(t string) bool {
	if !strings.HasPrefix(t, "itb_") || len(t) < 20 || len(t) > 120 {
		return false
	}
	for _, r := range t[4:] {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-') {
			return false
		}
	}
	return true
}

func tokenHint(t string) string {
	if len(t) < 4 {
		return ""
	}
	return "…" + t[len(t)-4:]
}

// ----------------------------------------------------------------- cloud API

type itHTTPError struct {
	Code int
	Msg  string
}

func (e *itHTTPError) Error() string {
	if e.Msg != "" {
		return e.Msg
	}
	return fmt.Sprintf("InvoiceTotally answered %d", e.Code)
}

func (c *ITConnector) call(ctx context.Context, server, token, method, path string, payload any, out any, timeout time.Duration) error {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(server, "/")+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "TallyBridge/"+Version)
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(raw, &e)
		if res.StatusCode == 401 {
			e.Error = "Token rejected — copy it again from InvoiceTotally (company → Tally bridge)"
		}
		return &itHTTPError{Code: res.StatusCode, Msg: e.Error}
	}
	if out != nil && len(raw) > 0 {
		return json.Unmarshal(raw, out)
	}
	return nil
}

func friendlyNetErr(err error) string {
	var he *itHTTPError
	if errors.As(err, &he) {
		return he.Error()
	}
	var dns *net.DNSError
	if errors.As(err, &dns) {
		return "Can't find the InvoiceTotally address — check the internet and the web address"
	}
	return "Can't reach InvoiceTotally — check the internet connection"
}

// ----------------------------------------------------------------- lifecycle

// Start brings the running companies in line with the saved settings: new
// companies start, removed ones stop, and companies that stay keep their open
// long-poll. (Dropping a poll that InvoiceTotally already answered would leave
// its jobs parked for a few minutes, so links are never restarted needlessly.)
func (c *ITConnector) Start() {
	c.mu.Lock()
	keep := []*itLink{}
	start := []*itLink{}
	want := map[string]ITConnection{}
	for _, conn := range c.cfg.Connections {
		want[conn.ID] = conn
	}
	for _, l := range c.links {
		conn, ok := want[l.conn.ID]
		if ok && l.serverURL == c.cfg.ServerURL && l.conn.Token == conn.Token && l.cancel != nil {
			l.conn.Company, l.conn.TallyCompany = conn.Company, conn.TallyCompany
			keep = append(keep, l)
			delete(want, l.conn.ID)
			continue
		}
		if l.cancel != nil {
			l.cancel()
		}
	}
	for _, conn := range c.cfg.Connections {
		if _, ok := want[conn.ID]; !ok {
			continue
		}
		l := &itLink{conn: conn, serverURL: c.cfg.ServerURL, wake: make(chan struct{}, 1), beat: make(chan struct{}, 1), server: itStatus{"idle", "Connecting…"}, stats: itStats{Day: today()}}
		keep = append(keep, l)
		if c.cfg.ServerURL != "" {
			start = append(start, l)
		}
	}
	// keep the saved order
	order := map[string]int{}
	for i, conn := range c.cfg.Connections {
		order[conn.ID] = i
	}
	sort.SliceStable(keep, func(i, j int) bool { return order[keep[i].conn.ID] < order[keep[j].conn.ID] })
	c.links = keep
	for _, l := range start {
		ctx, cancel := context.WithCancel(context.Background())
		l.cancel = cancel
		go c.heartbeatLoop(ctx, l)
		go c.jobLoop(ctx, l)
	}
	c.mu.Unlock()
}

func (c *ITConnector) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, l := range c.links {
		if l.cancel != nil {
			l.cancel()
			l.cancel = nil
		}
	}
}

func today() string { return time.Now().Format("2006-01-02") }

func sleepCtx(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

// tallyStatus is one shared, cached view of Tally for every company. It never
// probes while another request holds Tally, and reports "down" only after two
// failed probes in a row.
func (c *ITConnector) tallyStatus() itTally { return c.tallyStatusAt(false) }

// tallyStatusAt with fresh=true asks Tally for its open companies now (when
// Tally is free) — used right before running a job.
func (c *ITConnector) tallyStatusAt(fresh bool) itTally {
	live := c.s.tally.State()
	c.mu.Lock()
	// While Tally is down, don't probe it more than every downRecheck: each failed
	// probe starts Tally's safety cooldown, and constant probing would keep a
	// waiting post (or the phone) locked out once Tally comes back. Any request
	// that reached Tally since then ends the wait at once.
	recentlyDown := c.tally.Status == "down" && time.Since(c.tallyAt) < c.downRecheck && !live.LastSuccess.After(c.tallyAt)
	if (!fresh && time.Since(c.tallyAt) < c.tallyCache) || recentlyDown {
		t := c.tally
		c.mu.Unlock()
		return t
	}
	c.tallyAt = time.Now()
	c.mu.Unlock()

	names := func(cs []Company) []string {
		out := []string{}
		for _, x := range cs {
			if n := strings.TrimSpace(x.Name); n != "" {
				out = append(out, n)
			}
		}
		return out
	}
	failed := func() itTally {
		c.mu.Lock()
		c.fails++
		fails, prev := c.fails, c.tally
		c.mu.Unlock()
		if fails < 2 && prev.Status == "ok" {
			return prev
		}
		return itTally{Status: "down", Detail: "TallyPrime is not running, or its connectivity port (9000) is off", Companies: []string{}}
	}
	var st itTally
	reason, _, pending := c.s.tally.Pending()
	switch {
	case pending && reason == "busy":
		// Another request is talking to Tally right now: don't queue a probe behind it.
		st = itTally{Status: "ok", Detail: "Connected — busy", Companies: names(live.Companies)}
		if len(st.Companies) == 0 {
			st.Companies = c.tally.Companies
		}
	case pending && !live.TallyOnline:
		st = failed() // safety cooldown after a failed request
	case pending || (!fresh && live.TallyOnline && time.Since(live.LastSuccess) < 20*time.Second):
		st = itTally{Status: "ok", Detail: "Connected", Companies: names(live.Companies)}
	default:
		if lv, err := c.s.probeLive(); err == nil {
			c.mu.Lock()
			c.fails = 0
			c.mu.Unlock()
			st = itTally{Status: "ok", Detail: "Connected", Companies: names(lv.Companies)}
		} else if errors.Is(err, ErrBusy) || errors.Is(err, ErrCooldown) {
			st = c.tally
		} else {
			st = failed()
		}
	}
	if st.Companies == nil {
		st.Companies = []string{}
	}
	c.mu.Lock()
	c.tally = st
	c.mu.Unlock()
	return st
}

func companyOpenIn(t itTally, tallyCompany string) *bool {
	tc := strings.TrimSpace(tallyCompany)
	if t.Status != "ok" || tc == "" || len(t.Companies) == 0 {
		return nil
	}
	open := false
	for _, n := range t.Companies {
		if strings.EqualFold(strings.TrimSpace(n), tc) {
			open = true
		}
	}
	return &open
}

type itHeartbeatReply struct {
	Company      string `json:"company"`
	TallyCompany string `json:"tally_company"`
	Pending      int    `json:"pending"`
}

func (c *ITConnector) heartbeatLoop(ctx context.Context, l *itLink) {
	for {
		c.heartbeat(ctx, l)
		select {
		case <-ctx.Done():
			return
		case <-l.beat:
		case <-time.After(c.heartbeatEvery):
		}
	}
}

func (c *ITConnector) heartbeat(ctx context.Context, l *itLink) {
	cfg := c.config()
	t := c.tallyStatus()
	c.mu.Lock()
	open := companyOpenIn(t, l.conn.TallyCompany)
	c.mu.Unlock()
	var r itHeartbeatReply
	err := c.call(ctx, cfg.ServerURL, l.conn.Token, "POST", "/api/bridge/heartbeat", map[string]any{
		"tally_ok": t.Status == "ok" && (open == nil || *open), "version": Version, "client": "TallyBridge",
		"posting": cfg.PostingEnabled, "phone": !cfg.PhoneOff, "companies": t.Companies}, &r, 20*time.Second)
	if ctx.Err() != nil {
		return
	}
	if err != nil {
		msg := friendlyNetErr(err)
		c.mu.Lock()
		changed := l.server.Detail != msg
		l.server = itStatus{"down", msg}
		c.mu.Unlock()
		if changed {
			c.log(msg, "server", false, l)
		}
		return
	}
	c.mu.Lock()
	l.server = itStatus{"ok", "Connected"}
	l.pending = r.Pending
	l.companyOpen = companyOpenIn(t, r.TallyCompany)
	if r.Company != l.conn.Company || r.TallyCompany != l.conn.TallyCompany {
		l.conn.Company, l.conn.TallyCompany = r.Company, r.TallyCompany
		for i := range c.cfg.Connections {
			if c.cfg.Connections[i].ID == l.conn.ID {
				c.cfg.Connections[i].Company, c.cfg.Connections[i].TallyCompany = r.Company, r.TallyCompany
			}
		}
		_ = c.saveLocked()
	}
	c.mu.Unlock()
	if r.Pending > 0 {
		select {
		case l.wake <- struct{}{}:
		default:
		}
	}
}

type itJob struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	InvoiceID string `json:"invoice_id"`
	Payload   string `json:"payload"`
	Label     string `json:"label"`
}

func (c *ITConnector) jobLoop(ctx context.Context, l *itLink) {
	backoff := time.Duration(0)
	for ctx.Err() == nil {
		cfg := c.config()
		path := "/api/bridge/jobs?wait=20"
		if !cfg.PostingEnabled {
			path += "&kinds=export"
		}
		var r struct {
			Jobs []itJob `json:"jobs"`
		}
		err := c.call(ctx, l.serverURL, l.conn.Token, "GET", path, nil, &r, 45*time.Second)
		if err == nil {
			backoff = 0
			for _, j := range r.Jobs {
				if ctx.Err() != nil {
					return
				}
				c.handle(ctx, l, j)
			}
			if len(r.Jobs) > 0 {
				continue
			}
			backoff = 300 * time.Millisecond
		} else {
			var he *itHTTPError
			if errors.As(err, &he) && he.Code == 401 {
				backoff = 30 * time.Second
			} else if backoff < 2*time.Second {
				backoff = 2 * time.Second
			} else if backoff < 30*time.Second {
				backoff *= 2
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-l.wake:
		case <-time.After(backoff):
		}
	}
}

func (c *ITConnector) report(ctx context.Context, l *itLink, j itJob, result map[string]any) {
	cfg := c.config()
	for attempt := 0; attempt < 4; attempt++ {
		if err := c.call(ctx, cfg.ServerURL, l.conn.Token, "POST", "/api/bridge/jobs/"+url.PathEscape(j.ID), result, nil, 2*time.Minute); err == nil {
			return
		}
		if !sleepCtx(ctx, time.Duration(2*(attempt+1))*time.Second) {
			return
		}
	}
}

func (c *ITConnector) waitNote(l *itLink, text, kind string) {
	c.mu.Lock()
	due := time.Since(l.waitingLogged) > 2*time.Minute
	if due {
		l.waitingLogged = time.Now()
	}
	c.mu.Unlock()
	if due {
		c.log(text, kind, false, l)
	}
}

func (c *ITConnector) bump(l *itLink, key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if l.stats.Day != today() {
		l.stats = itStats{Day: today(), LastSync: l.stats.LastSync}
	}
	switch key {
	case "posted":
		l.stats.PostedToday++
	case "failed":
		l.stats.FailedToday++
	case "masters":
		l.stats.MastersToday++
	}
	l.stats.LastSync = time.Now()
}

func (c *ITConnector) handle(ctx context.Context, l *itLink, j itJob) {
	label := j.Label
	if label == "" {
		label = j.Kind
	}
	c.mu.Lock()
	tc := l.conn.TallyCompany
	c.mu.Unlock()
	// Never post into whichever company happens to be open: wait for this one.
	if open := companyOpenIn(c.tallyStatusAt(true), tc); open != nil && !*open {
		msg := fmt.Sprintf("Waiting for “%s” to be opened in Tally", tc)
		c.waitNote(l, msg+" — "+label+" will run then", j.Kind)
		c.report(ctx, l, j, map[string]any{"ok": false, "retry": true, "error": msg})
		sleepCtx(ctx, c.retryPause)
		return
	}
	if strings.HasPrefix(j.Kind, "export_") {
		c.handleExport(ctx, l, j, label, tc)
		return
	}
	if j.Kind != "import_voucher" && j.Kind != "import_master" {
		c.report(ctx, l, j, map[string]any{"ok": false, "error": "TallyBridge does not know this job type"})
		return
	}
	if !c.config().PostingEnabled {
		msg := "Posting is switched off in TallyBridge on the PC"
		c.waitNote(l, msg+" — "+label+" is waiting", j.Kind)
		c.report(ctx, l, j, map[string]any{"ok": false, "retry": true, "error": msg})
		sleepCtx(ctx, c.retryPause)
		return
	}
	// From here on the job must finish even if settings change mid-way.
	ctx = context.WithoutCancel(ctx)
	if d, seen := c.journalGet(j.ID); seen {
		res := d.Result
		if d.State != "done" || res == nil {
			res = map[string]any{"ok": false, "error": "TallyBridge stopped while posting this. Check in Tally whether it was saved before pushing again."}
		}
		c.log(label+" — already sent to Tally earlier; reporting that result again (not posted twice)", j.Kind, true, l)
		c.report(ctx, l, j, res)
		return
	}
	what, err := validatePostingXML([]byte(j.Payload), tc)
	if err != nil {
		msg := "TallyBridge refused this request: " + err.Error()
		c.log(label+" — "+msg, j.Kind, false, l)
		c.bump(l, "failed")
		c.report(ctx, l, j, map[string]any{"ok": false, "error": msg})
		return
	}
	if err := c.journalSet(j.ID, itDone{At: time.Now(), State: "running"}); err != nil {
		c.report(ctx, l, j, map[string]any{"ok": false, "retry": true, "error": "TallyBridge could not save its posting log: " + err.Error()})
		sleepCtx(ctx, c.retryPause)
		return
	}
	resp, err := c.s.tally.post(ctx, "InvoiceTotally: "+what, tc, []byte(j.Payload), 2*time.Minute)
	finish := func(res map[string]any) {
		_ = c.journalSet(j.ID, itDone{At: time.Now(), State: "done", Result: res})
		c.report(ctx, l, j, res)
	}
	switch {
	case errors.Is(err, errPostingUnreachable) || errors.Is(err, errPostingGate):
		c.mu.Lock()
		c.tally = itTally{Status: "down", Detail: "TallyPrime is not running, or its connectivity port (9000) is off", Companies: []string{}}
		c.tallyAt, c.fails = time.Now(), 2
		c.mu.Unlock()
		if errors.Is(err, errPostingGate) {
			c.mu.Lock()
			c.tally.Detail = "Tally stayed busy — will try again"
			c.mu.Unlock()
		}
		c.journalDelete(j.ID) // nothing reached Tally, so it is safe to run again
		c.waitNote(l, "Waiting for Tally — "+label+" will be posted when TallyPrime is open", j.Kind)
		c.report(ctx, l, j, map[string]any{"ok": false, "retry": true, "error": "Tally not reachable"})
		sleepCtx(ctx, c.retryPause)
		return
	case err != nil:
		c.log(label+" — "+err.Error(), j.Kind, false, l)
		c.bump(l, "failed")
		finish(map[string]any{"ok": false, "error": err.Error()})
		return
	}
	ok, errs := importOutcome(resp)
	finish(map[string]any{"ok": true, "response": resp})
	if ok {
		if j.Kind == "import_voucher" {
			c.bump(l, "posted")
			c.log("Posted "+label, j.Kind, true, l)
		} else {
			c.bump(l, "masters")
			c.log("Created "+label, j.Kind, true, l)
		}
	} else {
		c.bump(l, "failed")
		if errs == "" {
			errs = "unknown reason"
		}
		c.log("Tally rejected "+label+": "+errs, j.Kind, false, l)
	}
}

func (c *ITConnector) handleExport(ctx context.Context, l *itLink, j itJob, label, tc string) {
	if _, ok := itExportTags[j.Kind]; !ok {
		c.report(ctx, l, j, map[string]any{"ok": false, "error": "unknown master type"})
		return
	}
	waitCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	b, err := c.s.tally.requestWait(waitCtx, "InvoiceTotally: "+label, tc, []byte(j.Payload), 5*time.Minute)
	cancel()
	if err != nil {
		if errors.Is(err, ErrTallyRequest) {
			c.log(label+" failed: "+err.Error(), j.Kind, false, l)
			c.report(ctx, l, j, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		c.waitNote(l, "Waiting for Tally — "+label+" will run when TallyPrime answers", j.Kind)
		c.report(ctx, l, j, map[string]any{"ok": false, "retry": true, "error": "Tally not reachable"})
		sleepCtx(ctx, c.retryPause)
		return
	}
	rows, err := parseITMasters(j.Kind, b)
	if err != nil {
		c.log(label+" failed: "+err.Error(), j.Kind, false, l)
		c.report(ctx, l, j, map[string]any{"ok": false, "error": "Could not read Tally's reply: " + err.Error()})
		return
	}
	cfg := c.config()
	for part, i := 0, 0; i < len(rows) || part == 0; part, i = part+1, i+400 {
		end := i + 400
		if end > len(rows) {
			end = len(rows)
		}
		chunk := rows[i:end]
		if err := c.call(ctx, cfg.ServerURL, l.conn.Token, "POST", "/api/bridge/jobs/"+url.PathEscape(j.ID)+"/masters",
			map[string]any{"part": part, "rows": chunk}, nil, 2*time.Minute); err != nil {
			c.log(label+" failed: "+friendlyNetErr(err), j.Kind, false, l)
			c.report(ctx, l, j, map[string]any{"ok": false, "error": "Could not upload masters: " + friendlyNetErr(err)})
			return
		}
		if end >= len(rows) {
			break
		}
	}
	c.report(ctx, l, j, map[string]any{"ok": true, "response": fmt.Sprintf("%d records", len(rows))})
	c.mu.Lock()
	l.stats.LastSync = time.Now()
	c.mu.Unlock()
	c.log(fmt.Sprintf("%s: %d found", label, len(rows)), j.Kind, true, l)
}

// ----------------------------------------------------------------- views

type itConnView struct {
	ID           string   `json:"id"`
	Hint         string   `json:"hint"`
	Company      string   `json:"company"`
	TallyCompany string   `json:"tallyCompany"`
	Server       itStatus `json:"server"`
	Pending      int      `json:"pending"`
	CompanyOpen  *bool    `json:"companyOpen"`
	Stats        itStats  `json:"stats"`
}

func (c *ITConnector) snapshot() map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	conns := []itConnView{}
	for _, l := range c.links {
		st := l.stats
		if st.Day != today() {
			st = itStats{Day: today(), LastSync: st.LastSync}
		}
		conns = append(conns, itConnView{ID: l.conn.ID, Hint: tokenHint(l.conn.Token), Company: l.conn.Company, TallyCompany: l.conn.TallyCompany,
			Server: l.server, Pending: l.pending, CompanyOpen: l.companyOpen, Stats: st})
	}
	act := c.activity
	if len(act) > 120 {
		act = act[:120]
	}
	return map[string]any{
		"serverUrl": c.cfg.ServerURL, "postingEnabled": c.cfg.PostingEnabled, "phoneUploads": !c.cfg.PhoneOff,
		"configured":  c.cfg.ServerURL != "" && len(c.cfg.Connections) > 0,
		"connections": conns, "tally": c.tally, "activity": append([]ITActivity{}, act...),
	}
}

// phoneCompanies lists the companies the phone can send bills to. Tokens never leave the PC.
func (c *ITConnector) phoneCompanies() []map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := []map[string]any{}
	for _, l := range c.links {
		name := l.conn.Company
		if name == "" {
			name = "Company " + tokenHint(l.conn.Token)
		}
		out = append(out, map[string]any{"id": l.conn.ID, "name": name, "tallyCompany": l.conn.TallyCompany, "online": l.server.Status == "ok"})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(out[i]["name"].(string)) < strings.ToLower(out[j]["name"].(string))
	})
	return out
}

func (c *ITConnector) connection(id string) (ITConnection, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, x := range c.cfg.Connections {
		if x.ID == id {
			return x, true
		}
	}
	return ITConnection{}, false
}

// ----------------------------------------------------------------- setup

type itTestResult struct {
	Hint         string `json:"hint"`
	OK           bool   `json:"ok"`
	Company      string `json:"company"`
	TallyCompany string `json:"tallyCompany"`
	Detail       string `json:"detail"`
	Token        string `json:"-"`
}

func (c *ITConnector) testTokens(ctx context.Context, server string, tokens []string) []itTestResult {
	out := make([]itTestResult, len(tokens))
	var wg sync.WaitGroup
	for i, t := range tokens {
		wg.Add(1)
		go func(i int, t string) {
			defer wg.Done()
			r := itTestResult{Hint: tokenHint(t), Token: t}
			if !validITToken(t) {
				r.Detail = "This doesn't look like a bridge token (it starts with itb_)"
				out[i] = r
				return
			}
			var hb itHeartbeatReply
			if err := c.call(ctx, server, t, "POST", "/api/bridge/heartbeat", map[string]any{"test": true, "client": "TallyBridge", "version": Version}, &hb, 15*time.Second); err != nil {
				r.Detail = friendlyNetErr(err)
			} else {
				r.OK, r.Company, r.TallyCompany = true, hb.Company, hb.TallyCompany
				r.Detail = "Connected — “" + hb.Company + "”"
			}
			out[i] = r
		}(i, t)
	}
	wg.Wait()
	return out
}

type itLegacyConfig struct {
	ServerURL   string `json:"server_url"`
	TallyURL    string `json:"tally_url"`
	BridgeToken string `json:"bridge_token"`
	Connections []struct {
		Token string `json:"token"`
	} `json:"connections"`
}

func legacyBridgeConfig() (server string, tokens []string, found bool) {
	base := os.Getenv("APPDATA")
	if base == "" {
		h, _ := os.UserHomeDir()
		base = h
	}
	b, err := os.ReadFile(filepath.Join(base, "InvoiceTotallyBridge", "config.json"))
	if err != nil {
		return "", nil, false
	}
	var x itLegacyConfig
	if json.Unmarshal(b, &x) != nil {
		return "", nil, false
	}
	seen := map[string]bool{}
	for _, t := range append([]string{x.BridgeToken}, func() []string {
		o := []string{}
		for _, c := range x.Connections {
			o = append(o, c.Token)
		}
		return o
	}()...) {
		t = strings.TrimSpace(t)
		if t != "" && !seen[t] {
			seen[t] = true
			tokens = append(tokens, t)
		}
	}
	return x.ServerURL, tokens, x.ServerURL != "" && len(tokens) > 0
}

func legacyBridgeRunning() bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", itLegacyUIPort), 300*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func (s *Server) handleITAdmin(w http.ResponseWriter, r *http.Request) {
	c := s.it
	sub := strings.TrimPrefix(r.URL.Path, "/api/v1/admin/invoicetotally")
	switch {
	case sub == "" && r.Method == http.MethodGet:
		snap := c.snapshot()
		snap["uploads"] = s.bills.list(true)
		lsrv, ltok, found := legacyBridgeConfig()
		_ = lsrv
		snap["legacy"] = map[string]any{"found": found, "tokens": len(ltok), "running": legacyBridgeRunning()}
		jsonOut(w, snap)
	case sub == "/test" && r.Method == http.MethodPost:
		var q struct {
			ServerURL string   `json:"serverUrl"`
			Tokens    []string `json:"tokens"`
		}
		if readJSON(r, &q) != nil {
			jsonErr(w, 400, errors.New("invalid request"))
			return
		}
		server, err := normaliseServerURL(q.ServerURL)
		if err != nil {
			jsonErr(w, 400, err)
			return
		}
		jsonOut(w, map[string]any{"results": c.testTokens(r.Context(), server, cleanTokens(q.Tokens))})
	case sub == "/save" && r.Method == http.MethodPost:
		var q struct {
			ServerURL string   `json:"serverUrl"`
			Add       []string `json:"add"`
			Remove    []string `json:"remove"`
		}
		if readJSON(r, &q) != nil {
			jsonErr(w, 400, errors.New("invalid request"))
			return
		}
		res, err := c.save(r.Context(), q.ServerURL, cleanTokens(q.Add), q.Remove)
		if err != nil {
			jsonErr(w, 400, err)
			return
		}
		jsonOut(w, map[string]any{"ok": true, "results": res})
	case sub == "/import-legacy" && r.Method == http.MethodPost:
		server, tokens, found := legacyBridgeConfig()
		if !found {
			jsonErr(w, 404, errors.New("no InvoiceTotally Bridge settings were found on this PC"))
			return
		}
		res, err := c.save(r.Context(), server, tokens, nil)
		if err != nil {
			jsonErr(w, 400, err)
			return
		}
		jsonOut(w, map[string]any{"ok": true, "results": res, "legacyRunning": legacyBridgeRunning()})
	case sub == "/options" && r.Method == http.MethodPost:
		var q struct {
			PostingEnabled *bool `json:"postingEnabled"`
			PhoneUploads   *bool `json:"phoneUploads"`
		}
		if readJSON(r, &q) != nil {
			jsonErr(w, 400, errors.New("invalid request"))
			return
		}
		c.mu.Lock()
		before := c.cfg.PostingEnabled
		if q.PostingEnabled != nil {
			c.cfg.PostingEnabled = *q.PostingEnabled
		}
		if q.PhoneUploads != nil {
			c.cfg.PhoneOff = !*q.PhoneUploads
		}
		err := c.saveLocked()
		after := c.cfg.PostingEnabled
		links := append([]*itLink(nil), c.links...)
		c.mu.Unlock()
		if err != nil {
			jsonErr(w, 500, err)
			return
		}
		if before != after {
			if after {
				c.log("Posting to Tally switched ON — reviewed bills from InvoiceTotally will be posted", "settings", true, nil)
			} else {
				c.log("Posting to Tally switched OFF — TallyBridge will only read masters", "settings", true, nil)
			}
		}
		// Each company's next poll (within ~20 s) asks with the new choice; wake any that are pausing.
		for _, l := range links {
			for _, ch := range []chan struct{}{l.wake, l.beat} {
				select {
				case ch <- struct{}{}:
				default:
				}
			}
		}
		jsonOut(w, c.snapshot())
	case sub == "/uploads/retry" && r.Method == http.MethodPost, sub == "/uploads/delete" && r.Method == http.MethodPost:
		var q struct {
			ID string `json:"id"`
		}
		if readJSON(r, &q) != nil {
			jsonErr(w, 400, errors.New("invalid request"))
			return
		}
		var err error
		if strings.HasSuffix(sub, "retry") {
			err = s.bills.retry(q.ID)
		} else {
			err = s.bills.remove(q.ID)
		}
		if err != nil {
			jsonErr(w, 400, err)
			return
		}
		jsonOut(w, map[string]any{"ok": true})
	default:
		http.NotFound(w, r)
	}
}

func cleanTokens(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, t := range in {
		for _, part := range strings.FieldsFunc(t, func(r rune) bool { return r == ',' || r == '\n' || r == ' ' || r == '\t' || r == '\r' }) {
			if part != "" && !seen[part] {
				seen[part] = true
				out = append(out, part)
			}
		}
	}
	return out
}

// save checks every new token against the web app before keeping it, so a typo
// never ends up as a silently failing company.
func (c *ITConnector) save(ctx context.Context, rawServer string, add, remove []string) ([]itTestResult, error) {
	server, err := normaliseServerURL(rawServer)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	cfg := c.cfg
	cfg.Connections = append([]ITConnection(nil), c.cfg.Connections...)
	c.mu.Unlock()
	drop := map[string]bool{}
	for _, id := range remove {
		drop[id] = true
	}
	keep := []ITConnection{}
	have := map[string]bool{}
	for _, x := range cfg.Connections {
		if !drop[x.ID] {
			keep = append(keep, x)
			have[x.Token] = true
		}
	}
	fresh := []string{}
	for _, t := range add {
		if !have[t] {
			fresh = append(fresh, t)
		}
	}
	if len(keep)+len(fresh) > 30 {
		return nil, errors.New("up to 30 companies can be connected")
	}
	// Re-test kept tokens too when the web address changes.
	toTest := fresh
	if server != cfg.ServerURL {
		toTest = append([]string{}, fresh...)
		for _, x := range keep {
			toTest = append(toTest, x.Token)
		}
	}
	results := c.testTokens(ctx, server, toTest)
	byToken := map[string]itTestResult{}
	for _, r := range results {
		byToken[r.Token] = r
		if !r.OK {
			return results, fmt.Errorf("company token %s: %s", r.Hint, r.Detail)
		}
	}
	for _, t := range fresh {
		r := byToken[t]
		keep = append(keep, ITConnection{ID: randomHex(8), Token: t, Company: r.Company, TallyCompany: r.TallyCompany})
	}
	for i := range keep {
		if r, ok := byToken[keep[i].Token]; ok {
			keep[i].Company, keep[i].TallyCompany = r.Company, r.TallyCompany
		}
	}
	c.mu.Lock()
	c.cfg.ServerURL = server
	c.cfg.Connections = keep
	err = c.saveLocked()
	c.mu.Unlock()
	if err != nil {
		return results, err
	}
	names := []string{}
	for _, k := range keep {
		names = append(names, k.Company)
	}
	c.log(fmt.Sprintf("Connected to InvoiceTotally — %d compan%s: %s", len(keep), map[bool]string{true: "y", false: "ies"}[len(keep) == 1], strings.Join(names, ", ")), "settings", true, nil)
	c.Start()
	return results, nil
}
