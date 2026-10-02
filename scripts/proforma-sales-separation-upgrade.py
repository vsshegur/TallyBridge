from pathlib import Path


def replace_once(path, old, new):
    p = Path(path)
    text = p.read_text(encoding="utf-8")
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{path}: expected exactly 1 match, found {count}: {old[:120]!r}")
    p.write_text(text.replace(old, new, 1), encoding="utf-8")


# Backend/app version alignment.
replace_once("internal/app/types.go", 'const Version = "2.3.2"', 'const Version = "2.3.3"')
replace_once(
    "android/app/build.gradle",
    "        versionCode 20302\n        versionName '2.3.2'",
    "        versionCode 20303\n        versionName '2.3.3'",
)

# Preserve Tally's Optional flag on register rows.
replace_once(
    "internal/app/types.go",
    'VoucherType string   `json:"voucherType,omitempty"`\n}',
    'VoucherType string   `json:"voucherType,omitempty"`\n\tOptional    bool     `json:"optional,omitempty"`\n}',
)
replace_once(
    "internal/app/tally.go",
    '<NATIVEMETHOD>VoucherTypeName</NATIVEMETHOD><NATIVEMETHOD>Amount</NATIVEMETHOD>',
    '<NATIVEMETHOD>VoucherTypeName</NATIVEMETHOD><NATIVEMETHOD>IsOptional</NATIVEMETHOD><NATIVEMETHOD>Amount</NATIVEMETHOD>',
)
replace_once(
    "internal/app/tally.go",
    'out = append(out, VoucherRow{MasterID: mid, Date: date, Number: num, Party: party, Amount: abs(amount), VoucherType: firstDeep(n, "VOUCHERTYPENAME")})',
    'out = append(out, VoucherRow{MasterID: mid, Date: date, Number: num, Party: party, Amount: abs(amount), VoucherType: firstDeep(n, "VOUCHERTYPENAME"), Optional: tallyYes(firstDeep(n, "ISOPTIONAL", "OPTIONAL"))})',
)

# Accept a dedicated proforma register and split Optional Sales away from posted Sales.
replace_once(
    "internal/app/server.go",
    'if typ != "purchase" {\n\t\ttyp = "sales"\n\t}',
    'if typ != "purchase" && typ != "proforma" {\n\t\ttyp = "sales"\n\t}',
)
old_success = '''\t\tif e == nil {\n\t\t\tvs := parseVouchers(b)\n\t\t\ttotal := 0.0\n\t\t\tfor _, v := range vs {\n\t\t\t\ttotal += abs(v.Amount)\n\t\t\t}\n\t\t\tx := VoucherData{CommonData: CommonData{Live: true, Source: "live", FetchedAt: time.Now(), DurationMS: d}, Vouchers: vs, Total: total, Count: len(vs), Type: typ, Period: period}\n\t\t\t_ = s.cache.Put(kind, c, variant, x, x.FetchedAt)\n\t\t\treturn x, nil\n\t\t} else {'''
new_success = '''\t\tif e == nil {\n\t\t\tall := parseVouchers(b)\n\t\t\tat := time.Now()\n\t\t\tx := voucherDataFromRows(voucherRowsForRegister(all, typ), typ, period, d, at)\n\t\t\t_ = s.cache.Put(kind, c, variant, x, x.FetchedAt)\n\t\t\t// A Sales read already contains both posted and Optional vouchers. Cache the\n\t\t\t// sibling register too so Sales/Proforma stay separated without a second Tally read.\n\t\t\tif typ == "sales" || typ == "proforma" {\n\t\t\t\tsibling := "proforma"\n\t\t\t\tif typ == "proforma" { sibling = "sales" }\n\t\t\t\ty := voucherDataFromRows(voucherRowsForRegister(all, sibling), sibling, period, d, at)\n\t\t\t\t_ = s.cache.Put(sibling, c, variant, y, y.FetchedAt)\n\t\t\t}\n\t\t\treturn x, nil\n\t\t} else {'''
replace_once("internal/app/server.go", old_success, new_success)
replace_once(
    "internal/app/server.go",
    'for _, typ := range []string{"sales", "purchase"} {',
    'for _, typ := range []string{"sales", "proforma", "purchase"} {',
)

# Exported invoice PDFs begin with invoice number and include party name; never prefix "Invoice-".
replace_once(
    "internal/app/server.go",
    'name := sanitizeFileName(firstNonBlank(x.InvoiceNumber, "Invoice"))\n\tw.Header().Set("Content-Type", "application/pdf")\n\tw.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="Invoice-%s.pdf"`, name))',
    'name := invoicePDFBaseName(x)\n\tw.Header().Set("Content-Type", "application/pdf")\n\tw.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.pdf"`, name))',
)

# Android: separate Tally Optional/Proforma register and expose it both in Reports and Proforma tab.
replace_once(
    "android/app/src/main/java/in/tallybridge/app/MainActivity.java",
    '  data.add(row("Sales register","Financial year → active months → invoice","",()->open(()->register("sales"))));\n  data.add(row("Purchase register","Financial year → active months → voucher","",()->open(()->register("purchase"))));',
    '  data.add(row("Sales register","Posted sales invoices only","",()->open(()->register("sales"))));\n  data.add(row("Proforma invoices","Tally Optional sales • separate from Sales Register","",()->open(()->register("proforma"))));\n  data.add(row("Purchase register","Financial year → active months → voucher","",()->open(()->register("purchase"))));',
)
replace_once(
    "android/app/src/main/java/in/tallybridge/app/MainActivity.java",
    'private void register(String kind){String title=kind.equals("purchase")?"Purchase register":"Sales register";',
    'private void register(String kind){String title=kind.equals("purchase")?"Purchase register":kind.equals("proforma")?"Proforma invoices":"Sales register";',
)
replace_once(
    "android/app/src/main/java/in/tallybridge/app/MainActivity.java",
    'proformaDraft.render(body);body.addView(button("Discard draft",',
    'proformaDraft.render(body);body.addView(button("View Tally proforma invoices",()->open(()->register("proforma"))));body.addView(button("Discard draft",',
)
replace_once(
    "android/app/src/main/java/in/tallybridge/app/MainActivity.java",
    'shareRowsDual(data,x.optString("shareText"),x.optString("shareTextWithBank"),pdfPath,"Invoice-"+x.optString("invoiceNumber"));',
    'shareRowsDual(data,x.optString("shareText"),x.optString("shareTextWithBank"),pdfPath,x.optString("invoiceNumber")+" "+x.optString("customerName"));',
)

# Make Optional rows visibly proforma even when reached from search or another list.
replace_once(
    "android/app/src/main/java/in/tallybridge/app/MainActivity.java",
    'v.optString("date")+" • "+v.optString("voucherType"),money(v.optDouble("amount"))',
    'v.optString("date")+" • "+v.optString("voucherType")+(v.optBoolean("optional")?" • Optional / Proforma":""),money(v.optDouble("amount"))',
)

# New backend helper keeps classification and filename rules testable and centralized.
Path("internal/app/voucher_classification.go").write_text(r'''package app

import (
    "strings"
    "time"
)

func tallyYes(v string) bool {
    switch strings.ToLower(strings.TrimSpace(v)) {
    case "yes", "true", "1":
        return true
    default:
        return false
    }
}

// Optional vouchers are non-posting Tally vouchers. Optional Sales are shown as
// Proforma invoices and are never included in posted Sales/Purchase registers.
func voucherRowsForRegister(rows []VoucherRow, typ string) []VoucherRow {
    out := make([]VoucherRow, 0, len(rows))
    for _, v := range rows {
        if typ == "proforma" {
            if v.Optional {
                out = append(out, v)
            }
            continue
        }
        if v.Optional {
            continue
        }
        out = append(out, v)
    }
    return out
}

func voucherDataFromRows(rows []VoucherRow, typ, period string, duration int64, fetchedAt time.Time) VoucherData {
    total := 0.0
    for _, v := range rows {
        total += abs(v.Amount)
    }
    return VoucherData{
        CommonData: CommonData{Live: true, Source: "live", FetchedAt: fetchedAt, DurationMS: duration},
        Vouchers: rows,
        Total: total,
        Count: len(rows),
        Type: typ,
        Period: period,
    }
}

func invoicePDFBaseName(x VoucherDetail) string {
    number := sanitizeFileName(firstNonBlank(x.InvoiceNumber, "Voucher"))
    if party := strings.TrimSpace(x.CustomerName); party != "" {
        return number + "-" + sanitizeFileName(party)
    }
    return number
}
''', encoding="utf-8")

Path("internal/app/proforma_register_test.go").write_text(r'''package app

import (
    "strings"
    "testing"
    "time"
)

func TestVoucherListRequestsOptionalFlag(t *testing.T) {
    xml := string(voucherListXML("Demo Co", "Sales", "20260401", "20270331"))
    if !strings.Contains(xml, "<NATIVEMETHOD>IsOptional</NATIVEMETHOD>") {
        t.Fatal("Sales register XML must request IsOptional so posted and proforma vouchers can be separated")
    }
}

func TestOptionalSalesAreSeparatedFromPostedSales(t *testing.T) {
    raw := []byte(`<ENVELOPE><BODY><DATA><COLLECTION>
<VOUCHER><MASTERID>1</MASTERID><DATE>20261001</DATE><VOUCHERNUMBER>S-1</VOUCHERNUMBER><PARTYLEDGERNAME>Posted Buyer</PARTYLEDGERNAME><VOUCHERTYPENAME>Sales</VOUCHERTYPENAME><ISOPTIONAL>No</ISOPTIONAL><AMOUNT>-100</AMOUNT></VOUCHER>
<VOUCHER><MASTERID>2</MASTERID><DATE>20261002</DATE><VOUCHERNUMBER>PF-1</VOUCHERNUMBER><PARTYLEDGERNAME>Proforma Buyer</PARTYLEDGERNAME><VOUCHERTYPENAME>Sales</VOUCHERTYPENAME><ISOPTIONAL>Yes</ISOPTIONAL><AMOUNT>-200</AMOUNT></VOUCHER>
</COLLECTION></DATA></BODY></ENVELOPE>`)
    rows := parseVouchers(raw)
    if len(rows) != 2 {
        t.Fatalf("expected 2 vouchers, got %d", len(rows))
    }
    if rows[0].Optional || !rows[1].Optional {
        t.Fatalf("optional flags were not parsed correctly: %#v", rows)
    }
    sales := voucherRowsForRegister(rows, "sales")
    proforma := voucherRowsForRegister(rows, "proforma")
    if len(sales) != 1 || sales[0].Number != "S-1" {
        t.Fatalf("posted Sales register contains wrong rows: %#v", sales)
    }
    if len(proforma) != 1 || proforma[0].Number != "PF-1" {
        t.Fatalf("Proforma register contains wrong rows: %#v", proforma)
    }
    purchase := voucherRowsForRegister(rows, "purchase")
    if len(purchase) != 1 || purchase[0].Optional {
        t.Fatalf("optional vouchers must not leak into posted registers: %#v", purchase)
    }
}

func TestVoucherDataTotalsOnlyItsOwnRegister(t *testing.T) {
    hundred, twoHundred := 100.0, 200.0
    rows := []VoucherRow{{Number: "S-1", Amount: hundred}, {Number: "PF-1", Amount: twoHundred, Optional: true}}
    at := time.Now()
    sales := voucherDataFromRows(voucherRowsForRegister(rows, "sales"), "sales", "fy", 12, at)
    proforma := voucherDataFromRows(voucherRowsForRegister(rows, "proforma"), "proforma", "fy", 12, at)
    if sales.Count != 1 || sales.Total != hundred {
        t.Fatalf("sales total includes optional voucher: %#v", sales)
    }
    if proforma.Count != 1 || proforma.Total != twoHundred {
        t.Fatalf("proforma total wrong: %#v", proforma)
    }
}

func TestInvoicePDFNameStartsWithNumberAndParty(t *testing.T) {
    got := invoicePDFBaseName(VoucherDetail{InvoiceNumber: "12345", CustomerName: "ABC Traders"})
    if got != "12345-ABC-Traders" {
        t.Fatalf("unexpected invoice PDF base name %q", got)
    }
    if strings.HasPrefix(strings.ToLower(got), "invoice") {
        t.Fatalf("invoice PDF name must not start with Invoice: %q", got)
    }
}
''', encoding="utf-8")
