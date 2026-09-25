package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDesktopBankPersistsAndSharingMakesNoTallyCalls(t *testing.T) {
	var calls int32
	s, ts := testServerWithTally(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		io.WriteString(w, `<ENVELOPE><LEDGER NAME="SBI"><BANKNAME>State Bank</BANKNAME><BANKACCOUNTNUMBER>1234567890</BANKACCOUNTNUMBER><IFSCCODE>SBIN0000123</IFSCCODE></LEDGER></ENVELOPE>`)
	})
	defer ts.Close()
	_ = s.cache.Put("ledgers", "Company A", "", LedgerIndexData{Ledgers: []LedgerIndexRow{{Name: "SBI", Parent: "Bank Accounts"}}}, time.Now())
	request := httptest.NewRequest("POST", "http://localhost:8765/api/v1/admin/pdf-bank", strings.NewReader(`{"company":"Company A","ledger":"SBI"}`))
	request.RemoteAddr = "127.0.0.1:34567"
	request.Header.Set("X-TB-Admin", "1")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, request)
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	reopened, e := NewStore(s.store.dir)
	if e != nil {
		t.Fatal(e)
	}
	s.store = reopened
	for i := 0; i < 3; i++ {
		banks, e := s.optionalBank(" COMPANY A ", "Old phone bank")
		if e != nil || len(banks) != 1 || banks[0].AccountNumber != "1234567890" {
			t.Fatal(banks, e)
		}
	}
	banks, _ := s.optionalBank("Company B", "SBI")
	if len(banks) != 0 {
		t.Fatal("bank leaked across companies")
	}
	if calls != 1 {
		t.Fatal("PDF default requested Tally again", calls)
	}
	_ = s.store.SavePDFBank("Company A", BankDetails{})
	banks, _ = s.optionalBank("Company A", "")
	if len(banks) != 0 {
		t.Fatal("no-bank preference ignored")
	}
}
func TestOutstandingFallbackKeepsTimestampAndUnknownIsNotZero(t *testing.T) {
	s, ts := testServerWithTally(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `<ENVELOPE><COLLECTION/></ENVELOPE>`) })
	defer ts.Close()
	at := time.Now().Add(-24 * time.Hour).Truncate(time.Second)
	_ = s.cache.Put("receivable-party", "A", "", OutstandingData{Parties: []OutstandingParty{{Party: "Customer", Amount: 120, DrCr: "Dr"}}}, at)
	x, e := s.getLedgerOutstanding("A", "Customer")
	if e != nil || x.Live || !x.SummaryOnly || !x.FetchedAt.Equal(at) || x.Total != 120 {
		t.Fatalf("bad fallback %+v %v", x, e)
	}
	if _, e = s.getLedgerOutstanding("B", "Customer"); e == nil {
		t.Fatal("unknown balance shown as zero")
	}
}
func TestOutstandingBillParserDoesNotPairUnrelatedAncestors(t *testing.T) {
	xml := []byte(`<ENVELOPE><NAME>Unrelated</NAME><AMOUNT>9999</AMOUNT><BILL><BILLREF>A</BILLREF><PENDINGAMOUNT>500 Dr</PENDINGAMOUNT></BILL><BILL><BILLREF>B</BILLREF><PENDINGAMOUNT>100 Cr</PENDINGAMOUNT></BILL></ENVELOPE>`)
	rows := parsePartyBills(xml, "Customer")
	net := 0.0
	for _, r := range rows {
		net += r.SignedAmount
	}
	if len(rows) != 2 || net != -400 {
		t.Fatalf("bad bill net %+v", rows)
	}
}
func TestLedgerStatementUsesSelectedAllocationAndReconciles(t *testing.T) {
	var calls int32
	s, ts := testServerWithTally(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		atomic.AddInt32(&calls, 1)
		if strings.Contains(string(b), "TBLiveLedgerVouchers") {
			io.WriteString(w, `<ENVELOPE><VOUCHER><MASTERID>1</MASTERID><DATE>20260901</DATE><VOUCHERNUMBER>S1</VOUCHERNUMBER><AMOUNT>9999</AMOUNT><ALLLEDGERENTRIES.LIST><LEDGERNAME>Customer</LEDGERNAME><AMOUNT>-500</AMOUNT></ALLLEDGERENTRIES.LIST><ALLLEDGERENTRIES.LIST><LEDGERNAME>Other</LEDGERNAME><AMOUNT>500</AMOUNT></ALLLEDGERENTRIES.LIST></VOUCHER><VOUCHER><MASTERID>2</MASTERID><DATE>20260902</DATE><VOUCHERNUMBER>R1</VOUCHERNUMBER><ALLLEDGERENTRIES.LIST><LEDGERNAME>Customer</LEDGERNAME><AMOUNT>200</AMOUNT></ALLLEDGERENTRIES.LIST></VOUCHER></ENVELOPE>`)
		} else {
			io.WriteString(w, `<ENVELOPE><LEDGER NAME="Customer"><OPENINGBALANCE>-1000</OPENINGBALANCE><CLOSINGBALANCE>-1300</CLOSINGBALANCE></LEDGER></ENVELOPE>`)
		}
	})
	defer ts.Close()
	x, e := s.getLedgerStatement("A", "Customer", "2026-09-01", "2026-09-24", "20260901", "20260924")
	if e != nil {
		t.Fatal(e)
	}
	if calls != 2 || x.DebitTotal != 500 || x.CreditTotal != 200 || x.ClosingBalance == nil || *x.ClosingBalance != -1300 || *x.Vouchers[0].Balance != -1500 {
		t.Fatalf("incorrect ledger %+v", x)
	}
}
func TestEmptyPartyResponseDoesNotOverwriteGoodCache(t *testing.T) {
	s, ts := testServerWithTally(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `<ENVELOPE><COLLECTION/></ENVELOPE>`) })
	defer ts.Close()
	at := time.Now().Add(-time.Hour)
	_ = s.cache.Put("receivable-party", "A", "", OutstandingData{Parties: []OutstandingParty{{Party: "Customer", Amount: 300}}, Total: 300, Count: 1}, at)
	x, e := s.getOutstanding("A", "receivable", true)
	if e != nil || x.Total != 300 || x.Live || !x.FetchedAt.Equal(at) {
		t.Fatal(x, e)
	}
}
func TestNavigationDoesNotRequestAutoSync(t *testing.T) {
	b, _ := webFS.ReadFile("web/phone.js")
	s := string(b)
	for _, bad := range []string{"requestAutoSync('route')", "requestAutoSync('phone-open')", "id=\"bankSelect\"", "tbSelectedBank"} {
		if strings.Contains(s, bad) {
			t.Fatal("old phone behavior", bad)
		}
	}
	// Status and bootstrap must only read memory/disk.
	var calls int32
	srv, ts := testServerWithTally(t, func(w http.ResponseWriter, r *http.Request) { atomic.AddInt32(&calls, 1) })
	defer ts.Close()
	for i := 0; i < 5; i++ {
		srv.bootstrapPayload()
	}
	if calls != 0 {
		t.Fatal("status queried Tally")
	}
}
func TestPDFTemplatesWrapEscapeAndShowSavedBank(t *testing.T) {
	x := VoucherDetail{FirmName: "शेगुर टेक्सटाइल्स", CustomerName: `<script>alert(1)</script>`, Items: []InvoiceItem{{Name: "मऊ टॉवेल kitchen napkin with a long product description", Quantity: "12 pcs", Rate: "150.00/pcs", Amount: 1800}}, TotalAmount: 1800}
	doc := invoiceHTML(x, BankDetails{Ledger: "SBI", AccountNumber: "123456", IFSC: "SBIN0001"})
	for _, want := range []string{"शेगुर टेक्सटाइल्स", "मऊ टॉवेल", "table-header-group", "break-inside:avoid", "123456", "SBIN0001", "&lt;script&gt;"} {
		if !strings.Contains(doc, want) {
			t.Fatal("PDF template missing", want)
		}
	}
	if strings.Contains(doc, "<script>") || strings.Contains(doc, "<td>Discount</td>") {
		t.Fatal("unsafe markup or zero discount shown")
	}
}
func TestPDFVisualFixtures(t *testing.T) {
	dir := os.Getenv("TB_PDF_FIXTURES")
	if dir == "" {
		t.Skip("visual fixtures not requested")
	}
	_ = os.MkdirAll(dir, 0700)
	at := CommonData{Live: true, Source: "live", FetchedAt: time.Now()}
	x := VoucherDetail{CommonData: at, FirmName: "Demo Textiles / शेगुर टेक्सटाइल्स", InvoiceNumber: "VT-160-2026", InvoiceDate: "2026-09-24", CustomerName: "ABC Traders / श्री गणेश ट्रेडर्स", CustomerAddress: []string{"125 Market Road, Textile Wholesale Complex, Solapur, Maharashtra 413001"}, GSTIN: "27ABCDE1234F1Z5", DispatchedThrough: "VRL Logistics", LRNumber: "LR-396-9988", LRDate: "2026-09-24", Destination: "Pune", DiscountAmount: 50, TaxableAmount: 1650, CGST: 148.5, SGST: 148.5, TotalAmount: 1947}
	bank := BankDetails{Ledger: "SBI Current", BankName: "State Bank of India", AccountHolder: "Demo Textiles", AccountNumber: "123456789012", IFSC: "SBIN0000123", Branch: "Solapur", UPI: "demo@example"}
	x.Items = []InvoiceItem{{Name: "Microfiber Kitchen Napkin - soft absorbent material / मऊ टॉवेल", HSN: "6302", Quantity: "10 pcs", Rate: "100.00/pcs", Amount: 1000}, {Name: "Bath Towel 70 x 140 cm", HSN: "6302", Quantity: "2 pcs", Rate: "350.00/pcs", Amount: 700}}
	save := func(name, doc string) {
		t.Helper()
		_ = os.WriteFile(filepath.Join(dir, name+".html"), []byte(doc), 0600)
		if os.Getenv("TB_PDF_HTML_ONLY") == "1" {
			return
		}
		b, e := renderPDF(doc)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(dir, name+".pdf"), b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	save("invoice", invoiceHTML(x, bank))
	x.Items = nil
	x.DiscountAmount = 0
	x.TaxableAmount = 0
	x.CGST = 0
	x.SGST = 0
	x.TotalAmount = 0
	for i := 0; i < 55; i++ {
		x.Items = append(x.Items, InvoiceItem{Name: fmt.Sprintf("%02d - Premium microfiber kitchen napkin, assorted colours and woven border, pack of 12 / स्वयंपाकघर टॉवेल", i+1), HSN: "6302", Quantity: "120 pcs", Rate: "125.50/pcs", Amount: 15060})
		x.TotalAmount += 15060
	}
	save("invoice-multipage", invoiceHTML(x, bank))
	ls := LedgerStatementData{CommonData: at, Company: "Demo Textiles", Ledger: "ABC Traders", FromDate: "2026-09-01", ToDate: "2026-09-24", OpeningBalance: floatPtr(-1000), ClosingBalance: floatPtr(-1300), DebitTotal: 500, CreditTotal: 200, Vouchers: []VoucherRow{{Date: "2026-09-01", Number: "S1", VoucherType: "Sales", Debit: floatPtr(500), Credit: floatPtr(0), Balance: floatPtr(-1500)}, {Date: "2026-09-02", Number: "R1", VoucherType: "Receipt", Debit: floatPtr(0), Credit: floatPtr(200), Balance: floatPtr(-1300)}}}
	save("ledger", tableHTML(statementDoc(ls), bank))
	save("outstanding", tableHTML(outstandingDoc(LedgerOutstandingData{CommonData: CommonData{Source: "saved", FetchedAt: time.Now().Add(-time.Hour)}, Company: "Demo Textiles", Ledger: "ABC Traders", Total: 1300, SummaryOnly: true, DrCr: "Dr"}), bank))
}

// Opt-in local fixture for browser verification; never included in release binaries.
func TestBrowserFixture(t *testing.T) {
	if os.Getenv("TB_BROWSER_FIXTURE") == "" {
		t.Skip("browser fixture not requested")
	}
	s, ts := testServerWithTally(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		q := string(b)
		switch {
		case strings.Contains(q, "Ledger Outstandings"):
			io.WriteString(w, `<ENVELOPE><COLLECTION/></ENVELOPE>`)
		case strings.Contains(q, "<ID TYPE=\"Name\">SBI Current"):
			io.WriteString(w, `<ENVELOPE><LEDGER NAME="SBI Current"><BANKNAME>State Bank</BANKNAME><BANKACCOUNTNUMBER>123456789012</BANKACCOUNTNUMBER><IFSCCODE>SBIN0000123</IFSCCODE></LEDGER></ENVELOPE>`)
		default:
			io.WriteString(w, `<ENVELOPE><COLLECTION/></ENVELOPE>`)
		}
	})
	defer ts.Close()
	now := time.Now()
	_ = s.cache.Put("ledgers", "Demo Company", "", LedgerIndexData{Ledgers: []LedgerIndexRow{{Name: "ABC Traders", Parent: "Sundry Debtors"}, {Name: "SBI Current", Parent: "Bank Accounts"}}, Count: 2}, now)
	_ = s.cache.Put("receivable-party", "Demo Company", "", OutstandingData{Parties: []OutstandingParty{{Party: "ABC Traders", Amount: 48500, DrCr: "Dr"}}, Total: 48500, Count: 1}, now)
	_ = s.cache.Put("voucher-detail", "Demo Company", "1", VoucherDetail{CommonData: CommonData{Source: "saved", FetchedAt: now}, Company: "Demo Company", FirmName: "Demo Company", MasterID: "1", InvoiceNumber: "INV-1", InvoiceDate: "2026-09-24", CustomerName: "ABC Traders", Items: []InvoiceItem{{Name: "मऊ टॉवेल - Kitchen Napkin", Quantity: "10 pcs", Rate: "100/pcs", Amount: 1000}}, TotalAmount: 1000}, now)
	_ = s.cache.Put("sales", "Demo Company", "fy", VoucherData{Vouchers: []VoucherRow{{MasterID: "1", Number: "INV-1", Party: "ABC Traders", Date: "2026-09-24", Amount: 1000}}, Total: 1000, Count: 1}, now)
	code, _ := s.store.GeneratePairCode()
	d, e := s.store.Pair(code, "Browser test")
	if e != nil {
		t.Fatal(e)
	}
	b, _ := json.Marshal(map[string]string{"deviceId": d.ID, "deviceKey": d.Key})
	_ = os.WriteFile(os.Getenv("TB_BROWSER_FIXTURE"), b, 0600)
	// Serve fixture only on loopback; credentials belong to disposable test data.
	t.Fatal(http.ListenAndServe("127.0.0.1:18765", s.Handler()))
}

func TestDisplayedOutstandingIsThePDFSnapshot(t *testing.T) {
	s, ts := testServerWithTally(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `<ENVELOPE><COLLECTION/></ENVELOPE>`) })
	defer ts.Close()
	_ = s.cache.Put("ledger-outstanding", "A", "Customer", LedgerOutstandingData{Total: 999, Bills: []BillRow{{BillNumber: "Old", PendingAmount: 999}}}, time.Now().Add(-48*time.Hour))
	at := time.Now().Add(-time.Hour)
	_ = s.cache.Put("receivable-party", "A", "", OutstandingData{Parties: []OutstandingParty{{Party: "Customer", Amount: 120, DrCr: "Dr"}}}, at)
	shown, e := s.getLedgerOutstanding("A", "Customer")
	if e != nil {
		t.Fatal(e)
	}
	var pdfSnapshot LedgerOutstandingData
	if _, ok := s.cache.Get("ledger-outstanding-display", "A", "Customer", &pdfSnapshot); !ok || pdfSnapshot.Total != shown.Total || !pdfSnapshot.SummaryOnly || pdfSnapshot.Live {
		t.Fatal("PDF does not match shown balance", pdfSnapshot)
	}
}
func TestReceivableCreditsOffsetDebits(t *testing.T) {
	s, ts := testServerWithTally(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `<ENVELOPE><LEDGER NAME="Debtor"><CLOSINGBALANCE>500 Dr</CLOSINGBALANCE></LEDGER><LEDGER NAME="Advance"><CLOSINGBALANCE>100 Cr</CLOSINGBALANCE></LEDGER></ENVELOPE>`)
	})
	defer ts.Close()
	x, e := s.getOutstanding("A", "receivable", true)
	if e != nil || x.Total != 400 {
		t.Fatal(x, e)
	}
}
