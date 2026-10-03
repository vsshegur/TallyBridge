package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

type TallyPerformance struct {
	GateState       string    `json:"gateState"`
	BusyReport      string    `json:"busyReport,omitempty"`
	TotalReads      int       `json:"totalReads"`
	SuccessReads    int       `json:"successReads"`
	FailedReads     int       `json:"failedReads"`
	SuccessRate     int       `json:"successRate"`
	AverageMS       int64     `json:"averageMs"`
	P95MS           int64     `json:"p95Ms"`
	SlowReads       int       `json:"slowReads"`
	SanitizedChars  int       `json:"sanitizedChars"`
	CooldownSeconds int64     `json:"cooldownSeconds"`
	LastSuccess     time.Time `json:"lastSuccess,omitempty"`
	LastError       string    `json:"lastError,omitempty"`
}

type SmartAlert struct {
	ID       string `json:"id"`
	Severity string `json:"severity"`
	Title    string `json:"title"`
	Detail   string `json:"detail,omitempty"`
	Party    string `json:"party,omitempty"`
	DueDate  string `json:"dueDate,omitempty"`
	Action   string `json:"action,omitempty"`
}

type DailyWorkspace struct {
	Company          string           `json:"company"`
	Date             string           `json:"date"`
	PendingOrders    int              `json:"pendingOrders"`
	DispatchedToday  int              `json:"dispatchedToday"`
	OldPendingOrders int              `json:"oldPendingOrders"`
	FollowUpsDue     int              `json:"followUpsDue"`
	FollowUpsOverdue int              `json:"followUpsOverdue"`
	SalesToday       float64          `json:"salesToday"`
	SalesCount       int              `json:"salesCount"`
	Receivable       float64          `json:"receivable"`
	Payable          float64          `json:"payable"`
	TallyOnline      bool             `json:"tallyOnline"`
	LastSync         time.Time        `json:"lastSync,omitempty"`
	Alerts           []SmartAlert     `json:"alerts"`
	Health           CompanyHealth    `json:"health"`
	Performance      TallyPerformance `json:"performance"`
}

type FrequentItem struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type CustomerWorkspace struct {
	Customer         Customer360Data `json:"customer"`
	PendingOrders    []SalesOrder    `json:"pendingOrders"`
	DispatchedOrders []SalesOrder    `json:"dispatchedOrders"`
	FollowUps        []FollowUp      `json:"followUps"`
	LastPurchaseDate string          `json:"lastPurchaseDate,omitempty"`
	FrequentItems    []FrequentItem  `json:"frequentItems"`
}

type GlobalSearchResult struct {
	Kind        string  `json:"kind"`
	ID          string  `json:"id,omitempty"`
	Title       string  `json:"title"`
	Subtitle    string  `json:"subtitle,omitempty"`
	SearchText  string  `json:"searchText,omitempty"`
	Party       string  `json:"party,omitempty"`
	Ledger      string  `json:"ledger,omitempty"`
	Number      string  `json:"number,omitempty"`
	Date        string  `json:"date,omitempty"`
	MasterID    string  `json:"masterId,omitempty"`
	VoucherType string  `json:"voucherType,omitempty"`
	Status      string  `json:"status,omitempty"`
	Amount      float64 `json:"amount,omitempty"`
	Optional    bool    `json:"optional,omitempty"`
}

type GlobalSearchData struct {
	Company string               `json:"company"`
	Results []GlobalSearchResult `json:"results"`
	Count   int                  `json:"count"`
}

func (s *Server) tallyPerformance() TallyPerformance {
	logs := s.tally.Logs()
	st := s.stateSnapshot()
	x := TallyPerformance{GateState: st.GateState, BusyReport: st.BusyReport, LastSuccess: st.LastSuccess, LastError: st.LastError}
	if len(logs) > 100 {
		logs = logs[:100]
	}
	durations := make([]int64, 0, len(logs))
	var total int64
	for _, lg := range logs {
		x.TotalReads++
		total += lg.DurationMS
		durations = append(durations, lg.DurationMS)
		x.SanitizedChars += lg.SanitizedChars
		if lg.Success {
			x.SuccessReads++
		} else {
			x.FailedReads++
		}
		if lg.DurationMS >= 2000 {
			x.SlowReads++
		}
	}
	if x.TotalReads > 0 {
		x.AverageMS = total / int64(x.TotalReads)
		x.SuccessRate = x.SuccessReads * 100 / x.TotalReads
		sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
		idx := (len(durations)*95+99)/100 - 1
		if idx < 0 {
			idx = 0
		}
		if idx >= len(durations) {
			idx = len(durations) - 1
		}
		x.P95MS = durations[idx]
	}
	if reason, retry, pending := s.tally.Pending(); pending && reason == "cooldown" {
		x.CooldownSeconds = int64(retry.Round(time.Second) / time.Second)
		if x.CooldownSeconds < 1 {
			x.CooldownSeconds = 1
		}
	}
	return x
}

func cachedOutstandingTotal(s *Server, company, kind string) float64 {
	var x OutstandingData
	if _, ok := s.cache.Get(kind, company, "", &x); ok {
		return x.Total
	}
	return 0
}

func cachedTodaySales(s *Server, company string) (float64, int) {
	today := time.Now().Format("2006-01-02")
	for _, period := range []string{"month", "fy"} {
		var x VoucherData
		if _, ok := s.cache.Get("sales", company, period, &x); !ok || !voucherCacheIsClassified(x) {
			continue
		}
		var total float64
		count := 0
		for _, v := range x.Vouchers {
			if v.Date == today && !v.Optional {
				total += abs(v.Amount)
				count++
			}
		}
		return total, count
	}
	return 0, 0
}

func (s *Server) smartAlerts(company string) []SmartAlert {
	alerts := []SmartAlert{}
	st := s.stateSnapshot()
	if !st.TallyOnline {
		detail := "Saved data remains available."
		if strings.TrimSpace(st.LastError) != "" {
			detail = st.LastError
		}
		alerts = append(alerts, SmartAlert{ID: "tally-offline", Severity: "warning", Title: "Tally is unavailable", Detail: detail, Action: "diagnostics"})
	}
	sync := s.autoSyncSnapshot()
	if strings.TrimSpace(sync.LastError) != "" {
		alerts = append(alerts, SmartAlert{ID: "sync-error", Severity: "critical", Title: "Sync needs attention", Detail: sync.LastError, Action: "diagnostics"})
	}
	if orders, err := s.listOrders(company); err == nil {
		now := time.Now()
		for _, o := range orders.Pending {
			if !o.CreatedAt.IsZero() && now.Sub(o.CreatedAt) >= 48*time.Hour {
				alerts = append(alerts, SmartAlert{ID: "order-" + o.ID, Severity: "warning", Title: "Order pending for 2+ days", Detail: o.Number + " • " + o.Buyer, Party: o.Buyer, Action: "orders"})
			}
		}
	}
	if fs, err := s.listFollowUps(company, ""); err == nil {
		for _, f := range fs.Overdue {
			alerts = append(alerts, SmartAlert{ID: "followup-overdue-" + f.ID, Severity: "critical", Title: "Payment follow-up overdue", Detail: f.Party + " • promised " + f.PromiseDate, Party: f.Party, DueDate: f.PromiseDate, Action: "followups"})
		}
		for _, f := range fs.DueToday {
			alerts = append(alerts, SmartAlert{ID: "followup-today-" + f.ID, Severity: "warning", Title: "Payment follow-up due today", Detail: f.Party, Party: f.Party, DueDate: f.PromiseDate, Action: "followups"})
		}
	}
	overdueBills, overdueAmount := 0, 0.0
	overdueParties := map[string]bool{}
	today := time.Now().Format("2006-01-02")
	for _, d := range s.cache.Matching("party-outstanding", company) {
		var po PartyOutstandingData
		if json.Unmarshal(d.Data, &po) != nil {
			continue
		}
		for _, bill := range po.Bills {
			due := normISODate(firstNonBlank(bill.DueDate, bill.Date))
			if due != "" && due < today && abs(bill.PendingAmount) > 0 {
				overdueBills++
				overdueAmount += abs(bill.PendingAmount)
				overdueParties[po.Party] = true
			}
		}
	}
	if overdueBills > 0 {
		alerts = append(alerts, SmartAlert{ID: "customer-overdue", Severity: "warning", Title: "Customer payments overdue", Detail: fmt.Sprintf("%d bill(s) • INR %s across %d customer(s)", overdueBills, indianNumber(overdueAmount), len(overdueParties)), Action: "followups"})
	}
	h := s.companyHealth(company, false)
	missing := 0
	for _, item := range h.Items {
		if item.Required && !item.OK {
			missing++
		}
	}
	if missing > 0 {
		alerts = append(alerts, SmartAlert{ID: "company-health", Severity: "info", Title: "Company data needs attention", Detail: strings.TrimSpace(strings.Join(h.Issues, " • ")), Action: "health"})
	}
	return alerts
}

func (s *Server) dailyWorkspace(company string) DailyWorkspace {
	x := DailyWorkspace{Company: company, Date: time.Now().Format("2006-01-02")}
	st := s.stateSnapshot()
	x.TallyOnline = st.TallyOnline
	x.LastSync = s.autoSyncSnapshot().LastOK
	x.Receivable = cachedOutstandingTotal(s, company, "receivable-party")
	x.Payable = cachedOutstandingTotal(s, company, "payable-party")
	x.SalesToday, x.SalesCount = cachedTodaySales(s, company)
	if orders, err := s.listOrders(company); err == nil {
		x.PendingOrders = orders.PendingCount
		now := time.Now()
		for _, o := range orders.Pending {
			if !o.CreatedAt.IsZero() && now.Sub(o.CreatedAt) >= 48*time.Hour {
				x.OldPendingOrders++
			}
		}
		today := time.Now().Format("2006-01-02")
		for _, o := range orders.Dispatched {
			if !o.DispatchedAt.IsZero() && o.DispatchedAt.Local().Format("2006-01-02") == today {
				x.DispatchedToday++
			}
		}
	}
	if fs, err := s.listFollowUps(company, ""); err == nil {
		x.FollowUpsDue = fs.DueCount
		x.FollowUpsOverdue = fs.OverdueCount
	}
	x.Health = s.companyHealth(company, false)
	x.Performance = s.tallyPerformance()
	x.Alerts = s.smartAlerts(company)
	return x
}

func (s *Server) customerWorkspace(company, party string, refresh bool) (CustomerWorkspace, error) {
	base, err := s.buildCustomer360(company, party, refresh)
	if err != nil && !errors.Is(err, osErrNotExist) {
		return CustomerWorkspace{}, err
	}
	if errors.Is(err, osErrNotExist) {
		var old Customer360Data
		if _, ok := s.cache.Get("customer360", company, party, &old); ok {
			base = old
			err = nil
		}
	}
	out := CustomerWorkspace{Customer: base, PendingOrders: []SalesOrder{}, DispatchedOrders: []SalesOrder{}, FollowUps: []FollowUp{}}
	if orders, e := s.listOrders(company); e == nil {
		for _, o := range orders.Pending {
			if strings.EqualFold(strings.TrimSpace(o.Buyer), strings.TrimSpace(party)) {
				out.PendingOrders = append(out.PendingOrders, o)
			}
		}
		for _, o := range orders.Dispatched {
			if strings.EqualFold(strings.TrimSpace(o.Buyer), strings.TrimSpace(party)) {
				out.DispatchedOrders = append(out.DispatchedOrders, o)
			}
		}
	}
	if fs, e := s.listFollowUps(company, party); e == nil {
		out.FollowUps = append(out.FollowUps, fs.Open...)
	}
	counts := map[string]int{}
	for _, d := range s.cache.Matching("voucher-detail-v6", company) {
		var v VoucherDetail
		if json.Unmarshal(d.Data, &v) != nil || v.Optional || !strings.EqualFold(strings.TrimSpace(v.CustomerName), strings.TrimSpace(party)) {
			continue
		}
		vt := strings.ToLower(strings.TrimSpace(v.VoucherType))
		if vt != "" && !strings.Contains(vt, "sales") {
			continue
		}
		if dt := normISODate(v.InvoiceDate); dt > out.LastPurchaseDate {
			out.LastPurchaseDate = dt
		}
		for _, item := range v.Items {
			name := strings.TrimSpace(item.Name)
			if name != "" {
				counts[name]++
			}
		}
	}
	out.FrequentItems = []FrequentItem{}
	for name, count := range counts {
		out.FrequentItems = append(out.FrequentItems, FrequentItem{Name: name, Count: count})
	}
	sort.Slice(out.FrequentItems, func(i, j int) bool {
		if out.FrequentItems[i].Count == out.FrequentItems[j].Count {
			return strings.ToLower(out.FrequentItems[i].Name) < strings.ToLower(out.FrequentItems[j].Name)
		}
		return out.FrequentItems[i].Count > out.FrequentItems[j].Count
	})
	if len(out.FrequentItems) > 5 {
		out.FrequentItems = out.FrequentItems[:5]
	}
	return out, err
}

func searchText(parts ...string) string { return strings.ToLower(strings.Join(parts, " ")) }
func detailSubtitle(v VoucherDetail) string {
	parts := []string{}
	if strings.TrimSpace(v.CustomerName) != "" {
		parts = append(parts, v.CustomerName)
	}
	if strings.TrimSpace(v.LRNumber) != "" {
		parts = append(parts, "LR/RR "+v.LRNumber)
	}
	if strings.TrimSpace(v.GSTIN) != "" {
		parts = append(parts, "GSTIN "+v.GSTIN)
	}
	return strings.Join(parts, " • ")
}

func (s *Server) buildGlobalSearch(company string) GlobalSearchData {
	out := []GlobalSearchResult{}
	seen := map[string]bool{}
	var ledgers LedgerIndexData
	if _, ok := s.cache.Get("ledgers", company, "", &ledgers); ok {
		for _, l := range ledgers.Ledgers {
			key := "ledger|" + strings.ToLower(l.Name)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, GlobalSearchResult{Kind: "ledger", Title: l.Name, Subtitle: l.Parent, Ledger: l.Name, Party: l.Name, SearchText: searchText(l.Name, l.Parent)})
		}
	}
	for _, typ := range []string{"sales", "proforma", "purchase"} {
		for _, period := range []string{"month", "fy"} {
			var vd VoucherData
			if _, ok := s.cache.Get(typ, company, period, &vd); !ok || !voucherCacheIsClassified(vd) {
				continue
			}
			for _, v := range vd.Vouchers {
				key := typ + "|" + v.MasterID + "|" + v.Number + "|" + v.Date
				if seen[key] {
					continue
				}
				seen[key] = true
				out = append(out, GlobalSearchResult{Kind: typ, ID: v.MasterID, Title: firstNonBlank(v.Number, v.Party), Subtitle: v.Party + " • " + displayDate(v.Date), Party: v.Party, Ledger: v.Party, Number: v.Number, Date: v.Date, MasterID: v.MasterID, VoucherType: v.VoucherType, Amount: v.Amount, Optional: v.Optional, SearchText: searchText(v.Number, v.Party, v.Date, v.VoucherType)})
			}
		}
	}
	for _, d := range s.cache.Matching("voucher-detail-v6", company) {
		var v VoucherDetail
		if json.Unmarshal(d.Data, &v) != nil {
			continue
		}
		if v.LRNumber == "" && v.GSTIN == "" && len(v.Items) == 0 {
			continue
		}
		key := "detail|" + v.MasterID + "|" + v.InvoiceNumber
		if seen[key] {
			continue
		}
		seen[key] = true
		itemText := []string{}
		for _, item := range v.Items {
			itemText = append(itemText, item.Name, item.Description, item.HSN)
		}
		out = append(out, GlobalSearchResult{Kind: "invoice-detail", ID: v.MasterID, Title: firstNonBlank(v.InvoiceNumber, v.CustomerName), Subtitle: detailSubtitle(v), Party: v.CustomerName, Ledger: v.CustomerName, Number: v.InvoiceNumber, Date: v.InvoiceDate, MasterID: v.MasterID, VoucherType: v.VoucherType, Amount: v.TotalAmount, Optional: v.Optional, SearchText: searchText(v.InvoiceNumber, v.CustomerName, v.LRNumber, v.GSTIN, v.CustomerCity, v.VehicleNumber, v.EWayBillNumber, strings.Join(itemText, " "))})
	}
	if orders, err := s.listOrders(company); err == nil {
		addOrder := func(o SalesOrder) {
			items := make([]string, 0, len(o.Items))
			for _, it := range o.Items {
				items = append(items, it.Name)
			}
			out = append(out, GlobalSearchResult{Kind: "order", ID: o.ID, Title: firstNonBlank(o.Number, o.Buyer), Subtitle: o.Buyer + " • " + strings.ToUpper(o.Status), Party: o.Buyer, Number: o.Number, Status: o.Status, SearchText: searchText(o.Number, o.Buyer, o.DispatchedThrough, o.Notes, strings.Join(items, " "))})
		}
		for _, o := range orders.Pending {
			addOrder(o)
		}
		for _, o := range orders.Dispatched {
			addOrder(o)
		}
	}
	if fs, err := s.listFollowUps(company, ""); err == nil {
		all := append([]FollowUp{}, fs.Open...)
		all = append(all, fs.Completed...)
		for _, f := range all {
			date := firstNonBlank(f.PromiseDate, "No promise date")
			out = append(out, GlobalSearchResult{Kind: "followup", ID: f.ID, Title: f.Party, Subtitle: date + " • " + strings.ToUpper(f.Status), Party: f.Party, Date: f.PromiseDate, Status: f.Status, Amount: f.Amount, SearchText: searchText(f.Party, f.PromiseDate, f.Note, f.Status)})
		}
	}
	for _, d := range s.cache.Matching("ledger-details", company) {
		var x struct {
			Details map[string]string `json:"details"`
		}
		if json.Unmarshal(d.Data, &x) != nil || len(x.Details) == 0 {
			continue
		}
		name := firstNonBlank(x.Details["NAME"], x.Details["LEDGERNAME"], d.Variant)
		gst := firstNonBlank(x.Details["GSTIN"], x.Details["PARTYGSTIN"])
		phone := firstNonBlank(x.Details["MOBILE"], x.Details["MOBILENO"], x.Details["PHONE"])
		if gst == "" && phone == "" {
			continue
		}
		out = append(out, GlobalSearchResult{Kind: "party-detail", Title: name, Subtitle: strings.Trim(strings.Join([]string{gst, phone}, " • "), " •"), Party: name, Ledger: name, SearchText: searchText(name, gst, phone, x.Details["ADDRESS"], x.Details["CITY"])})
	}
	return GlobalSearchData{Company: company, Results: out, Count: len(out)}
}

func (s *Server) handleWorkspace(w http.ResponseWriter, r *http.Request) {
	c := strings.TrimSpace(r.URL.Query().Get("company"))
	if c == "" {
		jsonErr(w, 400, errors.New("company required"))
		return
	}
	jsonOut(w, s.dailyWorkspace(c))
}
func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	c := strings.TrimSpace(r.URL.Query().Get("company"))
	if c == "" {
		jsonErr(w, 400, errors.New("company required"))
		return
	}
	jsonOut(w, map[string]any{"company": c, "alerts": s.smartAlerts(c)})
}
func (s *Server) handleTallyPerformance(w http.ResponseWriter, r *http.Request) {
	jsonOut(w, s.tallyPerformance())
}
func (s *Server) handleCustomerWorkspace(w http.ResponseWriter, r *http.Request) {
	c, p := strings.TrimSpace(r.URL.Query().Get("company")), strings.TrimSpace(r.URL.Query().Get("party"))
	if c == "" || p == "" {
		jsonErr(w, 400, errors.New("company and party required"))
		return
	}
	x, err := s.customerWorkspace(c, p, r.URL.Query().Get("refresh") != "0")
	if err != nil {
		jsonErr(w, 503, err)
		return
	}
	jsonOut(w, x)
}
func (s *Server) handleGlobalSearch(w http.ResponseWriter, r *http.Request) {
	c := strings.TrimSpace(r.URL.Query().Get("company"))
	if c == "" {
		jsonErr(w, 400, errors.New("company required"))
		return
	}
	jsonOut(w, s.buildGlobalSearch(c))
}
