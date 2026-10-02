from pathlib import Path


def replace_once(path, old, new):
    p = Path(path)
    text = p.read_text(encoding="utf-8")
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{path}: expected exactly 1 match, found {count}: {old[:140]!r}")
    p.write_text(text.replace(old, new, 1), encoding="utf-8")


# VoucherHint must prefer the dedicated Proforma register and reject pre-2.3.3
# mixed caches. This lets offline/search detail flows recover Optional identity.
replace_once(
    "internal/app/cache.go",
    '''func (c *CacheStore) VoucherHint(company, masterID string) (VoucherRow, bool) {
	var vd VoucherData
	for _, kind := range []string{"sales", "purchase"} {
		for _, period := range []string{"fy", "month"} {
			if _, ok := c.Get(kind, company, period, &vd); !ok {
				continue
			}
			for _, v := range vd.Vouchers {
				if v.MasterID == masterID {
					return v, true
				}
			}
		}
	}
	return VoucherRow{}, false
}''',
    '''func (c *CacheStore) VoucherHint(company, masterID string) (VoucherRow, bool) {
	var vd VoucherData
	for _, kind := range []string{"proforma", "sales", "purchase"} {
		for _, period := range []string{"fy", "month"} {
			if _, ok := c.Get(kind, company, period, &vd); !ok || !voucherCacheIsClassified(vd) {
				continue
			}
			for _, v := range vd.Vouchers {
				if v.MasterID == masterID {
					return v, true
				}
			}
		}
	}
	return VoucherRow{}, false
}''',
)

# Cached details/PDFs must inherit Optional identity from the current register hint.
replace_once(
    "internal/app/voucher_classification.go",
    '''func voucherCacheIsClassified(x VoucherData) bool {
	return x.ClassificationVersion == voucherClassificationVersion
}

func invoicePDFBaseName''',
    '''func voucherCacheIsClassified(x VoucherData) bool {
	return x.ClassificationVersion == voucherClassificationVersion
}

func preserveOptionalHint(x *VoucherDetail, hint VoucherRow) {
	if x != nil && hint.Optional {
		x.Optional = true
	}
}

func invoicePDFBaseName''',
)

# Two offline detail fallback paths.
needle = '''		if at, ok := s.cache.Get("voucher-detail-v6", c, key, &old); ok && old.TotalsVersion == 2 {
'''
p = Path("internal/app/server.go")
text = p.read_text(encoding="utf-8")
if text.count(needle) != 2:
    raise SystemExit(f"server.go: expected 2 cached voucher-detail fallbacks, found {text.count(needle)}")
text = text.replace(needle, needle + '''			preserveOptionalHint(&old, hint)
''')
p.write_text(text, encoding="utf-8")

# PDF may use an existing cached detail directly; re-apply the request's Optional hint.
replace_once(
    "internal/app/server.go",
    '''	if x.RegisterAmount != 0 && !x.AmountVerified {
		jsonErr(w, 409, errors.New(x.VerificationNote))''',
    '''	preserveOptionalHint(&x, hint)
	if x.RegisterAmount != 0 && !x.AmountVerified {
		jsonErr(w, 409, errors.New(x.VerificationNote))''',
)

# When enriching by master ID, carry Optional from the classified register cache.
replace_once(
    "internal/app/server.go",
    '''			if hint.VoucherType == "" {
				hint.VoucherType = cached.VoucherType
			}
		}
	}''',
    '''			if hint.VoucherType == "" {
				hint.VoucherType = cached.VoucherType
			}
			if cached.Optional {
				hint.Optional = true
			}
		}
	}''',
)

# Saved invoice-detail search entries can predate the Optional field. Infer it from
# the now-classified register cache so Universal Search still opens Proforma correctly.
replace_once(
    "internal/app/server.go",
    '''		if v.LRNumber != "" {
			out = append(out, SearchResult{Kind: "invoice-detail", Title: firstNonBlank(v.InvoiceNumber, v.LRNumber), Subtitle: v.CustomerName + " • LR/RR " + v.LRNumber, MasterID: v.MasterID, Party: v.CustomerName, Number: v.InvoiceNumber, Date: v.InvoiceDate, LRNumber: v.LRNumber, Amount: v.TotalAmount, VoucherType: v.VoucherType})
		}''',
    '''		if hint, ok := s.cache.VoucherHint(c, v.MasterID); ok && hint.Optional {
			v.Optional = true
		}
		if v.LRNumber != "" {
			out = append(out, SearchResult{Kind: "invoice-detail", Title: firstNonBlank(v.InvoiceNumber, v.LRNumber), Subtitle: v.CustomerName + " • LR/RR " + v.LRNumber, MasterID: v.MasterID, Party: v.CustomerName, Number: v.InvoiceNumber, Date: v.InvoiceDate, LRNumber: v.LRNumber, Amount: v.TotalAmount, VoucherType: v.VoucherType, Optional: v.Optional})
		}''',
)

Path("internal/app/proforma_cache_identity_test.go").write_text(r'''package app

import (
    "testing"
    "time"
)

func TestVoucherHintPrefersClassifiedProformaAndRejectsOldMixedCache(t *testing.T) {
    cache, err := NewCacheStore(t.TempDir())
    if err != nil { t.Fatal(err) }
    old := VoucherData{Vouchers: []VoucherRow{{MasterID: "77", Number: "PF-77", Optional: false}}}
    if err := cache.Put("sales", "Demo", "fy", old, time.Now()); err != nil { t.Fatal(err) }
    if _, ok := cache.VoucherHint("Demo", "77"); ok {
        t.Fatal("unclassified pre-upgrade Sales cache must not supply voucher identity")
    }
    proforma := voucherDataFromRows([]VoucherRow{{MasterID: "77", Number: "PF-77", Optional: true}}, "proforma", "fy", 1, time.Now())
    sales := voucherDataFromRows([]VoucherRow{{MasterID: "88", Number: "S-88"}}, "sales", "fy", 1, time.Now())
    if err := cache.Put("sales", "Demo", "fy", sales, time.Now()); err != nil { t.Fatal(err) }
    if err := cache.Put("proforma", "Demo", "fy", proforma, time.Now()); err != nil { t.Fatal(err) }
    hint, ok := cache.VoucherHint("Demo", "77")
    if !ok || !hint.Optional || hint.Number != "PF-77" {
        t.Fatalf("classified proforma hint not recovered: %#v, ok=%v", hint, ok)
    }
}

func TestPreserveOptionalHintOnCachedDetail(t *testing.T) {
    detail := VoucherDetail{InvoiceNumber: "PF-9"}
    preserveOptionalHint(&detail, VoucherRow{Optional: true})
    if !detail.Optional { t.Fatal("cached voucher detail lost Optional / Proforma identity") }
}
''', encoding="utf-8")
