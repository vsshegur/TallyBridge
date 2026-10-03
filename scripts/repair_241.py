from pathlib import Path


def read(path):
    return Path(path).read_text(encoding="utf-8")


def write(path, text):
    Path(path).write_text(text, encoding="utf-8")


def one(path, old, new):
    text = read(path)
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{path}: expected 1 match, found {count}: {old[:120]!r}")
    write(path, text.replace(old, new, 1))


# Version.
one("internal/app/types.go", 'const Version = "2.4.0"', 'const Version = "2.4.1"')
one("android/app/build.gradle", "versionCode 20400", "versionCode 20401")
one("android/app/build.gradle", "versionName '2.4.0'", "versionName '2.4.1'")

# Never allow a CI production release to silently fall back to no/temporary signing.
one(
    "android/app/build.gradle",
    "def releaseSigningEnabled = System.getenv('TB_KEYSTORE') != null && !System.getenv('TB_KEYSTORE').isEmpty()\n",
    "def releaseSigningEnabled = System.getenv('TB_KEYSTORE') != null && !System.getenv('TB_KEYSTORE').isEmpty()\n"
    "def releaseTaskRequested = gradle.startParameter.taskNames.any { it.toLowerCase().contains('release') }\n"
    "if (System.getenv('CI') == 'true' && releaseTaskRequested && !releaseSigningEnabled) {\n"
    "    throw new GradleException('CI release builds require the permanent TallyBridge signing keystore')\n"
    "}\n",
)

# Tally: explicitly request TallyPrime's resolved item HSN method. AllInventoryEntries.*
# alone does not reliably emit computed methods such as GSTItemHSNCodeEx.
p = "internal/app/tally.go"
s = read(p)
old_fetch = "Amount,AllInventoryEntries.*,AllLedgerEntries.*,LedgerEntries.*</FETCH>"
new_fetch = "Amount,AllInventoryEntries.*,AllInventoryEntries.GSTItemHSNCodeEx,AllLedgerEntries.*,LedgerEntries.*</FETCH>"
if s.count(old_fetch) != 1:
    raise SystemExit("voucher detail FETCH anchor mismatch")
s = s.replace(old_fetch, new_fetch, 1)

marker = "func parseVoucherDetail(b []byte, company, masterID string, hint VoucherRow) VoucherDetail {"
if s.count(marker) != 1:
    raise SystemExit("parseVoucherDetail anchor mismatch")
helpers = r'''func cleanInvoiceAddressLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	seen := map[string]bool{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key := strings.ToLower(strings.Join(strings.Fields(line), " "))
		if !seen[key] {
			seen[key] = true
			out = append(out, line)
		}
	}
	return out
}

func customerAddressLines(r *xnode) []string {
	if r == nil {
		return nil
	}
	var lines []string
	for _, tag := range []string{"BASICBUYERADDRESS", "BUYERADDRESS", "CONSIGNEEADDRESS"} {
		lines = append(lines, allDeep(r, tag)...)
	}
	lines = cleanInvoiceAddressLines(lines)
	if len(lines) == 0 {
		lines = cleanInvoiceAddressLines(allDeep(r, "ADDRESS"))
	}
	return lines
}

func normalizeHSNValue(value string) string {
	value = strings.TrimSpace(stripTags(value))
	if value == "" {
		return ""
	}
	upper := strings.ToUpper(value)
	if upper == "NA" || upper == "N/A" || strings.Contains(upper, "NOT APPLICABLE") {
		return ""
	}
	var digits strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	code := digits.String()
	if len(code) >= 2 && len(code) <= 8 {
		return code
	}
	return ""
}

func inventoryHSN(n *xnode) string {
	if n == nil {
		return ""
	}
	// GSTItemHSNCodeEx is TallyPrime's resolved invoice-line method. It honors
	// Tally's configured GST hierarchy, so prefer it over raw master aliases.
	tags := []string{"GSTITEMHSNCODEEX", "GSTOVRDNHSNCODE", "GSTHSNCODE", "HSNCODE", "HSNSAC", "HSN", "SACCODE", "GSTHSNNAME"}
	for _, tag := range tags {
		if code := normalizeHSNValue(directChild(n, tag)); code != "" {
			return code
		}
	}
	for _, tag := range tags {
		if code := normalizeHSNValue(firstDeep(n, tag)); code != "" {
			return code
		}
	}
	raw := firstDeepTagMatch(n, func(tag string) bool {
		u := strings.ToUpper(tag)
		if !strings.Contains(u, "HSN") && !strings.Contains(u, "SAC") {
			return false
		}
		if strings.Contains(u, "APPLICABLE") || strings.Contains(u, "SOURCE") || strings.Contains(u, "DATE") || strings.Contains(u, "RATE") {
			return false
		}
		return strings.Contains(u, "CODE") || strings.HasSuffix(u, "HSN") || strings.HasSuffix(u, "SAC")
	})
	return normalizeHSNValue(raw)
}

'''
s = s.replace(marker, helpers + marker, 1)
old = 'x.CustomerAddress = allDeep(r, "ADDRESS")'
if s.count(old) != 1:
    raise SystemExit("customer address assignment mismatch")
s = s.replace(old, 'x.CustomerAddress = customerAddressLines(r)', 1)
old = 'HSN: firstDeep(n, "HSN", "HSNCODE", "HSNSAC")'
if s.count(old) != 1:
    raise SystemExit("inventory HSN assignment mismatch")
s = s.replace(old, 'HSN: inventoryHSN(n)', 1)
write(p, s)

# Regression coverage for resolved HSN + buyer address + invoice wording.
p = "internal/app/tally_test.go"
s = read(p)
s += r'''

func TestVoucherCollectionFetchesResolvedHSN(t *testing.T) {
	x := string(voucherDetailCollectionXML("Demo", "44"))
	if !strings.Contains(x, "AllInventoryEntries.GSTItemHSNCodeEx") {
		t.Fatalf("voucher detail request must fetch Tally resolved HSN method: %s", x)
	}
}

func TestVoucherDetailReadsResolvedHSNAndBuyerAddress(t *testing.T) {
	xmlb := []byte(`<VOUCHER><MASTERID>44</MASTERID><VOUCHERNUMBER>INV-44</VOUCHERNUMBER><DATE>20261003</DATE><BASICBUYERNAME>ABC Stores</BASICBUYERNAME><BASICBUYERCITY>Solapur</BASICBUYERCITY><BASICBUYERADDRESS.LIST><BASICBUYERADDRESS>12 Market Road</BASICBUYERADDRESS><BASICBUYERADDRESS>Solapur 413001</BASICBUYERADDRESS></BASICBUYERADDRESS.LIST><ALLINVENTORYENTRIES.LIST><STOCKITEMNAME>Bath Towel</STOCKITEMNAME><GSTITEMHSNCODEEX>63026090</GSTITEMHSNCODEEX><GSTHSNCODE>99999999</GSTHSNCODE><BILLEDQTY>2 pcs</BILLEDQTY><RATE>250.00/pcs</RATE><AMOUNT>-500.00</AMOUNT></ALLINVENTORYENTRIES.LIST><ALLLEDGERENTRIES.LIST><LEDGERNAME>ABC Stores</LEDGERNAME><AMOUNT>-500.00</AMOUNT></ALLLEDGERENTRIES.LIST></VOUCHER>`)
	x := parseVoucherDetail(xmlb, "VIPUL TEXTILES", "44", VoucherRow{MasterID: "44", Party: "ABC Stores", Amount: 500})
	if len(x.Items) != 1 || x.Items[0].HSN != "63026090" {
		t.Fatalf("resolved HSN did not win: %+v", x.Items)
	}
	joined := strings.Join(x.CustomerAddress, " | ")
	if !strings.Contains(joined, "12 Market Road") || !strings.Contains(joined, "Solapur 413001") {
		t.Fatalf("buyer address not recovered: %#v", x.CustomerAddress)
	}
	pdf := invoiceHTML(x)
	if strings.Contains(strings.ToLower(pdf), "read-only copy of data returned by tallyprime") {
		t.Fatal("old read-only invoice sentence must be removed")
	}
	for _, want := range []string{"BILL TO", "12 Market Road", "63026090", "INV-44"} {
		if !strings.Contains(pdf, want) {
			t.Fatalf("invoice missing %q", want)
		}
	}
}
'''
write(p, s)

# Invoice: customer-facing header/cards, address remains in Bill To, no internal
# read-only/freshness sentence, cleaner title.
p = "internal/app/pdf.go"
s = read(p)
css_anchor = '.narration{margin-top:4mm;white-space:pre-wrap}.meta{margin-bottom:4mm;font-size:8pt;color:#65758a}'
css_extra = css_anchor + '.invoice-hero{display:grid;grid-template-columns:1.15fr .85fr;gap:4mm;margin:1mm 0 4mm}.invoice-card{border:1px solid #dbe5ee;background:#f8fafc;padding:4mm;border-radius:2mm}.invoice-label{font-size:7.4pt;letter-spacing:.9pt;font-weight:700;color:#65798c;margin-bottom:1.5mm}.invoice-party{font-size:12.5pt;font-weight:700;color:#12324b;margin-bottom:1.5mm}.invoice-card p{margin:.7mm 0}.invoice-card .kv td{padding:.65mm 0}.invoice-card .kv td:last-child{color:#12324b}.invoice-card .kv tr:first-child td:last-child{font-size:13pt;font-weight:700}.dispatch-box{border:1px solid #e0e8ef;padding:3mm 4mm;margin:0 0 4mm;background:#fff}'
if s.count(css_anchor) != 1:
    raise SystemExit("invoice CSS anchor mismatch")
s = s.replace(css_anchor, css_extra, 1)
old = 'fmt.Fprintf(&b, `<div class="meta">%s</div><div class="grid"><section class="panel"><h3>Bill to</h3><p><b>%s</b></p>`, he(pdfDataNote(x.CommonData)), he(firstNonBlank(x.CustomerName, "Customer not returned")))'
new = 'fmt.Fprintf(&b, `<div class="invoice-hero"><section class="invoice-card"><div class="invoice-label">BILL TO</div><div class="invoice-party">%s</div>`, he(firstNonBlank(x.CustomerName, "Customer not returned")))'
if s.count(old) != 1:
    raise SystemExit("invoice opening markup mismatch")
s = s.replace(old, new, 1)
old = 'b.WriteString(`</section><section class="panel"><h3>Invoice details</h3><table class="kv">`)'
new = 'b.WriteString(`</section><section class="invoice-card"><div class="invoice-label">INVOICE DETAILS</div><table class="kv">`)'
if s.count(old) != 1:
    raise SystemExit("invoice details card mismatch")
s = s.replace(old, new, 1)
old = 'b.WriteString(`<h3>Dispatch details</h3><table class="kv">`)'
new = 'b.WriteString(`<section class="dispatch-box"><h3>Dispatch details</h3><table class="kv">`)'
if s.count(old) != 1:
    raise SystemExit("dispatch opening mismatch")
s = s.replace(old, new, 1)
old = 'b.WriteString(`</table>`)\n\t}\n\tb.WriteString(`<table class="items">'
new = 'b.WriteString(`</table></section>`)\n\t}\n\tb.WriteString(`<table class="items">'
if s.count(old) != 1:
    raise SystemExit("dispatch closing mismatch")
s = s.replace(old, new, 1)
old = 'title, subtitle := "INVOICE DETAIL", "Read-only copy of data returned by TallyPrime"\n\tif x.Optional {\n\t\ttitle, subtitle = "PROFORMA INVOICE", "Optional / non-posting voucher from TallyPrime"\n\t}'
new = 'title, subtitle := "INVOICE", ""\n\tif x.Optional {\n\t\ttitle, subtitle = "PROFORMA INVOICE", ""\n\t}'
if s.count(old) != 1:
    raise SystemExit("invoice title/subtitle mismatch")
s = s.replace(old, new, 1)
write(p, s)

# Android lifecycle: normal app switching should not clear screen/cache or force
# a fresh unlock/root navigation. Explicit Lock remains available.
p = "android/app/src/main/java/in/tallybridge/app/MainActivity.java"
s = read(p)
old = '@Override protected void onStop(){super.onStop();visible=false;if(!authenticating){unlocked=false;generation++;cache.clear();cacheAt.clear();} }'
if s.count(old) != 1:
    raise SystemExit("MainActivity onStop mismatch")
s = s.replace(old, '@Override protected void onStop(){super.onStop();visible=false;}', 1)
write(p, s)

one(
    "android/app/src/main/AndroidManifest.xml",
    'android:configChanges="orientation|screenSize|keyboardHidden"\n            android:windowSoftInputMode="adjustResize">',
    'android:configChanges="orientation|screenSize|keyboardHidden"\n            android:alwaysRetainTaskState="true" android:launchMode="singleTop"\n            android:windowSoftInputMode="adjustResize">',
)

# Responsive Orders widget. Compact height keeps both actions visible; taller
# sizes devote spare area to order names instead of blank space.
p = "android/app/src/main/java/in/tallybridge/app/OrdersWidget.java"
s = read(p)
start = s.index(" private static void paintCached(Context c,AppWidgetManager m,int id){")
end = s.index("\n private static void cache(", start)
paint = ''' private static void paintCached(Context c,AppWidgetManager m,int id){\n  android.content.SharedPreferences p=c.getSharedPreferences(PREFS,Context.MODE_PRIVATE);\n  RemoteViews v=new RemoteViews(c.getPackageName(),previewLayout(c));\n  int count=p.getInt("pending",0);String names=p.getString("names","");boolean offline=p.getBoolean("offline",false);\n  Bundle options=m.getAppWidgetOptions(id);int height=options==null?0:options.getInt(AppWidgetManager.OPTION_APPWIDGET_MIN_HEIGHT,0);boolean compact=height>0&&height<175;\n  v.setTextViewText(R.id.widget_pending,count+" pending order"+(count==1?"":"s"));\n  v.setTextViewText(R.id.widget_mode,offline?"OFFLINE • PHONE COPY":"SYNCED WITH PC");\n  v.setTextViewText(R.id.widget_names,names.isEmpty()?(count==0?"No pending orders. Add one when a customer calls.":"Tap Open pending to review orders."):names);\n  v.setViewVisibility(R.id.widget_mode,compact?android.view.View.GONE:android.view.View.VISIBLE);\n  v.setViewVisibility(R.id.widget_names,compact?android.view.View.GONE:android.view.View.VISIBLE);\n  v.setOnClickPendingIntent(R.id.widget_add,activity(c,OrdersActivity.ACTION_NEW,31));\n  v.setOnClickPendingIntent(R.id.widget_open,activity(c,null,32));\n  v.setOnClickPendingIntent(R.id.widget_refresh,refresh(c));\n  v.setOnClickPendingIntent(R.id.widget_names,activity(c,null,34));\n  v.setOnClickPendingIntent(R.id.widget_pending,activity(c,null,35));\n  m.updateAppWidget(id,v);\n }\n'''
s = s[:start] + paint + s[end:]
write(p, s)

for p in ["android/app/src/main/res/layout/orders_widget.xml", "android/app/src/main/res/layout/orders_widget_dark.xml"]:
    s = read(p)
    s = s.replace('android:textSize="9sp" android:textStyle="bold" android:letterSpacing="0.12"', 'android:textSize="10sp" android:textStyle="bold" android:letterSpacing="0.12"', 1)
    s = s.replace('android:textSize="17sp" android:textStyle="bold"', 'android:textSize="18sp" android:textStyle="bold"', 1)
    s = s.replace('android:textSize="20sp" android:textStyle="bold"', 'android:textSize="21sp" android:textStyle="bold"', 1)
    s = s.replace('android:textSize="9sp" android:textStyle="bold" />', 'android:textSize="10sp" android:textStyle="bold" />', 1)
    s = s.replace('android:id="@+id/widget_names" android:layout_width="match_parent" android:layout_height="wrap_content"', 'android:id="@+id/widget_names" android:layout_width="match_parent" android:layout_height="0dp" android:layout_weight="1" android:minHeight="26dp"', 1)
    s = s.replace('android:layout_marginTop="4dp" android:maxLines="1" android:ellipsize="end"', 'android:layout_marginTop="4dp" android:maxLines="3" android:ellipsize="end" android:gravity="center_vertical"', 1)
    s = s.replace('android:textSize="12sp" />', 'android:textSize="13sp" />', 1)
    s = s.replace('android:layout_height="42dp"', 'android:layout_height="46dp"')
    s = s.replace('android:textSize="12sp" android:textStyle="bold"', 'android:textSize="13sp" android:textStyle="bold"')
    write(p, s)

one("android/app/src/main/res/xml/orders_widget_info.xml", 'android:minHeight="120dp"', 'android:minHeight="170dp"')
one("android/app/src/main/res/xml/orders_widget_info.xml", 'android:minResizeWidth="180dp"', 'android:minResizeWidth="220dp"')
one("android/app/src/main/res/xml/orders_widget_info.xml", 'android:minResizeHeight="100dp"', 'android:minResizeHeight="150dp"')

# Release signing: fail before build if the supplied keystore is not the known
# permanent certificate, and fail again if the produced APK signer differs.
p = ".github/workflows/release.yml"
s = read(p)
anchor = "      - name: Test and build signed Android release\n"
if s.count(anchor) != 1:
    raise SystemExit("release build step anchor mismatch")
verify = '''      - name: Verify permanent Android signing identity\n        working-directory: android\n        shell: bash\n        run: |\n          EXPECTED_SHA1=$(sed -n 's/^SHA-1: //p' PERMANENT_SIGNING_IDENTITY.txt | tr -d '\\r')\n          EXPECTED_SHA256=$(sed -n 's/^SHA-256: //p' PERMANENT_SIGNING_IDENTITY.txt | tr -d '\\r')\n          KEYTOOL_OUT=$(keytool -list -v -keystore "$TB_KEYSTORE" -storepass "$TB_STORE_PASSWORD" -alias "$TB_KEY_ALIAS")\n          ACTUAL_SHA1=$(printf '%s\\n' "$KEYTOOL_OUT" | sed -n 's/^[[:space:]]*SHA1: //p' | head -1 | tr -d '\\r')\n          ACTUAL_SHA256=$(printf '%s\\n' "$KEYTOOL_OUT" | sed -n 's/^[[:space:]]*SHA256: //p' | head -1 | tr -d '\\r')\n          test "$ACTUAL_SHA1" = "$EXPECTED_SHA1" || { echo "Wrong permanent SHA-1: $ACTUAL_SHA1"; exit 1; }\n          test "$ACTUAL_SHA256" = "$EXPECTED_SHA256" || { echo "Wrong permanent SHA-256: $ACTUAL_SHA256"; exit 1; }\n'''
s = s.replace(anchor, verify + anchor, 1)
apk_anchor = '          grep -q "Verified using v2 scheme (APK Signature Scheme v2): true" app/build/outputs/apk/release/TallyBridge-Android-APK-SIGNATURE.txt\n'
if s.count(apk_anchor) != 1:
    raise SystemExit("release APK verify anchor mismatch")
apk_verify = apk_anchor + '''          EXPECTED_APK_SHA1=$(sed -n 's/^SHA-1: //p' PERMANENT_SIGNING_IDENTITY.txt | tr -d ':' | tr '[:upper:]' '[:lower:]')\n          EXPECTED_APK_SHA256=$(sed -n 's/^SHA-256: //p' PERMANENT_SIGNING_IDENTITY.txt | tr -d ':' | tr '[:upper:]' '[:lower:]')\n          ACTUAL_APK_SHA1=$(sed -n 's/^Signer #1 certificate SHA-1 digest: //p' app/build/outputs/apk/release/TallyBridge-Android-APK-SIGNATURE.txt | head -1 | tr '[:upper:]' '[:lower:]')\n          ACTUAL_APK_SHA256=$(sed -n 's/^Signer #1 certificate SHA-256 digest: //p' app/build/outputs/apk/release/TallyBridge-Android-APK-SIGNATURE.txt | head -1 | tr '[:upper:]' '[:lower:]')\n          test "$ACTUAL_APK_SHA1" = "$EXPECTED_APK_SHA1" || { echo "Final APK SHA-1 mismatch"; exit 1; }\n          test "$ACTUAL_APK_SHA256" = "$EXPECTED_APK_SHA256" || { echo "Final APK SHA-256 mismatch"; exit 1; }\n'''
s = s.replace(apk_anchor, apk_verify, 1)
write(p, s)
