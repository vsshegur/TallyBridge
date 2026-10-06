package app

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type GroupOutstandingData struct {
	CommonData
	Company        string    `json:"company"`
	Group          string    `json:"group"`
	Bills          []BillRow `json:"bills"`
	Total          float64   `json:"total"`
	Count          int       `json:"count"`
	ShareText      string    `json:"shareText"`
	PartialLedgers []string  `json:"partialLedgers,omitempty"`
}

func prettyBillDate(v string) string {
	if d, e := time.Parse("2006-01-02", normISODate(v)); e == nil {
		return d.Format("02 Jan 2006")
	}
	if strings.TrimSpace(v) == "" {
		return "Not available"
	}
	return v
}

func groupOutstandingShareText(x GroupOutstandingData) string {
	var b strings.Builder
	b.WriteString("*Outstanding by Group*\n")
	fmt.Fprintf(&b, "Company: %s\nGroup: %s\n", x.Company, x.Group)
	fmt.Fprintf(&b, "Pending bills: %d\nTotal pending: ₹%s\n", x.Count, indianNumber(x.Total))
	b.WriteString("Oldest bill first\n")
	for i, bill := range x.Bills {
		fmt.Fprintf(&b, "\n*%d. %s*\n", i+1, bill.Party)
		fmt.Fprintf(&b, "Bill date: %s\n", prettyBillDate(bill.Date))
		fmt.Fprintf(&b, "Invoice: %s\n", bill.BillNumber)
		fmt.Fprintf(&b, "Amount: ₹%s", indianNumber(bill.PendingAmount))
		if bill.DrCr != "" {
			fmt.Fprintf(&b, " %s", bill.DrCr)
		}
		b.WriteString("\n")
		if bill.InvoiceDays >= 0 {
			fmt.Fprintf(&b, "Age: %d days\n", bill.InvoiceDays)
		}
	}
	if len(x.PartialLedgers) > 0 {
		fmt.Fprintf(&b, "\nNote: Bill-wise breakup was unavailable for %d ledger(s).\n", len(x.PartialLedgers))
	}
	return strings.TrimSpace(b.String())
}

func (s *Server) getGroupOutstanding(company, group string) (GroupOutstandingData, error) {
	gb, err := s.getGroupBalances(company, group)
	if err != nil {
		return GroupOutstandingData{}, err
	}
	out := GroupOutstandingData{CommonData: gb.CommonData, Company: company, Group: group}
	out.Live = true
	out.Source = "live"
	out.FetchedAt = time.Now()
	for _, row := range gb.Ledgers {
		if row.Balance == 0 || strings.TrimSpace(row.Ledger) == "" {
			continue
		}
		led, e := s.getLedgerOutstanding(company, row.Ledger)
		if e != nil || led.SummaryOnly || len(led.Bills) == 0 {
			out.PartialLedgers = append(out.PartialLedgers, row.Ledger)
			out.Live = false
			out.Source = "mixed"
			continue
		}
		if !led.Live {
			out.Live = false
			out.Source = "mixed"
		}
		for _, bill := range led.Bills {
			bill.Party = row.Ledger
			bill.Date = normISODate(bill.Date)
			bill.InvoiceDays = invoiceDays(bill.Date)
			if bill.PendingAmount == 0 {
				bill.PendingAmount = abs(bill.SignedAmount)
			}
			out.Bills = append(out.Bills, bill)
			out.Total += abs(bill.PendingAmount)
		}
	}
	out.Bills = sortedBills(out.Bills)
	out.Count = len(out.Bills)
	if out.Count == 0 && len(out.PartialLedgers) > 0 {
		return out, errors.New("bill-wise outstanding is not available yet; open the affected ledger outstanding once and retry")
	}
	out.ShareText = groupOutstandingShareText(out)
	return out, nil
}

func (s *Server) handleGroupOutstanding(w http.ResponseWriter, r *http.Request) {
	c, g := strings.TrimSpace(r.URL.Query().Get("company")), strings.TrimSpace(r.URL.Query().Get("group"))
	if c == "" || g == "" {
		jsonErr(w, 400, errors.New("company and group required"))
		return
	}
	x, e := s.getGroupOutstanding(c, g)
	if e != nil {
		jsonErr(w, 503, e)
		return
	}
	jsonOut(w, x)
}
