package app

import (
	"strings"
	"testing"
)

func TestVoucherDetailUsesResolvedHSNAndBuyerAddress(t *testing.T) {
	raw := []byte(`<ENVELOPE><VOUCHER>
<MASTERID>42</MASTERID><DATE>20261003</DATE><VOUCHERNUMBER>INV-42</VOUCHERNUMBER><VOUCHERTYPENAME>Sales</VOUCHERTYPENAME>
<PARTYLEDGERNAME>ABC Textiles</PARTYLEDGERNAME><BASICBUYERNAME>ABC Textiles</BASICBUYERNAME><BASICBUYERCITY>Solapur</BASICBUYERCITY><PARTYGSTIN>27ABCDE1234F1Z5</PARTYGSTIN>
<BASICBUYERADDRESS.LIST><BASICBUYERADDRESS>12 Market Road</BASICBUYERADDRESS><BASICBUYERADDRESS>Solapur</BASICBUYERADDRESS></BASICBUYERADDRESS.LIST>
<ALLINVENTORYENTRIES.LIST><STOCKITEMNAME>Ashoka Towel</STOCKITEMNAME><BILLEDQTY>10 PCS</BILLEDQTY><RATE>100/PCS</RATE><AMOUNT>-1000</AMOUNT><HSNCODE>9999</HSNCODE><GSTITEMHSNCODEEX>6302</GSTITEMHSNCODEEX></ALLINVENTORYENTRIES.LIST>
<ALLLEDGERENTRIES.LIST><LEDGERNAME>ABC Textiles</LEDGERNAME><AMOUNT>1000</AMOUNT></ALLLEDGERENTRIES.LIST>
</VOUCHER></ENVELOPE>`)
	x := parseVoucherDetail(raw, "Demo", "42", VoucherRow{Number:"INV-42", Date:"2026-10-03", Party:"ABC Textiles", Amount:1000})
	if len(x.Items) != 1 { t.Fatalf("expected one item, got %+v", x.Items) }
	if x.Items[0].HSN != "6302" { t.Fatalf("expected resolved GSTItemHSNCodeEx 6302, got %q", x.Items[0].HSN) }
	if len(x.CustomerAddress) != 2 || x.CustomerAddress[0] != "12 Market Road" || x.CustomerAddress[1] != "Solapur" { t.Fatalf("buyer address not preserved: %#v", x.CustomerAddress) }
	if x.CustomerCity != "" { t.Fatalf("duplicate city should be suppressed: %q", x.CustomerCity) }
	for _, request := range [][]byte{voucherDetailCollectionXML("Demo","42"), voucherDayXML("Demo","2026-10-03")} {
		if !strings.Contains(string(request), "GSTItemHSNCodeEx") { t.Fatal("resolved HSN method missing from voucher fetch") }
	}
}

func TestInvoiceHTMLIsCustomerFacingAndShowsPartyAddress(t *testing.T) {
	x := VoucherDetail{Company:"Demo", FirmName:"Demo", InvoiceNumber:"INV-42", InvoiceDate:"2026-10-03", VoucherType:"Sales", CustomerName:"ABC Textiles", CustomerAddress:[]string{"12 Market Road","Solapur"}, GSTIN:"27ABCDE1234F1Z5", Items:[]InvoiceItem{{Name:"Ashoka Towel", HSN:"6302", Quantity:"10 PCS", Rate:"100/PCS", Amount:1000}}, TotalAmount:1000}
	h := invoiceHTML(x)
	for _, want := range []string{"TAX INVOICE","BILL TO","12 Market Road","Solapur","27ABCDE1234F1Z5","6302"} {
		if !strings.Contains(h,want) { t.Fatalf("invoice missing %q",want) }
	}
	for _, bad := range []string{"Read-only copy of data returned by TallyPrime","TallyBridge  |  READ ONLY"} {
		if strings.Contains(h,bad) { t.Fatalf("customer-facing invoice still contains %q",bad) }
	}
}
