package app

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestOutstandingViewSelectionAndFormatting(t *testing.T) {
	x := LedgerOutstandingData{Company: "A", Ledger: "Customer", Total: 250, DrCr: "Dr", CommonData: CommonData{FetchedAt: time.Now()}, Bills: []BillRow{
		{BillNumber: "NEW", Date: "2026-09-02", DueDate: "2099-01-01", PendingAmount: 200, SignedAmount: -200, DrCr: "Dr"},
		{BillNumber: "OLD", Date: "2026-01-01", PendingAmount: 100, SignedAmount: -100, DrCr: "Dr"},
		{BillNumber: "CREDIT", Date: "2026-02-01", PendingAmount: 50, SignedAmount: 50, DrCr: "Cr"},
	}}
	all, e := outstandingView(x, url.Values{})
	if e != nil {
		t.Fatal(e)
	}
	if all.Bills[0].BillNumber != "OLD" || x.Bills[0].BillNumber != "NEW" {
		t.Fatal("sort must not mutate cache")
	}
	q := url.Values{"bills": {all.Bills[0].ID + "," + all.Bills[1].ID}, "at": {x.FetchedAt.Format(time.RFC3339Nano)}}
	selected, e := outstandingView(x, q)
	if e != nil {
		t.Fatal(e)
	}
	if selected.Total != 50 || selected.DrCr != "Dr" || selected.Count != 2 {
		t.Fatalf("incorrect selected net: %+v", selected)
	}
	if strings.Contains(selected.ShareText, "NEW") || strings.Contains(selected.ShareText, "2099") || !strings.Contains(selected.ShareText, "Due for:") {
		t.Fatal(selected.ShareText)
	}
	reminder := paymentReminder(selected, BankDetails{AccountNumber: "001234", IFSC: "SBIN0000123"})
	if !strings.Contains(reminder, "001234") || strings.Contains(reminder, "NEW") {
		t.Fatal(reminder)
	}
	for _, q := range []url.Values{{"at": {"stale"}}, {"min": {"NaN"}}, {"from": {"bad"}}, {"min": {"20"}, "max": {"10"}}, {"bills": {"missing"}}} {
		if _, e := outstandingView(x, q); e == nil {
			t.Fatalf("accepted invalid %v", q)
		}
	}
	filtered, e := outstandingView(x, url.Values{"search": {"new"}, "min": {"150"}})
	if e != nil || filtered.Total != 200 || filtered.Count != 1 {
		t.Fatal(filtered, e)
	}
	if invoiceDays("invalid") != -1 || invoiceDays("2099-01-01") != 0 {
		t.Fatal("unknown or future date age incorrect")
	}
}

func TestBankAliasesAndAmbiguousLedger(t *testing.T) {
	m := parseLedgerDetails([]byte(`<ENVELOPE><LEDGER NAME="SBI"><BANKACNO>0012345678</BANKACNO><BANKIFSC>SBIN0000123</BANKIFSC><BANKACCHOLDERNAME>Company A</BANKACCHOLDERNAME></LEDGER></ENVELOPE>`))
	if m["BANKACCOUNTNUMBER"] != "0012345678" || m["IFSCCODE"] != "SBIN0000123" || m["BANKNAME"] != "" {
		t.Fatal(m)
	}
	if m := parseLedgerDetails([]byte(`<ENVELOPE><LEDGER NAME="A"/><LEDGER NAME="B"><BANKACNO>123</BANKACNO></LEDGER></ENVELOPE>`)); len(m) > 0 {
		t.Fatal("must not borrow another ledger's bank", m)
	}
	if !strings.Contains(string(bankLedgerXML("A", "SBI")), "<FETCH>*</FETCH>") {
		t.Fatal("bank request must fetch full object")
	}
}

func TestManualBankSavePreservesPreviousOnInvalidInput(t *testing.T) {
	s, ts := testServerWithTally(t, func(w http.ResponseWriter, r *http.Request) { t.Error("manual bank must not request Tally") })
	defer ts.Close()
	_ = s.cache.Put("ledgers", "A", "", LedgerIndexData{Ledgers: []LedgerIndexRow{{Name: "SBI"}}}, time.Now())
	for i, body := range []string{`{"company":"A","ledger":"SBI","details":{"accountNumber":"001234","ifsc":"sbin0000123"}}`, `{"company":"A","ledger":"SBI","details":{"accountNumber":"","ifsc":"SBIN0000123"}}`, `{"company":"A","ledger":"SBI","details":{"accountNumber":"999","ifsc":"bad"}}`} {
		r := httptest.NewRequest("POST", "http://localhost:8765/api/v1/admin/pdf-bank", strings.NewReader(body))
		r.RemoteAddr = "127.0.0.1:12345"
		r.Header.Set("X-TB-Admin", "1")
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		if (i == 0) != (w.Code == 200) {
			t.Fatal(w.Code, w.Body.String())
		}
		bank, ok := s.store.PDFBank("A")
		if !ok || bank.AccountNumber != "001234" || bank.IFSC != "SBIN0000123" {
			t.Fatal(bank)
		}
	}
	if _, ok := s.store.PDFBank("B"); ok {
		t.Fatal("bank leaked across companies")
	}
}
