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


func TestModernMasterFallbackQueries(t *testing.T) {
	stock := string(stockItemsXML("Demo Company"))
	for _, want := range []string{"HSNDetails.*", "HSNCode", "HSNMasterName"} {
		if !strings.Contains(stock, want) { t.Fatalf("stock query missing %s", want) }
	}
	gst := string(companyGSTXML("Demo Company"))
	if !strings.Contains(gst, "@@CMPGSTaxNumber") || !strings.Contains(gst, "<TALLYREQUEST>Export</TALLYREQUEST>") { t.Fatal(gst) }
	ledger := string(ledgerDetailCollectionXML("Demo Company", "ABC & Co"))
	if !strings.Contains(ledger, "LedGSTRegDetails.*") || !strings.Contains(ledger, "LedMailingDetails.*") || !strings.Contains(ledger, "ABC &amp; Co") { t.Fatal(ledger) }
	hint := VoucherRow{Number:"INV-22", Date:"2026-09-27", VoucherType:"Sales"}
	v := string(voucherDetailHintXML("Demo Company", hint))
	if !strings.Contains(v, "20260927") || !strings.Contains(v, "INV-22") || !strings.Contains(v, "<CHILDOF>Sales</CHILDOF>") { t.Fatal(v) }
}

func TestModernNestedBuyerAndHSNParsing(t *testing.T) {
	buyer := parseLedgerDetails([]byte(`<LEDGER><LEDGSTREGDETAILS.LIST><GSTIN>27ABCDE1234F1Z5</GSTIN><STATE>Maharashtra</STATE></LEDGSTREGDETAILS.LIST><LEDMAILINGDETAILS.LIST><ADDRESS.LIST><ADDRESS>Market Road</ADDRESS><ADDRESS>Solapur</ADDRESS></ADDRESS.LIST><CITY>Solapur</CITY><PINCODE>413001</PINCODE></LEDMAILINGDETAILS.LIST></LEDGER>`))
	if buyer["PARTYGSTIN"] != "27ABCDE1234F1Z5" || buyer["CITY"] != "Solapur" || !strings.Contains(buyer["ADDRESS"], "Market Road") { t.Fatalf("%#v", buyer) }
	root, _ := parseTree([]byte(`<STOCKITEM NAME="Towel"><HSNDETAILS.LIST><APPLICABLEFROM>20260401</APPLICABLEFROM><HSNCODE>63026000</HSNCODE></HSNDETAILS.LIST></STOCKITEM>`))
	var nodes []*xnode; nodesByName(root, "STOCKITEM", &nodes)
	if len(nodes)!=1 || hsnFromNode(nodes[0], "2026-09-27")!="63026000" { t.Fatal("HSN details not parsed") }
}


func TestAuthoritativeTallyMasterMethods(t *testing.T) {
	company := string(companyGSTXML("Demo Company"))
	if !strings.Contains(company, "$GSTRegNumber:TaxUnit:($ExciseUnitName:Company:##SVCurrentCompany)") || !strings.Contains(company, "$$SysName:XML") { t.Fatal(company) }
	stock := string(stockItemsXML("Demo Company"))
	if !strings.Contains(stock, "<METHOD>TBHSN:$GSTItemHSNCodeEx</METHOD>") || !strings.Contains(stock, "<NATIVEMETHOD>Name</NATIVEMETHOD>") { t.Fatal(stock) }
	ledger := string(ledgerDetailCollectionXML("Demo Company", "Buyer"))
	for _, want := range []string{"<METHOD>TBGSTIN:$PartyGSTIN</METHOD>","<METHOD>TBSTATE:$LedStateName</METHOD>","<METHOD>TBCITY:$City</METHOD>"} {
		if !strings.Contains(ledger, want) { t.Fatalf("missing %s", want) }
	}
}
func TestComputedBuyerAndHSNFields(t *testing.T) {
	buyer := parseLedgerDetails([]byte(`<LEDGER><TBGSTIN>27ABCDE1234F1Z5</TBGSTIN><TBSTATE>Maharashtra</TBSTATE><TBCITY>Solapur</TBCITY><TBPINCODE>413001</TBPINCODE><ADDRESS>Market Road</ADDRESS></LEDGER>`))
	if buyer["PARTYGSTIN"]!="27ABCDE1234F1Z5" || buyer["STATENAME"]!="Maharashtra" || buyer["CITY"]!="Solapur" || buyer["PINCODE"]!="413001" { t.Fatal(buyer) }
	items := parseStockItems([]byte(`<ENVELOPE><STOCKITEM NAME="Towel"><BASEUNITS>pcs</BASEUNITS><TBHSN>63026000</TBHSN></STOCKITEM></ENVELOPE>`), "2026-09-27")
	if len(items)!=1 || items[0].HSN!="63026000" { t.Fatal(items) }
}
