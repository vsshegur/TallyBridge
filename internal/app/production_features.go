package app

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"
)

type HealthItem struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	OK       bool   `json:"ok"`
	Value    string `json:"value,omitempty"`
	Required bool   `json:"required,omitempty"`
	Hint     string `json:"hint,omitempty"`
}
type CompanyHealth struct {
	Company          string          `json:"company"`
	Ready            bool            `json:"ready"`
	ReadyForPDF      bool            `json:"readyForPdf"`
	ReadyForPayments bool            `json:"readyForPayments"`
	Score            int             `json:"score"`
	Profile          CompanyProfile  `json:"profile"`
	Bank             BankDetails     `json:"bank"`
	Items            []HealthItem    `json:"items"`
	CacheLatestAt    time.Time       `json:"cacheLatestAt,omitempty"`
	TallyOnline      bool            `json:"tallyOnline"`
	LastSuccess      time.Time       `json:"lastSuccess,omitempty"`
	Issues           []string        `json:"issues,omitempty"`
}
type DiagnosticContext struct {
	Ledger      string
	Number      string
	Date        string
	MasterID    string
	VoucherType string
}
func (s *Server) setDiagnosticLedgerContext(company, ledger string) {
	s.diagMu.Lock()
	defer s.diagMu.Unlock()
	if s.diagByCompany==nil { s.diagByCompany=map[string]DiagnosticContext{} }
	k:=strings.ToLower(strings.TrimSpace(company))
	x:=s.diagByCompany[k]
	x.Ledger=strings.TrimSpace(ledger)
	s.diagByCompany[k]=x
}
func (s *Server) setDiagnosticInvoiceContext(company string, v VoucherDetail) {
	s.diagMu.Lock()
	defer s.diagMu.Unlock()
	if s.diagByCompany==nil { s.diagByCompany=map[string]DiagnosticContext{} }
	k:=strings.ToLower(strings.TrimSpace(company))
	x:=s.diagByCompany[k]
	x.Ledger=firstNonBlank(strings.TrimSpace(v.CustomerName),x.Ledger)
	x.Number=strings.TrimSpace(v.InvoiceNumber)
	x.Date=strings.TrimSpace(v.InvoiceDate)
	x.MasterID=strings.TrimSpace(v.MasterID)
	x.VoucherType=strings.TrimSpace(v.VoucherType)
	s.diagByCompany[k]=x
}
func (s *Server) diagnosticContext(company string) DiagnosticContext {
	s.diagMu.Lock()
	defer s.diagMu.Unlock()
	return s.diagByCompany[strings.ToLower(strings.TrimSpace(company))]
}

type DiagnosticReport struct {
	Version      string         `json:"version"`
	GeneratedAt  time.Time      `json:"generatedAt"`
	Company      string         `json:"company,omitempty"`
	TallyOnline  bool           `json:"tallyOnline"`
	GateState    string         `json:"gateState,omitempty"`
	BusyReport   string         `json:"busyReport,omitempty"`
	LastSuccess  time.Time      `json:"lastSuccess,omitempty"`
	LastError    string         `json:"lastError,omitempty"`
	Cache        CacheStats     `json:"cache"`
	AutoSync     AutoSyncState  `json:"autoSync"`
	Health       CompanyHealth  `json:"health"`
	RecentReads  []RequestLog   `json:"recentReads,omitempty"`
	Buyer        map[string]string `json:"buyer,omitempty"`
	Invoice      map[string]any `json:"invoice,omitempty"`
}
type UpdateInfo struct {
	CheckedAt  time.Time `json:"checkedAt"`
	Current    string    `json:"current"`
	Latest     string    `json:"latest,omitempty"`
	Available  bool      `json:"available"`
	URL        string    `json:"url,omitempty"`
	Notes      string    `json:"notes,omitempty"`
	Error      string    `json:"error,omitempty"`
}

func companyCacheLatest(c *CacheStore, company string) time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var latest time.Time
	want := strings.ToLower(strings.TrimSpace(company))
	for _, d := range c.entries {
		if strings.ToLower(strings.TrimSpace(d.Company)) == want && d.FetchedAt.After(latest) {
			latest = d.FetchedAt
		}
	}
	return latest
}
func (s *Server) companyHealth(company string, refresh bool) CompanyHealth {
	company = strings.TrimSpace(company)
	profile := s.savedCompanyProfile(company)
	if refresh && company != "" && s.stateSnapshot().TallyOnline {
		if p, err := s.companyProfile(company, true); err == nil {
			profile = p
		}
	}
	bank, _ := s.store.PDFBank(company)
	st := s.stateSnapshot()
	add := func(items *[]HealthItem, key, label, value, hint string, required bool) {
		ok := strings.TrimSpace(value) != ""
		*items = append(*items, HealthItem{Key:key, Label:label, OK:ok, Value:value, Required:required, Hint:hint})
	}
	items := []HealthItem{}
	add(&items,"gstin","GSTIN",profile.GSTIN,"Add GSTIN in the Tally company GST registration.",true)
	add(&items,"address","Address",companyProfileAddress(profile),"Add the company mailing address in Tally.",true)
	add(&items,"phone","Mobile number",profile.Phone,"Add the company mobile/phone number in Tally.",true)
	add(&items,"email","Email",profile.Email,"Optional, but useful on PDFs.",false)
	add(&items,"bank","Bank account",bank.AccountNumber,"Save/verify the payment bank in Windows → Payments.",true)
	add(&items,"ifsc","IFSC",bank.IFSC,"Enter the bank IFSC in Windows → Payments.",bank.AccountNumber!="")
	add(&items,"upi","UPI ID",bank.UPI,"Add a UPI ID to enable payment QR codes.",false)
	qrValue := ""
	if strings.TrimSpace(bank.UPI)!="" { qrValue = "UPI payment QR enabled" }
	add(&items,"qr","QR ready",qrValue,"Add a UPI ID to enable customer payment QR codes.",false)
	okCount, reqCount, reqOK := 0,0,true
	issues := []string{}
	for _, it := range items {
		if it.OK { okCount++ }
		if it.Required {
			reqCount++
			if !it.OK { reqOK=false; issues=append(issues,it.Label+" is missing") }
		}
	}
	if !st.TallyOnline { issues=append(issues,"Tally is not currently connected") }
	score := 0
	if len(items)>0 { score = int(math.Round(float64(okCount)*100/float64(len(items)))) }
	pdfReady := profile.GSTIN!="" && companyProfileAddress(profile)!="" && profile.Phone!=""
	payReady := bank.UPI!="" || (bank.AccountNumber!="" && bank.IFSC!="")
	return CompanyHealth{
		Company:company, Ready:reqOK && st.TallyOnline, ReadyForPDF:pdfReady,
		ReadyForPayments:payReady, Score:score, Profile:profile, Bank:bank,
		Items:items, CacheLatestAt:companyCacheLatest(s.cache,company),
		TallyOnline:st.TallyOnline, LastSuccess:st.LastSuccess, Issues:issues,
	}
}
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	c := strings.TrimSpace(r.URL.Query().Get("company"))
	if c == "" { jsonErr(w,400,errors.New("company required")); return }
	jsonOut(w, s.companyHealth(c, r.URL.Query().Get("refresh")=="1"))
}
func (s *Server) diagnostics(company string, r *http.Request) DiagnosticReport {
	st := s.stateSnapshot()
	logs := s.tally.Logs()
	if len(logs) > 12 { logs = logs[len(logs)-12:] }
	d := DiagnosticReport{
		Version:Version, GeneratedAt:time.Now(), Company:company,
		TallyOnline:st.TallyOnline, GateState:st.GateState, BusyReport:st.BusyReport,
		LastSuccess:st.LastSuccess, LastError:st.LastError, Cache:s.cache.Stats(),
		AutoSync:s.autoSyncSnapshot(), Health:s.companyHealth(company,false), RecentReads:logs,
	}
	if r != nil {
		ctx := s.diagnosticContext(company)
		ledger := firstNonBlank(strings.TrimSpace(r.URL.Query().Get("ledger")),ctx.Ledger)
		if ledger != "" {
			var saved struct { Details map[string]string `json:"details"` }
			if _, ok := s.cache.Get("ledger-details", company, ledger, &saved); ok && len(saved.Details) > 0 {
				d.Buyer = saved.Details
			}
		}
		number := firstNonBlank(strings.TrimSpace(r.URL.Query().Get("number")),ctx.Number)
		date := normISODate(firstNonBlank(strings.TrimSpace(r.URL.Query().Get("date")),ctx.Date))
		masterID := firstNonBlank(strings.TrimSpace(r.URL.Query().Get("masterId")),ctx.MasterID)
		voucherType := firstNonBlank(strings.TrimSpace(r.URL.Query().Get("voucherType")),ctx.VoucherType)
		if number!="" && date!="" {
			key := voucherCacheKey(masterID, VoucherRow{Number:number,Date:date,VoucherType:voucherType})
			var x VoucherDetail
			if _, ok := s.cache.Get("voucher-detail-v6", company, key, &x); ok {
				d.Invoice = map[string]any{
					"number":x.InvoiceNumber,"date":x.InvoiceDate,"masterId":x.MasterID,
					"voucherType":x.VoucherType,"party":x.CustomerName,
					"total":x.TotalAmount,"taxable":x.TaxableAmount,"discount":x.DiscountAmount,
					"freight":x.Freight,"cgst":x.CGST,"sgst":x.SGST,"igst":x.IGST,"roundOff":x.RoundOff,
					"registerAmount":x.RegisterAmount,"delta":x.AmountDelta,
					"verified":x.AmountVerified,"note":x.VerificationNote,"source":x.Source,
				}
			}
		}
	}
	return d
}
func (s *Server) handleDiagnostics(w http.ResponseWriter, r *http.Request) {
	c := strings.TrimSpace(r.URL.Query().Get("company"))
	if c == "" {
		cs,_ := s.cache.Companies()
		if len(cs)>0 { c=cs[0].Name }
	}
	jsonOut(w, s.diagnostics(c,r))
}
const invoiceVerificationTolerance = 0.01

func (s *Server) verifyInvoiceAmount(x *VoucherDetail, hint VoucherRow) {
	if x == nil { return }
	if hint.Amount == 0 {
		x.AmountVerified = true
		x.VerificationNote = "No register amount was supplied; detail total is internally balanced."
		return
	}
	x.RegisterAmount = math.Abs(hint.Amount)
	x.AmountDelta = math.Abs(math.Abs(x.TotalAmount)-x.RegisterAmount)
	x.AmountVerified = x.AmountDelta <= invoiceVerificationTolerance+0.0000001
	if x.AmountVerified {
		x.VerificationNote = "Invoice total matches the register amount within ₹0.01."
	} else {
		x.VerificationNote = fmt.Sprintf("Amount mismatch detected. Register: ₹%.2f • Invoice detail: ₹%.2f • Difference: ₹%.2f.", x.RegisterAmount, math.Abs(x.TotalAmount), x.AmountDelta)
	}
}
func validPDFTemplate(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "compact","detailed","standard":
		return strings.ToLower(strings.TrimSpace(v))
	default:
		return "standard"
	}
}
func applyPDFTemplate(html, template string) string {
	template = validPDFTemplate(template)
	var css string
	switch template {
	case "compact":
		css = `body{font-size:8pt!important}.heading{margin-bottom:3mm!important;padding-bottom:3mm!important}.panel{padding:2.6mm!important}table.items{font-size:7.7pt!important;margin:2mm 0 3mm!important}.items th,.items td{padding:1.7mm 1.5mm!important}.bank{margin-top:3mm!important;padding:3mm!important}.totals td{padding:1.2mm 1.5mm!important}`
	case "detailed":
		css = `body{font-size:9.4pt!important}.heading{padding-bottom:5mm!important;margin-bottom:6mm!important}.panel{padding:4.2mm!important}.items th,.items td{padding:3mm 2.2mm!important}.bank{margin-top:7mm!important;padding:4.8mm!important}`
	default:
		css = ""
	}
	if css=="" { return html }
	return strings.Replace(html,"</style>",css+"</style>",1)
}
func (s *Server) handlePaymentQR(w http.ResponseWriter, r *http.Request) {
	c := strings.TrimSpace(r.URL.Query().Get("company"))
	if c=="" { jsonErr(w,400,errors.New("company required")); return }
	bank, ok := s.store.PDFBank(c)
	if !ok || strings.TrimSpace(bank.UPI)=="" { jsonErr(w,404,errors.New("UPI ID is not configured for this company")); return }
	amount,_ := strconv.ParseFloat(strings.TrimSpace(r.URL.Query().Get("amount")),64)
	if amount < 0 || amount > 1000000000 { jsonErr(w,400,errors.New("invalid payment amount")); return }
	ref := strings.TrimSpace(r.URL.Query().Get("reference"))
	if len(ref)>120 { ref=ref[:120] }
	uri := upiPaymentURI(bank,PaymentQR{Amount:amount,Reference:ref,Payee:firstNonBlank(bank.AccountHolder,c)})
	png,err := qrcode.Encode(uri,qrcode.Medium,512)
	if err!=nil { jsonErr(w,503,err); return }
	w.Header().Set("Content-Type","image/png")
	w.Header().Set("Content-Disposition",`inline; filename="TallyBridge-UPI-QR.png"`)
	w.Header().Set("Cache-Control","no-store")
	_,_ = w.Write(png)
}

var backupNames = map[string]bool{
	"settings.json":true,"phone.json":true,"pdf-banks.json":true,"native-security.json":true,
}
func (s *Server) handleBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method==http.MethodGet {
		var buf bytes.Buffer
		zw:=zip.NewWriter(&buf)
		mf,_:=zw.Create("manifest.json")
		_ = json.NewEncoder(mf).Encode(map[string]any{"format":1,"version":Version,"createdAt":time.Now()})
		for name := range backupNames {
			b,err:=os.ReadFile(filepath.Join(s.store.dir,name))
			if err!=nil { if os.IsNotExist(err){continue}; jsonErr(w,500,err); return }
			f,_:=zw.Create(name); _,_ = f.Write(b)
		}
		_ = zw.Close()
		w.Header().Set("Content-Type","application/zip")
		w.Header().Set("Content-Disposition",`attachment; filename="TallyBridge-Backup.zip"`)
		_,_ = w.Write(buf.Bytes())
		return
	}
	if r.Method!=http.MethodPost { jsonErr(w,405,errors.New("GET or POST required")); return }
	raw,err:=io.ReadAll(io.LimitReader(r.Body,8<<20))
	if err!=nil || len(raw)==0 || len(raw)>=8<<20 { jsonErr(w,400,errors.New("invalid or oversized backup")); return }
	zr,err:=zip.NewReader(bytes.NewReader(raw),int64(len(raw)))
	if err!=nil { jsonErr(w,400,errors.New("invalid backup ZIP")); return }
	files:=map[string][]byte{}
	for _,zf:=range zr.File {
		name:=filepath.Base(zf.Name)
		if !backupNames[name] { continue }
		if zf.UncompressedSize64>2<<20 { jsonErr(w,400,errors.New("backup entry too large")); return }
		rc,e:=zf.Open(); if e!=nil { jsonErr(w,400,e); return }
		b,e:=io.ReadAll(io.LimitReader(rc,2<<20)); _=rc.Close()
		if e!=nil { jsonErr(w,400,e); return }
		var tmp any
		if json.Unmarshal(b,&tmp)!=nil { jsonErr(w,400,errors.New("backup contains invalid JSON")); return }
		files[name]=b
	}
	if len(files)==0 { jsonErr(w,400,errors.New("backup contains no TallyBridge settings")); return }
	for name,b:=range files {
		if err:=atomicWrite(filepath.Join(s.store.dir,name),b,0600); err!=nil { jsonErr(w,500,err); return }
	}
	jsonOut(w,map[string]any{"ok":true,"restartRequired":true,"restoredFiles":len(files)})
}

func parseVersionParts(v string) []int {
	v=strings.TrimPrefix(strings.TrimSpace(v),"v")
	v=strings.SplitN(v,"-",2)[0]
	out:=[]int{}
	for _,p:=range strings.Split(v,".") { n,_:=strconv.Atoi(p); out=append(out,n) }
	for len(out)<3 { out=append(out,0) }
	return out
}
func newerVersion(a,b string) bool {
	x,y:=parseVersionParts(a),parseVersionParts(b)
	for i:=0;i<3;i++ { if x[i]!=y[i] { return x[i]>y[i] } }
	return false
}
func checkLatestRelease(ctx context.Context) UpdateInfo {
	info:=UpdateInfo{CheckedAt:time.Now(),Current:Version}
	req,_:=http.NewRequestWithContext(ctx,http.MethodGet,"https://api.github.com/repos/vsshegur/TallyBridge/releases/latest",nil)
	req.Header.Set("Accept","application/vnd.github+json")
	req.Header.Set("User-Agent","TallyBridge/"+Version)
	client:=&http.Client{Timeout:4*time.Second}
	resp,err:=client.Do(req)
	if err!=nil { info.Error=err.Error(); return info }
	defer resp.Body.Close()
	if resp.StatusCode==http.StatusNotFound { info.Notes="No production release has been published yet."; return info }
	if resp.StatusCode<200||resp.StatusCode>=300 { info.Error="GitHub update check returned "+resp.Status; return info }
 	// Decode explicitly because GitHub fields use different names.
	var raw struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
		Body string `json:"body"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body,1<<20)).Decode(&raw)!=nil { info.Error="Could not read release information"; return info }
	info.Latest=strings.TrimPrefix(raw.TagName,"v")
	info.URL=raw.HTMLURL
	if len(raw.Body)>500 { raw.Body=raw.Body[:500] }
	info.Notes=raw.Body
	info.Available=newerVersion(info.Latest,Version)
	return info
}
func (s *Server) updateSnapshot() UpdateInfo {
	s.updateMu.Lock()
	defer s.updateMu.Unlock()
	return s.updateInfo
}
func (s *Server) storeUpdateInfo(x UpdateInfo) {
	s.updateMu.Lock()
	s.updateInfo=x
	s.updateChecking=false
	s.updateMu.Unlock()
}
func (s *Server) maybeCheckUpdates() {
	s.updateMu.Lock()
	if s.updateChecking || (!s.updateInfo.CheckedAt.IsZero() && time.Since(s.updateInfo.CheckedAt) < 6*time.Hour) {
		s.updateMu.Unlock()
		return
	}
	s.updateChecking=true
	s.updateMu.Unlock()
	go func(){
		ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second)
		defer cancel()
		s.storeUpdateInfo(checkLatestRelease(ctx))
	}()
}
func (s *Server) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	ctx,cancel:=context.WithTimeout(r.Context(),5*time.Second); defer cancel()
	x:=checkLatestRelease(ctx)
	s.storeUpdateInfo(x)
	jsonOut(w,x)
}

func (s *Server) TrayStatus() string {
	st:=s.stateSnapshot()
	sync:=s.autoSyncSnapshot()
	if sync.Running {
		step:=strings.TrimSpace(sync.Step)
		if step=="" { step="syncing" }
		return "TallyBridge • Syncing: "+step
	}
	if st.TallyOnline {
		n:=len(st.Companies)
		if n==0 { return "TallyBridge • Tally connected" }
		return fmt.Sprintf("TallyBridge • Tally connected • %d company",n)+map[bool]string{true:"",false:"ies"}[n==1]
	}
	if !st.LastSuccess.IsZero() {
		return "TallyBridge • Tally unavailable • saved data ready"
	}
	return "TallyBridge • Waiting for Tally"
}

func (s *Server) TrayLevel() string {
	st:=s.stateSnapshot()
	if st.TallyOnline { return "green" }
	if strings.TrimSpace(st.LastError)!="" { return "red" }
	return "yellow"
}
