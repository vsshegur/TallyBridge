package app

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestOutstandingSiblingBills(t *testing.T) {
	xml := `<ENVELOPE><BILLFIXED><BILLDATE>1-Sep-2026</BILLDATE><BILLREF>A-1</BILLREF><BILLPARTY>ABC</BILLPARTY></BILLFIXED><BILLOP>-1200</BILLOP><BILLCL>-800</BILLCL><BILLDUE>30-Sep-2026</BILLDUE><BILLOVERDUE>0</BILLOVERDUE><BILLFIXED><BILLREF>OTHER</BILLREF><BILLPARTY>Other Party</BILLPARTY></BILLFIXED><BILLCL>-9999</BILLCL><BILLFIXED><BILLREF>Missing balance</BILLREF></BILLFIXED><BILLFIXED><BILLDATE>2-Sep-2026</BILLDATE><BILLREF>Credit-1</BILLREF><BILLPARTY>ABC</BILLPARTY></BILLFIXED><BILLCL>100</BILLCL></ENVELOPE>`
	s, ts := testServerWithTally(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, xml) })
	defer ts.Close()
	x, e := s.getLedgerOutstanding("Demo", "ABC")
	if e != nil {
		t.Fatal(e)
	}
	if x.SummaryOnly || x.Count != 2 || x.Total != 700 || x.DrCr != "Dr" {
		t.Fatalf("incorrect net outstanding: %+v", x)
	}
	if x.Bills[0].PendingAmount != 800 || x.Bills[0].OriginalAmount != 1200 || x.Bills[0].DueDate != "2026-09-30" {
		t.Fatalf("incorrect bill fields: %+v", x.Bills)
	}
	if !strings.Contains(x.ShareText, "A-1") || !strings.Contains(x.ShareText, "Credit-1") || strings.Contains(x.ShareText, "OTHER") {
		t.Fatal(x.ShareText)
	}
	if !strings.Contains(tableHTML(outstandingDoc(x)), "A-1") {
		t.Fatal("bill absent from PDF")
	}
}

func TestOutstandingPrefersPendingOverOriginalAmount(t *testing.T) {
	rows := parsePartyBills([]byte(`<BILL><BILLREF>A</BILLREF><AMOUNT>-1200</AMOUNT><CLOSINGBALANCE>-400</CLOSINGBALANCE></BILL>`), "ABC")
	if len(rows) != 1 || rows[0].PendingAmount != 400 {
		t.Fatalf("wrong pending amount: %+v", rows)
	}
}

func TestPDFMustBeComplete(t *testing.T) {
	for _, data := range []string{"", "%PDF-1.7\npartial", "<html>error</html>"} {
		if completePDF([]byte(data)) {
			t.Fatal("accepted incomplete PDF")
		}
	}
	if !completePDF([]byte("%PDF-1.7\ncontent\n%%EOF\n")) {
		t.Fatal("rejected complete PDF")
	}
}
