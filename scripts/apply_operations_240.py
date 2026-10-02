from pathlib import Path


def replace(path, old, new, count=1):
    p=Path(path); s=p.read_text(encoding='utf-8')
    n=s.count(old)
    if n < count:
        raise SystemExit(f'{path}: expected at least {count} occurrence(s), found {n}: {old[:80]!r}')
    s=s.replace(old,new,count)
    p.write_text(s,encoding='utf-8')

# Version alignment.
replace('internal/app/types.go','const Version = "2.3.4"','const Version = "2.4.0"')
replace('android/app/build.gradle','versionCode 20304','versionCode 20400')
replace('android/app/build.gradle',"versionName '2.3.4'","versionName '2.4.0'")

# Shared Operations Center APIs for both localhost desktop and authenticated Android.
replace('internal/app/server.go',
'\tm.HandleFunc("/api/v1/admin/diagnostics", s.localAdmin(s.handleDiagnostics))\n',
'\tm.HandleFunc("/api/v1/admin/diagnostics", s.localAdmin(s.handleDiagnostics))\n'
'\tm.HandleFunc("/api/v1/admin/workspace", s.localAdmin(s.handleWorkspace))\n'
'\tm.HandleFunc("/api/v1/admin/alerts", s.localAdmin(s.handleAlerts))\n'
'\tm.HandleFunc("/api/v1/admin/followups", s.localAdmin(s.handleFollowUps))\n'
'\tm.HandleFunc("/api/v1/admin/global-search", s.localAdmin(s.handleGlobalSearch))\n'
'\tm.HandleFunc("/api/v1/admin/customer-workspace", s.localAdmin(s.handleCustomerWorkspace))\n'
'\tm.HandleFunc("/api/v1/admin/tally-performance", s.localAdmin(s.handleTallyPerformance))\n')
replace('internal/app/server.go',
'\tm.HandleFunc("/api/v1/mobile/diagnostics", s.mobileAuth(s.handleDiagnostics))\n',
'\tm.HandleFunc("/api/v1/mobile/diagnostics", s.mobileAuth(s.handleDiagnostics))\n'
'\tm.HandleFunc("/api/v1/mobile/workspace", s.mobileAuth(s.handleWorkspace))\n'
'\tm.HandleFunc("/api/v1/mobile/alerts", s.mobileAuth(s.handleAlerts))\n'
'\tm.HandleFunc("/api/v1/mobile/followups", s.mobileAuth(s.handleFollowUps))\n'
'\tm.HandleFunc("/api/v1/mobile/global-search", s.mobileAuth(s.handleGlobalSearch))\n'
'\tm.HandleFunc("/api/v1/mobile/customer-workspace", s.mobileAuth(s.handleCustomerWorkspace))\n'
'\tm.HandleFunc("/api/v1/mobile/tally-performance", s.mobileAuth(s.handleTallyPerformance))\n')

# Health now checks whether daily operational data is actually ready as well.
replace('internal/app/production_features.go',
'\tadd(&items,"qr","QR ready",qrValue,"Add a UPI ID to enable customer payment QR codes.",false)\n',
'\tadd(&items,"qr","QR ready",qrValue,"Add a UPI ID to enable customer payment QR codes.",false)\n'
'\tsalesReady := ""; var sales VoucherData; if _,ok:=s.cache.Get("sales",company,"month",&sales); ok && voucherCacheIsClassified(sales) { salesReady="Sales register synced" }\n'
'\tproformaReady := ""; var proforma VoucherData; if _,ok:=s.cache.Get("proforma",company,"month",&proforma); ok && voucherCacheIsClassified(proforma) { proformaReady="Proforma register synced" }\n'
'\tordersReady := ""; if _,err:=s.listOrders(company); err==nil { ordersReady="Orders store ready" }\n'
'\tadd(&items,"sales-sync","Sales sync",salesReady,"Run Sync now once while the company is open in Tally.",true)\n'
'\tadd(&items,"proforma-sync","Proforma sync",proformaReady,"Open/sync the Sales register so Optional vouchers are classified separately.",false)\n'
'\tadd(&items,"orders-sync","Orders",ordersReady,"Repair the local Orders store if it cannot be opened.",true)\n')

# Update checker returns the exact official signed Android APK asset.
replace('internal/app/production_features.go',
'\tError      string    `json:"error,omitempty"`\n}',
'\tError      string    `json:"error,omitempty"`\n\tAPKURL     string    `json:"apkUrl,omitempty"`\n\tAPKName    string    `json:"apkName,omitempty"`\n}')
replace('internal/app/production_features.go',
'\t\tBody string `json:"body"`\n\t}',
'\t\tBody string `json:"body"`\n\t\tAssets []struct { Name string `json:"name"`; BrowserDownloadURL string `json:"browser_download_url"` } `json:"assets"`\n\t}')
replace('internal/app/production_features.go',
'\tinfo.Available=newerVersion(info.Latest,Version)\n\treturn info\n}',
'\tinfo.Available=newerVersion(info.Latest,Version)\n'
'\tfor _,a:=range raw.Assets { n:=strings.ToLower(strings.TrimSpace(a.Name)); if strings.HasSuffix(n,".apk") && strings.Contains(n,"tallybridge") { info.APKName=a.Name; info.APKURL=a.BrowserDownloadURL; break } }\n'
'\treturn info\n}')

# Adaptive one-page profile for short invoice/proforma PDFs.
replace('internal/app/pdf.go',
'func invoiceHTML(x VoucherDetail, bank ...BankDetails) string {',
'''func adaptiveInvoiceHTML(doc string, itemCount int) string {
	if itemCount > 7 { return doc }
	css := `@page{margin:10mm 10mm 12mm!important}.heading{padding-bottom:2.2mm!important;margin-bottom:2.8mm!important}h1{font-size:18pt!important}.supplier-line{font-size:7.6pt!important}.grid{margin-bottom:2.5mm!important}.panel{padding:2.2mm!important}.panel p{margin-bottom:.8mm!important}.kv td{padding:.55mm 0!important}table.items{margin:1.8mm 0 2.5mm!important;font-size:7.9pt!important}.items th{padding:1.6mm 1.4mm!important}.items td{padding:1.55mm 1.4mm!important}.totals{width:58%!important}.totals td{padding:.9mm 1.4mm!important}.grand td{padding:1.8mm 1.4mm!important;font-size:10pt!important}.bank{margin-top:2.8mm!important;padding:2.4mm!important}.bank .kv td{padding:.35mm 0!important}.bank-qr img{width:25mm!important;height:25mm!important}.note{padding:2mm!important;margin:2mm 0!important}.narration{margin-top:2mm!important}`
	if itemCount <= 3 { css += `body{font-size:8.4pt!important}.caption,.label{font-size:7.4pt!important}` }
	return strings.Replace(doc,"</style>",css+"</style>",1)
}
func invoiceHTML(x VoucherDetail, bank ...BankDetails) string {''')
replace('internal/app/pdf.go',
'\treturn pdfDocument(firstNonBlank(x.FirmName, x.Company), title, subtitle, b.String(), x.Supplier)\n}',
'\treturn adaptiveInvoiceHTML(pdfDocument(firstNonBlank(x.FirmName, x.Company), title, subtitle, b.String(), x.Supplier), len(x.Items))\n}',1)
replace('internal/app/proforma.go',
'\treturn pdfDocument(p.Company, "PROFORMA INVOICE", "Not a tax invoice", b.String(), profile)\n}',
'\treturn adaptiveInvoiceHTML(pdfDocument(p.Company, "PROFORMA INVOICE", "Not a tax invoice", b.String(), profile), len(p.Items))\n}')

# Android updater permission + organized Work activity.
replace('android/app/src/main/AndroidManifest.xml',
'    <uses-permission android:name="android.permission.INTERNET" />\n',
'    <uses-permission android:name="android.permission.INTERNET" />\n    <uses-permission android:name="android.permission.REQUEST_INSTALL_PACKAGES" />\n')
replace('android/app/src/main/AndroidManifest.xml',
'        <activity android:name=".OrdersActivity" android:exported="false"\n',
'        <activity android:name=".WorkActivity" android:exported="false" android:label="Today" />\n        <activity android:name=".OrdersActivity" android:exported="false"\n')

# Provider may grant a verified downloaded APK to Android Package Installer.
replace('android/app/src/main/java/in/tallybridge/app/PdfProvider.java',
'if(name==null||(!name.endsWith(".pdf")&&!name.endsWith(".png")))throw new IOException();',
'if(name==null||(!name.endsWith(".pdf")&&!name.endsWith(".png")&&!name.endsWith(".apk")))throw new IOException();')
replace('android/app/src/main/java/in/tallybridge/app/PdfProvider.java',
'@Override public String getType(Uri uri){String n=uri.getLastPathSegment();return n!=null&&n.endsWith(".png")?"image/png":"application/pdf";}',
'@Override public String getType(Uri uri){String n=uri.getLastPathSegment();if(n!=null&&n.endsWith(".png"))return "image/png";if(n!=null&&n.endsWith(".apk"))return "application/vnd.android.package-archive";return "application/pdf";}')

# Reorganize bottom navigation: daily work and orders are first-class; Proforma/Payments remain reachable inside reports/more.
replace('android/app/src/main/java/in/tallybridge/app/MainActivity.java',
'for(String label:new String[]{"Home","Reports","Proforma","Payments","More"})',
'for(String label:new String[]{"Home","Work","Orders","Reports","More"})')
replace('android/app/src/main/java/in/tallybridge/app/MainActivity.java',
'private void rootTab(String name){history.clear();tab=name;switch(name){case "Home":home();break;case "Reports":reports();break;case "Proforma":proforma();break;case "Payments":payments();break;default:more();}}',
'private void rootTab(String name){history.clear();tab=name;switch(name){case "Home":home();break;case "Work":startActivity(new Intent(this,WorkActivity.class));break;case "Orders":startActivity(new Intent(this,OrdersActivity.class));break;case "Reports":reports();break;case "Proforma":proforma();break;case "Payments":payments();break;default:more();}}')
replace('android/app/src/main/java/in/tallybridge/app/MainActivity.java',
'  data.add(heading("Sales & purchases","Registers and dispatch"));\n',
'  data.add(heading("Sales & purchases","Registers and dispatch"));\n  data.add(row("Create proforma","Prepare a new proforma from Tally masters","",()->open(this::proforma)));\n')
replace('android/app/src/main/java/in/tallybridge/app/MainActivity.java',
'  data.add(row("Payment settings",object(boot,"pdfBanks").optString(company.trim().toLowerCase(Locale.ROOT),"Not selected")+" • change on Windows","",()->rootTab("Payments")));',
'  data.add(row("Payment settings",object(boot,"pdfBanks").optString(company.trim().toLowerCase(Locale.ROOT),"Not selected")+" • change on Windows","",()->open(this::payments)));')
replace('android/app/src/main/java/in/tallybridge/app/MainActivity.java',
'  String release=x.optString("url");if(x.optBoolean("available")&&release.startsWith("https://github.com/vsshegur/TallyBridge/releases/"))data.add(row("Open release / download","Review the signed release before installing","",()->openExternal(release)));',
'  String release=x.optString("url"),apk=x.optString("apkUrl");if(x.optBoolean("available")&&!apk.isEmpty())data.add(row("Install signed update","Downloads and verifies the permanent TallyBridge signature first","",()->UpdateInstaller.install(this,apk)));if(x.optBoolean("available")&&release.startsWith("https://github.com/vsshegur/TallyBridge/releases/"))data.add(row("Open release page","Review release notes and files","",()->openExternal(release)));')
replace('android/app/src/main/java/in/tallybridge/app/MainActivity.java',
'  data.add(row("Selected company",x.optString("company",company),"",null));',
'  data.add(row("Selected company",x.optString("company",company),"",null));\n  data.add(row("Android signing",AppIdentity.status(this),"",null));\n  data.add(row("SHA-1",AppIdentity.sha1(this),"",null));\n  data.add(row("SHA-256",AppIdentity.sha256(this),"",null));')

# Quick Order voice capture inside the existing offline-safe order form.
replace('android/app/src/main/java/in/tallybridge/app/OrdersActivity.java',
'import android.os.*;\n',
'import android.os.*;\nimport android.speech.RecognizerIntent;\n')
replace('android/app/src/main/java/in/tallybridge/app/OrdersActivity.java',
' private TextView pendingTabView,dispatchedTabView;\n',
' private TextView pendingTabView,dispatchedTabView;\n private static final int VOICE_ORDER=840; private AutoCompleteTextView voiceBuyer; private EditText voiceNotes; private LinearLayout voiceItemsBox;\n')
replace('android/app/src/main/java/in/tallybridge/app/OrdersActivity.java',
'  body.addView(txt("BUYER",11,MUTED),full(14));AutoCompleteTextView buyer=auto("Search saved Tally buyer or type name");',
'  body.addView(txt("BUYER",11,MUTED),full(14));AutoCompleteTextView buyer=auto("Search saved Tally buyer or type name");')
replace('android/app/src/main/java/in/tallybridge/app/OrdersActivity.java',
'  body.addView(txt("NOTES",11,MUTED),full(16));EditText notes=input("Colours, packing, delivery instructions…",InputType.TYPE_CLASS_TEXT|InputType.TYPE_TEXT_FLAG_MULTI_LINE);notes.setMinLines(2);if(order!=null)notes.setText(order.optString("notes"));body.addView(notes,full(6));body.addView(primary(order==null?"Save pending order":"Update pending order",v->saveForm(order,companySpin,buyer,dispatchedThrough,notes)),full(18));',
'  body.addView(txt("NOTES",11,MUTED),full(16));EditText notes=input("Colours, packing, delivery instructions…",InputType.TYPE_CLASS_TEXT|InputType.TYPE_TEXT_FLAG_MULTI_LINE);notes.setMinLines(2);if(order!=null)notes.setText(order.optString("notes"));body.addView(notes,full(6));if(order==null)body.addView(btn("🎙 Speak order",v->startVoiceOrder(buyer,notes,itemsBox)),full(8));body.addView(primary(order==null?"Save pending order":"Update pending order",v->saveForm(order,companySpin,buyer,dispatchedThrough,notes)),full(18));')
# Insert voice helpers before refreshBootstrapSilently.
replace('android/app/src/main/java/in/tallybridge/app/OrdersActivity.java',
' private void refreshBootstrapSilently(){',
''' private void startVoiceOrder(AutoCompleteTextView buyer,EditText notes,LinearLayout itemsBox){
  voiceBuyer=buyer;voiceNotes=notes;voiceItemsBox=itemsBox;
  Intent i=new Intent(RecognizerIntent.ACTION_RECOGNIZE_SPEECH);i.putExtra(RecognizerIntent.EXTRA_LANGUAGE_MODEL,RecognizerIntent.LANGUAGE_MODEL_FREE_FORM);i.putExtra(RecognizerIntent.EXTRA_PROMPT,"Say party, item and quantity");
  try{startActivityForResult(i,VOICE_ORDER);}catch(Exception e){toast("Voice recognition is not available on this phone");}
 }
 @Override protected void onActivityResult(int request,int result,Intent data){super.onActivityResult(request,result,data);if(request!=VOICE_ORDER||result!=RESULT_OK||data==null)return;ArrayList<String> xs=data.getStringArrayListExtra(RecognizerIntent.EXTRA_RESULTS);if(xs==null||xs.isEmpty())return;applyVoiceOrder(xs.get(0));}
 private void applyVoiceOrder(String spoken){
  if(voiceNotes!=null)voiceNotes.setText(spoken);String lower=spoken.toLowerCase(Locale.ROOT);
  if(voiceBuyer!=null&&parties!=null){String best="";for(int i=0;i<parties.length();i++){JSONObject p=parties.optJSONObject(i);String n=p==null?parties.optString(i):p.optString("name");if(!n.isEmpty()&&lower.contains(n.toLowerCase(Locale.ROOT))&&n.length()>best.length())best=n;}if(!best.isEmpty())voiceBuyer.setText(best,false);}
  JSONObject matched=null;String matchedName="";if(stock!=null){for(int i=0;i<stock.length();i++){JSONObject x=stock.optJSONObject(i);String n=x==null?"":x.optString("name");if(!n.isEmpty()&&lower.contains(n.toLowerCase(Locale.ROOT))&&n.length()>matchedName.length()){matched=x;matchedName=n;}}}
  if(matched!=null){double q=1;java.util.regex.Matcher m=java.util.regex.Pattern.compile("(?:^|\\s)(\\d+(?:\\.\\d+)?)\\s+(?:dozen\\s+|pcs?\\s+|pieces?\\s+)?"+java.util.regex.Pattern.quote(matchedName.toLowerCase(Locale.ROOT))).matcher(lower);if(m.find())try{q=Double.parseDouble(m.group(1));}catch(Exception ignored){}JSONObject existing=new JSONObject();try{existing.put("name",matchedName).put("quantity",q).put("unit",matched.optString("unit"));}catch(Exception ignored){}if(itemRows.size()==1&&itemRows.get(0).item.getText().toString().trim().isEmpty()){ItemRow r=itemRows.get(0);r.item.setText(matchedName,false);r.qty.setText(qty(q));r.unit.setText(matched.optString("unit"));}else if(voiceItemsBox!=null)addItemRow(voiceItemsBox,existing);}
  toast("Voice order captured • review before saving");
 }
 private void refreshBootstrapSilently(){''')

# Desktop loads the Operations Center extension.
replace('internal/app/web/admin.html','<script src="/assets/admin.js"></script><script src="/assets/orders.js"></script>','<script src="/assets/admin.js"></script><script src="/assets/orders.js"></script><script src="/assets/operations.js"></script>')

# Release workflow must refuse any key except the original September 28 permanent identity.
p=Path('.github/workflows/release.yml'); s=p.read_text(encoding='utf-8')
if 'EXPECTED_ANDROID_SHA1:' not in s:
    s=s.replace('      TB_KEY_PASSWORD: ${{ secrets.ANDROID_KEY_PASSWORD }}\n','      TB_KEY_PASSWORD: ${{ secrets.ANDROID_KEY_PASSWORD }}\n      EXPECTED_ANDROID_SHA1: 97:43:9E:2E:F7:40:C7:6C:0C:25:CD:AB:0F:AE:B1:B7:B2:83:AA:CE\n      EXPECTED_ANDROID_SHA256: 7F:17:BA:19:5D:05:7C:66:11:DE:AE:88:1D:85:B0:A0:D8:0D:BA:6A:44:EC:7C:21:43:40:05:3D:B4:D1:DB:AB\n')
    s=s.replace('          chmod 600 "$RUNNER_TEMP/tallybridge-release.jks"\n','          chmod 600 "$RUNNER_TEMP/tallybridge-release.jks"\n          ACTUAL_SHA1=$(keytool -list -v -keystore "$RUNNER_TEMP/tallybridge-release.jks" -storepass "$TB_STORE_PASSWORD" -alias "$TB_KEY_ALIAS" | sed -nE \'s/^[[:space:]]*SHA1:[[:space:]]*//p\' | head -1)\n          ACTUAL_SHA256=$(keytool -list -v -keystore "$RUNNER_TEMP/tallybridge-release.jks" -storepass "$TB_STORE_PASSWORD" -alias "$TB_KEY_ALIAS" | sed -nE \'s/^[[:space:]]*SHA256:[[:space:]]*//p\' | head -1)\n          test "$ACTUAL_SHA1" = "$EXPECTED_ANDROID_SHA1" || { echo "Wrong Android signing SHA-1"; exit 1; }\n          test "$ACTUAL_SHA256" = "$EXPECTED_ANDROID_SHA256" || { echo "Wrong Android signing SHA-256"; exit 1; }\n')
    p.write_text(s,encoding='utf-8')

print('operations 2.4.0 guarded patch applied')
