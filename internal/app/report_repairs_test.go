package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTallyShortYearDatesSortAndAge(t *testing.T) {
	for raw, want := range map[string]string{"1-Apr-26": "2026-04-01", "31-Dec-25": "2025-12-31", "01/04/2026": "2026-04-01", "20260401": "2026-04-01", "2026-13-40": ""} {
		if got := normISODate(raw); got != want {
			t.Fatalf("%s: got %s want %s", raw, got, want)
		}
	}
	xml := `<ENVELOPE><BILLFIXED><BILLDATE>2-Sep-26</BILLDATE><BILLREF>NEW</BILLREF></BILLFIXED><BILLCL>200 Dr</BILLCL><BILLFIXED><BILLDATE>31-Dec-25</BILLDATE><BILLREF>OLD</BILLREF></BILLFIXED><BILLCL>100 Dr</BILLCL></ENVELOPE>`
	rows := parsePartyBills([]byte(xml), "Customer")
	if len(rows) != 2 || rows[0].BillNumber != "OLD" || invoiceDays(rows[0].Date) < 1 {
		t.Fatal(rows)
	}
	x := LedgerOutstandingData{Bills: []BillRow{{BillNumber: "NEW", Date: "2-Sep-26"}, {BillNumber: "OLD", Date: "31-Dec-25"}}}
	text := ledgerOutstandingShareText(x)
	if strings.Index(text, "OLD") > strings.Index(text, "NEW") || strings.Contains(text, "Not available") {
		t.Fatal(text)
	}
	doc := outstandingDoc(x)
	if doc.Rows[0][0] != "OLD" || doc.Rows[0][2] == "Not available" {
		t.Fatal(doc.Rows)
	}
}
func TestZeroGroupsLiveAndSaved(t *testing.T) {
	rows := parseGroupBalances([]byte(`<ENVELOPE><LEDGER NAME="Paid"><CLOSINGBALANCE>0.00</CLOSINGBALANCE></LEDGER><LEDGER NAME="Tiny"><CLOSINGBALANCE>0.001</CLOSINGBALANCE></LEDGER><LEDGER NAME="Pending"><CLOSINGBALANCE>-20</CLOSINGBALANCE></LEDGER></ENVELOPE>`))
	if len(rows) != 1 || rows[0].Ledger != "Pending" {
		t.Fatal(rows)
	}
	x := nonzeroGroup(GroupBalanceData{Ledgers: []GroupLedgerRow{{Ledger: "Paid"}, {Ledger: "Pending", Balance: 20, DrCr: "Dr"}}})
	if x.Count != 1 || x.Total != -20 || strings.Contains(x.ShareText, "Paid") || len(groupDoc(x).Rows) != 1 {
		t.Fatal(x)
	}
}
func TestBankExplicitMethodsFallback(t *testing.T) {
	calls := 0
	s, ts := testServerWithTally(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		b, _ := io.ReadAll(r.Body)
		if calls == 1 {
			io.WriteString(w, `<ENVELOPE><LEDGER NAME="SBI"><BANKNAME>State Bank</BANKNAME></LEDGER></ENVELOPE>`)
			return
		}
		if !strings.Contains(string(b), "<FETCH>BankAccNo</FETCH>") {
			t.Error("explicit bank fetch missing")
		}
		io.WriteString(w, `<ENVELOPE><LEDGER NAME="SBI"><BANKACNO>001234</BANKACNO><IFSCODE>SBIN0000123</IFSCODE></LEDGER></ENVELOPE>`)
	})
	defer ts.Close()
	_ = s.cache.Put("ledgers", "A", "", LedgerIndexData{Ledgers: []LedgerIndexRow{{Name: "SBI"}}}, time.Now())
	r := httptest.NewRequest("POST", "http://localhost:8765/api/v1/admin/pdf-bank", strings.NewReader(`{"company":"A","ledger":"SBI"}`))
	r.RemoteAddr = "127.0.0.1:12345"
	r.Header.Set("X-TB-Admin", "1")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	bank, _ := s.store.PDFBank("A")
	if w.Code != 200 || calls != 2 || bank.AccountNumber != "001234" || bank.IFSC != "SBIN0000123" {
		t.Fatal(w.Code, w.Body.String(), bank, calls)
	}
}
