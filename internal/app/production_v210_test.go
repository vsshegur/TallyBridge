package app

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestInvoiceAmountVerificationGuard(t *testing.T) {
	s := &Server{}
	x := VoucherDetail{TotalAmount:1180}
	s.verifyInvoiceAmount(&x, VoucherRow{Amount:1180})
	if !x.AmountVerified || x.RegisterAmount!=1180 || x.AmountDelta!=0 { t.Fatalf("expected verified: %+v",x) }
	within := VoucherDetail{TotalAmount:1180.01}
	s.verifyInvoiceAmount(&within, VoucherRow{Amount:1180})
	if !within.AmountVerified || within.AmountDelta < 0.009 || within.AmountDelta > 0.011 { t.Fatalf("₹0.01 must verify: %+v",within) }
	outside := VoucherDetail{TotalAmount:1180.02}
	s.verifyInvoiceAmount(&outside, VoucherRow{Amount:1180})
	if outside.AmountVerified || outside.AmountDelta < 0.019 || !strings.Contains(strings.ToLower(outside.VerificationNote),"mismatch") { t.Fatalf("₹0.02 must fail: %+v",outside) }
}

func TestCompanyHealthUsesSavedProfileAndPaymentSettings(t *testing.T) {
	s, ts := testServerWithTally(t, func(w http.ResponseWriter,r *http.Request){ io.WriteString(w,`<ENVELOPE/>`) })
	defer ts.Close()
	_ = s.cache.Put("company-profile-v1","Demo","",CompanyProfile{
		Name:"Demo",GSTIN:"27ABCDE1234F1Z5",Address:"Market Road",City:"Solapur",
		Pincode:"413001",Phone:"9876543210",Email:"accounts@example.com",
	},time.Now())
	_ = s.store.SavePDFBank("Demo",BankDetails{Ledger:"SBI",AccountNumber:"123456",IFSC:"SBIN0000123",UPI:"demo@upi"})
	h := s.companyHealth("Demo",false)
	if !h.ReadyForPDF || !h.ReadyForPayments || h.Score < 90 || h.Profile.GSTIN=="" || h.Bank.UPI!="demo@upi" {
		t.Fatalf("bad health %+v",h)
	}
}

func TestPaymentQRAndTemplatesAreLocalAndDeterministic(t *testing.T) {
	bank:=BankDetails{AccountHolder:"Demo",UPI:"demo@upi"}
	uri:=upiPaymentURI(bank,PaymentQR{Amount:1250,Reference:"INV-10",Payee:"Demo"})
	if !strings.Contains(uri,"pa=demo%40upi") || !strings.Contains(uri,"am=1250.00") || !strings.Contains(uri,"tn=INV-10") {
		t.Fatal(uri)
	}
	base:="<html><style>body{}</style><body>x</body></html>"
	if applyPDFTemplate(base,"standard")!=base { t.Fatal("standard template changed base") }
	for _,name:=range []string{"compact","detailed"} {
		out:=applyPDFTemplate(base,name)
		if out==base || !strings.Contains(out,"!important") { t.Fatalf("template %s not applied",name) }
	}
}

func TestProformaPDFUsesInvoiceHierarchyWithoutDuplicateIdentity(t *testing.T) {
	p:=exampleProforma()
	p.SupplierGST="27ABCDE1234F1Z5"
	p.SupplierAddress="Market Road, Solapur"
	p.SupplierPhone="9876543210"
	p.Bank=BankDetails{Ledger:"SBI",UPI:"demo@upi"}
	x,err:=calculateProforma(p)
	if err!=nil { t.Fatal(err) }
	html:=proformaHTML(x)
	if strings.Count(html,p.Company)!=1 {
		t.Fatalf("supplier company should appear exactly once, got %d",strings.Count(html,p.Company))
	}
	if strings.Count(html,p.Number)!=1 {
		t.Fatalf("proforma number should appear exactly once, got %d",strings.Count(html,p.Number))
	}
	if strings.Count(html,displayDate(p.Date))!=1 {
		t.Fatalf("proforma date should appear exactly once, got %d",strings.Count(html,displayDate(p.Date)))
	}
	for _,want:=range []string{"Bill to","Proforma details","Product","Taxable amount","Total amount","Payment bank details"} {
		if !strings.Contains(html,want) { t.Fatalf("missing %s",want) }
	}
}

func TestBackupContainsOnlyWhitelistedSettingsAndRestoreRequiresRestart(t *testing.T) {
	s,ts:=testServerWithTally(t, func(w http.ResponseWriter,r *http.Request){ io.WriteString(w,`<ENVELOPE/>`) })
	defer ts.Close()
	_ = s.store.UpdateSettings(Settings{TallyURL:ts.URL,TimeoutSec:2,CooldownSec:5,Port:8765,PDFTemplate:"compact",AutoUpdate:true})
	_ = s.store.SavePDFBank("Demo",BankDetails{Ledger:"SBI",UPI:"demo@upi"})
	req:=httptest.NewRequest(http.MethodGet,"http://localhost/api/v1/admin/backup",nil)
	rec:=httptest.NewRecorder()
	s.handleBackup(rec,req)
	if rec.Code!=200 { t.Fatal(rec.Code,rec.Body.String()) }
	zr,err:=zip.NewReader(bytes.NewReader(rec.Body.Bytes()),int64(rec.Body.Len()))
	if err!=nil { t.Fatal(err) }
	for _,f:=range zr.File {
		if f.Name=="manifest.json" { continue }
		if !backupNames[f.Name] { t.Fatalf("unexpected backup entry %s",f.Name) }
	}
	var restore bytes.Buffer
	zw:=zip.NewWriter(&restore)
	mf,_:=zw.Create("settings.json")
	_,_=mf.Write([]byte(`{"tallyUrl":"http://127.0.0.1:9000","timeoutSec":6,"cooldownSec":12,"port":8765,"pdfTemplate":"detailed","autoUpdate":true}`))
	_ = zw.Close()
	post:=httptest.NewRequest(http.MethodPost,"http://localhost/api/v1/admin/backup",&restore)
	out:=httptest.NewRecorder()
	s.handleBackup(out,post)
	if out.Code!=200 { t.Fatal(out.Code,out.Body.String()) }
	var response map[string]any
	if json.Unmarshal(out.Body.Bytes(),&response)!=nil || response["restartRequired"]!=true {
		t.Fatal(out.Body.String())
	}
}


func TestInvoiceShareTextHasFullNonZeroCalculationBreakdown(t *testing.T) {
	x:=VoucherDetail{FirmName:"Demo",CustomerName:"Buyer",InvoiceNumber:"INV-1",InvoiceDate:"2026-09-28",TaxableAmount:1000,DiscountAmount:50,Freight:25,CGST:90,SGST:90,RoundOff:-0.25,TotalAmount:1154.75}
	s:=dispatchShareText(x)
	for _,want:=range []string{"Taxable Amount - ₹1,000.00","Discount Amount - ₹50.00","Freight Amount - ₹25.00","CGST Amount - ₹90.00","SGST Amount - ₹90.00","Round Off - ₹-0.25","Total Amount - ₹1,154.75"} {
		if !strings.Contains(s,want){t.Fatalf("missing %q in %s",want,s)}
	}
	if strings.Contains(s,"IGST Amount") || strings.Contains(s,"Other Charges") { t.Fatal(s) }
}

func TestDiagnosticsIncludesBuyerAndInvoiceContextWithoutRawXML(t *testing.T) {
	s,ts:=testServerWithTally(t,func(w http.ResponseWriter,r *http.Request){io.WriteString(w,`<ENVELOPE/>`)})
	defer ts.Close()
	_ = s.cache.Put("ledger-details","Demo","Buyer",map[string]any{"details":map[string]string{"PARTYGSTIN":"27AAAAA0000A1Z5","ADDRESS":"Buyer Road","CITY":"Solapur"}},time.Now())
	x:=VoucherDetail{Company:"Demo",MasterID:"10",InvoiceNumber:"INV-10",InvoiceDate:"2026-09-28",VoucherType:"Sales",CustomerName:"Buyer",TotalAmount:1180,RegisterAmount:1180,AmountVerified:true,TaxableAmount:1000,CGST:90,SGST:90}
	_ = s.cache.Put("voucher-detail-v6","Demo","id:10",x,time.Now())
	req:=httptest.NewRequest(http.MethodGet,"/api/v1/mobile/diagnostics?company=Demo&ledger=Buyer&number=INV-10&date=2026-09-28&masterId=10&voucherType=Sales",nil)
	d:=s.diagnostics("Demo",req)
	if d.Buyer["PARTYGSTIN"]=="" || d.Invoice["masterId"]!="10" || d.Invoice["verified"]!=true { t.Fatalf("missing context buyer=%v invoice=%v",d.Buyer,d.Invoice) }
	raw,_:=json.Marshal(d)
	if strings.Contains(string(raw),"<ENVELOPE") { t.Fatal("raw XML leaked into diagnostics") }
}
