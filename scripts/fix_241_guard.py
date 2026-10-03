from pathlib import Path

p = Path('scripts/apply_241_core.py')
s = p.read_text()

# Align the guarded invoice opening block with the exact multiline 2.4.0 source.
start = s.index("old='''func invoiceHTML")
end = s.index("new='''func invoiceHTML", start)
exact = """old='''func invoiceHTML(x VoucherDetail, bank ...BankDetails) string {\n\tvar b strings.Builder\n\tfmt.Fprintf(&b, `<div class=\"meta\">%s</div><div class=\"grid\"><section class=\"panel\"><h3>Bill to</h3><p><b>%s</b></p>`, he(pdfDataNote(x.CommonData)), he(firstNonBlank(x.CustomerName, \"Customer not returned\")))\n\tfor _, a := range x.CustomerAddress {\n\t\tif a != \"\" {\n\t\t\tfmt.Fprintf(&b, \"<p>%s</p>\", he(a))\n\t\t}\n\t}\n\tif x.CustomerCity != \"\" {\n\t\tfmt.Fprintf(&b, \"<p>%s</p>\", he(x.CustomerCity))\n\t}\n\tif x.GSTIN != \"\" {\n\t\tfmt.Fprintf(&b, `<p><span class=\"label\">GSTIN</span> %s</p>`, he(x.GSTIN))\n\t}\n\tb.WriteString(`</section><section class=\"panel\"><h3>Invoice details</h3><table class=\"kv\">`)'''\n"""
s = s[:start] + exact + s[end:]

# apply_241_core.py stores the replacement Go block with literal backslash
# escapes, so match that source representation rather than interpreted tabs.
old_marker = r'''\ttitle := "TAX INVOICE"\n\tif x.Optional { title = "PROFORMA INVOICE" }\n\tsubtitleParts:=[]string{}'''
new_marker = r'''\ttitle := "TAX INVOICE"\n\tsubtitleParts:=[]string{}\n\tif x.Optional { title = "PROFORMA INVOICE"; subtitleParts=append(subtitleParts,"Optional / non-posting") }'''
if s.count(old_marker) != 1:
    raise SystemExit(f'proforma identity marker count {s.count(old_marker)}')
s = s.replace(old_marker, new_marker, 1)

p.write_text(s)
print('guard aligned; proforma identity preserved')
