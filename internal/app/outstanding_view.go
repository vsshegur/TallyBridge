package app

import (
	"crypto/sha256"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

func invoiceDays(date string) int {
	d, e := time.Parse("2006-01-02", date)
	if e != nil {
		return -1
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	days := int(today.Sub(d).Hours() / 24)
	if days < 0 {
		return 0
	}
	return days
}
func invoiceDaysLabel(date string) string {
	n := invoiceDays(date)
	if n < 0 {
		return "Not available"
	}
	return fmt.Sprintf("%d days", n)
}
func sortedBills(bills []BillRow) []BillRow {
	out := append([]BillRow(nil), bills...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].Date, out[j].Date
		if a == "" {
			return false
		}
		if b == "" {
			return true
		}
		if a == b {
			return out[i].BillNumber < out[j].BillNumber
		}
		return a < b
	})
	return out
}
func outstandingView(x LedgerOutstandingData, q url.Values) (LedgerOutstandingData, error) {
	if at := q.Get("at"); at != "" && at != x.FetchedAt.Format(time.RFC3339Nano) {
		return x, fmt.Errorf("report changed on the PC; refresh outstanding and select bills again")
	}
	from, to := q.Get("from"), q.Get("to")
	for _, d := range []string{from, to} {
		if d != "" {
			if _, e := time.Parse("2006-01-02", d); e != nil {
				return x, fmt.Errorf("use invoice dates in YYYY-MM-DD format")
			}
		}
	}
	if from != "" && to != "" && from > to {
		return x, fmt.Errorf("end date must be on or after start date")
	}
	bounds := map[string]float64{}
	for _, key := range []string{"min", "max", "minDays", "maxDays"} {
		if s := q.Get(key); s != "" {
			v, e := strconv.ParseFloat(s, 64)
			if e != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
				return x, fmt.Errorf("%s must be a non-negative number", key)
			}
			bounds[key] = v
		}
	}
	for _, pair := range [][2]string{{"min", "max"}, {"minDays", "maxDays"}} {
		lo, lok := bounds[pair[0]]
		hi, hok := bounds[pair[1]]
		if lok && hok && lo > hi {
			return x, fmt.Errorf("minimum cannot exceed maximum")
		}
	}
	selected := map[string]bool{}
	if raw := q.Get("bills"); raw != "" {
		for _, id := range strings.Split(raw, ",") {
			selected[id] = true
		}
		if len(selected) > 150 {
			return x, fmt.Errorf("select up to 150 bills per message")
		}
	}
	query := strings.ToLower(strings.TrimSpace(q.Get("search")))
	filtered := from != "" || to != "" || len(bounds) > 0 || query != "" || len(selected) > 0
	if x.SummaryOnly && filtered {
		return x, fmt.Errorf("bill filters require bill-wise details; only a summary is available")
	}
	var result []BillRow
	net := 0.0
	matched := 0
	for i, b := range sortedBills(x.Bills) {
		b.ID = fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%s|%.2f|%d", x.Company, x.Ledger, b.BillNumber, b.Date, b.SignedAmount, i))))[:20]
		b.InvoiceDays = invoiceDays(b.Date)
		if len(selected) > 0 && !selected[b.ID] {
			continue
		}
		if from != "" && (b.Date == "" || b.Date < from) || to != "" && (b.Date == "" || b.Date > to) {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(b.BillNumber+" "+b.Date+" "+indianNumber(b.PendingAmount)+" "+strconv.FormatFloat(b.PendingAmount, 'f', 2, 64)), query) {
			continue
		}
		if v, ok := bounds["min"]; ok && b.PendingAmount < v {
			continue
		}
		if v, ok := bounds["max"]; ok && b.PendingAmount > v {
			continue
		}
		if v, ok := bounds["minDays"]; ok && (b.InvoiceDays < 0 || float64(b.InvoiceDays) < v) {
			continue
		}
		if v, ok := bounds["maxDays"]; ok && (b.InvoiceDays < 0 || float64(b.InvoiceDays) > v) {
			continue
		}
		result = append(result, b)
		net += b.SignedAmount
		matched++
	}
	if len(selected) > 0 && matched != len(selected) {
		return x, fmt.Errorf("selected bills changed; refresh and select again")
	}
	x.Bills = result
	x.Count = len(result)
	if filtered {
		x.Total = abs(net)
		x.DrCr = balanceSide(net)
		x.FilterLabel = "Filtered bills"
		if len(selected) > 0 {
			x.FilterLabel = "Selected bills"
		}
		x.BalanceFrom = x.FilterLabel
	}
	x.ShareText = ledgerOutstandingShareText(x)
	return x, nil
}
func paymentReminder(x LedgerOutstandingData, bank BankDetails) string {
	if x.Total <= 0 || x.DrCr != "Dr" || len(x.Bills) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Hello " + x.Ledger + ",\n\nA gentle reminder for the pending invoices below. Please arrange payment when possible, or share the payment reference if already paid.\n\n")
	b.WriteString(x.ShareText)
	if bank.AccountNumber != "" || bank.UPI != "" {
		b.WriteString("\n\n*Payment details*\n")
		for _, field := range [][2]string{{"Bank", bank.BankName}, {"Account holder", bank.AccountHolder}, {"Account", bank.AccountNumber}, {"IFSC", bank.IFSC}, {"UPI", bank.UPI}} {
			if field[1] != "" {
				fmt.Fprintf(&b, "%s: %s\n", field[0], field[1])
			}
		}
	}
	b.WriteString("\nThank you.\n" + x.Company)
	return b.String()
}
