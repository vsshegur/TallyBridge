package app

import (
	"strings"
	"testing"
)

func TestProformaSupplierUsesTaxUnitAndBuyerDetails(t *testing.T) {
	tax := string(taxUnitsXML("Vijayalaxmi Textiles"))
	if !strings.Contains(tax, "<TYPE>Tax Unit</TYPE>") || !strings.Contains(tax, "GSTRegNumber") || strings.Contains(strings.ToUpper(tax), "<TALLYREQUEST>IMPORT") {
		t.Fatal(tax)
	}
	ledger := string(ledgerDetailXML("Vijayalaxmi Textiles", "ABC Buyer"))
	for _, want := range []string{"<FETCH>City</FETCH>", "<FETCH>PartyGSTIN</FETCH>", "<FETCH>LedMultiAddressList.*</FETCH>", "<FETCH>LedGSTRegDetails.*</FETCH>"} {
		if !strings.Contains(ledger, want) {
			t.Fatalf("buyer detail query missing %s", want)
		}
	}
	raw := []byte(`<LEDGER><NAME>ABC Buyer</NAME><LEDGSTREGDETAILS.LIST><PARTYGSTIN>27ABCDE1234F1Z5</PARTYGSTIN><STATENAME>Maharashtra</STATENAME><CITY>Solapur</CITY><PINCODE>413001</PINCODE><ADDRESS>Market Yard Road</ADDRESS></LEDGSTREGDETAILS.LIST></LEDGER>`)
	m := parseLedgerDetails(raw)
	if m["PARTYGSTIN"] != "27ABCDE1234F1Z5" || m["STATENAME"] != "Maharashtra" || m["CITY"] != "Solapur" || m["PINCODE"] != "413001" || !strings.Contains(m["ADDRESS"], "Market Yard Road") {
		t.Fatalf("buyer master fields not parsed: %#v", m)
	}
	supplier := supplierDetailsFromXML([]byte(`<TAXUNIT><GSTREGNUMBER>27ABCDE1234F1Z5</GSTREGNUMBER><STATENAME>Maharashtra</STATENAME><PINCODE>413001</PINCODE><ADDRESS>Textile Market</ADDRESS></TAXUNIT>`))
	if supplier["gstin"] != "27ABCDE1234F1Z5" || supplier["stateCode"] != "27" || supplier["state"] != "Maharashtra" || supplier["pincode"] != "413001" {
		t.Fatalf("supplier tax unit not parsed: %#v", supplier)
	}
}

func TestVoucherDetailEnrichmentAvoidsFragileObjectID(t *testing.T) {
	x := string(voucherDetailObjectXML("Vijayalaxmi Textiles", "43795"))
	if strings.Contains(x, "<TYPE>Object</TYPE>") || strings.Contains(x, "ID:'43795'") || !strings.Contains(x, "<TYPE>Collection</TYPE>") || !strings.Contains(x, "$MasterID = 43795") {
		t.Fatal(x)
	}
}
