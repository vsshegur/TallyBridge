from pathlib import Path


def replace_once(path, old, new):
    p = Path(path)
    text = p.read_text(encoding="utf-8")
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{path}: expected exactly 1 match, found {count}: {old[:120]!r}")
    p.write_text(text.replace(old, new, 1), encoding="utf-8")


def replace_in_section(path, start_marker, end_marker, old, new):
    p = Path(path)
    text = p.read_text(encoding="utf-8")
    start = text.index(start_marker)
    end = text.index(end_marker, start)
    section = text[start:end]
    count = section.count(old)
    if count != 1:
        raise SystemExit(f"{path} section {start_marker}: expected exactly 1 match, found {count}: {old[:120]!r}")
    section = section.replace(old, new, 1)
    p.write_text(text[:start] + section + text[end:], encoding="utf-8")


# The base guarded script originally matched two XML builders. Narrow that one edit
# to voucherListXML before executing the rest of its guarded transformations.
base = Path("scripts/proforma-sales-separation-upgrade.py").read_text(encoding="utf-8")
ambiguous = '''replace_once(\n    "internal/app/tally.go",\n    '<NATIVEMETHOD>VoucherTypeName</NATIVEMETHOD><NATIVEMETHOD>Amount</NATIVEMETHOD>',\n    '<NATIVEMETHOD>VoucherTypeName</NATIVEMETHOD><NATIVEMETHOD>IsOptional</NATIVEMETHOD><NATIVEMETHOD>Amount</NATIVEMETHOD>',\n)'''
scoped = '''p = Path("internal/app/tally.go")\ntext = p.read_text(encoding="utf-8")\nstart = text.index("func voucherListXML")\nend = text.index("func voucherDetailCollectionXML", start)\nsection = text[start:end]\nold = '<NATIVEMETHOD>VoucherTypeName</NATIVEMETHOD><NATIVEMETHOD>Amount</NATIVEMETHOD>'\nnew = '<NATIVEMETHOD>VoucherTypeName</NATIVEMETHOD><NATIVEMETHOD>IsOptional</NATIVEMETHOD><NATIVEMETHOD>Amount</NATIVEMETHOD>'\nif section.count(old) != 1:\n    raise SystemExit(f"voucherListXML: expected 1 VoucherTypeName/Amount pair, found {section.count(old)}")\np.write_text(text[:start] + section.replace(old, new, 1) + text[end:], encoding="utf-8")'''
if base.count(ambiguous) != 1:
    raise SystemExit("Could not locate the ambiguous voucherListXML transformation")
exec(compile(base.replace(ambiguous, scoped, 1), "proforma-sales-separation-upgrade.py", "exec"), {"__name__": "__main__"})

# Cache schema v1 ensures pre-upgrade registers that mixed Optional vouchers are
# never shown as correctly classified data while the PC/Tally is unavailable.
replace_once(
    "internal/app/types.go",
    'Period   string       `json:"period,omitempty"`\n}',
    'Period   string       `json:"period,omitempty"`\n\tClassificationVersion int `json:"classificationVersion,omitempty"`\n}',
)
replace_once(
    "internal/app/voucher_classification.go",
    'func tallyYes(v string) bool {',
    'const voucherClassificationVersion = 1\n\nfunc tallyYes(v string) bool {',
)
replace_once(
    "internal/app/voucher_classification.go",
    'Period: period,\n    }',
    'Period: period,\n        ClassificationVersion: voucherClassificationVersion,\n    }',
)
replace_once(
    "internal/app/voucher_classification.go",
    'func invoicePDFBaseName(x VoucherDetail) string {',
    'func voucherCacheIsClassified(x VoucherData) bool {\n    return x.ClassificationVersion == voucherClassificationVersion\n}\n\nfunc invoicePDFBaseName(x VoucherDetail) string {',
)

# Old register caches are deliberately ignored until a new classified read succeeds.
replace_in_section(
    "internal/app/server.go", "func (s *Server) getVouchers", "func (s *Server) fetchVoucherDetail",
    'if at, ok := s.cache.Get(kind, c, variant, &old); ok {',
    'if at, ok := s.cache.Get(kind, c, variant, &old); ok && voucherCacheIsClassified(old) {',
)
replace_in_section(
    "internal/app/server.go", "func (s *Server) getVouchers", "func (s *Server) fetchVoucherDetail",
    'if at, ok := s.cache.Get(kind, c, variant, &x); ok {',
    'if at, ok := s.cache.Get(kind, c, variant, &x); ok && voucherCacheIsClassified(x) {',
)
replace_in_section(
    "internal/app/server.go", "func (s *Server) summaries", "func (s *Server) Settings",
    'if at, ok := s.cache.Get("sales", name, "month", &v); ok {',
    'if at, ok := s.cache.Get("sales", name, "month", &v); ok && voucherCacheIsClassified(v) {',
)
replace_in_section(
    "internal/app/server.go", "func (s *Server) handleSearchIndex", "func parseISOOrZero",
    'if at, ok := s.cache.Get(typ, c, period, &vd); ok {',
    'if at, ok := s.cache.Get(typ, c, period, &vd); ok && voucherCacheIsClassified(vd) {',
)
replace_in_section(
    "internal/app/server.go", "func (s *Server) buildCustomer360", "func (s *Server) handleCustomer360",
    'if at, ok := s.cache.Get("sales", c, "fy", &sales); ok {',
    'if at, ok := s.cache.Get("sales", c, "fy", &sales); ok && voucherCacheIsClassified(sales) {',
)

# Carry Optional state into invoice detail, PDF, search results, and Android drill-down.
replace_once(
    "internal/app/types.go",
    'VoucherType       string        `json:"voucherType"`\n\tCustomerName',
    'VoucherType       string        `json:"voucherType"`\n\tOptional          bool          `json:"optional,omitempty"`\n\tCustomerName',
)
replace_once(
    "internal/app/types.go",
    'VoucherType string  `json:"voucherType,omitempty"`\n}',
    'VoucherType string  `json:"voucherType,omitempty"`\n\tOptional    bool    `json:"optional,omitempty"`\n}',
)
replace_in_section(
    "internal/app/tally.go", "func voucherDetailCollectionXML", "func voucherDetailObjectXML",
    'MasterID,Date,VoucherNumber,VoucherTypeName,PartyLedgerName',
    'MasterID,Date,VoucherNumber,VoucherTypeName,IsOptional,PartyLedgerName',
)
replace_in_section(
    "internal/app/tally.go", "func voucherDayXML", "func ledgerDetailXML",
    'MasterID,Date,VoucherNumber,VoucherTypeName,PartyLedgerName',
    'MasterID,Date,VoucherNumber,VoucherTypeName,IsOptional,PartyLedgerName',
)
replace_in_section(
    "internal/app/tally.go", "func parseVoucherDetailNode", "func parseVoucherDayDetail",
    'x.VoucherType = firstDeep(r, "VOUCHERTYPENAME")',
    'x.VoucherType = firstDeep(r, "VOUCHERTYPENAME")\n\tx.Optional = hint.Optional || tallyYes(firstDeep(r, "ISOPTIONAL", "OPTIONAL"))',
)
replace_in_section(
    "internal/app/server.go", "func mergeVoucherDetail", "func sanitizeFileName",
    'if a.VoucherType == "" {\n\t\ta.VoucherType = b.VoucherType\n\t}',
    'if a.VoucherType == "" {\n\t\ta.VoucherType = b.VoucherType\n\t}\n\tif b.Optional { a.Optional = true }',
)
replace_in_section(
    "internal/app/server.go", "func voucherRequestHint", "func voucherCacheKey",
    'VoucherType: strings.TrimSpace(r.URL.Query().Get("voucherType")),',
    'VoucherType: strings.TrimSpace(r.URL.Query().Get("voucherType")),\n\t\tOptional: strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("optional")), "true") || strings.TrimSpace(r.URL.Query().Get("optional")) == "1",',
)
replace_in_section(
    "internal/app/server.go", "func (s *Server) handleSearchIndex", "func parseISOOrZero",
    'VoucherType: v.VoucherType})',
    'VoucherType: v.VoucherType, Optional: v.Optional})',
)

# Make an opened Optional voucher clearly Proforma and preserve that flag on all API calls.
replace_once(
    "android/app/src/main/java/in/tallybridge/app/MainActivity.java",
    'String id=hint.optString("masterId"),number=hint.optString("number"),date=hint.optString("date"),party=hint.optString("party"),amount=hint.optString("amount"),voucherType=hint.optString("voucherType");',
    'String id=hint.optString("masterId"),number=hint.optString("number"),date=hint.optString("date"),party=hint.optString("party"),amount=hint.optString("amount"),voucherType=hint.optString("voucherType"); boolean proformaHint=hint.optBoolean("optional")||hint.optString("kind").equals("proforma");',
)
replace_once(
    "android/app/src/main/java/in/tallybridge/app/MainActivity.java",
    'String detailPath=path("voucher-detail","masterId",id,"number",number,"date",date,"party",party,"amount",amount,"voucherType",voucherType);\n  load("Invoice "+number,detailPath,',
    'String detailPath=path("voucher-detail","masterId",id,"number",number,"date",date,"party",party,"amount",amount,"voucherType",voucherType,"optional",Boolean.toString(proformaHint));\n  load((proformaHint?"Proforma ":"Invoice ")+number,detailPath,',
)
replace_once(
    "android/app/src/main/java/in/tallybridge/app/MainActivity.java",
    'String pdfPath=path("voucher-pdf","masterId",x.optString("masterId",id),"number",x.optString("invoiceNumber",number),"date",x.optString("invoiceDate",date),"party",x.optString("customerName",party),"amount",amount,"voucherType",x.optString("voucherType",voucherType));',
    'String pdfPath=path("voucher-pdf","masterId",x.optString("masterId",id),"number",x.optString("invoiceNumber",number),"date",x.optString("invoiceDate",date),"party",x.optString("customerName",party),"amount",amount,"voucherType",x.optString("voucherType",voucherType),"optional",Boolean.toString(x.optBoolean("optional",proformaHint)));',
)
replace_once(
    "android/app/src/main/java/in/tallybridge/app/MainActivity.java",
    'rows("Invoice "+x.optString("invoiceNumber"),data,false);',
    'rows((x.optBoolean("optional",proformaHint)?"Proforma ":"Invoice ")+x.optString("invoiceNumber"),data,false);',
)
replace_once(
    "android/app/src/main/java/in/tallybridge/app/MainActivity.java",
    'paymentQR("Invoice "+x.optString("invoiceNumber"),x.optDouble("totalAmount"))',
    'paymentQR((x.optBoolean("optional",proformaHint)?"Proforma ":"Invoice ")+x.optString("invoiceNumber"),x.optDouble("totalAmount"))',
)

# Proforma PDF/document text should never present an Optional voucher as a posted invoice.
replace_once(
    "internal/app/pdf.go",
    'return pdfDocument(firstNonBlank(x.FirmName, x.Company), "INVOICE DETAIL", "Read-only copy of data returned by TallyPrime", b.String(), x.Supplier)',
    'title, subtitle := "INVOICE DETAIL", "Read-only copy of data returned by TallyPrime"\n\tif x.Optional { title, subtitle = "PROFORMA INVOICE", "Optional / non-posting voucher from TallyPrime" }\n\treturn pdfDocument(firstNonBlank(x.FirmName, x.Company), title, subtitle, b.String(), x.Supplier)',
)
replace_in_section(
    "internal/app/tally.go", "func dispatchShareText", "func ledgerStatementShareText",
    'fmt.Fprintf(&b, "Dispatch Details\\n1. Firm Name - %s',
    'heading := "Dispatch Details"\n\tif x.Optional { heading = "Proforma Details" }\n\tfmt.Fprintf(&b, heading+"\\n1. Firm Name - %s',
)

# Extend regression coverage for stale caches and proforma document identity.
test = Path("internal/app/proforma_register_test.go")
test.write_text(test.read_text(encoding="utf-8") + r'''

func TestClassifiedCacheVersionRequired(t *testing.T) {
    if voucherCacheIsClassified(VoucherData{}) {
        t.Fatal("pre-upgrade mixed register cache must not be trusted")
    }
    x := voucherDataFromRows([]VoucherRow{{Number: "S-1", Amount: 10}}, "sales", "month", 1, time.Now())
    if !voucherCacheIsClassified(x) {
        t.Fatal("newly separated register cache must be classified")
    }
}

func TestProformaInvoiceDocumentIdentity(t *testing.T) {
    html := invoiceHTML(VoucherDetail{Company: "Demo", FirmName: "Demo", InvoiceNumber: "PF-1", CustomerName: "Buyer", Optional: true})
    if !strings.Contains(html, "PROFORMA INVOICE") || !strings.Contains(html, "Optional / non-posting") {
        t.Fatalf("optional voucher PDF is not visibly proforma: %s", html)
    }
    share := dispatchShareText(VoucherDetail{Optional: true, FirmName: "Demo", InvoiceNumber: "PF-1"})
    if !strings.HasPrefix(share, "Proforma Details") {
        t.Fatalf("optional voucher share text is not proforma: %q", share)
    }
}
''', encoding="utf-8")
