package app

import (
	"strings"
	"time"
)

const voucherClassificationVersion = 1

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
		CommonData:            CommonData{Live: true, Source: "live", FetchedAt: fetchedAt, DurationMS: duration},
		Vouchers:              rows,
		Total:                 total,
		Count:                 len(rows),
		Type:                  typ,
		Period:                period,
		ClassificationVersion: voucherClassificationVersion,
	}
}

func voucherCacheIsClassified(x VoucherData) bool {
	return x.ClassificationVersion == voucherClassificationVersion
}

func preserveOptionalHint(x *VoucherDetail, hint VoucherRow) {
	if x != nil && hint.Optional {
		x.Optional = true
	}
}

func invoicePDFBaseName(x VoucherDetail) string {
	number := sanitizeFileName(firstNonBlank(x.InvoiceNumber, "Voucher"))
	if party := strings.TrimSpace(x.CustomerName); party != "" {
		return number + "-" + sanitizeFileName(party)
	}
	return number
}
