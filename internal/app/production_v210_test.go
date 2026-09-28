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
	x := VoucherDetail{TotalAmount:1180}
	s := &Server{}
	s.verifyInvoiceAmount(&x, VoucherRow{Amount:1180})
	if !x.AmountVerified || x.RegisterAmount!=1180 || x.AmountDelta!=0 {
		t.Fatalf("expected verified: %+v",x)
	}
	y := VoucherDetail{TotalAmount:2360}
	s.verifyInvoiceAmount(&y, VoucherRow{Amount:1180})
	if y.AmountVerified || y.AmountDelta!=1180 || !strings.Contains(y.VerificationNote,"mismatch") {
		t.Fatalf("expected mismatch: %+v",y)
	}
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
