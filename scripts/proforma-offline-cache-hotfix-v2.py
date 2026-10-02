from pathlib import Path

src = Path("scripts/proforma-offline-cache-hotfix.py").read_text(encoding="utf-8")
old = 'if _, ok := c.Get(kind, company, period, &vd); !ok || !voucherCacheIsClassified(vd) {'
new = 'if _, ok := c.Get(kind, company, period, &vd); !ok || (kind != "purchase" && !voucherCacheIsClassified(vd)) {'
if src.count(old) != 1:
    raise SystemExit(f"expected one VoucherHint classification guard, found {src.count(old)}")
exec(compile(src.replace(old, new, 1), "proforma-offline-cache-hotfix.py", "exec"), {"__name__": "__main__"})
