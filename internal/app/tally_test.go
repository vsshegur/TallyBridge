package app

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

func TestStrictReadOnly(t *testing.T) {
	if err := validateExportXML([]byte(`<ENVELOPE><HEADER><TALLYREQUEST>Export</TALLYREQUEST></HEADER></ENVELOPE>`)); err != nil {
		t.Fatal(err)
	}
	for _, x := range []string{`<TALLYREQUEST>Import</TALLYREQUEST>`, `<TALLYREQUEST>Export</TALLYREQUEST><VOUCHER ACTION="Create">`, `<TALLYREQUEST>Export</TALLYREQUEST><VOUCHER ACTION="Alter">`} {
		if validateExportXML([]byte(x)) == nil {
			t.Fatalf("write XML must be rejected: %s", x)
		}
	}
}
func TestSanitizeIllegalXMLControl(t *testing.T) {
	in := []byte("<A>OK\x04TEXT</A>")
	out, n := sanitizeXML(in)
	if n != 1 || string(out) != "<A>OKTEXT</A>" {
		t.Fatalf("cleanup got %q n=%d", out, n)
	}
}
func TestSanitizeIllegalXMLNumericReferences(t *testing.T) {
	in := []byte(`<A>one&#4;two&#x4;three&#X0004;four&#9;tab&#x20;space</A>`)
	out, n := sanitizeXML(in)
	want := `<A>onetwothreefour&#9;tab&#x20;space</A>`
	if n != 3 || string(out) != want {
		t.Fatalf("numeric cleanup got %q n=%d", out, n)
	}
	dec := xml.NewDecoder(bytes.NewReader(out))
	for {
		_, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("sanitized XML must parse: %v", err)
		}
	}
}
func TestVoucherDetailHydratesRequiredDispatchFields(t *testing.T) {
	xmlb := []byte(`<ENVELOPE><BODY><DATA><COLLECTION><VOUCHER><MASTERID>396</MASTERID><BASICBUYERNAME>ABC Traders</BASICBUYERNAME><BASICBUYERCITY>Solapur</BASICBUYERCITY><BASICSHIPPEDBY>VRL Logistics</BASICSHIPPEDBY><BILLOFLADINGNO>LR-9988</BILLOFLADINGNO><BILLOFLADINGDATE>20260924</BILLOFLADINGDATE><ALLINVENTORYENTRIES.LIST><STOCKITEMNAME>Kitchen Napkin</STOCKITEMNAME><BILLEDQTY>10 pcs</BILLEDQTY><RATE>100.00/pcs</RATE><AMOUNT>-1000.00</AMOUNT></ALLINVENTORYENTRIES.LIST><ALLLEDGERENTRIES.LIST><LEDGERNAME>Discount</LEDGERNAME><AMOUNT>50.00</AMOUNT></ALLLEDGERENTRIES.LIST><ALLLEDGERENTRIES.LIST><LEDGERNAME>ABC Traders</LEDGERNAME><AMOUNT>-950.00</AMOUNT></ALLLEDGERENTRIES.LIST></VOUCHER></COLLECTION></DATA></BODY></ENVELOPE>`)
	hint := VoucherRow{MasterID: "396", Number: "VT-396", Date: "2026-09-24", Party: "ABC Traders", Amount: 950}
	x := parseVoucherDetail(xmlb, "DEMO TEXTILES", "396", hint)
	if x.InvoiceNumber != "VT-396" || x.InvoiceDate != "2026-09-24" || x.DispatchedThrough != "VRL Logistics" || x.LRNumber != "LR-9988" {
		t.Fatalf("critical fields missing: %+v", x)
	}
	if !strings.Contains(x.ShareText, "Invoice Number - VT-396") || !strings.Contains(x.ShareText, "Invoice Date - 24-09-2026") || !strings.Contains(x.ShareText, "Dispatched Through - VRL Logistics") || !strings.Contains(x.ShareText, "LR-RR No - LR-9988") {
		t.Fatalf("share text incomplete:\n%s", x.ShareText)
	}
	pdf := []byte(invoiceHTML(x))
	for _, want := range []string{"VT-396", "24-09-2026", "VRL Logistics", "LR-9988", "Kitchen Napkin"} {
		if !bytes.Contains(pdf, []byte(want)) {
			t.Fatalf("PDF missing %q", want)
		}
	}
}
func TestVoucherDetailReadsNestedAliases(t *testing.T) {
	xmlb := []byte(`<VOUCHER><VOUCHERNUMBER>INV-7</VOUCHERNUMBER><DATE>20260923</DATE><BASICSHIPPINGDETAILS.LIST><DISPATCHEDTHROUGH>Blue Dart</DISPATCHEDTHROUGH><TRANSPORTDOCNO>RR-7</TRANSPORTDOCNO></BASICSHIPPINGDETAILS.LIST></VOUCHER>`)
	x := parseVoucherDetail(xmlb, "Firm", "7", VoucherRow{})
	if x.DispatchedThrough != "Blue Dart" || x.LRNumber != "RR-7" {
		t.Fatalf("nested fallback aliases failed: %+v", x)
	}
}
func TestPhoneHasFullPreviewAndThreeActions(t *testing.T) {
	b, e := webFS.ReadFile("web/phone.js")
	if e != nil {
		t.Fatal(e)
	}
	s := string(b)
	for _, w := range []string{"Share Full Invoice PDF", "Share Details Only", "View PDF Here", "Full invoice preview", "invoice-preview-page", "data-date=", "data-party="} {
		if !strings.Contains(s, w) {
			t.Fatalf("phone UI missing %q", w)
		}
	}
	if strings.Contains(s, "<iframe class=\"pdf-frame\"") {
		t.Fatal("Android iframe PDF preview must not be used")
	}
}
func TestVoucherCollectionIncludesStandardTallyDispatchMethods(t *testing.T) {
	x := string(voucherDetailCollectionXML("Demo", "396"))
	for _, w := range []string{"BasicShippedBy", "BasicShipDocumentNo", "BillOfLadingNo", "BillOfLadingDate", "VoucherNumber", "Date"} {
		if !strings.Contains(x, w) {
			t.Fatalf("voucher XML missing %s", w)
		}
	}
}

func TestVoucherDetailReadsCustomUDFStyleDispatchTags(t *testing.T) {
	xmlb := []byte(`<VOUCHER><UDFINVOICENO>VT-901</UDFINVOICENO><UDFINVOICEDATE>20260924</UDFINVOICEDATE><UDFDISPATCHEDTHROUGH>Gati Cargo</UDFDISPATCHEDTHROUGH><UDFLRRRNO>RR-901</UDFLRRRNO></VOUCHER>`)
	x := parseVoucherDetail(xmlb, "Firm", "901", VoucherRow{})
	if x.InvoiceNumber != "VT-901" || x.InvoiceDate != "2026-09-24" || x.DispatchedThrough != "Gati Cargo" || x.LRNumber != "RR-901" {
		t.Fatalf("custom/UDF dispatch tags not recovered: %+v", x)
	}
}

func TestIndianNumberFormatting(t *testing.T) {
	if got := indianNumber(12345678.9); got != "1,23,45,678.90" {
		t.Fatalf("Indian number format = %q", got)
	}
}

func TestDispatchShareOmitsZeroDiscount(t *testing.T) {
	x := VoucherDetail{FirmName: "Firm", CustomerName: "Party", InvoiceNumber: "INV-1", InvoiceDate: "2026-09-24", TotalAmount: 1250, DiscountAmount: 0}
	txt := dispatchShareText(x)
	if strings.Contains(txt, "Discount Amount") {
		t.Fatalf("zero discount must be omitted: %s", txt)
	}
	if !strings.Contains(txt, "Total Amount - ₹1,250.00") {
		t.Fatalf("total missing: %s", txt)
	}
}

func TestLedgerAndGroupShareText(t *testing.T) {
	ls := LedgerStatementData{Company: "DEMO TEXTILES", Ledger: "ABC Traders", FromDate: "2026-09-01", ToDate: "2026-09-24", Vouchers: []VoucherRow{{Date: "2026-09-24", Number: "VT-1", VoucherType: "Sales", Amount: 500}}, Total: 500, Count: 1}
	if txt := ledgerStatementShareText(ls); !strings.Contains(txt, "Ledger Statement") || !strings.Contains(txt, "VT-1") || !strings.Contains(txt, "₹500.00") {
		t.Fatalf("bad ledger share: %s", txt)
	}
	lo := LedgerOutstandingData{Company: "DEMO TEXTILES", Ledger: "ABC Traders", Bills: []BillRow{{BillNumber: "VT-1", Date: "2026-09-24", PendingAmount: 500}}, Total: 500, Count: 1}
	if txt := ledgerOutstandingShareText(lo); !strings.Contains(txt, "Outstanding Balance") || !strings.Contains(txt, "Total Outstanding - ₹500.00") {
		t.Fatalf("bad outstanding share: %s", txt)
	}
	g := GroupBalanceData{Company: "DEMO TEXTILES", Group: "Dealers", Ledgers: []GroupLedgerRow{{Ledger: "ABC Traders", Balance: 500, DrCr: "Dr"}}, Total: 500, Count: 1}
	if txt := groupBalanceShareText(g); !strings.Contains(txt, "Ledger Group Details") || !strings.Contains(txt, "ABC Traders - ₹500.00 Dr") {
		t.Fatalf("bad group share: %s", txt)
	}
}

func TestPhoneLedgerSharingAndGroupsUI(t *testing.T) {
	b, e := webFS.ReadFile("web/phone.js")
	if e != nil {
		t.Fatal(e)
	}
	s := string(b)
	for _, w := range []string{"Show by Date", "Outstanding Balance", "Share Text Details", "Share PDF", "Groups", "group-balances", "ledger-statement-pdf", "ledger-outstanding-pdf"} {
		if !strings.Contains(s, w) {
			t.Fatalf("phone UI missing %q", w)
		}
	}
}

func TestV150PhoneUpgradeFeaturesPresent(t *testing.T) {
	b, e := webFS.ReadFile("web/phone.js")
	if e != nil {
		t.Fatal(e)
	}
	s := string(b)
	for _, w := range []string{
		"Customer 360", "Today’s Dispatch", "Universal Search", "Outstanding Ageing",
		"Purchase Register", "Bank account for customer PDFs", "customer360-pdf",
		"today-dispatch", "search-index", "register-month-open", "Share 360 Text",
	} {
		if !strings.Contains(s, w) {
			t.Fatalf("v1.5 phone UI missing %q", w)
		}
	}
}
