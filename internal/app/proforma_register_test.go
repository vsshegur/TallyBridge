package app

import (
	"strings"
	"testing"
	"time"
)

func TestVoucherListRequestsOptionalFlag(t *testing.T) {
	xml := string(voucherListXML("Demo Co", "Sales", "20260401", "20270331"))
	if !strings.Contains(xml, "<NATIVEMETHOD>IsOptional</NATIVEMETHOD>") {
		t.Fatal("Sales register XML must request IsOptional so posted and proforma vouchers can be separated")
	}
}

func TestOptionalSalesAreSeparatedFromPostedSales(t *testing.T) {
	raw := []byte(`<ENVELOPE><BODY><DATA><COLLECTION>
<VOUCHER><MASTERID>1</MASTERID><DATE>20261001</DATE><VOUCHERNUMBER>S-1</VOUCHERNUMBER><PARTYLEDGERNAME>Posted Buyer</PARTYLEDGERNAME><VOUCHERTYPENAME>Sales</VOUCHERTYPENAME><ISOPTIONAL>No</ISOPTIONAL><AMOUNT>-100</AMOUNT></VOUCHER>
<VOUCHER><MASTERID>2</MASTERID><DATE>20261002</DATE><VOUCHERNUMBER>PF-1</VOUCHERNUMBER><PARTYLEDGERNAME>Proforma Buyer</PARTYLEDGERNAME><VOUCHERTYPENAME>Sales</VOUCHERTYPENAME><ISOPTIONAL>Yes</ISOPTIONAL><AMOUNT>-200</AMOUNT></VOUCHER>
</COLLECTION></DATA></BODY></ENVELOPE>`)
	rows := parseVouchers(raw)
	if len(rows) != 2 {
		t.Fatalf("expected 2 vouchers, got %d", len(rows))
	}
	if rows[0].Optional || !rows[1].Optional {
		t.Fatalf("optional flags were not parsed correctly: %#v", rows)
	}
	sales := voucherRowsForRegister(rows, "sales")
	proforma := voucherRowsForRegister(rows, "proforma")
	if len(sales) != 1 || sales[0].Number != "S-1" {
		t.Fatalf("posted Sales register contains wrong rows: %#v", sales)
	}
	if len(proforma) != 1 || proforma[0].Number != "PF-1" {
		t.Fatalf("Proforma register contains wrong rows: %#v", proforma)
	}
	purchase := voucherRowsForRegister(rows, "purchase")
	if len(purchase) != 1 || purchase[0].Optional {
		t.Fatalf("optional vouchers must not leak into posted registers: %#v", purchase)
	}
}

func TestVoucherDataTotalsOnlyItsOwnRegister(t *testing.T) {
	hundred, twoHundred := 100.0, 200.0
	rows := []VoucherRow{{Number: "S-1", Amount: hundred}, {Number: "PF-1", Amount: twoHundred, Optional: true}}
	at := time.Now()
	sales := voucherDataFromRows(voucherRowsForRegister(rows, "sales"), "sales", "fy", 12, at)
	proforma := voucherDataFromRows(voucherRowsForRegister(rows, "proforma"), "proforma", "fy", 12, at)
	if sales.Count != 1 || sales.Total != hundred {
		t.Fatalf("sales total includes optional voucher: %#v", sales)
	}
	if proforma.Count != 1 || proforma.Total != twoHundred {
		t.Fatalf("proforma total wrong: %#v", proforma)
	}
}

func TestInvoicePDFNameStartsWithNumberAndParty(t *testing.T) {
	got := invoicePDFBaseName(VoucherDetail{InvoiceNumber: "12345", CustomerName: "ABC Traders"})
	if got != "12345-ABC-Traders" {
		t.Fatalf("unexpected invoice PDF base name %q", got)
	}
	if strings.HasPrefix(strings.ToLower(got), "invoice") {
		t.Fatalf("invoice PDF name must not start with Invoice: %q", got)
	}
}

func TestClassifiedCacheVersionRequired(t *testing.T) {
	if voucherCacheIsClassified(VoucherData{}) {
		t.Fatal("pre-upgrade mixed register cache must not be trusted")
	}
	x := voucherDataFromRows([]VoucherRow{{Number: "S-1", Amount: 10}}, "sales", "month", 1, time.Now())
	if !voucherCacheIsClassified(x) {
		t.Fatal("newly separated register cache must be classified")
	}
}

func TestProformaInvoiceDocumentIdentity(t *testing.T) {
	html := invoiceHTML(VoucherDetail{Company: "Demo", FirmName: "Demo", InvoiceNumber: "PF-1", CustomerName: "Buyer", Optional: true})
	if !strings.Contains(html, "PROFORMA INVOICE") || !strings.Contains(html, "Optional / non-posting") {
		t.Fatalf("optional voucher PDF is not visibly proforma: %s", html)
	}
	share := dispatchShareText(VoucherDetail{Optional: true, FirmName: "Demo", InvoiceNumber: "PF-1"})
	if !strings.HasPrefix(share, "Proforma Details") {
		t.Fatalf("optional voucher share text is not proforma: %q", share)
	}
}
