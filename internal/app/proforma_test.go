package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func exampleProforma() Proforma {
	return Proforma{Company: "Vijayalaxmi Textiles", Number: "PI-TEST-1", Date: "2026-09-27", SupplierState: "27", Buyer: "ABC Trading", City: "Solapur", PlaceState: "27", Dispatch: "VRL Logistics", Items: []ProformaItem{{Name: "Cotton towel", Description: "Blue border\nPacked in sets of six", HSN: "6302", Unit: "pcs", Quantity: "10", Rate: "100", Discount: "10", GST: "5"}}}
}
func TestProformaDiscountAndStateTaxes(t *testing.T) {
	p, e := calculateProforma(exampleProforma())
	if e != nil {
		t.Fatal(e)
	}
	if p.Gross != 1000 || p.Discount != 100 || p.Taxable != 900 || p.CGST != 22.5 || p.SGST != 22.5 || p.IGST != 0 || p.Total != 945 {
		t.Fatal(p)
	}
	p.PlaceState = "29"
	p, e = calculateProforma(p)
	if e != nil || p.IGST != 45 || p.CGST != 0 || p.SGST != 0 || p.Total != 945 {
		t.Fatal(p, e)
	}
	if !strings.Contains(p.ShareText, "PROFORMA") || !strings.Contains(p.ShareText, "Blue border") || !strings.Contains(p.ShareText, "945.00") {
		t.Fatal(p.ShareText)
	}
}
func TestProformaValidationAndRounding(t *testing.T) {
	for _, mutate := range []func(*Proforma){func(p *Proforma) { p.Items[0].GST = "" }, func(p *Proforma) { p.Items[0].Discount = "101" }, func(p *Proforma) { p.Items[0].Quantity = "0" }, func(p *Proforma) { p.Items[0].Rate = "NaN" }, func(p *Proforma) { p.BuyerGST = "not-gst" }, func(p *Proforma) { p.PlaceState = "" }, func(p *Proforma) { p.Date = "2026-13-12" }} {
		p := exampleProforma()
		mutate(&p)
		if _, e := calculateProforma(p); e == nil {
			t.Fatal("accepted invalid draft", p)
		}
	}
	p := exampleProforma()
	p.Items[0].Quantity = "3"
	p.Items[0].Rate = "0.335"
	p.Items[0].Discount = "0"
	p.Items[0].GST = "5"
	p, e := calculateProforma(p)
	if e != nil || p.Gross != 1.01 || p.CGST != 0.03 || p.SGST != 0.03 || p.Total != 1.07 {
		t.Fatal(p, e)
	}
}
func TestProformaUsesCompanyBankAndNeverWritesVoucher(t *testing.T) {
	s, ts := testServerWithTally(t, func(w http.ResponseWriter, r *http.Request) { t.Error("preview must not call Tally") })
	defer ts.Close()
	_ = s.cache.Put("stock-items", "Vijayalaxmi Textiles", "", StockItemsData{Items: []StockItemMaster{{Name: "Cotton towel"}}}, time.Now())
	_ = s.store.SavePDFBank("Vijayalaxmi Textiles", BankDetails{Ledger: "SBI", AccountNumber: "001234", IFSC: "SBIN0000123"})
	_ = s.store.SavePDFBank("Other Company", BankDetails{Ledger: "HDFC", AccountNumber: "9999"})
	p := exampleProforma()
	p.Bank = BankDetails{AccountNumber: "ATTACK"}
	raw, _ := json.Marshal(p)
	w := httptest.NewRecorder()
	s.handleProforma(w, httptest.NewRequest("POST", "/api/v1/mobile/proforma-preview", strings.NewReader(string(raw))))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "001234") || strings.Contains(w.Body.String(), "ATTACK") || strings.Contains(w.Body.String(), "9999") {
		t.Fatal(w.Code, w.Body.String())
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("proforma must not be cached")
	}
	if len(s.cache.Matching("proforma-preview", p.Company)) > 0 {
		t.Fatal("draft was cached")
	}
}
func TestStockMasterRateDatesAndDescription(t *testing.T) {
	raw := []byte(`<ENVELOPE><STOCKITEM NAME="Cotton towel"><BASEUNITS>pcs</BASEUNITS><DESCRIPTION>Soft cotton</DESCRIPTION><GSTDETAILS.LIST><APPLICABLEFROM>20250101</APPLICABLEFROM><HSNCODE>6302</HSNCODE><RATEDETAILS.LIST><GSTRATEDUTYHEAD>Integrated Tax</GSTRATEDUTYHEAD><GSTRATE>12</GSTRATE></RATEDETAILS.LIST></GSTDETAILS.LIST><GSTDETAILS.LIST><APPLICABLEFROM>20260901</APPLICABLEFROM><RATEDETAILS.LIST><GSTRATEDUTYHEAD>Integrated Tax</GSTRATEDUTYHEAD><GSTRATE>5</GSTRATE></RATEDETAILS.LIST></GSTDETAILS.LIST><GSTDETAILS.LIST><APPLICABLEFROM>20270101</APPLICABLEFROM><RATEDETAILS.LIST><GSTRATEDUTYHEAD>Integrated Tax</GSTRATEDUTYHEAD><GSTRATE>18</GSTRATE></RATEDETAILS.LIST></GSTDETAILS.LIST></STOCKITEM></ENVELOPE>`)
	rows := parseStockItems(raw, "2026-09-27")
	if len(rows) != 1 || rows[0].GST != "5" || rows[0].Description != "Soft cotton" || rows[0].Name != "Cotton towel" {
		t.Fatal(rows)
	}
	if !strings.Contains(string(stockItemsXML("A&B")), "A&amp;B") || strings.Contains(string(stockItemsXML("A")), "Import") {
		t.Fatal("master query must be read-only")
	}
}
func TestInvoiceAdditionalDescriptionsAreEscapedAndShared(t *testing.T) {
	raw := []byte(`<VOUCHER><PARTYLEDGERNAME>Buyer</PARTYLEDGERNAME><ALLINVENTORYENTRIES.LIST><STOCKITEMNAME>Towel</STOCKITEMNAME><AMOUNT>100</AMOUNT><BASICUSERDESCRIPTION.LIST><BASICUSERDESCRIPTION>Blue &amp; white</BASICUSERDESCRIPTION><BASICUSERDESCRIPTION>&lt;script&gt;test&lt;/script&gt;</BASICUSERDESCRIPTION></BASICUSERDESCRIPTION.LIST></ALLINVENTORYENTRIES.LIST><LEDGERENTRIES.LIST><LEDGERNAME>Buyer</LEDGERNAME><AMOUNT>-100</AMOUNT></LEDGERENTRIES.LIST></VOUCHER>`)
	x := parseVoucherDetail(raw, "Firm", "1", VoucherRow{})
	if !strings.Contains(x.ShareText, "Blue & white") || strings.Contains(invoiceHTML(x), "<script>test</script>") || !strings.Contains(invoiceHTML(x), "&lt;script&gt;") {
		t.Fatal(x)
	}
}
func TestWriteProformaFixture(t *testing.T) {
	dir := os.Getenv("TB_PROFORMA_FIXTURE")
	if dir == "" {
		t.Skip("set fixture output directory")
	}
	p := exampleProforma()
	p.SupplierAddress = "Textile Market, Solapur, Maharashtra"
	p.Address = "125 Market Road, Solapur"
	p.Bank = BankDetails{Ledger: "SBI", BankName: "State Bank", AccountHolder: p.Company, AccountNumber: "001234567890", IFSC: "SBIN0000123"}
	p.Items = append(p.Items, ProformaItem{Name: "Microfiber kitchen napkin with a long stock item name", Description: "Soft touch • Assorted colours\nधुण्यास सोपे - Pack of six", Quantity: "12", Rate: "55.50", Discount: "5", GST: "18", HSN: "6302", Unit: "pcs"})
	x, e := calculateProforma(p)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.MkdirAll(dir, 0700); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(dir, "proforma.html"), []byte(proformaHTML(x)), 0600); e != nil {
		t.Fatal(e)
	}
	if runtime.GOOS == "windows" {
		data, e := renderPDFMode(proformaHTML(x), false)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(dir, "proforma.pdf"), data, 0600); e != nil {
			t.Fatal(e)
		}
	}
}

func TestProformaNativeEndpointsRequireAuthentication(t *testing.T) {
	s, _ := nativeFixture(t)
	for _, endpoint := range []string{"proforma-preview", "proforma-pdf"} {
		r := httptest.NewRequest("POST", "/api/v1/mobile/"+endpoint, strings.NewReader(`{}`))
		w := serve(s.NativeHandler(), r)
		if w.Code != 401 {
			t.Fatalf("unauthenticated %s got %d", endpoint, w.Code)
		}
		w = serve(s.NativeHandler(), httptest.NewRequest("GET", "/api/v1/mobile/"+endpoint, nil))
		if w.Code != 405 {
			t.Fatalf("GET accepted for %s", endpoint)
		}
	}
}
