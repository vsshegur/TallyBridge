from pathlib import Path

ROOT=Path('.')
def read(p): return (ROOT/p).read_text()
def write(p,s): (ROOT/p).write_text(s)
def one(p, old, new):
    s=read(p); n=s.count(old)
    if n!=1: raise SystemExit(f'{p}: expected 1 occurrence, got {n}: {old[:90]!r}')
    write(p,s.replace(old,new,1))
def contains(p, needle):
    if needle not in read(p): raise SystemExit(f'{p}: missing {needle!r}')

one('internal/app/types.go','const Version = "2.4.0"','const Version = "2.4.1"')
one('android/app/build.gradle','versionCode 20400','versionCode 20401')
one('android/app/build.gradle',"versionName '2.4.0'","versionName '2.4.1'")

# New parser semantics must not reuse older cached HSN/address results.
for p in (ROOT/'internal/app').glob('*.go'):
    s=p.read_text()
    if 'voucher-detail-v6' in s: p.write_text(s.replace('voucher-detail-v6','voucher-detail-v7'))

# Ask Tally for its resolved invoice HSN method and prefer that value.
p='internal/app/tally.go'; s=read(p)
s=s.replace('AllInventoryEntries.*,AllLedgerEntries.*','AllInventoryEntries.*,AllInventoryEntries.GSTItemHSNCodeEx,AllLedgerEntries.*')
s=s.replace('<FETCH>AllInventoryEntries.*</FETCH><FETCH>AllLedgerEntries.*</FETCH>','<FETCH>AllInventoryEntries.*</FETCH><FETCH>AllInventoryEntries.GSTItemHSNCodeEx</FETCH><FETCH>AllLedgerEntries.*</FETCH>')
if 'GSTItemHSNCodeEx' not in s: raise SystemExit('resolved HSN fetch was not inserted')
anchor='''func parseVoucherDetail(b []byte, company, masterID string, hint VoucherRow) VoucherDetail {\n\tr, _ := parseTree(b)\n\treturn parseVoucherDetailNode(r, company, masterID, hint)\n}\n'''
helper=r'''func invoiceItemHSN(n *xnode) string {
	if n == nil { return "" }
	v := firstNonBlank(firstDeep(n, "GSTITEMHSNCODEEX"), firstDeep(n, "GSTITEMHSNCODE"), firstDeep(n, "HSNCODE"), firstDeep(n, "HSNSAC"), firstDeep(n, "HSN"))
	if v == "" {
		v = firstDeepTagMatch(n, func(tag string) bool {
			u := strings.ToUpper(strings.TrimSpace(tag))
			if strings.Contains(u, "DESCRIPTION") || strings.Contains(u, "SOURCE") { return false }
			return strings.Contains(u, "HSN") && (strings.Contains(u, "CODE") || strings.Contains(u, "SAC"))
		})
	}
	return strings.TrimSpace(v)
}
func cleanInvoiceAddressLines(values []string) []string {
	out := make([]string,0,len(values)); seen := map[string]bool{}
	for _, value := range values {
		for _, line := range strings.Split(strings.ReplaceAll(value,"\r","\n"),"\n") {
			line=strings.TrimSpace(line); key:=strings.ToLower(strings.Join(strings.Fields(line)," "))
			if line!="" && !seen[key] { seen[key]=true; out=append(out,line) }
		}
	}
	return out
}
func invoiceBuyerAddress(r *xnode) []string {
	if r==nil { return nil }
	lines:=cleanInvoiceAddressLines(allDeep(r,"BASICBUYERADDRESS"))
	if len(lines)==0 { lines=cleanInvoiceAddressLines(allDeep(r,"BUYERADDRESS")) }
	if len(lines)==0 { lines=cleanInvoiceAddressLines(allDeep(r,"ADDRESS")) }
	return lines
}
func addressHasLine(lines []string,value string) bool {
	want:=strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value))," ")); if want=="" { return false }
	for _, line:=range lines { if strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(line))," "))==want { return true } }
	return false
}
func parseVoucherDetail(b []byte, company, masterID string, hint VoucherRow) VoucherDetail {
	r, _ := parseTree(b)
	return parseVoucherDetailNode(r, company, masterID, hint)
}
'''
if s.count(anchor)!=1: raise SystemExit(f'parse anchor count {s.count(anchor)}')
s=s.replace(anchor,helper,1)
old='''\tx.CustomerAddress = allDeep(r, "ADDRESS")\n\tfor _, n := range invoiceInventoryRows(r) {'''
new='''\tx.CustomerAddress = invoiceBuyerAddress(r)\n\tif addressHasLine(x.CustomerAddress, x.CustomerCity) { x.CustomerCity = "" }\n\tfor _, n := range invoiceInventoryRows(r) {'''
if s.count(old)!=1: raise SystemExit('buyer address anchor missing')
s=s.replace(old,new,1)
old='HSN: firstDeep(n, "HSN", "HSNCODE", "HSNSAC")'
if s.count(old)!=1: raise SystemExit('HSN parser anchor missing')
s=s.replace(old,'HSN: invoiceItemHSN(n)',1); write(p,s)

# Reuse saved ledger identity if voucher XML omits buyer address/city/GSTIN.
p='internal/app/server.go'
old='''\tprofile, _ := s.companyProfile(company, false)\n\tx.Supplier = profile\n\tx.PDFTemplate = validPDFTemplate(s.store.Settings().PDFTemplate)'''
new='''\tprofile, _ := s.companyProfile(company, false)\n\tx.Supplier = profile\n\tif strings.TrimSpace(x.CustomerName) != "" && (len(x.CustomerAddress) == 0 || x.CustomerCity == "" || x.GSTIN == "") {\n\t\tvar saved struct { Details map[string]string `json:"details"` }\n\t\tif _, ok := s.cache.Get("ledger-details", company, x.CustomerName, &saved); ok {\n\t\t\tif len(x.CustomerAddress)==0 { if address:=strings.TrimSpace(saved.Details["ADDRESS"]); address!="" { x.CustomerAddress=cleanInvoiceAddressLines([]string{strings.ReplaceAll(address, ", ", "\\n")}) } }\n\t\t\tif x.CustomerCity=="" && !addressHasLine(x.CustomerAddress,saved.Details["CITY"]) { x.CustomerCity=strings.TrimSpace(saved.Details["CITY"]) }\n\t\t\tif x.GSTIN=="" { x.GSTIN=strings.TrimSpace(saved.Details["PARTYGSTIN"]) }\n\t\t}\n\t}\n\tx.PDFTemplate = validPDFTemplate(s.store.Settings().PDFTemplate)'''
one(p,old,new)

# Customer-facing invoice redesign: no technical read-only message/footer; better Bill To hierarchy.
p='internal/app/pdf.go'; s=read(p)
old='''func invoiceHTML(x VoucherDetail, bank ...BankDetails) string {\n\tvar b strings.Builder\n\tfmt.Fprintf(&b, `<div class="meta">%s</div><div class="grid"><section class="panel"><h3>Bill to</h3><p><b>%s</b></p>`, he(pdfDataNote(x.CommonData)), he(firstNonBlank(x.CustomerName, "Customer not returned")))\n\tfor _, a := range x.CustomerAddress {\n\t\tif a != "" { fmt.Fprintf(&b, "<p>%s</p>", he(a)) }\n\t}\n\tif x.CustomerCity != "" { fmt.Fprintf(&b, "<p>%s</p>", he(x.CustomerCity)) }\n\tif x.GSTIN != "" { fmt.Fprintf(&b, `<p><span class="label">GSTIN</span> %s</p>`, he(x.GSTIN)) }\n\tb.WriteString(`</section><section class="panel"><h3>Invoice details</h3><table class="kv">`)'''
new='''func invoiceHTML(x VoucherDetail, bank ...BankDetails) string {\n\tvar b strings.Builder\n\tfmt.Fprintf(&b, `<div class="grid invoice-grid"><section class="panel party-panel"><div class="invoice-panel-label">BILL TO</div><p class="party-name"><b>%s</b></p>`, he(firstNonBlank(x.CustomerName, "Customer")))\n\tseenAddress := map[string]bool{}\n\tfor _, a := range x.CustomerAddress {\n\t\ta=strings.TrimSpace(a); key:=strings.ToLower(strings.Join(strings.Fields(a)," "))\n\t\tif a!="" && !seenAddress[key] { seenAddress[key]=true; fmt.Fprintf(&b,"<p>%s</p>",he(a)) }\n\t}\n\tcityKey:=strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(x.CustomerCity))," "))\n\tif x.CustomerCity!="" && !seenAddress[cityKey] { fmt.Fprintf(&b,"<p>%s</p>",he(x.CustomerCity)) }\n\tif x.GSTIN!="" { fmt.Fprintf(&b, `<p class="gst-line"><span class="label">GSTIN</span> <b>%s</b></p>`, he(x.GSTIN)) }\n\tb.WriteString(`</section><section class="panel invoice-meta-panel"><div class="invoice-panel-label">INVOICE DETAILS</div><table class="kv">`)'''
if s.count(old)!=1: raise SystemExit('invoice opening anchor missing')
s=s.replace(old,new,1)
old='''\ttitle, subtitle := "INVOICE DETAIL", "Read-only copy of data returned by TallyPrime"\n\tif x.Optional {\n\t\ttitle, subtitle = "PROFORMA INVOICE", "Optional / non-posting voucher from TallyPrime"\n\t}\n\treturn adaptiveInvoiceHTML(pdfDocument(firstNonBlank(x.FirmName, x.Company), title, subtitle, b.String(), x.Supplier), len(x.Items))\n}'''
new='''\ttitle := "TAX INVOICE"\n\tif x.Optional { title = "PROFORMA INVOICE" }\n\tsubtitleParts:=[]string{}\n\tif strings.TrimSpace(x.InvoiceNumber)!="" { subtitleParts=append(subtitleParts,"No. "+strings.TrimSpace(x.InvoiceNumber)) }\n\tif strings.TrimSpace(x.InvoiceDate)!="" { subtitleParts=append(subtitleParts,"Date "+displayDate(x.InvoiceDate)) }\n\tdoc:=pdfDocument(firstNonBlank(x.FirmName,x.Company),title,strings.Join(subtitleParts,"  •  "),b.String(),x.Supplier)\n\tdoc=strings.Replace(doc,"TallyBridge  |  READ ONLY","TallyBridge",1)\n\tinvoiceCSS:=`.invoice-grid{margin-bottom:3.5mm}.invoice-grid .panel{background:#fff;border:1pt solid #d8e2ec;border-top:2.5pt solid #176b5b}.invoice-grid .panel+.panel{border-left:4mm solid white}.invoice-panel-label{font-size:7.4pt;letter-spacing:.12em;font-weight:800;color:#176b5b;margin-bottom:2mm}.party-name{font-size:11pt;color:#132c49;margin-bottom:2mm!important}.gst-line{margin-top:2mm!important;padding-top:1.5mm;border-top:.7pt solid #e2e8ef}.items th{background:#153c5a}.grand td{background:#e7f2ef;border-top-color:#176b5b}.bank{border-left:2.5pt solid #176b5b;background:#f7faf9}`\n\tdoc=strings.Replace(doc,"</style>",invoiceCSS+"</style>",1)\n\treturn adaptiveInvoiceHTML(doc,len(x.Items))\n}'''
if s.count(old)!=1: raise SystemExit('invoice title anchor missing')
s=s.replace(old,new,1); write(p,s)

# Do not reset/finish screens merely because the user switched to another app.
one('android/app/src/main/java/in/tallybridge/app/MainActivity.java',' @Override protected void onStop(){super.onStop();visible=false;if(!authenticating){unlocked=false;generation++;cache.clear();cacheAt.clear();} }\n',' @Override protected void onStop(){super.onStop();visible=false;/* Preserve current screen while process remains alive. */}\n')
one('android/app/src/main/java/in/tallybridge/app/WorkActivity.java',' @Override protected void onStop(){super.onStop();if(!isChangingConfigurations())finish();}\n',' @Override protected void onStop(){super.onStop();/* Preserve Today/Customer/System while backgrounded. */}\n')

# Official permanent certificate from 2.4.1 onward.
one('android/app/src/main/java/in/tallybridge/app/AppIdentity.java','static final String EXPECTED_SHA1="97:43:9E:2E:F7:40:C7:6C:0C:25:CD:AB:0F:AE:B1:B7:B2:83:AA:CE";','static final String EXPECTED_SHA1="55:CC:4A:6D:74:68:51:43:A7:E1:E9:14:B7:39:CF:C6:FF:4F:2A:BE";')
one('android/app/src/main/java/in/tallybridge/app/AppIdentity.java','static final String EXPECTED_SHA256="7F:17:BA:19:5D:05:7C:66:11:DE:AE:88:1D:85:B0:A0:D8:0D:BA:6A:44:EC:7C:21:43:40:05:3D:B4:D1:DB:AB";','static final String EXPECTED_SHA256="55:2C:7E:A3:B6:93:BE:84:19:59:24:83:CD:5A:26:1B:F3:AF:83:AD:5B:93:F7:EF:35:C4:17:27:64:B0:63:DF";')
write('android/PERMANENT_SIGNING_IDENTITY.txt','''TallyBridge permanent Android signing identity — official from 2.4.1 onward\n\nPackage: in.tallybridge.app\nCertificate subject: CN=TallyBridge Android Release, OU=Mobile, O=TallyBridge, L=Solapur, ST=Maharashtra, C=IN\nSHA-1: 55:CC:4A:6D:74:68:51:43:A7:E1:E9:14:B7:39:CF:C6:FF:4F:2A:BE\nSHA-256: 55:2C:7E:A3:B6:93:BE:84:19:59:24:83:CD:5A:26:1B:F3:AF:83:AD:5B:93:F7:EF:35:C4:17:27:64:B0:63:DF\nCertificate valid until: 2126-09-08\n\nAll production Android releases from 2.4.1 onward must use this exact identity. The release workflow fails closed if another keystore is supplied. Earlier unrelated signing identities require one uninstall/reinstall transition after offline Orders/photos are synced; after that, future releases update normally.\n''')
old='''          echo "TB_KEYSTORE=$RUNNER_TEMP/tallybridge-release.jks" >> "$GITHUB_ENV"\n      - name: Test and build signed Android release'''
new='''          echo "TB_KEYSTORE=$RUNNER_TEMP/tallybridge-release.jks" >> "$GITHUB_ENV"\n      - name: Verify permanent Android signing identity\n        shell: bash\n        run: |\n          IDENTITY="$(keytool -list -v -keystore "$TB_KEYSTORE" -storepass "$TB_STORE_PASSWORD" -alias "$TB_KEY_ALIAS")"\n          echo "$IDENTITY" | grep -F "SHA1: 55:CC:4A:6D:74:68:51:43:A7:E1:E9:14:B7:39:CF:C6:FF:4F:2A:BE"\n          echo "$IDENTITY" | grep -F "SHA256: 55:2C:7E:A3:B6:93:BE:84:19:59:24:83:CD:5A:26:1B:F3:AF:83:AD:5B:93:F7:EF:35:C4:17:27:64:B0:63:DF"\n      - name: Test and build signed Android release'''
one('.github/workflows/release.yml',old,new)

# Permanent CI includes lifecycle + compact/tall widget proof.
one('.github/workflows/build.yml',"grep -q 'OK (10 tests)' /tmp/design-results.txt","grep -q 'OK (11 tests)' /tmp/design-results.txt")
one('.github/workflows/build.yml','26-work-today 27-customer-360 28-work-system; do','26-work-today 27-customer-360 28-work-system 29-orders-widget-compact 30-orders-widget-tall; do')

contains('internal/app/tally.go','GSTItemHSNCodeEx'); contains('internal/app/pdf.go','TAX INVOICE'); contains('android/app/src/main/java/in/tallybridge/app/AppIdentity.java','55:CC:4A'); contains('.github/workflows/build.yml','OK (11 tests)')
print('2.4.1 core patch applied')
