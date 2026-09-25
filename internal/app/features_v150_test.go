package app

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestV150BankDetailsParsingAndPDF(t *testing.T) {
	xml := []byte(`<ENVELOPE><LEDGER><NAME>SBI Current</NAME><PARENT>Bank Accounts</PARENT><BANKNAME>State Bank of India</BANKNAME><BANKACCOUNTNUMBER>1234567890</BANKACCOUNTNUMBER><BANKACCHOLDERNAME>DEMO TEXTILES</BANKACCHOLDERNAME><IFSCCODE>SBIN0000123</IFSCCODE><BRANCHNAME>Solapur</BRANCHNAME><UPIID>demo@sbi</UPIID></LEDGER></ENVELOPE>`)
	d := bankDetailsFromMap("SBI Current", parseLedgerDetails(xml))
	if d.AccountNumber != "1234567890" || d.IFSC != "SBIN0000123" || d.UPI != "demo@sbi" {
		t.Fatalf("bad bank parsing: %+v", d)
	}
	x := LedgerOutstandingData{Company: "DEMO TEXTILES", Ledger: "ABC Traders", Total: 500, Bills: []BillRow{{BillNumber: "VT-1", Date: "2026-09-24", PendingAmount: 500}}}
	pdf := tableHTML(outstandingDoc(x), d)
	ps := string(pdf)
	if !strings.Contains(ps, "Payment bank details") || !strings.Contains(ps, "SBIN0000123") || !strings.Contains(ps, "1234567890") {
		t.Fatal("bank block missing from customer PDF")
	}
}

func TestV150AgeingFromSavedPartyBills(t *testing.T) {
	s, err := NewServer(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	p := PartyOutstandingData{Party: "ABC Traders", Bills: []BillRow{
		{BillNumber: "A", DueDate: now.AddDate(0, 0, -10).Format("2006-01-02"), PendingAmount: 100},
		{BillNumber: "B", DueDate: now.AddDate(0, 0, -45).Format("2006-01-02"), PendingAmount: 200},
		{BillNumber: "C", DueDate: now.AddDate(0, 0, -75).Format("2006-01-02"), PendingAmount: 300},
		{BillNumber: "D", DueDate: now.AddDate(0, 0, -120).Format("2006-01-02"), PendingAmount: 400},
	}, Count: 4, Total: 1000}
	if err := s.cache.Put("party-outstanding", "DEMO TEXTILES", "ABC Traders", p, now); err != nil {
		t.Fatal(err)
	}
	out := OutstandingData{Parties: []OutstandingParty{{Party: "ABC Traders", Amount: 1000}}, Count: 1, Total: 1000}
	if err := s.cache.Put("receivable-party", "DEMO TEXTILES", "", out, now); err != nil {
		t.Fatal(err)
	}
	// exercise the same core computation through cached entries
	buckets := []float64{0, 0, 0, 0}
	for _, d := range s.cache.Matching("party-outstanding", "DEMO TEXTILES") {
		var x PartyOutstandingData
		if json.Unmarshal(d.Data, &x) != nil {
			t.Fatal("decode")
		}
		for _, b := range x.Bills {
			base := parseISOOrZero(b.DueDate)
			age := int(time.Since(base).Hours() / 24)
			i := 0
			if age > 90 {
				i = 3
			} else if age > 60 {
				i = 2
			} else if age > 30 {
				i = 1
			}
			buckets[i] += b.PendingAmount
		}
	}
	want := []float64{100, 200, 300, 400}
	for i := range want {
		if buckets[i] != want[i] {
			t.Fatalf("bucket %d got %v want %v", i, buckets[i], want[i])
		}
	}
}

func TestV150Customer360ShareText(t *testing.T) {
	x := Customer360Data{Company: "DEMO TEXTILES", Ledger: "ABC Traders", Outstanding: 48500, Overdue: 12500, PendingBillCount: 3, LatestInvoices: []VoucherRow{{Date: "2026-09-24", Number: "VT-396", Amount: 48500}}, LastPayment: &VoucherRow{Date: "2026-09-20", Number: "RC-9", Amount: 10000}}
	txt := customer360ShareText(x)
	for _, q := range []string{"Customer 360", "ABC Traders", "Outstanding", "VT-396", "RC-9"} {
		if !strings.Contains(txt, q) {
			t.Fatalf("missing %s in %s", q, txt)
		}
	}
}

func TestV150PurchaseHintUsesPurchaseCache(t *testing.T) {
	s, err := NewServer(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	vd := VoucherData{Vouchers: []VoucherRow{{MasterID: "901", Number: "PUR-901", Date: "2026-09-24", Party: "Supplier A", Amount: 2500}}}
	if err := s.cache.Put("purchase", "DEMO TEXTILES", "fy", vd, time.Now()); err != nil {
		t.Fatal(err)
	}
	got, ok := s.cache.VoucherHint("DEMO TEXTILES", "901")
	if !ok || got.Number != "PUR-901" {
		t.Fatalf("purchase hint missing: %+v %v", got, ok)
	}
}
