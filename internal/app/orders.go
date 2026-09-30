package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const dispatchedOrderRetention = 7 * 24 * time.Hour

type OrderItem struct {
	Name     string   `json:"name"`
	Unit     string   `json:"unit,omitempty"`
	Quantity float64  `json:"quantity"`
	Rate     *float64 `json:"rate,omitempty"`
}

type SalesOrder struct {
	ID           string            `json:"id"`
	Number       string            `json:"number"`
	Company      string            `json:"company"`
	Buyer        string            `json:"buyer"`
	BuyerDetails map[string]string `json:"buyerDetails,omitempty"`
	Items        []OrderItem       `json:"items"`
	Notes        string            `json:"notes,omitempty"`
	Status       string            `json:"status"`
	ShareText    string            `json:"shareText,omitempty"`
	CreatedAt    time.Time         `json:"createdAt"`
	UpdatedAt    time.Time         `json:"updatedAt"`
	DispatchedAt time.Time         `json:"dispatchedAt,omitempty"`
}

type orderFile struct {
	Orders []SalesOrder `json:"orders"`
}

type OrderList struct {
	Pending         []SalesOrder `json:"pending"`
	Dispatched      []SalesOrder `json:"dispatched"`
	PendingCount    int          `json:"pendingCount"`
	DispatchedCount int          `json:"dispatchedCount"`
}

var orderMu sync.Mutex
var orderJanitorOnce sync.Once

func init() {
	backupNames["orders.json"] = true
}

func (s *Server) ordersPath() string { return filepath.Join(s.store.dir, "orders.json") }

func (s *Server) startOrderJanitor() {
	orderJanitorOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(time.Hour)
			defer ticker.Stop()
			for range ticker.C {
				orderMu.Lock()
				_, _ = s.loadOrdersLocked(time.Now())
				orderMu.Unlock()
			}
		}()
	})
}

func (s *Server) loadOrdersLocked(now time.Time) ([]SalesOrder, error) {
	var f orderFile
	b, err := os.ReadFile(s.ordersPath())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err == nil && len(b) > 0 {
		if e := json.Unmarshal(b, &f); e != nil {
			return nil, fmt.Errorf("orders file is damaged: %w", e)
		}
	}
	kept := make([]SalesOrder, 0, len(f.Orders))
	changed := false
	for _, o := range f.Orders {
		if o.Status == "dispatched" && !o.DispatchedAt.IsZero() && !o.DispatchedAt.Add(dispatchedOrderRetention).After(now) {
			changed = true
			continue
		}
		kept = append(kept, o)
	}
	if changed {
		if e := s.saveOrdersLocked(kept); e != nil {
			return nil, e
		}
	}
	return kept, nil
}

func (s *Server) saveOrdersLocked(orders []SalesOrder) error {
	b, err := json.MarshalIndent(orderFile{Orders: orders}, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(s.ordersPath(), b, 0600)
}

func compactOrderText(s string, max int) string {
	s = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(s, "\r", " "), "\n", " "))
	if len(s) > max {
		s = s[:max]
	}
	return strings.TrimSpace(s)
}

func orderBuyerField(details map[string]string, names ...string) string {
	for _, want := range names {
		for k, v := range details {
			if strings.EqualFold(strings.TrimSpace(k), want) && strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		}
	}
	return ""
}

func validateOrder(in SalesOrder) (SalesOrder, error) {
	in.Company = compactOrderText(in.Company, 160)
	in.Buyer = compactOrderText(in.Buyer, 160)
	in.Notes = strings.TrimSpace(in.Notes)
	if len(in.Notes) > 1000 {
		return SalesOrder{}, errors.New("notes are too long")
	}
	if in.Company == "" || in.Buyer == "" {
		return SalesOrder{}, errors.New("company and buyer are required")
	}
	if len(in.Items) == 0 || len(in.Items) > 100 {
		return SalesOrder{}, errors.New("add between 1 and 100 order items")
	}
	cleanDetails := map[string]string{}
	for k, v := range in.BuyerDetails {
		k = compactOrderText(k, 80)
		v = compactOrderText(v, 320)
		if k != "" && v != "" && len(cleanDetails) < 40 {
			cleanDetails[k] = v
		}
	}
	in.BuyerDetails = cleanDetails
	for i := range in.Items {
		in.Items[i].Name = compactOrderText(in.Items[i].Name, 200)
		in.Items[i].Unit = compactOrderText(in.Items[i].Unit, 40)
		if in.Items[i].Name == "" || in.Items[i].Quantity <= 0 || in.Items[i].Quantity > 100000000 {
			return SalesOrder{}, fmt.Errorf("item %d needs a valid product and quantity", i+1)
		}
		if in.Items[i].Rate != nil {
			if *in.Items[i].Rate < 0 || *in.Items[i].Rate > 1000000000 {
				return SalesOrder{}, fmt.Errorf("item %d has an invalid rate", i+1)
			}
			rounded := float64(int64(*in.Items[i].Rate*100+0.5)) / 100
			in.Items[i].Rate = &rounded
		}
	}
	return in, nil
}

func nextOrderNumber(orders []SalesOrder, now time.Time) string {
	prefix := "ORD-" + now.Format("20060102") + "-"
	max := 0
	for _, o := range orders {
		if !strings.HasPrefix(o.Number, prefix) {
			continue
		}
		if n, err := strconv.Atoi(strings.TrimPrefix(o.Number, prefix)); err == nil && n > max {
			max = n
		}
	}
	return fmt.Sprintf("%s%03d", prefix, max+1)
}

func orderShareText(o SalesOrder) string {
	if o.Status != "pending" {
		return ""
	}
	var b strings.Builder
	b.WriteString("*ORDER DETAILS*\n")
	b.WriteString("Firm: " + o.Company + "\n")
	b.WriteString("Customer: " + o.Buyer + "\n")
	if phone := orderBuyerField(o.BuyerDetails, "MOBILE", "MOBILENO", "MOBILENUMBER", "PHONE", "PHONENUMBER"); phone != "" {
		b.WriteString("Mobile: " + phone + "\n")
	}
	address := orderBuyerField(o.BuyerDetails, "ADDRESS")
	city := orderBuyerField(o.BuyerDetails, "CITY")
	if address != "" || city != "" {
		b.WriteString("Address: " + strings.Trim(strings.Join([]string{address, city}, ", "), ", ") + "\n")
	}
	b.WriteString("Order No: " + o.Number + "\n")
	b.WriteString("Order Date: " + o.CreatedAt.Local().Format("02-01-2006 03:04 PM") + "\n\n")
	allRated := true
	total := 0.0
	for i, item := range o.Items {
		qty := strconv.FormatFloat(item.Quantity, 'f', -1, 64)
		line := fmt.Sprintf("%d. %s — Qty: %s", i+1, item.Name, qty)
		if item.Unit != "" {
			line += " " + item.Unit
		}
		if item.Rate != nil {
			amount := item.Quantity * *item.Rate
			total += amount
			line += fmt.Sprintf(" — Rate: ₹%.2f — Amount: ₹%.2f", *item.Rate, amount)
		} else {
			allRated = false
		}
		b.WriteString(line + "\n")
	}
	if allRated && len(o.Items) > 0 {
		b.WriteString(fmt.Sprintf("\nOrder Total: ₹%.2f\n", total))
	}
	if o.Notes != "" {
		b.WriteString("\nNotes: " + o.Notes + "\n")
	}
	b.WriteString("Status: Pending")
	return b.String()
}

func withOrderPresentation(o SalesOrder) SalesOrder {
	if o.Status == "pending" {
		o.ShareText = orderShareText(o)
	} else {
		o.ShareText = ""
	}
	return o
}

func orderList(orders []SalesOrder, company string) OrderList {
	company = strings.TrimSpace(company)
	out := OrderList{Pending: []SalesOrder{}, Dispatched: []SalesOrder{}}
	for _, raw := range orders {
		if company != "" && !strings.EqualFold(strings.TrimSpace(raw.Company), company) {
			continue
		}
		o := withOrderPresentation(raw)
		if o.Status == "dispatched" {
			out.Dispatched = append(out.Dispatched, o)
		} else {
			out.Pending = append(out.Pending, o)
		}
	}
	sort.Slice(out.Pending, func(i, j int) bool { return out.Pending[i].CreatedAt.After(out.Pending[j].CreatedAt) })
	sort.Slice(out.Dispatched, func(i, j int) bool { return out.Dispatched[i].DispatchedAt.After(out.Dispatched[j].DispatchedAt) })
	out.PendingCount, out.DispatchedCount = len(out.Pending), len(out.Dispatched)
	return out
}

func (s *Server) listOrders(company string) (OrderList, error) {
	orderMu.Lock()
	defer orderMu.Unlock()
	orders, err := s.loadOrdersLocked(time.Now())
	if err != nil {
		return OrderList{}, err
	}
	return orderList(orders, company), nil
}

func (s *Server) saveOrder(in SalesOrder) (SalesOrder, error) {
	clean, err := validateOrder(in)
	if err != nil {
		return SalesOrder{}, err
	}
	orderMu.Lock()
	defer orderMu.Unlock()
	orders, err := s.loadOrdersLocked(time.Now())
	if err != nil {
		return SalesOrder{}, err
	}
	now := time.Now()
	if strings.TrimSpace(clean.ID) == "" {
		clean.ID = randomHex(12)
		clean.Number = nextOrderNumber(orders, now)
		clean.Status = "pending"
		clean.CreatedAt = now
		clean.UpdatedAt = now
		clean.DispatchedAt = time.Time{}
		orders = append(orders, clean)
	} else {
		found := false
		for i := range orders {
			if orders[i].ID != clean.ID {
				continue
			}
			found = true
			if orders[i].Status == "dispatched" {
				return SalesOrder{}, errors.New("dispatched orders cannot be edited")
			}
			clean.Number = orders[i].Number
			clean.Status = "pending"
			clean.CreatedAt = orders[i].CreatedAt
			clean.UpdatedAt = now
			clean.DispatchedAt = time.Time{}
			orders[i] = clean
			break
		}
		if !found {
			return SalesOrder{}, errors.New("order not found")
		}
	}
	if err := s.saveOrdersLocked(orders); err != nil {
		return SalesOrder{}, err
	}
	return withOrderPresentation(clean), nil
}

func (s *Server) dispatchOrder(id string) (SalesOrder, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return SalesOrder{}, errors.New("order id required")
	}
	orderMu.Lock()
	defer orderMu.Unlock()
	orders, err := s.loadOrdersLocked(time.Now())
	if err != nil {
		return SalesOrder{}, err
	}
	now := time.Now()
	for i := range orders {
		if orders[i].ID != id {
			continue
		}
		if orders[i].Status != "dispatched" {
			orders[i].Status = "dispatched"
			orders[i].DispatchedAt = now
			orders[i].UpdatedAt = now
		}
		orders[i].ShareText = ""
		if err := s.saveOrdersLocked(orders); err != nil {
			return SalesOrder{}, err
		}
		return withOrderPresentation(orders[i]), nil
	}
	return SalesOrder{}, errors.New("order not found")
}

func (s *Server) handleOrderMasters(w http.ResponseWriter, r *http.Request) {
	c := strings.TrimSpace(r.URL.Query().Get("company"))
	if c == "" {
		jsonErr(w, 400, errors.New("select a company"))
		return
	}
	idx, err := s.getLedgers(c, false)
	if err != nil {
		jsonErr(w, 503, errors.New("sync the selected company's ledgers first"))
		return
	}
	var stocks StockItemsData
	at, ok := s.cache.Get("stock-items", c, "", &stocks)
	if !ok {
		raw, ms, _, readErr := s.tally.request("Read order stock masters", c, stockItemsXML(c), 0)
		if readErr != nil {
			jsonErr(w, 503, readErr)
			return
		}
		stocks = StockItemsData{CommonData: CommonData{Live: true, Source: "live", FetchedAt: time.Now(), DurationMS: ms}, Items: parseStockItems(raw, time.Now().Format("2006-01-02"))}
		_ = s.cache.Put("stock-items", c, "", stocks, stocks.FetchedAt)
	} else {
		stocks.Live = false
		stocks.Source = "saved"
		stocks.Stale = true
		stocks.FetchedAt = at
	}
	jsonOut(w, map[string]any{"company": c, "parties": idx.Ledgers, "stock": stocks})
}

func (s *Server) handleOrders(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/api/v1/mobile/orders/masters":
		if r.Method != "GET" {
			jsonErr(w, 405, errors.New("GET required"))
			return
		}
		s.handleOrderMasters(w, r)
	case r.URL.Path == "/api/v1/mobile/orders" || r.URL.Path == "/api/v1/admin/orders":
		switch r.Method {
		case "GET":
			list, err := s.listOrders(r.URL.Query().Get("company"))
			if err != nil {
				jsonErr(w, 500, err)
				return
			}
			jsonOut(w, list)
		case "POST":
			var q SalesOrder
			if readJSON(r, &q) != nil {
				jsonErr(w, 400, errors.New("invalid order"))
				return
			}
			o, err := s.saveOrder(q)
			if err != nil {
				jsonErr(w, 400, err)
				return
			}
			jsonOut(w, o)
		default:
			jsonErr(w, 405, errors.New("method not allowed"))
		}
	case r.URL.Path == "/api/v1/mobile/orders/dispatch" || r.URL.Path == "/api/v1/admin/orders/dispatch":
		if r.Method != "POST" {
			jsonErr(w, 405, errors.New("POST required"))
			return
		}
		var q struct {
			ID string `json:"id"`
		}
		if readJSON(r, &q) != nil {
			jsonErr(w, 400, errors.New("invalid dispatch request"))
			return
		}
		o, err := s.dispatchOrder(q.ID)
		if err != nil {
			jsonErr(w, 400, err)
			return
		}
		jsonOut(w, o)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) nativeOrdersAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "POST" {
			jsonErr(w, 405, errors.New("method not allowed"))
			return
		}
		var email string
		var err error
		if s.native.config().GoogleClientID != "" {
			if strings.HasPrefix(r.Header.Get("Authorization"), "TallyBridge ") {
				email, err = s.native.verifySession(r)
			} else {
				email, err = s.verifyGoogleRequest(r)
			}
		} else {
			email, err = s.native.verifyAccess(r.Context(), r.Header.Get("Cf-Access-Jwt-Assertion"))
		}
		if err != nil {
			jsonErr(w, 401, err)
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), nativeContextKey{}, email))
		if _, err := s.verifyNativeRequest(r, true); err != nil {
			jsonErr(w, 403, err)
			return
		}
		next(w, r)
	}
}

func (s *Server) DesktopHandlerWithOrders() http.Handler {
	s.startOrderJanitor()
	base := s.DesktopHandler()
	orders := s.localAdmin(s.handleOrders)
	return securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/assets/orders.js":
			s.serveFile("web/orders.js", "application/javascript; charset=utf-8")(w, r)
		case strings.HasPrefix(r.URL.Path, "/api/v1/admin/orders"):
			orders(w, r)
		default:
			base.ServeHTTP(w, r)
		}
	}))
}

func (s *Server) NativeHandlerWithOrders() http.Handler {
	s.startOrderJanitor()
	base := s.NativeHandler()
	orders := s.nativeOrdersAuth(s.handleOrders)
	return securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/mobile/orders") {
			orders(w, r)
			return
		}
		base.ServeHTTP(w, r)
	}))
}
