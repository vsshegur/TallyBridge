package app

import (
	"strings"
	"testing"
)

func TestPurchaseGstRequestIsReadOnly(t *testing.T) {
	req := itSample(t, "purchase_gst_request.xml")
	if err := validateExportXML([]byte(req)); err != nil {
		t.Fatal(err)
	}
	if !itRowKinds["export_purchase_gst"] {
		t.Fatal("export_purchase_gst is not a row kind")
	}
	// the same posting guard still refuses it as a posting
	if _, err := validatePostingXML([]byte(req), itCompany); err == nil {
		t.Fatal("a read request was accepted for posting")
	}
}

func TestPurchaseGstRowsKeepInwardGstVouchers(t *testing.T) {
	reply := `<ENVELOPE><BODY><DATA><COLLECTION>
<VOUCHER><MASTERID>101</MASTERID><DATE>20260805</DATE><VOUCHERNUMBER>P-17</VOUCHERNUMBER><VOUCHERTYPENAME>GST Purchase</VOUCHERTYPENAME><ITKIND>inv</ITKIND>
 <REFERENCE>INV/245/26-27</REFERENCE><REFERENCEDATE>20260803</REFERENCEDATE><PARTYLEDGERNAME>R&amp;D Traders</PARTYLEDGERNAME><PARTYGSTIN>27aapfr1234k1z5</PARTYGSTIN>
 <ISCANCELLED>No</ISCANCELLED><ISOPTIONAL>No</ISOPTIONAL>
 <ALLLEDGERENTRIES.LIST><LEDGERNAME>R&amp;D Traders</LEDGERNAME><AMOUNT>11800.00</AMOUNT></ALLLEDGERENTRIES.LIST>
 <ALLLEDGERENTRIES.LIST><LEDGERNAME>Input CGST 9%</LEDGERNAME><AMOUNT>-900.00</AMOUNT></ALLLEDGERENTRIES.LIST>
 <ALLLEDGERENTRIES.LIST><LEDGERNAME>Input SGST 9%</LEDGERNAME><AMOUNT>-900.00</AMOUNT></ALLLEDGERENTRIES.LIST>
 <ALLINVENTORYENTRIES.LIST><STOCKITEMNAME>Yarn</STOCKITEMNAME><AMOUNT>-10000.00</AMOUNT>
  <ACCOUNTINGALLOCATIONS.LIST><LEDGERNAME>Purchase GST 18%</LEDGERNAME><AMOUNT>-10000.00</AMOUNT></ACCOUNTINGALLOCATIONS.LIST></ALLINVENTORYENTRIES.LIST>
</VOUCHER>
<VOUCHER><MASTERID>102</MASTERID><DATE>20260810</DATE><VOUCHERNUMBER>DN-3</VOUCHERNUMBER><VOUCHERTYPENAME>Debit Note</VOUCHERTYPENAME>
 <REFERENCE>CN-55</REFERENCE><PARTYLEDGERNAME>R&amp;D Traders</PARTYLEDGERNAME>
 <ALLLEDGERENTRIES.LIST><LEDGERNAME>R&amp;D Traders</LEDGERNAME><AMOUNT>-1180.00</AMOUNT></ALLLEDGERENTRIES.LIST>
 <ALLLEDGERENTRIES.LIST><LEDGERNAME>Purchase Return</LEDGERNAME><AMOUNT>1000.00</AMOUNT></ALLLEDGERENTRIES.LIST>
 <ALLLEDGERENTRIES.LIST><LEDGERNAME>IGST</LEDGERNAME><AMOUNT>180.00</AMOUNT></ALLLEDGERENTRIES.LIST>
</VOUCHER>
<VOUCHER><MASTERID>103</MASTERID><DATE>20260811</DATE><VOUCHERNUMBER>J-9</VOUCHERNUMBER><VOUCHERTYPENAME>Journal</VOUCHERTYPENAME><ITKIND>jv</ITKIND>
 <ALLLEDGERENTRIES.LIST><LEDGERNAME>Salary</LEDGERNAME><AMOUNT>-5000.00</AMOUNT></ALLLEDGERENTRIES.LIST>
 <ALLLEDGERENTRIES.LIST><LEDGERNAME>Ramesh</LEDGERNAME><AMOUNT>5000.00</AMOUNT></ALLLEDGERENTRIES.LIST>
</VOUCHER>
<VOUCHER><MASTERID>104</MASTERID><DATE>20260812</DATE><VOUCHERNUMBER>J-10</VOUCHERNUMBER><VOUCHERTYPENAME>Journal</VOUCHERTYPENAME><ITKIND>jv</ITKIND>
 <PARTYLEDGERNAME>Shree Rent Co</PARTYLEDGERNAME>
 <ALLLEDGERENTRIES.LIST><LEDGERNAME>Rent</LEDGERNAME><AMOUNT>-20000.00</AMOUNT></ALLLEDGERENTRIES.LIST>
 <ALLLEDGERENTRIES.LIST><LEDGERNAME>Input IGST</LEDGERNAME><AMOUNT>-3600.00</AMOUNT></ALLLEDGERENTRIES.LIST>
 <ALLLEDGERENTRIES.LIST><LEDGERNAME>Shree Rent Co</LEDGERNAME><AMOUNT>23600.00</AMOUNT></ALLLEDGERENTRIES.LIST>
</VOUCHER>
<VOUCHER><MASTERID>105</MASTERID><DATE>20260813</DATE><VOUCHERNUMBER>P-18</VOUCHERNUMBER><VOUCHERTYPENAME>Purchase</VOUCHERTYPENAME><ISOPTIONAL>Yes</ISOPTIONAL>
 <ALLLEDGERENTRIES.LIST><LEDGERNAME>X</LEDGERNAME><AMOUNT>100.00</AMOUNT></ALLLEDGERENTRIES.LIST></VOUCHER>
<VOUCHER><MASTERID>106</MASTERID><DATE>20260814</DATE><VOUCHERNUMBER>P-19</VOUCHERNUMBER><VOUCHERTYPENAME>Purchase</VOUCHERTYPENAME><ISCANCELLED>Yes</ISCANCELLED>
 <ALLLEDGERENTRIES.LIST><LEDGERNAME>X</LEDGERNAME><AMOUNT>100.00</AMOUNT></ALLLEDGERENTRIES.LIST></VOUCHER>
</COLLECTION></DATA></BODY></ENVELOPE>`
	rows := itTallyRows("export_purchase_gst", []byte(itSample(t, "purchase_gst_request.xml")), []byte(reply))
	if len(rows) != 3 {
		t.Fatalf("want purchase, debit note and GST journal, got %d: %v", len(rows), rows)
	}
	p := rows[0]
	if p["mid"] != "101" || p["date"] != "2026-08-05" || p["number"] != "P-17" || p["ref"] != "INV/245/26-27" || p["refdate"] != "2026-08-03" ||
		p["party"] != "R&D Traders" || p["gstin"] != "27AAPFR1234K1Z5" || p["kind"] != "inv" || p["inv"] != -10000.0 {
		t.Fatalf("purchase row: %v", p)
	}
	e := p["e"].([][2]any)
	if len(e) != 3 || e[0][0] != "R&D Traders" || e[0][1] != 11800.0 || e[1][1] != -900.0 {
		t.Fatalf("ledger lines: %v", e)
	}
	// no ITKIND (older Tally): the voucher type name decides
	if rows[1]["kind"] != "dn" || rows[1]["inv"] != 0.0 {
		t.Fatalf("debit note row: %v", rows[1])
	}
	if rows[2]["kind"] != "jv" || rows[2]["party"] != "Shree Rent Co" {
		t.Fatalf("journal row: %v", rows[2])
	}
	if strings.Contains(stringify(rows), "Salary") {
		t.Fatal("journal without GST was kept")
	}
}

func stringify(rows []map[string]any) string {
	var b strings.Builder
	for _, r := range rows {
		for _, e := range r["e"].([][2]any) {
			b.WriteString(e[0].(string))
			b.WriteByte(' ')
		}
	}
	return b.String()
}
