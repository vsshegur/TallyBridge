from pathlib import Path

src = Path("scripts/run-proforma-sales-separation-upgrade.py").read_text(encoding="utf-8")
old = '''replace_in_section(\n    "internal/app/server.go", "func (s *Server) handleSearchIndex", "func parseISOOrZero",\n    'VoucherType: v.VoucherType})',\n    'VoucherType: v.VoucherType, Optional: v.Optional})',\n)'''
new = '''replace_in_section(\n    "internal/app/server.go", "func (s *Server) handleSearchIndex", "func parseISOOrZero",\n    'out = append(out, SearchResult{Kind: typ, Title: firstNonBlank(v.Number, v.Party), Subtitle: v.Party + " • " + displayDate(v.Date), MasterID: v.MasterID, Party: v.Party, Number: v.Number, Date: v.Date, Amount: v.Amount, VoucherType: v.VoucherType})',\n    'out = append(out, SearchResult{Kind: typ, Title: firstNonBlank(v.Number, v.Party), Subtitle: v.Party + " • " + displayDate(v.Date), MasterID: v.MasterID, Party: v.Party, Number: v.Number, Date: v.Date, Amount: v.Amount, VoucherType: v.VoucherType, Optional: v.Optional})',\n)'''
if src.count(old) != 1:
    raise SystemExit(f"expected one ambiguous search-result patch block, found {src.count(old)}")
exec(compile(src.replace(old, new, 1), "run-proforma-sales-separation-upgrade.py", "exec"), {"__name__": "__main__"})
