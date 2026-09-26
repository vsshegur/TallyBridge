package app

import (
	"crypto/hmac"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed web/*
var webFS embed.FS

type Server struct {
	native          *NativeSecurity
	store           *Store
	cache           *CacheStore
	tally           *TallyClient
	mu              sync.Mutex
	auto            AutoSyncState
	nonces          map[string]map[string]time.Time
	tailscaleOnline bool
	tailscaleStatus string
	phoneURL        string
}

func NewServer(base string) (*Server, error) {
	st, e := NewStore(base)
	if e != nil {
		return nil, e
	}
	c, e := NewCacheStore(base)
	if e != nil {
		return nil, e
	}
	n, e := newNative(base)
	if e != nil {
		return nil, e
	}
	s := &Server{store: st, cache: c, native: n, nonces: map[string]map[string]time.Time{}}
	s.tally = NewTallyClient(st)
	return s, nil
}
func (s *Server) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("/", s.handleRoot)
	m.HandleFunc("/api/v1/admin/cloudflare", s.localAdmin(s.handleCloudConfig))
	m.HandleFunc("/api/v1/admin/native-device", s.localAdmin(s.handleNativeApproval))
	m.HandleFunc("/admin", s.localAdmin(s.serveFile("web/admin.html", "text/html; charset=utf-8")))
	m.HandleFunc("/phone", s.serveFile("web/phone.html", "text/html; charset=utf-8"))
	m.HandleFunc("/sw.js", s.serveFile("web/sw.js", "application/javascript; charset=utf-8"))
	m.HandleFunc("/manifest.webmanifest", s.serveFile("web/manifest.webmanifest", "application/manifest+json"))
	m.HandleFunc("/assets/", s.handleAsset)
	m.HandleFunc("/api/v1/public/live", s.handlePublicLive)
	m.HandleFunc("/api/v1/pair", s.handlePair)
	m.HandleFunc("/api/v1/admin/snapshot", s.localAdmin(s.handleAdminSnapshot))
	m.HandleFunc("/api/v1/admin/probe", s.localAdmin(s.handleAdminProbe))
	m.HandleFunc("/api/v1/admin/paircode", s.localAdmin(s.handlePairCode))
	m.HandleFunc("/api/v1/admin/revoke", s.localAdmin(s.handleRevoke))
	m.HandleFunc("/api/v1/admin/pdf-bank", s.localAdmin(s.handlePDFBank))
	m.HandleFunc("/api/v1/admin/settings", s.localAdmin(s.handleSettings))
	m.HandleFunc("/api/v1/admin/sync", s.localAdmin(s.handleAdminSync))
	m.HandleFunc("/api/v1/admin/report", s.localAdmin(s.handleAdminReport))
	m.HandleFunc("/api/v1/admin/tailscale/check", s.localAdmin(s.handleTailscaleCheck))
	m.HandleFunc("/api/v1/admin/tailscale/configure", s.localAdmin(s.handleTailscaleConfigure))
	m.HandleFunc("/api/v1/mobile/auth-check", s.mobileAuth(s.handleAuthCheck))
	m.HandleFunc("/api/v1/mobile/bootstrap", s.mobileAuth(s.handleBootstrap))
	m.HandleFunc("/api/v1/mobile/status", s.mobileAuth(s.handleMobileStatus))
	m.HandleFunc("/api/v1/mobile/sync", s.mobileAuth(s.handleMobileSync))
	m.HandleFunc("/api/v1/mobile/probe", s.mobileAuth(s.handleMobileProbe))
	m.HandleFunc("/api/v1/mobile/ledgers", s.mobileAuth(s.handleMobileLedgers))
	m.HandleFunc("/api/v1/mobile/outstanding", s.mobileAuth(s.handleMobileOutstanding))
	m.HandleFunc("/api/v1/mobile/party-outstanding", s.mobileAuth(s.handlePartyOutstanding))
	m.HandleFunc("/api/v1/mobile/vouchers", s.mobileAuth(s.handleMobileVouchers))
	m.HandleFunc("/api/v1/mobile/voucher-detail", s.mobileAuth(s.handleMobileVoucherDetail))
	m.HandleFunc("/api/v1/mobile/voucher-pdf", s.mobileAuth(s.handleMobileVoucherPDF))
	m.HandleFunc("/api/v1/mobile/ledger-details", s.mobileAuth(s.handleLedgerDetails))
	m.HandleFunc("/api/v1/mobile/ledger-statement", s.mobileAuth(s.handleLedgerStatement))
	m.HandleFunc("/api/v1/mobile/ledger-statement-pdf", s.mobileAuth(s.handleLedgerStatementPDF))
	m.HandleFunc("/api/v1/mobile/ledger-outstanding", s.mobileAuth(s.handleLedgerOutstanding))
	m.HandleFunc("/api/v1/mobile/ledger-outstanding-pdf", s.mobileAuth(s.handleLedgerOutstandingPDF))
	m.HandleFunc("/api/v1/mobile/group-balances", s.mobileAuth(s.handleGroupBalances))
	m.HandleFunc("/api/v1/mobile/group-balances-pdf", s.mobileAuth(s.handleGroupBalancesPDF))
	m.HandleFunc("/api/v1/mobile/customer360", s.mobileAuth(s.handleCustomer360))
	m.HandleFunc("/api/v1/mobile/customer360-pdf", s.mobileAuth(s.handleCustomer360PDF))
	m.HandleFunc("/api/v1/mobile/today-dispatch", s.mobileAuth(s.handleTodayDispatch))
	m.HandleFunc("/api/v1/mobile/search-index", s.mobileAuth(s.handleSearchIndex))
	m.HandleFunc("/api/v1/mobile/ageing", s.mobileAuth(s.handleAgeing))
	m.HandleFunc("/api/v1/mobile/banks", s.mobileAuth(s.handleBanks))
	m.HandleFunc("/api/v1/mobile/bank-details", s.mobileAuth(s.handleBankDetails))
	m.HandleFunc("/api/v1/mobile/report", s.mobileAuth(s.handleMobileReport))
	return securityHeaders(m)
}
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-src 'self' blob:; worker-src 'self' blob:; object-src 'none'; base-uri 'self'; form-action 'self'")
		next.ServeHTTP(w, r)
	})
}
func (s *Server) serveFile(name, ct string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, e := webFS.ReadFile(name)
		if e != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", ct)
		_, _ = w.Write(b)
	}
}
func (s *Server) handleAsset(w http.ResponseWriter, r *http.Request) {
	name := "web/" + strings.TrimPrefix(r.URL.Path, "/assets/")
	b, e := webFS.ReadFile(name)
	if e != nil {
		http.NotFound(w, r)
		return
	}
	ct := "application/octet-stream"
	switch {
	case strings.HasSuffix(name, ".js") || strings.HasSuffix(name, ".mjs"):
		ct = "application/javascript; charset=utf-8"
	case strings.HasSuffix(name, ".css"):
		ct = "text/css; charset=utf-8"
	case strings.HasSuffix(name, ".svg"):
		ct = "image/svg+xml"
	case strings.HasSuffix(name, ".png"):
		ct = "image/png"
	}
	w.Header().Set("Content-Type", ct)
	_, _ = w.Write(b)
}
func localHostRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
	}
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(peer)
	return ip != nil && ip.IsLoopback() && (host == "localhost" || host == "127.0.0.1" || host == "::1") && r.Header.Get("Cf-Access-Jwt-Assertion") == "" && r.Header.Get("X-Forwarded-For") == ""
}
func (s *Server) localAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !localHostRequest(r) || r.Header.Get("Sec-Fetch-Site") == "cross-site" || (r.Header.Get("Origin") != "" && r.Header.Get("Origin") != "http://"+r.Host) {
			jsonErr(w, 403, errors.New("desktop admin is localhost only"))
			return
		}
		if r.Method != "GET" && r.Header.Get("X-TB-Admin") != "1" {
			jsonErr(w, 403, errors.New("desktop setup request header missing"))
			return
		}
		next(w, r)
	}
}
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if localHostRequest(r) {
		http.Redirect(w, r, "/admin", http.StatusFound)
	} else {
		http.Redirect(w, r, "/phone", http.StatusFound)
	}
}
func jsonOut(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}
func jsonErr(w http.ResponseWriter, code int, e error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": e.Error()})
}
func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(v)
}
func (s *Server) stateSnapshot() LiveState {
	st := s.tally.State()
	s.mu.Lock()
	defer s.mu.Unlock()
	st.TailscaleOnline = s.tailscaleOnline
	st.TailscaleStatus = s.tailscaleStatus
	st.PhoneURL = s.phoneURL
	return st
}
func (s *Server) handlePublicLive(w http.ResponseWriter, r *http.Request) {
	st := s.stateSnapshot()
	jsonOut(w, map[string]any{"bridgeOnline": true, "tallyOnline": st.TallyOnline, "version": Version})
}
func (s *Server) handlePair(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonErr(w, 405, errors.New("POST required"))
		return
	}
	var q struct {
		Code string `json:"code"`
		Name string `json:"name"`
	}
	if readJSON(r, &q) != nil {
		jsonErr(w, 400, errors.New("invalid request"))
		return
	}
	d, e := s.store.Pair(strings.TrimSpace(q.Code), strings.TrimSpace(q.Name))
	if e != nil {
		jsonErr(w, 401, e)
		return
	}
	jsonOut(w, map[string]any{"deviceId": d.ID, "deviceKey": d.Key, "name": d.Name})
}
func (s *Server) handleAdminSnapshot(w http.ResponseWriter, r *http.Request) {
	cfg, devs := s.store.Snapshot()
	for i := range devs {
		devs[i].Key = ""
	}
	cc, at := s.cache.Companies()
	jsonOut(w, map[string]any{"live": s.stateSnapshot(), "settings": cfg, "devices": devs, "logs": s.tally.Logs(), "cache": s.cache.Stats(), "cachedCompanies": cc, "cachedCompaniesAt": at, "autoSync": s.autoSyncSnapshot()})
}
func (s *Server) handleAdminProbe(w http.ResponseWriter, r *http.Request) {
	st, e := s.probeLive()
	if e != nil {
		jsonErr(w, 503, e)
		return
	}
	jsonOut(w, st)
}
func (s *Server) handlePairCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonErr(w, 405, errors.New("POST required"))
		return
	}
	c, e := s.store.GeneratePairCode()
	jsonOut(w, map[string]any{"code": c, "expiresAt": e})
}
func (s *Server) handleRevoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonErr(w, 405, errors.New("POST required"))
		return
	}
	var q struct {
		ID string `json:"id"`
	}
	if readJSON(r, &q) != nil || q.ID == "" {
		jsonErr(w, 400, errors.New("device id required"))
		return
	}
	if e := s.store.Revoke(q.ID); e != nil {
		jsonErr(w, 500, e)
		return
	}
	jsonOut(w, map[string]any{"ok": true})
}
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonErr(w, 405, errors.New("POST required"))
		return
	}
	var q struct {
		TallyURL    string `json:"tallyUrl"`
		TimeoutSec  int    `json:"timeoutSec"`
		CooldownSec int    `json:"cooldownSec"`
	}
	if readJSON(r, &q) != nil {
		jsonErr(w, 400, errors.New("invalid settings"))
		return
	}
	if e := validateLocalTallyURL(q.TallyURL); e != nil {
		jsonErr(w, 400, e)
		return
	}
	if q.TimeoutSec < 2 || q.TimeoutSec > 20 || q.CooldownSec < 5 || q.CooldownSec > 120 {
		jsonErr(w, 400, errors.New("timeout/cooldown out of range"))
		return
	}
	cfg := s.store.Settings()
	cfg.TallyURL = q.TallyURL
	cfg.TimeoutSec = q.TimeoutSec
	cfg.CooldownSec = q.CooldownSec
	if e := s.store.UpdateSettings(cfg); e != nil {
		jsonErr(w, 500, e)
		return
	}
	jsonOut(w, map[string]any{"ok": true})
}
func (s *Server) handleAdminSync(w http.ResponseWriter, r *http.Request) {
	var q struct {
		Company string `json:"company"`
	}
	_ = readJSON(r, &q)
	started := s.startAutoSync(q.Company, "desktop")
	jsonOut(w, map[string]any{"started": started, "autoSync": s.autoSyncSnapshot()})
}
func (s *Server) handleAdminReport(w http.ResponseWriter, r *http.Request) {
	c := r.URL.Query().Get("company")
	k := r.URL.Query().Get("kind")
	var v any
	var e error
	switch k {
	case "ledgers":
		v, e = s.getLedgers(c, true)
	case "receivable":
		v, e = s.getOutstanding(c, "receivable", true)
	case "payable":
		v, e = s.getOutstanding(c, "payable", true)
	case "sales":
		v, e = s.getVouchers(c, "sales", "month", true)
	default:
		e = errors.New("unknown report")
	}
	if e != nil {
		jsonErr(w, 503, e)
		return
	}
	jsonOut(w, v)
}
func (s *Server) handleTailscaleCheck(w http.ResponseWriter, r *http.Request) {
	on, status, url, e := checkTailscale(s.store.Settings().Port)
	s.mu.Lock()
	s.tailscaleOnline = on
	s.tailscaleStatus = status
	if url != "" {
		s.phoneURL = url
	}
	s.mu.Unlock()
	if e != nil {
		jsonErr(w, 503, e)
		return
	}
	jsonOut(w, map[string]any{"online": on, "status": status, "phoneUrl": url})
}
func (s *Server) handleTailscaleConfigure(w http.ResponseWriter, r *http.Request) {
	on, status, url, e := configureTailscale(s.store.Settings().Port)
	s.mu.Lock()
	s.tailscaleOnline = on
	s.tailscaleStatus = status
	if url != "" {
		s.phoneURL = url
	}
	s.mu.Unlock()
	if e != nil {
		jsonErr(w, 503, e)
		return
	}
	jsonOut(w, map[string]any{"online": on, "status": status, "phoneUrl": url})
}

func (s *Server) probeLive() (LiveState, error) {
	b, _, _, e := s.tally.request("Live probe", "", companyListXML(), 6*time.Second)
	if e != nil {
		return s.stateSnapshot(), e
	}
	cs := parseCompanies(b)
	s.tally.mu.Lock()
	s.tally.companies = cs
	s.tally.mu.Unlock()
	st := s.stateSnapshot()
	return st, nil
}

func (s *Server) mobileAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Context().Value(nativeContextKey{}) != nil {
			if _, e := s.verifyNativeRequest(r, true); e != nil {
				jsonErr(w, 403, e)
				return
			}
			next(w, r)
			return
		}
		id := r.Header.Get("X-TB-Device")
		d, ok := s.store.Device(id)
		if !ok {
			jsonErr(w, 401, errors.New("device not authorized"))
			return
		}
		ts, err := strconv.ParseInt(r.Header.Get("X-TB-Time"), 10, 64)
		if err != nil || absDuration(time.Since(time.Unix(ts, 0))) > 2*time.Minute {
			jsonErr(w, 401, errors.New("request timestamp invalid"))
			return
		}
		nonce := r.Header.Get("X-TB-Nonce")
		if len(nonce) < 16 {
			jsonErr(w, 401, errors.New("nonce invalid"))
			return
		}
		body, _ := io.ReadAll(io.LimitReader(r.Body, 2<<20))
		r.Body = io.NopCloser(strings.NewReader(string(body)))
		bh := sha256.Sum256(body)
		msg := strings.Join([]string{r.Method, r.URL.RequestURI(), r.Header.Get("X-TB-Time"), nonce, hex.EncodeToString(bh[:])}, "\n")
		key, err := hex.DecodeString(d.Key)
		if err != nil {
			jsonErr(w, 401, errors.New("device key invalid"))
			return
		}
		mac := hmac.New(sha256.New, key)
		_, _ = mac.Write([]byte(msg))
		want := hex.EncodeToString(mac.Sum(nil))
		if !hmac.Equal([]byte(strings.ToLower(r.Header.Get("X-TB-Signature"))), []byte(want)) {
			jsonErr(w, 401, errors.New("signature invalid"))
			return
		}
		if !s.acceptNonce(id, nonce) {
			jsonErr(w, 401, errors.New("replayed request"))
			return
		}
		s.store.Touch(id)
		next(w, r)
	}
}
func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
func (s *Server) acceptNonce(id, n string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.nonces[id]
	if m == nil {
		m = map[string]time.Time{}
		s.nonces[id] = m
	}
	now := time.Now()
	for k, t := range m {
		if now.Sub(t) > 3*time.Minute {
			delete(m, k)
		}
	}
	if _, ok := m[n]; ok {
		return false
	}
	m[n] = now
	return true
}
func (s *Server) handleAuthCheck(w http.ResponseWriter, r *http.Request) {
	jsonOut(w, map[string]any{"ok": true})
}
func (s *Server) bootstrapPayload() map[string]any {
	st := s.stateSnapshot()
	cc, _ := s.cache.Companies()
	cs := st.Companies
	if len(cs) == 0 {
		cs = cc
	}
	return map[string]any{"bridgeOnline": true, "tallyOnline": st.TallyOnline, "companies": cs, "lastSuccess": st.LastSuccess, "cacheLatestAt": s.cache.Stats().LatestAt, "summaries": s.summaries(cs), "autoSync": s.autoSyncSnapshot(), "version": Version, "pdfBanks": s.store.PDFBankNames()}
}
func (s *Server) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	s.ensureAutoSync()
	jsonOut(w, s.bootstrapPayload())
}
func (s *Server) handleMobileStatus(w http.ResponseWriter, r *http.Request) {
	jsonOut(w, s.bootstrapPayload())
}
func (s *Server) handleMobileSync(w http.ResponseWriter, r *http.Request) {
	var q struct {
		Company string `json:"company"`
		Reason  string `json:"reason"`
	}
	_ = readJSON(r, &q)
	started := s.startAutoSync(q.Company, q.Reason)
	jsonOut(w, map[string]any{"started": started, "autoSync": s.autoSyncSnapshot()})
}
func (s *Server) handleMobileProbe(w http.ResponseWriter, r *http.Request) {
	st, e := s.probeLive()
	if e != nil {
		jsonErr(w, 503, e)
		return
	}
	jsonOut(w, st)
}

func (s *Server) handleMobileLedgers(w http.ResponseWriter, r *http.Request) {
	c := r.URL.Query().Get("company")
	v, e := s.getLedgers(c, r.URL.Query().Get("refresh") == "1")
	if e != nil {
		if errors.Is(e, osErrNotExist) {
			jsonErr(w, 404, e)
		} else {
			jsonErr(w, 503, e)
		}
		return
	}
	jsonOut(w, v)
}
func (s *Server) handleMobileOutstanding(w http.ResponseWriter, r *http.Request) {
	c := r.URL.Query().Get("company")
	typ := r.URL.Query().Get("type")
	v, e := s.getOutstanding(c, typ, r.URL.Query().Get("refresh") == "1")
	if e != nil {
		if errors.Is(e, osErrNotExist) {
			jsonErr(w, 404, e)
		} else {
			jsonErr(w, 503, e)
		}
		return
	}
	jsonOut(w, v)
}
func (s *Server) handlePartyOutstanding(w http.ResponseWriter, r *http.Request) {
	c, p := r.URL.Query().Get("company"), r.URL.Query().Get("party")
	if c == "" || p == "" {
		jsonErr(w, 400, errors.New("company and party are required"))
		return
	}
	x, e := s.getLedgerOutstanding(c, p)
	if e != nil {
		jsonErr(w, 503, e)
		return
	}
	po := PartyOutstandingData{CommonData: x.CommonData, Party: p, Bills: x.Bills, Total: x.Total, Count: x.Count, SummaryOnly: x.SummaryOnly, BalanceFrom: x.BalanceFrom, DrCr: x.DrCr}
	if !x.SummaryOnly && x.Live {
		_ = s.cache.Put("party-outstanding", c, p, po, x.FetchedAt)
	}
	jsonOut(w, po)
}

func (s *Server) handleMobileVouchers(w http.ResponseWriter, r *http.Request) {
	c := r.URL.Query().Get("company")
	typ := r.URL.Query().Get("type")
	period := r.URL.Query().Get("period")
	v, e := s.getVouchers(c, typ, period, r.URL.Query().Get("refresh") == "1")
	if e != nil {
		if errors.Is(e, osErrNotExist) {
			jsonErr(w, 404, e)
		} else {
			jsonErr(w, 503, e)
		}
		return
	}
	jsonOut(w, v)
}
func (s *Server) handleMobileVoucherDetail(w http.ResponseWriter, r *http.Request) {
	c := r.URL.Query().Get("company")
	mid := r.URL.Query().Get("masterId")
	if c == "" || mid == "" {
		jsonErr(w, 400, errors.New("company and masterId required"))
		return
	}
	if reason, retry, p := s.tally.Pending(); p {
		var old VoucherDetail
		if at, ok := s.cache.Get("voucher-detail", c, mid, &old); ok {
			old.Live = false
			old.Source = "saved"
			old.Stale = true
			old.FetchedAt = at
			jsonOut(w, old)
			return
		}
		jsonOut(w, map[string]any{"pending": true, "reason": reason, "retryAfterMs": retry.Milliseconds()})
		return
	}
	hint := VoucherRow{MasterID: mid, Number: r.URL.Query().Get("number"), Date: normISODate(r.URL.Query().Get("date")), Party: r.URL.Query().Get("party"), Amount: parseAmount(r.URL.Query().Get("amount"))}
	x, e := s.fetchVoucherDetail(c, mid, hint)
	if e != nil {
		var old VoucherDetail
		if at, ok := s.cache.Get("voucher-detail", c, mid, &old); ok {
			old.Live = false
			old.Source = "saved"
			old.Stale = true
			old.FetchedAt = at
			old.LiveError = e.Error()
			jsonOut(w, old)
			return
		}
		jsonErr(w, 503, e)
		return
	}
	jsonOut(w, x)
}
func (s *Server) handleMobileVoucherPDF(w http.ResponseWriter, r *http.Request) {
	c := r.URL.Query().Get("company")
	mid := r.URL.Query().Get("masterId")
	var x VoucherDetail
	if _, ok := s.cache.Get("voucher-detail", c, mid, &x); !ok {
		var e error
		x, e = s.fetchVoucherDetail(c, mid, VoucherRow{})
		if e != nil {
			jsonErr(w, 503, e)
			return
		}
	}
	bank, e := s.optionalBank(c, r.URL.Query().Get("bank"))
	if e != nil {
		jsonErr(w, 503, e)
		return
	}
	pdf, pe := invoicePDF(x, bank...)
	if pe != nil {
		jsonErr(w, 503, pe)
		return
	}
	name := sanitizeFileName(firstNonBlank(x.InvoiceNumber, "Invoice"))
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="Invoice-%s.pdf"`, name))
	_, _ = w.Write(pdf)
}
func (s *Server) handleLedgerDetails(w http.ResponseWriter, r *http.Request) {
	c, l := r.URL.Query().Get("company"), r.URL.Query().Get("ledger")
	if c == "" || l == "" {
		jsonErr(w, 400, errors.New("company and ledger required"))
		return
	}
	b, d, _, e := s.tally.request("Ledger details", c, ledgerDetailXML(c, l), 0)
	if e != nil {
		var old map[string]any
		if at, ok := s.cache.Get("ledger-details", c, l, &old); ok {
			old["live"] = false
			old["source"] = "saved"
			old["stale"] = true
			old["fetchedAt"] = at
			old["liveError"] = e.Error()
			jsonOut(w, old)
			return
		}
		jsonErr(w, 503, e)
		return
	}
	details := parseLedgerDetails(b)
	if len(details) == 0 {
		jsonErr(w, 503, errors.New("Tally returned no recognized ledger details"))
		return
	}
	x := map[string]any{"live": true, "source": "live", "fetchedAt": time.Now(), "durationMs": d, "details": details}
	_ = s.cache.Put("ledger-details", c, l, x, time.Now())
	jsonOut(w, x)
}
func queryDateRange(r *http.Request) (isoFrom, isoTo, tallyFrom, tallyTo string, err error) {
	isoFrom = strings.TrimSpace(r.URL.Query().Get("from"))
	isoTo = strings.TrimSpace(r.URL.Query().Get("to"))
	if isoFrom == "" && isoTo == "" {
		tallyFrom, tallyTo = financialYearRange(time.Now())
		isoFrom, isoTo = normISODate(tallyFrom), normISODate(tallyTo)
		return
	}
	if isoFrom == "" || isoTo == "" {
		err = errors.New("both from and to dates are required")
		return
	}
	f, e1 := time.Parse("2006-01-02", isoFrom)
	t, e2 := time.Parse("2006-01-02", isoTo)
	if e1 != nil || e2 != nil || f.After(t) {
		err = errors.New("invalid date range")
		return
	}
	tallyFrom, tallyTo = f.Format("20060102"), t.Format("20060102")
	return
}

func (s *Server) handleLedgerStatement(w http.ResponseWriter, r *http.Request) {
	c, l := r.URL.Query().Get("company"), r.URL.Query().Get("ledger")
	if c == "" || l == "" {
		jsonErr(w, 400, errors.New("company and ledger required"))
		return
	}
	isoFrom, isoTo, tf, tt, e := queryDateRange(r)
	if e != nil {
		jsonErr(w, 400, e)
		return
	}
	x, e := s.getLedgerStatement(c, l, isoFrom, isoTo, tf, tt)
	if e != nil {
		jsonErr(w, 503, e)
		return
	}
	jsonOut(w, x)
}
func (s *Server) handleLedgerStatementPDF(w http.ResponseWriter, r *http.Request) {
	c, l := r.URL.Query().Get("company"), r.URL.Query().Get("ledger")
	if c == "" || l == "" {
		jsonErr(w, 400, errors.New("company and ledger required"))
		return
	}
	isoFrom, isoTo, tf, tt, e := queryDateRange(r)
	if e != nil {
		jsonErr(w, 400, e)
		return
	}
	key := l + "|" + isoFrom + "|" + isoTo
	var x LedgerStatementData
	if _, ok := s.cache.Get("ledger-statement", c, key, &x); !ok {
		x, e = s.getLedgerStatement(c, l, isoFrom, isoTo, tf, tt)
		if e != nil {
			jsonErr(w, 503, e)
			return
		}
	}
	bank, be := s.optionalBank(c, r.URL.Query().Get("bank"))
	if be != nil {
		jsonErr(w, 503, be)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="Ledger-%s-%s-to-%s.pdf"`, sanitizeFileName(l), isoFrom, isoTo))
	data, pe := ledgerStatementPDF(x, bank...)
	if pe != nil {
		w.Header().Del("Content-Disposition")
		jsonErr(w, 503, pe)
		return
	}
	_, _ = w.Write(data)
}

func (s *Server) cachedPartyBalance(company, ledger string) (float64, string, bool) {
	for _, item := range []struct {
		kind  string
		label string
	}{
		{kind: "receivable-party", label: "receivable ledger balance"},
		{kind: "payable-party", label: "payable ledger balance"},
	} {
		var x OutstandingData
		if _, ok := s.cache.Get(item.kind, company, "", &x); !ok {
			continue
		}
		for _, p := range x.Parties {
			if strings.EqualFold(strings.TrimSpace(p.Party), strings.TrimSpace(ledger)) && p.Amount != 0 {
				return abs(p.Amount), item.label, true
			}
		}
	}
	return 0, "", false
}

func (s *Server) getLedgerOutstanding(c, l string) (LedgerOutstandingData, error) {
	b, d, _, err := s.tally.request("Ledger outstandings", c, partyOutstandingXML(c, l), 10*time.Second)
	if err == nil {
		rows := parsePartyBills(b, l)
		if len(rows) > 0 {
			net := 0.0
			for _, v := range rows {
				net += v.SignedAmount
			}
			x := LedgerOutstandingData{CommonData: CommonData{Live: true, Source: "live", FetchedAt: time.Now(), DurationMS: d}, Company: c, Ledger: l, Bills: rows, Total: abs(net), DrCr: balanceSide(net), Count: len(rows), BalanceFrom: "bill-wise Ledger Outstandings"}
			x.ShareText = ledgerOutstandingShareText(x)
			_ = s.cache.Put("ledger-outstanding", c, l, x, x.FetchedAt)
			_ = s.cache.Put("ledger-outstanding-display", c, l, x, x.FetchedAt)
			return x, nil
		}
		err = errors.New("Tally did not return a recognized bill-wise breakup")
	}
	// Prefer the known party total; it may be newer than previously opened bill details.
	for _, kind := range []string{"receivable-party", "payable-party"} {
		var index OutstandingData
		if at, ok := s.cache.Get(kind, c, "", &index); ok {
			for _, party := range index.Parties {
				if strings.EqualFold(strings.TrimSpace(party.Party), strings.TrimSpace(l)) {
					x := LedgerOutstandingData{CommonData: CommonData{Source: "saved", Stale: true, FetchedAt: at, LiveError: err.Error()}, Company: c, Ledger: l, Total: party.Amount, DrCr: party.DrCr, SummaryOnly: true, BalanceFrom: "saved party ledger balance"}
					x.ShareText = ledgerOutstandingShareText(x)
					_ = s.cache.Put("ledger-outstanding-display", c, l, x, x.FetchedAt)
					return x, nil
				}
			}
		}
	}
	var old LedgerOutstandingData
	if at, ok := s.cache.Get("ledger-outstanding", c, l, &old); ok && (!old.SummaryOnly && len(old.Bills) > 0 || old.Total != 0) {
		old.CommonData = CommonData{Source: "saved", Stale: true, FetchedAt: at, LiveError: err.Error()}
		old.ShareText = ledgerOutstandingShareText(old)
		_ = s.cache.Put("ledger-outstanding-display", c, l, old, old.FetchedAt)
		return old, nil
	}
	return LedgerOutstandingData{}, fmt.Errorf("outstanding not updated: %w; no verified saved balance is available", err)
}

func (s *Server) handleLedgerOutstanding(w http.ResponseWriter, r *http.Request) {
	c, l := r.URL.Query().Get("company"), r.URL.Query().Get("ledger")
	if c == "" || l == "" {
		jsonErr(w, 400, errors.New("company and ledger required"))
		return
	}
	x, e := s.getLedgerOutstanding(c, l)
	if e != nil {
		jsonErr(w, 503, e)
		return
	}
	jsonOut(w, x)
}
func (s *Server) handleLedgerOutstandingPDF(w http.ResponseWriter, r *http.Request) {
	c, l := r.URL.Query().Get("company"), r.URL.Query().Get("ledger")
	if c == "" || l == "" {
		jsonErr(w, 400, errors.New("company and ledger required"))
		return
	}
	var x LedgerOutstandingData
	var e error
	if _, ok := s.cache.Get("ledger-outstanding-display", c, l, &x); !ok {
		x, e = s.getLedgerOutstanding(c, l)
		if e != nil {
			jsonErr(w, 503, e)
			return
		}
	}
	bank, be := s.optionalBank(c, r.URL.Query().Get("bank"))
	if be != nil {
		jsonErr(w, 503, be)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="Outstanding-%s.pdf"`, sanitizeFileName(l)))
	data, pe := ledgerOutstandingPDF(x, bank...)
	if pe != nil {
		w.Header().Del("Content-Disposition")
		jsonErr(w, 503, pe)
		return
	}
	_, _ = w.Write(data)
}
func (s *Server) getGroupBalances(c, g string) (GroupBalanceData, error) {
	b, d, _, e := s.tally.request("Group balances", c, groupBalancesXML(c, g), 0)
	if e != nil {
		var old GroupBalanceData
		if at, ok := s.cache.Get("group-balances", c, g, &old); ok {
			old.Live = false
			old.Source = "saved"
			old.Stale = true
			old.FetchedAt = at
			old.LiveError = e.Error()
			return old, nil
		}
		return GroupBalanceData{}, e
	}
	rows := parseGroupBalances(b)
	total := 0.0
	for _, v := range rows {
		if v.DrCr == "Dr" {
			total -= v.Balance
		} else {
			total += v.Balance
		}
	}
	x := GroupBalanceData{CommonData: CommonData{Live: true, Source: "live", FetchedAt: time.Now(), DurationMS: d}, Company: c, Group: g, Ledgers: rows, Total: total, Count: len(rows)}
	x.ShareText = groupBalanceShareText(x)
	_ = s.cache.Put("group-balances", c, g, x, x.FetchedAt)
	return x, nil
}
func (s *Server) handleGroupBalances(w http.ResponseWriter, r *http.Request) {
	c, g := r.URL.Query().Get("company"), r.URL.Query().Get("group")
	if c == "" || g == "" {
		jsonErr(w, 400, errors.New("company and group required"))
		return
	}
	x, e := s.getGroupBalances(c, g)
	if e != nil {
		jsonErr(w, 503, e)
		return
	}
	jsonOut(w, x)
}
func (s *Server) handleGroupBalancesPDF(w http.ResponseWriter, r *http.Request) {
	c, g := r.URL.Query().Get("company"), r.URL.Query().Get("group")
	if c == "" || g == "" {
		jsonErr(w, 400, errors.New("company and group required"))
		return
	}
	var x GroupBalanceData
	var e error
	if _, ok := s.cache.Get("group-balances", c, g, &x); !ok {
		x, e = s.getGroupBalances(c, g)
		if e != nil {
			jsonErr(w, 503, e)
			return
		}
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="Group-%s.pdf"`, sanitizeFileName(g)))
	data, pe := groupBalancesPDF(x)
	if pe != nil {
		w.Header().Del("Content-Disposition")
		jsonErr(w, 503, pe)
		return
	}
	_, _ = w.Write(data)
}

func (s *Server) getBankDetails(c, ledger string) (BankDetails, error) {
	if strings.TrimSpace(ledger) == "" {
		return BankDetails{}, errors.New("bank ledger required")
	}
	var old BankDetails
	if _, ok := s.cache.Get("bank-details", c, ledger, &old); ok && old.AccountNumber != "" {
		return old, nil
	}
	b, _, _, e := s.tally.request("Bank details", c, ledgerDetailXML(c, ledger), 0)
	if e != nil {
		if _, ok := s.cache.Get("bank-details", c, ledger, &old); ok {
			return old, nil
		}
		return BankDetails{}, e
	}
	x := bankDetailsFromMap(ledger, parseLedgerDetails(b))
	_ = s.cache.Put("bank-details", c, ledger, x, time.Now())
	return x, nil
}

func (s *Server) optionalBank(c, ignoredPhoneSelection string) ([]BankDetails, error) {
	// Desktop is the sole source of PDF bank settings. Old phone parameters are ignored.
	bank, ok := s.store.PDFBank(c)
	if !ok {
		return nil, nil
	}
	return []BankDetails{bank}, nil
}

func (s *Server) handleBanks(w http.ResponseWriter, r *http.Request) {
	c := strings.TrimSpace(r.URL.Query().Get("company"))
	if c == "" {
		jsonErr(w, 400, errors.New("company required"))
		return
	}
	idx, e := s.getLedgers(c, false)
	if e != nil {
		jsonErr(w, 404, errors.New("ledger index is not updated yet"))
		return
	}
	banks := []BankDetails{}
	for _, l := range idx.Ledgers {
		p := strings.ToLower(strings.TrimSpace(l.Parent))
		if strings.Contains(p, "bank account") || p == "bank" || strings.Contains(p, "bank accounts") {
			banks = append(banks, BankDetails{Ledger: l.Name, BankName: l.Name})
		}
	}
	sort.Slice(banks, func(i, j int) bool { return strings.ToLower(banks[i].Ledger) < strings.ToLower(banks[j].Ledger) })
	jsonOut(w, BankListData{CommonData: CommonData{Live: false, Source: "saved", Stale: true, FetchedAt: idx.FetchedAt}, Company: c, Banks: banks, Count: len(banks)})
}

func (s *Server) handleBankDetails(w http.ResponseWriter, r *http.Request) {
	c, ledger := strings.TrimSpace(r.URL.Query().Get("company")), strings.TrimSpace(r.URL.Query().Get("ledger"))
	if c == "" || ledger == "" {
		jsonErr(w, 400, errors.New("company and bank ledger required"))
		return
	}
	x, e := s.getBankDetails(c, ledger)
	if e != nil {
		jsonErr(w, 503, e)
		return
	}
	jsonOut(w, x)
}

func (s *Server) handleTodayDispatch(w http.ResponseWriter, r *http.Request) {
	c := strings.TrimSpace(r.URL.Query().Get("company"))
	if c == "" {
		jsonErr(w, 400, errors.New("company required"))
		return
	}
	refresh := r.URL.Query().Get("refresh") != "0"
	x, e := s.getVouchers(c, "sales", "month", refresh)
	if e != nil {
		jsonErr(w, 503, e)
		return
	}
	today := time.Now().Format("2006-01-02")
	rows := make([]VoucherRow, 0)
	total := 0.0
	for _, v := range x.Vouchers {
		if v.Date == today {
			rows = append(rows, v)
			total += abs(v.Amount)
		}
	}
	x.Vouchers, x.Total, x.Count, x.Type, x.Period = rows, total, len(rows), "dispatch", "today"
	jsonOut(w, x)
}

func (s *Server) handleSearchIndex(w http.ResponseWriter, r *http.Request) {
	c := strings.TrimSpace(r.URL.Query().Get("company"))
	if c == "" {
		jsonErr(w, 400, errors.New("company required"))
		return
	}
	out := []SearchResult{}
	latest := time.Time{}
	var idx LedgerIndexData
	if at, ok := s.cache.Get("ledgers", c, "", &idx); ok {
		if at.After(latest) {
			latest = at
		}
		for _, l := range idx.Ledgers {
			out = append(out, SearchResult{Kind: "ledger", Title: l.Name, Subtitle: l.Parent, Ledger: l.Name})
		}
	}
	seenVoucher := map[string]bool{}
	for _, typ := range []string{"sales", "purchase"} {
		for _, period := range []string{"fy", "month"} {
			var vd VoucherData
			if at, ok := s.cache.Get(typ, c, period, &vd); ok {
				if at.After(latest) {
					latest = at
				}
				for _, v := range vd.Vouchers {
					key := typ + "|" + v.MasterID + "|" + v.Number + "|" + v.Date
					if seenVoucher[key] {
						continue
					}
					seenVoucher[key] = true
					out = append(out, SearchResult{Kind: typ, Title: firstNonBlank(v.Number, v.Party), Subtitle: v.Party + " • " + displayDate(v.Date), MasterID: v.MasterID, Party: v.Party, Number: v.Number, Date: v.Date, Amount: v.Amount, VoucherType: v.VoucherType})
				}
			}
		}
	}
	for _, d := range s.cache.Matching("party-outstanding", c) {
		var po PartyOutstandingData
		if json.Unmarshal(d.Data, &po) != nil {
			continue
		}
		if d.FetchedAt.After(latest) {
			latest = d.FetchedAt
		}
		for _, b := range po.Bills {
			out = append(out, SearchResult{Kind: "bill", Title: firstNonBlank(b.BillNumber, po.Party), Subtitle: po.Party + " • pending " + indianNumber(b.PendingAmount), Party: po.Party, Ledger: po.Party, Number: b.BillNumber, Date: b.Date, Amount: b.PendingAmount})
		}
	}
	for _, d := range s.cache.Matching("voucher-detail", c) {
		var v VoucherDetail
		if json.Unmarshal(d.Data, &v) != nil {
			continue
		}
		if d.FetchedAt.After(latest) {
			latest = d.FetchedAt
		}
		if v.LRNumber != "" {
			out = append(out, SearchResult{Kind: "invoice-detail", Title: firstNonBlank(v.InvoiceNumber, v.LRNumber), Subtitle: v.CustomerName + " • LR/RR " + v.LRNumber, MasterID: v.MasterID, Party: v.CustomerName, Number: v.InvoiceNumber, Date: v.InvoiceDate, LRNumber: v.LRNumber, Amount: v.TotalAmount, VoucherType: v.VoucherType})
		}
	}
	jsonOut(w, SearchIndexData{CommonData: CommonData{Live: false, Source: "saved", Stale: true, FetchedAt: latest}, Company: c, Results: out, Count: len(out)})
}

func parseISOOrZero(s string) time.Time {
	t, _ := time.Parse("2006-01-02", normISODate(s))
	return t
}

func (s *Server) handleAgeing(w http.ResponseWriter, r *http.Request) {
	c := strings.TrimSpace(r.URL.Query().Get("company"))
	if c == "" {
		jsonErr(w, 400, errors.New("company required"))
		return
	}
	buckets := []AgeingBucket{{Label: "0–30 days"}, {Label: "31–60 days"}, {Label: "61–90 days"}, {Label: "90+ days"}}
	total, bills, covered := 0.0, 0, 0
	latest := time.Time{}
	now := time.Now()
	for _, d := range s.cache.Matching("party-outstanding", c) {
		var x PartyOutstandingData
		if json.Unmarshal(d.Data, &x) != nil {
			continue
		}
		covered++
		if d.FetchedAt.After(latest) {
			latest = d.FetchedAt
		}
		for _, bill := range x.Bills {
			base := parseISOOrZero(firstNonBlank(bill.DueDate, bill.Date))
			age := 0
			if !base.IsZero() && now.After(base) {
				age = int(now.Sub(base).Hours() / 24)
			}
			i := 0
			switch {
			case age > 90:
				i = 3
			case age > 60:
				i = 2
			case age > 30:
				i = 1
			}
			amt := abs(bill.PendingAmount)
			buckets[i].Amount += amt
			buckets[i].Bills++
			total += amt
			bills++
		}
	}
	totalParties := 0
	var parties OutstandingData
	if _, ok := s.cache.Get("receivable-party", c, "", &parties); ok {
		totalParties = parties.Count
	}
	pct := 0
	if totalParties > 0 {
		pct = covered * 100 / totalParties
		if pct > 100 {
			pct = 100
		}
	}
	jsonOut(w, AgeingData{CommonData: CommonData{Live: false, Source: "saved", Stale: true, FetchedAt: latest}, Company: c, Buckets: buckets, Total: total, BillCount: bills, CoveredParties: covered, TotalParties: totalParties, CoveragePercent: pct})
}

func (s *Server) buildCustomer360(c, ledger string, refresh bool) (Customer360Data, error) {
	x := Customer360Data{Company: c, Ledger: ledger}
	latest := time.Time{}
	for _, kind := range []string{"receivable-party", "payable-party"} {
		var o OutstandingData
		if at, ok := s.cache.Get(kind, c, "", &o); ok {
			if at.After(latest) {
				latest = at
			}
			for _, p := range o.Parties {
				if strings.EqualFold(strings.TrimSpace(p.Party), strings.TrimSpace(ledger)) {
					x.Outstanding = abs(p.Amount)
					break
				}
			}
		}
	}
	var po PartyOutstandingData
	if at, ok := s.cache.Get("party-outstanding", c, ledger, &po); ok {
		if at.After(latest) {
			latest = at
		}
		x.PendingBillCount = po.Count
		now := time.Now()
		for _, b := range po.Bills {
			d := parseISOOrZero(firstNonBlank(b.DueDate, b.Date))
			if !d.IsZero() && d.Before(now) {
				x.Overdue += abs(b.PendingAmount)
			}
		}
	}
	var sales VoucherData
	if at, ok := s.cache.Get("sales", c, "fy", &sales); ok {
		if at.After(latest) {
			latest = at
		}
		for i := len(sales.Vouchers) - 1; i >= 0 && len(x.LatestInvoices) < 5; i-- {
			v := sales.Vouchers[i]
			if strings.EqualFold(strings.TrimSpace(v.Party), strings.TrimSpace(ledger)) {
				x.LatestInvoices = append(x.LatestInvoices, v)
			}
		}
	}
	from, to := financialYearRange(time.Now())
	isoFrom, isoTo := normISODate(from), normISODate(to)
	var st LedgerStatementData
	var e error
	if refresh {
		st, e = s.getLedgerStatement(c, ledger, isoFrom, isoTo, from, to)
	} else if at, ok := s.cache.Get("ledger-statement", c, ledger+"|"+isoFrom+"|"+isoTo, &st); ok {
		latest = at
	} else {
		e = osErrNotExist
	}
	if e == nil {
		if st.FetchedAt.After(latest) {
			latest = st.FetchedAt
		}
		vs := append([]VoucherRow(nil), st.Vouchers...)
		sort.Slice(vs, func(i, j int) bool { return vs[i].Date > vs[j].Date })
		if len(vs) > 8 {
			vs = vs[:8]
		}
		x.RecentActivity = vs
		for i := range vs {
			t := strings.ToLower(vs[i].VoucherType)
			if strings.Contains(t, "receipt") || strings.Contains(t, "payment") {
				v := vs[i]
				x.LastPayment = &v
				break
			}
		}
	}
	x.CommonData = CommonData{Live: e == nil && st.Live, Source: "saved", Stale: true, FetchedAt: latest}
	if e == nil {
		x.Source = st.Source
		x.Live = st.Live
		x.Stale = st.Stale
		if st.FetchedAt.After(x.FetchedAt) {
			x.FetchedAt = st.FetchedAt
		}
		x.LiveError = st.LiveError
	}
	x.ShareText = customer360ShareText(x)
	_ = s.cache.Put("customer360", c, ledger, x, time.Now())
	return x, nil
}

func (s *Server) handleCustomer360(w http.ResponseWriter, r *http.Request) {
	c, l := strings.TrimSpace(r.URL.Query().Get("company")), strings.TrimSpace(r.URL.Query().Get("ledger"))
	if c == "" || l == "" {
		jsonErr(w, 400, errors.New("company and ledger required"))
		return
	}
	x, e := s.buildCustomer360(c, l, r.URL.Query().Get("refresh") != "0")
	if e != nil && errors.Is(e, osErrNotExist) {
		var old Customer360Data
		if at, ok := s.cache.Get("customer360", c, l, &old); ok {
			old.FetchedAt = at
			jsonOut(w, old)
			return
		}
	}
	if e != nil {
		jsonErr(w, 503, e)
		return
	}
	jsonOut(w, x)
}

func (s *Server) handleCustomer360PDF(w http.ResponseWriter, r *http.Request) {
	c, l := strings.TrimSpace(r.URL.Query().Get("company")), strings.TrimSpace(r.URL.Query().Get("ledger"))
	if c == "" || l == "" {
		jsonErr(w, 400, errors.New("company and ledger required"))
		return
	}
	var x Customer360Data
	if _, ok := s.cache.Get("customer360", c, l, &x); !ok {
		var e error
		x, e = s.buildCustomer360(c, l, true)
		if e != nil {
			jsonErr(w, 503, e)
			return
		}
	}
	bank, e := s.optionalBank(c, r.URL.Query().Get("bank"))
	if e != nil {
		jsonErr(w, 503, e)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="Customer-%s.pdf"`, sanitizeFileName(l)))
	data, pe := customer360PDF(x, bank...)
	if pe != nil {
		w.Header().Del("Content-Disposition")
		jsonErr(w, 503, pe)
		return
	}
	_, _ = w.Write(data)
}

func (s *Server) handleMobileReport(w http.ResponseWriter, r *http.Request) {
	c, n := r.URL.Query().Get("company"), r.URL.Query().Get("name")
	if c == "" || n == "" {
		jsonErr(w, 400, errors.New("company and report required"))
		return
	}
	b, d, _, e := s.tally.request(n, c, genericReportXML(c, n), 0)
	if e != nil {
		var old map[string]any
		if at, ok := s.cache.Get("report", c, n, &old); ok {
			old["live"] = false
			old["source"] = "saved"
			old["stale"] = true
			old["fetchedAt"] = at
			old["liveError"] = e.Error()
			jsonOut(w, old)
			return
		}
		jsonErr(w, 503, e)
		return
	}
	x := map[string]any{"live": true, "source": "live", "fetchedAt": time.Now(), "durationMs": d, "rows": parseGenericRows(b)}
	_ = s.cache.Put("report", c, n, x, time.Now())
	jsonOut(w, x)
}

var osErrNotExist = errors.New("saved data not available yet")

func (s *Server) getLedgers(c string, refresh bool) (LedgerIndexData, error) {
	if c == "" {
		return LedgerIndexData{}, errors.New("company required")
	}
	if refresh {
		b, d, _, e := s.tally.request("Ledgers", c, ledgerIndexXML(c), 0)
		if e == nil && len(parseLedgers(b)) == 0 {
			e = errors.New("ledger response was empty or unrecognized; saved index kept")
		}
		if e == nil {
			xs := parseLedgers(b)
			x := LedgerIndexData{CommonData: CommonData{Live: true, Source: "live", FetchedAt: time.Now(), DurationMS: d}, Ledgers: xs, Count: len(xs)}
			_ = s.cache.Put("ledgers", c, "", x, x.FetchedAt)
			return x, nil
		} else {
			var old LedgerIndexData
			if at, ok := s.cache.Get("ledgers", c, "", &old); ok {
				old.Live = false
				old.Source = "saved"
				old.Stale = true
				old.FetchedAt = at
				old.LiveError = e.Error()
				return old, nil
			}
			return LedgerIndexData{}, e
		}
	}
	var x LedgerIndexData
	if at, ok := s.cache.Get("ledgers", c, "", &x); ok {
		x.Live = false
		x.Source = "saved"
		x.Stale = true
		x.FetchedAt = at
		return x, nil
	}
	return x, osErrNotExist
}
func (s *Server) getOutstanding(c, typ string, refresh bool) (OutstandingData, error) {
	if typ != "payable" {
		typ = "receivable"
	}
	kind := typ + "-party"
	if refresh {
		grp, id := "$$GroupSundryDebtors", "TBReceivableParties"
		if typ == "payable" {
			grp, id = "$$GroupSundryCreditors", "TBPayableParties"
		}
		b, d, _, e := s.tally.request(strings.Title(typ)+" party balances", c, partyBalancesXML(c, grp, id), 0)
		if e == nil && !hasLedgerBalances(b) {
			e = errors.New("party balance response was empty or unrecognized; saved balances kept")
		}
		if e == nil {
			ps := parsePartyBalances(b)
			total := 0.0
			for _, p := range ps {
				if typ == "receivable" && p.DrCr == "Cr" || typ == "payable" && p.DrCr == "Dr" {
					total -= p.Amount
				} else {
					total += p.Amount
				}
			}
			x := OutstandingData{CommonData: CommonData{Live: true, Source: "live", FetchedAt: time.Now(), DurationMS: d}, Parties: ps, Total: total, Count: len(ps), Type: typ}
			_ = s.cache.Put(kind, c, "", x, x.FetchedAt)
			return x, nil
		} else {
			var old OutstandingData
			if at, ok := s.cache.Get(kind, c, "", &old); ok {
				old.Live = false
				old.Source = "saved"
				old.Stale = true
				old.FetchedAt = at
				old.LiveError = e.Error()
				return old, nil
			}
			return OutstandingData{}, e
		}
	}
	var x OutstandingData
	if at, ok := s.cache.Get(kind, c, "", &x); ok {
		x.Live = false
		x.Source = "saved"
		x.Stale = true
		x.FetchedAt = at
		return x, nil
	}
	return x, osErrNotExist
}
func (s *Server) getVouchers(c, typ, period string, refresh bool) (VoucherData, error) {
	if typ != "purchase" {
		typ = "sales"
	}
	vtype := "Sales"
	if typ == "purchase" {
		vtype = "Purchase"
	}
	if period != "month" {
		period = "fy"
	}
	kind := typ
	variant := period
	if refresh {
		var from, to string
		if period == "month" {
			from, to = monthRange(time.Now())
		} else {
			from, to = financialYearRange(time.Now())
		}
		b, d, _, e := s.tally.request(strings.Title(typ)+" Register", c, voucherListXML(c, vtype, from, to), 0)
		if e == nil && len(parseVouchers(b)) == 0 {
			var previous VoucherData
			if !recognizedCollection(b) {
				e = errors.New("unrecognized register response; saved data kept")
			} else if _, ok := s.cache.Get(kind, c, variant, &previous); ok && previous.Count > 0 {
				e = errors.New("empty register response requires verification; previous saved register kept")
			}
		}
		if e == nil {
			vs := parseVouchers(b)
			total := 0.0
			for _, v := range vs {
				total += abs(v.Amount)
			}
			x := VoucherData{CommonData: CommonData{Live: true, Source: "live", FetchedAt: time.Now(), DurationMS: d}, Vouchers: vs, Total: total, Count: len(vs), Type: typ, Period: period}
			_ = s.cache.Put(kind, c, variant, x, x.FetchedAt)
			return x, nil
		} else {
			var old VoucherData
			if at, ok := s.cache.Get(kind, c, variant, &old); ok {
				old.Live = false
				old.Source = "saved"
				old.Stale = true
				old.FetchedAt = at
				old.LiveError = e.Error()
				return old, nil
			}
			return VoucherData{}, e
		}
	}
	var x VoucherData
	if at, ok := s.cache.Get(kind, c, variant, &x); ok {
		x.Live = false
		x.Source = "saved"
		x.Stale = true
		x.FetchedAt = at
		return x, nil
	}
	return x, osErrNotExist
}
func (s *Server) fetchVoucherDetail(c, mid string, supplied VoucherRow) (VoucherDetail, error) {
	if c == "" || !regexp.MustCompile(`^[0-9]+$`).MatchString(mid) {
		return VoucherDetail{}, errors.New("company and numeric voucher ID required")
	}

	hint, _ := s.cache.VoucherHint(c, mid)
	if supplied.Number != "" {
		hint.Number = supplied.Number
	}
	if supplied.Date != "" {
		hint.Date = supplied.Date
	}
	if supplied.Party != "" {
		hint.Party = supplied.Party
	}
	if supplied.Amount != 0 {
		hint.Amount = supplied.Amount
	}

	// First use a compact Voucher collection. These methods are widely supported
	// across TallyPrime versions and include the standard dispatch/LR fields.
	b, d1, _, err := s.tally.request("Invoice detail", c, voucherDetailCollectionXML(c, mid), 12*time.Second)
	var x VoucherDetail
	if err == nil && !hasXMLNode(b, "VOUCHER") {
		err = fmt.Errorf("%w: no voucher returned", ErrTallyRequest)
	}
	if err == nil {
		x = parseVoucherDetail(b, c, mid, hint)
	} else if !errors.Is(err, ErrTallyRequest) {
		return VoucherDetail{}, err
	}

	// If the compact response was rejected or omitted dispatch data, enrich from
	// the single Voucher object with FETCH *. This remains one voucher only and
	// runs sequentially, never in parallel. Request-specific ERRORMSG responses do
	// not trip the global safety cooldown.
	needObject := err != nil || x.InvoiceNumber == "" || x.InvoiceDate == "" || x.DispatchedThrough == "" || x.LRNumber == "" || len(x.Items) == 0
	if needObject {
		b2, d2, _, e2 := s.tally.request("Invoice detail enrich", c, voucherDetailObjectXML(c, mid), 12*time.Second)
		if e2 == nil && !hasXMLNode(b2, "VOUCHER") {
			e2 = errors.New("Tally returned no recognized voucher")
		}
		if e2 == nil {
			y := parseVoucherDetail(b2, c, mid, hint)
			x = mergeVoucherDetail(x, y)
			if d2 > d1 {
				d1 = d2
			}
		} else if err != nil { // both strategies failed
			return VoucherDetail{}, e2
		}
	}

	// Identity values shown in Sales Register are authoritative hints for the
	// selected row and guarantee share/PDF output even when Tally nests them in a
	// different XML level.
	if x.Company == "" {
		x.Company = c
	}
	if x.FirmName == "" {
		x.FirmName = c
	}
	if x.MasterID == "" {
		x.MasterID = mid
	}
	if x.InvoiceNumber == "" {
		x.InvoiceNumber = hint.Number
	}
	if x.InvoiceDate == "" {
		x.InvoiceDate = hint.Date
	}
	if x.CustomerName == "" {
		x.CustomerName = hint.Party
	}
	if x.TotalAmount == 0 {
		x.TotalAmount = abs(hint.Amount)
	}
	x.CommonData = CommonData{Live: true, Source: "live", FetchedAt: time.Now(), DurationMS: d1}
	x.ShareText = dispatchShareText(x)
	_ = s.cache.Put("voucher-detail", c, mid, x, x.FetchedAt)
	return x, nil
}

func mergeVoucherDetail(a, b VoucherDetail) VoucherDetail {
	if a.Company == "" {
		a.Company = b.Company
	}
	if a.FirmName == "" {
		a.FirmName = b.FirmName
	}
	if a.MasterID == "" {
		a.MasterID = b.MasterID
	}
	if a.InvoiceNumber == "" {
		a.InvoiceNumber = b.InvoiceNumber
	}
	if a.InvoiceDate == "" {
		a.InvoiceDate = b.InvoiceDate
	}
	if a.VoucherType == "" {
		a.VoucherType = b.VoucherType
	}
	if a.CustomerName == "" {
		a.CustomerName = b.CustomerName
	}
	if a.CustomerCity == "" {
		a.CustomerCity = b.CustomerCity
	}
	if len(a.CustomerAddress) == 0 {
		a.CustomerAddress = b.CustomerAddress
	}
	if a.GSTIN == "" {
		a.GSTIN = b.GSTIN
	}
	if a.DispatchedThrough == "" {
		a.DispatchedThrough = b.DispatchedThrough
	}
	if a.LRNumber == "" {
		a.LRNumber = b.LRNumber
	}
	if a.LRDate == "" {
		a.LRDate = b.LRDate
	}
	if a.Destination == "" {
		a.Destination = b.Destination
	}
	if a.VehicleNumber == "" {
		a.VehicleNumber = b.VehicleNumber
	}
	if a.EWayBillNumber == "" {
		a.EWayBillNumber = b.EWayBillNumber
	}
	if len(a.Items) == 0 {
		a.Items = b.Items
	}
	if a.DiscountAmount == 0 {
		a.DiscountAmount = b.DiscountAmount
	}
	if a.TaxableAmount == 0 {
		a.TaxableAmount = b.TaxableAmount
	}
	if a.CGST == 0 {
		a.CGST = b.CGST
	}
	if a.SGST == 0 {
		a.SGST = b.SGST
	}
	if a.IGST == 0 {
		a.IGST = b.IGST
	}
	if a.Freight == 0 {
		a.Freight = b.Freight
	}
	if a.RoundOff == 0 {
		a.RoundOff = b.RoundOff
	}
	if a.TotalAmount == 0 {
		a.TotalAmount = b.TotalAmount
	}
	if a.Narration == "" {
		a.Narration = b.Narration
	}
	return a
}

func sanitizeFileName(s string) string {
	r := regexpMust(`[^A-Za-z0-9._-]+`)
	s = r.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "Invoice"
	}
	return s
}
func regexpMust(s string) *regexp.Regexp { return regexp.MustCompile(s) }
func (s *Server) summaries(cs []Company) map[string]Summary {
	out := map[string]Summary{}
	for _, c := range cs {
		name := c.Name
		sum := Summary{CommonData: CommonData{Live: false, Source: "saved"}}
		var r, p OutstandingData
		var v VoucherData
		var latest time.Time
		if at, ok := s.cache.Get("receivable-party", name, "", &r); ok {
			sum.Receivable = &r.Total
			sum.RCount = &r.Count
			sum.RFetchedAt = at
			if at.After(latest) {
				latest = at
			}
		}
		if at, ok := s.cache.Get("payable-party", name, "", &p); ok {
			sum.Payable = &p.Total
			sum.PCount = &p.Count
			sum.PFetchedAt = at
			if at.After(latest) {
				latest = at
			}
		}
		if at, ok := s.cache.Get("sales", name, "month", &v); ok {
			sum.Sales = &v.Total
			sum.SCount = &v.Count
			sum.SFetchedAt = at
			if at.After(latest) {
				latest = at
			}
		}
		sum.FetchedAt = latest
		sum.Stale = true
		out[name] = sum
	}
	return out
}

func (s *Server) Settings() Settings { return s.store.Settings() }
func OpenDesktop(u string) error     { return openDesktop(u) }

// DesktopHandler is setup only. The legacy web phone and mobile API are not published.
func (s *Server) DesktopHandler() http.Handler {
	h := s.Handler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !localHostRequest(r) {
			http.Error(w, "Desktop setup is local only", 403)
			return
		}
		if r.URL.Path != "/" && r.URL.Path != "/admin" && !strings.HasPrefix(r.URL.Path, "/api/v1/admin/") && r.URL.Path != "/assets/admin.js" && r.URL.Path != "/assets/styles.css" && r.URL.Path != "/assets/favicon.svg" {
			http.NotFound(w, r)
			return
		}
		h.ServeHTTP(w, r)
	})
}
