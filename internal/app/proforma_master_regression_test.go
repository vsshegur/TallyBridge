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
	for _, want := range []string{"<FETCH>City</FETCH>", "<FETCH>PartyGSTIN</FETCH>", "<FETCH>LedMailingDetails.*</FETCH>", "<FETCH>LedMultiAddressList.*</FETCH>", "<FETCH>LedGSTRegDetails.*</FETCH>"} {
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
	for _, want := range []string{"HSNDetails.*", "HSNCode", "TBHSN"} {
		if !strings.Contains(stock, want) { t.Fatalf("stock query missing %s", want) }
	}
	gst := string(companyGSTXML("Demo Company"))
	if !strings.Contains(gst, "$GSTRegNumber:TaxUnit:($ExciseUnitName:Company:##SVCurrentCompany)") || !strings.Contains(gst, "<TALLYREQUEST>Export</TALLYREQUEST>") { t.Fatal(gst) }
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


func TestBuyerFallbackUsesValidXMLAndModernAggregates(t *testing.T) {
	obj := string(ledgerDetailXML("Demo Company", "ABC Buyer"))
	for _, want := range []string{"<SVEXPORTFORMAT>$$SysName:XML</SVEXPORTFORMAT>","<FETCH>*</FETCH>","<FETCH>LedMailingDetails.*</FETCH>","<FETCH>LedGSTRegDetails.*</FETCH>"} {
		if !strings.Contains(obj, want) { t.Fatalf("ledger object missing %s", want) }
	}
	fallback := string(ledgerDetailCollectionXML("Demo Company", "ABC Buyer"))
	if strings.Contains(fallback, "<SVEXPORTFORMAT>$SysName:XML</SVEXPORTFORMAT>") || !strings.Contains(fallback, "<SVEXPORTFORMAT>$$SysName:XML</SVEXPORTFORMAT>") { t.Fatal(fallback) }
	m := parseLedgerDetails([]byte(`<LEDGER><LEDGSTREGDETAILS.LIST><GSTIN>27ABCDE1234F1Z5</GSTIN><STATE>Maharashtra</STATE></LEDGSTREGDETAILS.LIST><LEDMAILINGDETAILS.LIST><ADDRESS>Market Yard</ADDRESS><ADDRESS>Solapur</ADDRESS><PINCODE>413001</PINCODE></LEDMAILINGDETAILS.LIST></LEDGER>`))
	if m["PARTYGSTIN"]!="27ABCDE1234F1Z5" || m["CITY"]!="Solapur" || m["PINCODE"]!="413001" || !strings.Contains(m["ADDRESS"],"Market Yard") { t.Fatalf("%#v",m) }
}

func TestVoucherDayLookupAvoidsTallyNumberFilter(t *testing.T) {
	x := string(voucherDayXML("Demo Company", "2026-09-27"))
	if strings.Contains(x, "TBVoucherNumberFilter") || strings.Contains(x, "<CHILDOF>") || !strings.Contains(x, "<TYPE>Voucher</TYPE>") || !strings.Contains(x, "20260927") { t.Fatal(x) }
	raw := []byte(`<ENVELOPE><VOUCHER><MASTERID>1</MASTERID><DATE>20260927</DATE><VOUCHERNUMBER>OTHER</VOUCHERNUMBER><VOUCHERTYPENAME>Sales</VOUCHERTYPENAME><PARTYLEDGERNAME>X</PARTYLEDGERNAME><ALLLEDGERENTRIES.LIST><LEDGERNAME>X</LEDGERNAME><ISPARTYLEDGER>Yes</ISPARTYLEDGER><AMOUNT>-50</AMOUNT></ALLLEDGERENTRIES.LIST></VOUCHER><VOUCHER><MASTERID>43795</MASTERID><DATE>20260927</DATE><VOUCHERNUMBER>INV-22</VOUCHERNUMBER><VOUCHERTYPENAME>GST Sales</VOUCHERTYPENAME><PARTYLEDGERNAME>ABC Buyer</PARTYLEDGERNAME><ALLINVENTORYENTRIES.LIST><STOCKITEMNAME>Towel</STOCKITEMNAME><BILLEDQTY>2 pcs</BILLEDQTY><RATE>100/pcs</RATE><AMOUNT>-200</AMOUNT></ALLINVENTORYENTRIES.LIST><ALLLEDGERENTRIES.LIST><LEDGERNAME>ABC Buyer</LEDGERNAME><ISPARTYLEDGER>Yes</ISPARTYLEDGER><AMOUNT>-200</AMOUNT></ALLLEDGERENTRIES.LIST></VOUCHER></ENVELOPE>`)
	detail, ok := parseVoucherDayDetail(raw, "Demo Company", "43795", VoucherRow{Number:"INV-22",Date:"2026-09-27",Party:"ABC Buyer",VoucherType:"GST Sales",Amount:200})
	if !ok || detail.InvoiceNumber!="INV-22" || detail.CustomerName!="ABC Buyer" || len(detail.Items)!=1 || detail.TotalAmount!=200 { t.Fatalf("ok=%v detail=%+v",ok,detail) }
}


func TestCompanyProfileParsesContactAndBuyerSelection(t *testing.T) {
	d := supplierDetailsFromXML([]byte(`<COMPANY><MAILINGNAME>Demo Textiles</MAILINGNAME><ADDRESS>Market Road</ADDRESS><ADDRESS>Solapur</ADDRESS><PINCODE>413001</PINCODE><MOBILENUMBER>9876543210</MOBILENUMBER><EMAIL>accounts@example.com</EMAIL><GSTIN>27ABCDE1234F1Z5</GSTIN></COMPANY>`))
	p := profileFromDetails("Fallback", d)
	if p.Name!="Demo Textiles" || p.GSTIN!="27ABCDE1234F1Z5" || p.Phone!="9876543210" || p.Email!="accounts@example.com" || !strings.Contains(companyProfileAddress(p),"413001") { t.Fatalf("%+v",p) }
	multi := []byte(`<ENVELOPE><LEDGER NAME="Wrong"><ADDRESS>Wrong City</ADDRESS></LEDGER><LEDGER NAME="ABC Buyer"><LEDGSTREGDETAILS.LIST><GSTIN>27AAAAA0000A1Z5</GSTIN></LEDGSTREGDETAILS.LIST><LEDMAILINGDETAILS.LIST><ADDRESS>Buyer Road</ADDRESS><ADDRESS>Solapur</ADDRESS><PINCODE>413001</PINCODE></LEDMAILINGDETAILS.LIST></LEDGER></ENVELOPE>`)
	m := parseLedgerDetailsFor(multi,"ABC Buyer")
	if m["PARTYGSTIN"]!="27AAAAA0000A1Z5" || !strings.Contains(m["ADDRESS"],"Buyer Road") || m["CITY"]!="Solapur" { t.Fatalf("%#v",m) }
}
