package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testServerWithTally(t *testing.T, h http.HandlerFunc) (*Server, *httptest.Server) {
	t.Helper()
	ts := httptest.NewServer(h)
	base := t.TempDir()
	s, e := NewServer(base)
	if e != nil {
		t.Fatal(e)
	}
	cfg := s.store.Settings()
	cfg.TallyURL = ts.URL
	cfg.TimeoutSec = 2
	cfg.CooldownSec = 5
	if e = s.store.UpdateSettings(cfg); e != nil {
		t.Fatal(e)
	}
	return s, ts
}

func TestInvoiceDetailUsesSalesHintsAndStandardDispatch(t *testing.T) {
	var calls int32
	s, ts := testServerWithTally(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		b, _ := io.ReadAll(r.Body)
		q := string(b)
		if !strings.Contains(q, "<TALLYREQUEST>Export</TALLYREQUEST>") {
			t.Errorf("non-export sent")
		}
		io.WriteString(w, `<ENVELOPE><BODY><DATA><COLLECTION><VOUCHER><MASTERID>396</MASTERID><BASICBUYERNAME>ABC Traders</BASICBUYERNAME><BASICBUYERCITY>Solapur</BASICBUYERCITY><BASICSHIPPEDBY>VRL Logistics</BASICSHIPPEDBY><BILLOFLADINGNO>LR-396</BILLOFLADINGNO><ALLINVENTORYENTRIES.LIST><STOCKITEMNAME>Towel</STOCKITEMNAME><BILLEDQTY>10 pcs</BILLEDQTY><RATE>125.00/pcs</RATE><AMOUNT>-1250</AMOUNT></ALLINVENTORYENTRIES.LIST><ALLLEDGERENTRIES.LIST><LEDGERNAME>ABC Traders</LEDGERNAME><AMOUNT>-1250</AMOUNT></ALLLEDGERENTRIES.LIST></VOUCHER></COLLECTION></DATA></BODY></ENVELOPE>`)
	})
	defer ts.Close()
	hint := VoucherRow{MasterID: "396", Number: "VT-396", Date: "2026-09-24", Party: "ABC Traders", Amount: 1250}
	x, e := s.fetchVoucherDetail("DEMO TEXTILES", "396", hint)
	if e != nil {
		t.Fatal(e)
	}
	if calls != 1 {
		t.Fatalf("expected one compact voucher request, got %d", calls)
	}
	if x.InvoiceNumber != "VT-396" || x.InvoiceDate != "2026-09-24" || x.DispatchedThrough != "VRL Logistics" || x.LRNumber != "LR-396" {
		t.Fatalf("bad detail %+v", x)
	}
	if !strings.Contains(x.ShareText, "3. Invoice Number - VT-396") || !strings.Contains(x.ShareText, "5. Dispatched Through - VRL Logistics") {
		t.Fatalf("bad share text %s", x.ShareText)
	}
	var saved VoucherDetail
	if _, ok := s.cache.Get("voucher-detail", "DEMO TEXTILES", "396", &saved); !ok || saved.LRNumber != "LR-396" {
		t.Fatal("invoice detail not atomically saved")
	}
}

func TestInvoiceDetailEnrichesMissingDispatchFromObject(t *testing.T) {
	var calls int32
	s, ts := testServerWithTally(t, func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			io.WriteString(w, `<ENVELOPE><VOUCHER><MASTERID>5</MASTERID><VOUCHERNUMBER>INV-5</VOUCHERNUMBER><DATE>20260924</DATE></VOUCHER></ENVELOPE>`)
			return
		}
		io.WriteString(w, `<ENVELOPE><VOUCHER><MASTERID>5</MASTERID><VOUCHERNUMBER>INV-5</VOUCHERNUMBER><DATE>20260924</DATE><BASICSHIPPINGDETAILS.LIST><BASICSHIPPEDBY>TCI Freight</BASICSHIPPEDBY><BILLOFLADINGNO>LR55</BILLOFLADINGNO></BASICSHIPPINGDETAILS.LIST><ALLINVENTORYENTRIES.LIST><STOCKITEMNAME>Napkin</STOCKITEMNAME><BILLEDQTY>6 pcs</BILLEDQTY><RATE>50/pcs</RATE><AMOUNT>-300</AMOUNT></ALLINVENTORYENTRIES.LIST></VOUCHER></ENVELOPE>`)
	})
	defer ts.Close()
	x, e := s.fetchVoucherDetail("Demo", "5", VoucherRow{Number: "INV-5", Date: "2026-09-24", Amount: 300})
	if e != nil {
		t.Fatal(e)
	}
	if calls != 2 {
		t.Fatalf("expected compact + object enrich, got %d", calls)
	}
	if x.DispatchedThrough != "TCI Freight" || x.LRNumber != "LR55" || len(x.Items) != 1 {
		t.Fatalf("enrich failed %+v", x)
	}
}

func TestRequestGateRejectsConcurrentRead(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	s, ts := testServerWithTally(t, func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		io.WriteString(w, `<ENVELOPE><BODY/></ENVELOPE>`)
	})
	defer ts.Close()
	done := make(chan error, 1)
	go func() {
		_, _, _, e := s.tally.request("one", "Demo", genericReportXML("Demo", "Trial Balance"), time.Second)
		done <- e
	}()
	<-entered
	if _, _, _, e := s.tally.request("two", "Demo", genericReportXML("Demo", "Day Book"), time.Second); e != ErrBusy {
		t.Fatalf("second read should be ErrBusy, got %v", e)
	}
	close(release)
	if e := <-done; e != nil {
		t.Fatal(e)
	}
}

func TestPDFIsRenderableStructure(t *testing.T) {
	x := VoucherDetail{FirmName: "DEMO TEXTILES", InvoiceNumber: "VT-396", InvoiceDate: "2026-09-24", CustomerName: "ABC Traders", CustomerCity: "Solapur", DispatchedThrough: "VRL Logistics", LRNumber: "LR-396", Items: []InvoiceItem{{Name: "Kitchen Napkin", Quantity: "10 pcs", Rate: "100/pcs", Amount: 1000}}, TotalAmount: 1000}
	b := invoiceHTML(x)
	if !strings.Contains(b, "<table class=\"items\">") || !strings.Contains(b, "Total amount") || !strings.Contains(string(b), "VRL Logistics") {
		t.Fatal("invalid/incomplete PDF")
	}
	_ = filepath.Base("x")
}

func TestLedgerStatementAndGroupAreSingleReadAndShareable(t *testing.T) {
	var calls int32
	s, ts := testServerWithTally(t, func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		b, _ := io.ReadAll(r.Body)
		q := string(b)
		if n == 1 {
			if !strings.Contains(q, "TBLiveLedgerVouchers") || !strings.Contains(q, "20260901") || !strings.Contains(q, "20260924") {
				t.Errorf("bad ledger XML: %s", q)
			}
			io.WriteString(w, `<ENVELOPE><VOUCHER><MASTERID>1</MASTERID><DATE>20260924</DATE><VOUCHERNUMBER>VT-1</VOUCHERNUMBER><VOUCHERTYPENAME>Sales</VOUCHERTYPENAME><PARTYLEDGERNAME>ABC Traders</PARTYLEDGERNAME><AMOUNT>-500</AMOUNT></VOUCHER></ENVELOPE>`)
			return
		}
		if !strings.Contains(q, "TBGroupLedgers") || !strings.Contains(q, "<CHILDOF>Dealers</CHILDOF>") {
			t.Errorf("bad group XML: %s", q)
		}
		io.WriteString(w, `<ENVELOPE><LEDGER><NAME>ABC Traders</NAME><PARENT>Dealers</PARENT><CLOSINGBALANCE>-500 Dr</CLOSINGBALANCE></LEDGER></ENVELOPE>`)
	})
	defer ts.Close()
	ls, e := s.getLedgerStatement("DEMO TEXTILES", "ABC Traders", "2026-09-01", "2026-09-24", "20260901", "20260924")
	if e != nil {
		t.Fatal(e)
	}
	if ls.Count != 1 || !strings.Contains(ls.ShareText, "VT-1") {
		t.Fatalf("bad ledger statement %+v", ls)
	}
	g, e := s.getGroupBalances("DEMO TEXTILES", "Dealers")
	if e != nil {
		t.Fatal(e)
	}
	if calls != 2 || g.Count != 1 || g.Ledgers[0].Ledger != "ABC Traders" || !strings.Contains(g.ShareText, "ABC Traders") {
		t.Fatalf("bad group result calls=%d %+v", calls, g)
	}
}

func TestLedgerOutstandingShareAndCache(t *testing.T) {
	s, ts := testServerWithTally(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `<ENVELOPE><BILL><BILLREF>VT-1</BILLREF><BILLDATE>20260924</BILLDATE><DUEDATE>20261015</DUEDATE><PENDINGAMOUNT>500</PENDINGAMOUNT></BILL></ENVELOPE>`)
	})
	defer ts.Close()
	x, e := s.getLedgerOutstanding("DEMO TEXTILES", "ABC Traders")
	if e != nil {
		t.Fatal(e)
	}
	if x.Total == 0 || !strings.Contains(x.ShareText, "Outstanding Balance") {
		t.Fatalf("bad outstanding %+v", x)
	}
	var saved LedgerOutstandingData
	if _, ok := s.cache.Get("ledger-outstanding", "DEMO TEXTILES", "ABC Traders", &saved); !ok {
		t.Fatal("outstanding not cached")
	}
}
