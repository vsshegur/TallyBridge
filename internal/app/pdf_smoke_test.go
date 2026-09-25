package app

import (
	"bytes"
	"os"
	"testing"
)

func TestWritePDFSmoke(t *testing.T) {
	p := os.Getenv("TB_PDF_SMOKE")
	if p == "" {
		t.Skip("smoke output not requested")
	}
	x := VoucherDetail{FirmName: "DEMO TEXTILES", InvoiceNumber: "VT-396", InvoiceDate: "2026-09-24", CustomerName: "ABC Traders", CustomerCity: "Solapur", GSTIN: "27ABCDE1234F1Z5", DispatchedThrough: "VRL Logistics", LRNumber: "LR-396", LRDate: "2026-09-24", Destination: "Pune", VehicleNumber: "MH13AB1234", Items: []InvoiceItem{{Name: "Microfiber Kitchen Napkin", HSN: "6302", Quantity: "10 pcs", Rate: "100.00/pcs", Amount: 1000}, {Name: "Bath Towel 70x140 cm", HSN: "6302", Quantity: "2 pcs", Rate: "350.00/pcs", Amount: 700}}, DiscountAmount: 50, TaxableAmount: 1650, CGST: 148.5, SGST: 148.5, TotalAmount: 1947}
	data, err := invoicePDF(x)
	if err != nil {
		t.Fatal(err)
	}
	if e := os.WriteFile(p, data, 0644); e != nil {
		t.Fatal(e)
	}
}

func TestLedgerAndGroupPDFs(t *testing.T) {
	ls := LedgerStatementData{Company: "DEMO TEXTILES", Ledger: "ABC Traders", FromDate: "2026-09-01", ToDate: "2026-09-24", Vouchers: []VoucherRow{{Date: "2026-09-24", Number: "VT-1", VoucherType: "Sales", Party: "ABC Traders", Amount: 500}}, Total: 500, Count: 1}
	if b := []byte(tableHTML(statementDoc(ls))); !bytes.Contains(b, []byte("LEDGER STATEMENT")) || !bytes.Contains(b, []byte("VT-1")) {
		t.Fatal("ledger statement PDF incomplete")
	}
	lo := LedgerOutstandingData{Company: "DEMO TEXTILES", Ledger: "ABC Traders", Bills: []BillRow{{BillNumber: "VT-1", Date: "2026-09-24", PendingAmount: 500}}, Total: 500, Count: 1}
	if b := []byte(tableHTML(outstandingDoc(lo))); !bytes.Contains(b, []byte("OUTSTANDING BALANCE")) || !bytes.Contains(b, []byte("VT-1")) {
		t.Fatal("outstanding PDF incomplete")
	}
	g := GroupBalanceData{Company: "DEMO TEXTILES", Group: "Dealers", Ledgers: []GroupLedgerRow{{Ledger: "ABC Traders", Parent: "Dealers", Balance: 500, DrCr: "Dr"}}, Total: 500, Count: 1}
	if b := []byte(tableHTML(groupDoc(g))); !bytes.Contains(b, []byte("LEDGER GROUP DETAILS")) || !bytes.Contains(b, []byte("ABC Traders")) {
		t.Fatal("group PDF incomplete")
	}
}
