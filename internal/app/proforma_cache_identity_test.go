package app

import (
	"testing"
	"time"
)

func TestVoucherHintPrefersClassifiedProformaAndRejectsOldMixedCache(t *testing.T) {
	cache, err := NewCacheStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	old := VoucherData{Vouchers: []VoucherRow{{MasterID: "77", Number: "PF-77", Optional: false}}}
	if err := cache.Put("sales", "Demo", "fy", old, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, ok := cache.VoucherHint("Demo", "77"); ok {
		t.Fatal("unclassified pre-upgrade Sales cache must not supply voucher identity")
	}
	proforma := voucherDataFromRows([]VoucherRow{{MasterID: "77", Number: "PF-77", Optional: true}}, "proforma", "fy", 1, time.Now())
	sales := voucherDataFromRows([]VoucherRow{{MasterID: "88", Number: "S-88"}}, "sales", "fy", 1, time.Now())
	if err := cache.Put("sales", "Demo", "fy", sales, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := cache.Put("proforma", "Demo", "fy", proforma, time.Now()); err != nil {
		t.Fatal(err)
	}
	hint, ok := cache.VoucherHint("Demo", "77")
	if !ok || !hint.Optional || hint.Number != "PF-77" {
		t.Fatalf("classified proforma hint not recovered: %#v, ok=%v", hint, ok)
	}
}

func TestPreserveOptionalHintOnCachedDetail(t *testing.T) {
	detail := VoucherDetail{InvoiceNumber: "PF-9"}
	preserveOptionalHint(&detail, VoucherRow{Optional: true})
	if !detail.Optional {
		t.Fatal("cached voucher detail lost Optional / Proforma identity")
	}
}
