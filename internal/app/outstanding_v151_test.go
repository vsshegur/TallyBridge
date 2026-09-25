package app

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestV151OutstandingPageUsesPartyBalancesContract(t *testing.T) {
	b, err := webFS.ReadFile("web/phone.js")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{"renderOutstandingParties(x.parties||x.bills)", "data-action=\"party-outstanding\""} {
		if !strings.Contains(s, want) {
			t.Fatalf("phone outstanding UI missing %q", want)
		}
	}
}

func TestV151LedgerOutstandingFallsBackToKnownPartyBalance(t *testing.T) {
	s, ts := testServerWithTally(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		// Valid Tally response, but no bill-wise nodes. Previously this produced a false ₹0.
		io.WriteString(w, `<ENVELOPE><BODY><DATA><COLLECTION></COLLECTION></DATA></BODY></ENVELOPE>`)
	})
	defer ts.Close()
	now := time.Now()
	known := OutstandingData{Parties: []OutstandingParty{{Party: "ABC Traders", Amount: 48500}}, Total: 48500, Count: 1, Type: "receivable"}
	if err := s.cache.Put("receivable-party", "DEMO TEXTILES", "", known, now); err != nil {
		t.Fatal(err)
	}
	x, err := s.getLedgerOutstanding("DEMO TEXTILES", "ABC Traders")
	if err != nil {
		t.Fatal(err)
	}
	if x.Total != 48500 || !x.SummaryOnly || x.Count != 0 {
		t.Fatalf("expected safe summary fallback, got %+v", x)
	}
	if !strings.Contains(x.ShareText, "Total Outstanding - ₹48,500.00") {
		t.Fatalf("share text did not preserve fallback total: %s", x.ShareText)
	}
}

func TestV151ParsesTallyDisplayOutstandingShape(t *testing.T) {
	xml := []byte(`<ENVELOPE><BODY><DATA><TALLYMESSAGE><DSPACCNAME><DSPDISPNAME>VT-101</DSPDISPNAME><DSPBILLDATE>20260901</DSPBILLDATE><DSPDUEDATE>20260930</DSPDUEDATE><DSPCLAMT>1250 Dr</DSPCLAMT></DSPACCNAME></TALLYMESSAGE></DATA></BODY></ENVELOPE>`)
	rows := parsePartyBills(xml, "ABC Traders")
	if len(rows) != 1 || rows[0].BillNumber != "VT-101" || rows[0].PendingAmount != 1250 {
		t.Fatalf("display-style outstanding not parsed: %+v", rows)
	}
}
