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
