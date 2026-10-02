package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestInvoiceDiscountGSTUsesPartyTotalNotLargestAmount(t *testing.T) {
	xml := []byte(`<VOUCHER><PARTYLEDGERNAME>Buyer</PARTYLEDGERNAME><VOUCHERNUMBER>INV-1</VOUCHERNUMBER><ALLINVENTORYENTRIES.LIST><STOCKITEMNAME>Towels</STOCKITEMNAME><AMOUNT>1000</AMOUNT></ALLINVENTORYENTRIES.LIST><LEDGERENTRIES.LIST><LEDGERNAME>Sales</LEDGERNAME><AMOUNT>1000</AMOUNT></LEDGERENTRIES.LIST><LEDGERENTRIES.LIST><LEDGERNAME>Discount</LEDGERNAME><AMOUNT>-200</AMOUNT></LEDGERENTRIES.LIST><LEDGERENTRIES.LIST><LEDGERNAME>CGST</LEDGERNAME><AMOUNT>72</AMOUNT></LEDGERENTRIES.LIST><LEDGERENTRIES.LIST><LEDGERNAME>SGST</LEDGERNAME><AMOUNT>72</AMOUNT></LEDGERENTRIES.LIST><LEDGERENTRIES.LIST><LEDGERNAME>Buyer</LEDGERNAME><AMOUNT>-944</AMOUNT><BILLALLOCATIONS.LIST><AMOUNT>-944</AMOUNT></BILLALLOCATIONS.LIST></LEDGERENTRIES.LIST></VOUCHER>`)
	x := parseVoucherDetail(xml, "Firm", "1", VoucherRow{Amount: 1000})
	if x.TotalAmount != 944 || x.TaxableAmount != 800 || x.DiscountAmount != 200 || x.CGST != 72 || x.SGST != 72 || x.OtherCharges != 0 || x.TotalsVersion != 2 {
		t.Fatalf("wrong invoice: %+v", x)
	}
	if !strings.Contains(x.ShareText, "Total Amount - ₹944.00") {
		t.Fatal(x.ShareText)
	}
	html := invoiceHTML(x)
	if !strings.Contains(html, "944.00") || !strings.Contains(html, "800.00") || !strings.Contains(html, "Invoice discount (deducted)") {
		t.Fatal("PDF totals missing")
	}
	rows := parseVouchers(xml)
	if len(rows) != 1 || rows[0].Amount != 944 {
		t.Fatal(rows)
	}
}
func TestItemPercentDiscountIsNotMoneyAndRepeatedItemsAreKept(t *testing.T) {
	xml := []byte(`<VOUCHER><PARTYLEDGERNAME>Buyer</PARTYLEDGERNAME><ALLINVENTORYENTRIES.LIST><STOCKITEMNAME>Towel</STOCKITEMNAME><RATE>100/pcs</RATE><BILLEDQTY>1 pcs</BILLEDQTY><DISCOUNT>10</DISCOUNT><AMOUNT>90</AMOUNT></ALLINVENTORYENTRIES.LIST><ALLINVENTORYENTRIES.LIST><STOCKITEMNAME>Towel</STOCKITEMNAME><RATE>100/pcs</RATE><BILLEDQTY>1 pcs</BILLEDQTY><DISCOUNT>10</DISCOUNT><AMOUNT>90</AMOUNT></ALLINVENTORYENTRIES.LIST><LEDGERENTRIES.LIST><LEDGERNAME>IGST</LEDGERNAME><AMOUNT>32.4</AMOUNT></LEDGERENTRIES.LIST><LEDGERENTRIES.LIST><LEDGERNAME>Round Off</LEDGERNAME><AMOUNT>-0.4</AMOUNT></LEDGERENTRIES.LIST><LEDGERENTRIES.LIST><LEDGERNAME>Buyer</LEDGERNAME><AMOUNT>-212</AMOUNT></LEDGERENTRIES.LIST></VOUCHER>`)
	x := parseVoucherDetail(xml, "Firm", "1", VoucherRow{})
	if len(x.Items) != 2 || x.DiscountAmount != 0 || x.TaxableAmount != 180 || x.TotalAmount != 212 || x.RoundOff != -0.4 || x.OtherCharges != 0 {
		t.Fatal(x)
	}
	if strings.Contains(x.ShareText, "Discount Amount") {
		t.Fatal("percentage was treated as currency")
	}
}
func TestInvoiceTotalNotGuessedFromInventoryOrHint(t *testing.T) {
	x := parseVoucherDetail([]byte(`<VOUCHER><ALLINVENTORYENTRIES.LIST><STOCKITEMNAME>Towel</STOCKITEMNAME><AMOUNT>1000</AMOUNT></ALLINVENTORYENTRIES.LIST></VOUCHER>`), "Firm", "1", VoucherRow{Amount: 1000})
	if x.TotalsVersion != 0 || x.TotalAmount != 0 {
		t.Fatal("unverified amount accepted", x)
	}
}
func TestInvoiceObjectReplacesMonetarySnapshotTogether(t *testing.T) {
	a := VoucherDetail{TotalsVersion: 2, TotalAmount: 1000, TaxableAmount: 1000, Items: []InvoiceItem{{Amount: 1000}}}
	b := VoucherDetail{TotalsVersion: 2, TotalAmount: 944, TaxableAmount: 800, DiscountAmount: 200, CGST: 72, SGST: 72, Items: []InvoiceItem{{Amount: 1000}}}
	x := mergeVoucherDetail(a, b)
	if x.TotalAmount != 944 || x.DiscountAmount != 200 || x.TaxableAmount != 800 {
		t.Fatal(x)
	}
}

func TestOldInvoiceCacheCannotBeSharedAsPDF(t *testing.T) {
	s, ts := testServerWithTally(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `<ENVELOPE><COLLECTION/></ENVELOPE>`) })
	defer ts.Close()
	_ = s.cache.Put("voucher-detail", "Firm", "1", VoucherDetail{Company: "Firm", MasterID: "1", TotalAmount: 999999}, time.Now())
	r := httptest.NewRequest("GET", "/api/v1/mobile/voucher-pdf?company=Firm&masterId=1", nil)
	w := httptest.NewRecorder()
	s.handleMobileVoucherPDF(w, r)
	if w.Code != 503 {
		t.Fatalf("old monetary cache accepted: %d %s", w.Code, w.Body.String())
	}
}

func TestInvoiceDoesNotDoubleAliasLedgerAndInventoryRepresentations(t *testing.T) {
	xml := []byte(`<VOUCHER>
<PARTYLEDGERNAME>Buyer</PARTYLEDGERNAME><VOUCHERNUMBER>INV-DUP</VOUCHERNUMBER>
<ALLINVENTORYENTRIES.LIST><STOCKITEMNAME>Towel</STOCKITEMNAME><RATE>1000/pcs</RATE><BILLEDQTY>1 pcs</BILLEDQTY><AMOUNT>1000</AMOUNT></ALLINVENTORYENTRIES.LIST>
<INVENTORYENTRIES.LIST><STOCKITEMNAME>Towel</STOCKITEMNAME><RATE>1000/pcs</RATE><BILLEDQTY>1 pcs</BILLEDQTY><AMOUNT>1000</AMOUNT></INVENTORYENTRIES.LIST>
<ALLLEDGERENTRIES.LIST><LEDGERNAME>Sales</LEDGERNAME><AMOUNT>1000</AMOUNT></ALLLEDGERENTRIES.LIST>
<ALLLEDGERENTRIES.LIST><LEDGERNAME>CGST</LEDGERNAME><AMOUNT>90</AMOUNT></ALLLEDGERENTRIES.LIST>
<ALLLEDGERENTRIES.LIST><LEDGERNAME>SGST</LEDGERNAME><AMOUNT>90</AMOUNT></ALLLEDGERENTRIES.LIST>
<ALLLEDGERENTRIES.LIST><LEDGERNAME>Buyer</LEDGERNAME><ISPARTYLEDGER>Yes</ISPARTYLEDGER><AMOUNT>-1180</AMOUNT></ALLLEDGERENTRIES.LIST>
<LEDGERENTRIES.LIST><LEDGERNAME>Sales</LEDGERNAME><AMOUNT>1000</AMOUNT></LEDGERENTRIES.LIST>
<LEDGERENTRIES.LIST><LEDGERNAME>CGST</LEDGERNAME><AMOUNT>90</AMOUNT></LEDGERENTRIES.LIST>
<LEDGERENTRIES.LIST><LEDGERNAME>SGST</LEDGERNAME><AMOUNT>90</AMOUNT></LEDGERENTRIES.LIST>
<LEDGERENTRIES.LIST><LEDGERNAME>Buyer</LEDGERNAME><ISPARTYLEDGER>Yes</ISPARTYLEDGER><AMOUNT>-1180</AMOUNT></LEDGERENTRIES.LIST>
</VOUCHER>`)
	x := parseVoucherDetail(xml, "Firm", "1", VoucherRow{Amount: 1180})
	if x.TotalAmount != 1180 || x.CGST != 90 || x.SGST != 90 || x.IGST != 0 || x.TaxableAmount != 1000 || x.OtherCharges != 0 || len(x.Items) != 1 {
		t.Fatalf("duplicate aliases changed invoice: %+v", x)
	}
	if !strings.Contains(x.ShareText, "CGST Amount - ₹90.00") || !strings.Contains(x.ShareText, "SGST Amount - ₹90.00") || strings.Contains(x.ShareText, "IGST Amount") {
		t.Fatal(x.ShareText)
	}
}

func TestInvoiceDetailsShareShowsIGSTWhenApplicable(t *testing.T) {
	x := VoucherDetail{FirmName: "Firm", CustomerName: "Buyer", InvoiceNumber: "INV-IGST", InvoiceDate: "2026-09-27", IGST: 180, TotalAmount: 1180}
	text := dispatchShareText(x)
	if !strings.Contains(text, "IGST Amount - ₹180.00") || strings.Contains(text, "CGST Amount") || strings.Contains(text, "SGST Amount") {
		t.Fatal(text)
	}
}
