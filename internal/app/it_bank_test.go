package app

import (
	"strings"
	"testing"
)

func TestValidatePostingXMLAcceptsBankEntries(t *testing.T) {
	for file, want := range map[string]string{"receipt.xml": "Receipt voucher", "payment.xml": "Payment voucher", "contra.xml": "Contra voucher"} {
		got, err := validatePostingXML([]byte(itSample(t, file)), itCompany)
		if err != nil || got != want {
			t.Fatalf("%s: %q %v", file, got, err)
		}
	}
	j := strings.NewReplacer(`VCHTYPE="Payment"`, `VCHTYPE="Journal"`, "<VOUCHERTYPENAME>Payment<", "<VOUCHERTYPENAME>Journal<").Replace(itSample(t, "payment.xml"))
	if _, err := validatePostingXML([]byte(j), itCompany); err == nil {
		t.Fatal("journal voucher accepted")
	}
	// bank reads stay read-only requests
	for _, f := range []string{"bills_request.xml", "bank_entries_request.xml"} {
		if err := validateExportXML([]byte(itSample(t, f))); err != nil {
			t.Fatal(f, err)
		}
	}
}

func TestBillsReportRowsForEveryParty(t *testing.T) {
	reply := `<ENVELOPE>
<BILLFIXED><BILLDATE>10-Sep-2026</BILLDATE><BILLREF>INV/245/26-27</BILLREF><BILLPARTY>R&amp;D Traders</BILLPARTY></BILLFIXED>
<BILLCL>-42212.00</BILLCL><BILLDUE>10-Oct-2026</BILLDUE><BILLOVERDUE>9</BILLOVERDUE>
<BILLFIXED><BILLDATE>1-Aug-2026</BILLDATE><BILLREF>301</BILLREF><BILLPARTY>Vipul Agencies</BILLPARTY></BILLFIXED>
<BILLCL>-3000.00</BILLCL><BILLDUE>1-Aug-2026</BILLDUE>
</ENVELOPE>`
	rows := itTallyRows("export_bills_receivable", []byte(itSample(t, "bills_request.xml")), []byte(reply))
	if len(rows) != 2 {
		t.Fatalf("%v", rows)
	}
	byName := map[string]map[string]any{}
	for _, r := range rows {
		byName[r["name"].(string)] = r
	}
	r := byName["INV/245/26-27"]
	if r["party"] != "R&D Traders" || r["amount"] != 42212.0 || r["date"] != "2026-09-10" || r["due"] != "2026-10-10" {
		t.Fatalf("%v", r)
	}
	if byName["301"]["party"] != "Vipul Agencies" {
		t.Fatalf("%v", rows)
	}
}

func TestBankEntriesRowsUseTheBankLedgerSide(t *testing.T) {
	reply := `<ENVELOPE><BODY><DATA><COLLECTION>
<VOUCHER><DATE>20261004</DATE><VOUCHERNUMBER>55</VOUCHERNUMBER><PARTYLEDGERNAME>Ramesh Kumar</PARTYLEDGERNAME><VOUCHERTYPENAME>Payment</VOUCHERTYPENAME>
 <ALLLEDGERENTRIES.LIST><LEDGERNAME>Ramesh Kumar</LEDGERNAME><AMOUNT>-900.00</AMOUNT></ALLLEDGERENTRIES.LIST>
 <ALLLEDGERENTRIES.LIST><LEDGERNAME>HDFC Bank &amp; Co</LEDGERNAME><AMOUNT>900.00</AMOUNT></ALLLEDGERENTRIES.LIST></VOUCHER>
<VOUCHER><DATE>20261001</DATE><VOUCHERNUMBER>12</VOUCHERNUMBER><PARTYLEDGERNAME>R&amp;D Traders</PARTYLEDGERNAME><VOUCHERTYPENAME>Receipt</VOUCHERTYPENAME>
 <ALLLEDGERENTRIES.LIST><LEDGERNAME>HDFC Bank &amp; Co</LEDGERNAME><AMOUNT>-42212.00</AMOUNT></ALLLEDGERENTRIES.LIST>
 <ALLLEDGERENTRIES.LIST><LEDGERNAME>R&amp;D Traders</LEDGERNAME><AMOUNT>42212.00</AMOUNT></ALLLEDGERENTRIES.LIST></VOUCHER>
</COLLECTION></DATA></BODY></ENVELOPE>`
	rows := itTallyRows("export_bank_entries", []byte(itSample(t, "bank_entries_request.xml")), []byte(reply))
	if len(rows) != 2 || rows[0]["amount"] != 42212.0 || rows[0]["vtype"] != "Receipt" || rows[1]["amount"] != -900.0 || rows[1]["party"] != "Ramesh Kumar" {
		t.Fatalf("%v", rows)
	}
}
