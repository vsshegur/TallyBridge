package app

import (
	"math"
	"regexp"
	"strings"
)

// Inward vouchers for InvoiceTotally's GSTR-2B check. The request comes from
// InvoiceTotally and is an ordinary read-only collection export (it goes through
// validateExportXML like every other read). Here each voucher becomes a small row:
// party, supplier invoice number/date, GSTIN and its ledger lines. InvoiceTotally
// knows which ledgers are GST ledgers (from the masters) and works out the tax.

var gstLikeLedgerRE = regexp.MustCompile(`(?i)gst|integrated|central tax|state tax|\bcess\b|input`)

func purchaseGstRows(reply []byte) []map[string]any {
	out := []map[string]any{}
	root, err := parseTree(reply)
	if err != nil {
		return out
	}
	var vouchers []*xnode
	nodesByName(root, "VOUCHER", &vouchers)
	seen := map[string]bool{}
	for _, v := range vouchers {
		if tallyYes(directChild(v, "ISCANCELLED")) || tallyYes(directChild(v, "ISOPTIONAL")) {
			continue
		}
		date := normISODate(directChild(v, "DATE"))
		if date == "" {
			continue
		}
		mid := directChild(v, "MASTERID")
		number := directChild(v, "VOUCHERNUMBER")
		key := mid + "|" + date + "|" + number
		if seen[key] {
			continue
		}
		seen[key] = true
		vtype := directChild(v, "VOUCHERTYPENAME")
		kind := strings.ToLower(directChild(v, "ITKIND"))
		if kind == "" {
			kind = guessGstVoucherKind(vtype)
		}
		entries := [][2]any{}
		gstLike := false
		lists := directChildren(v, "ALLLEDGERENTRIES.LIST")
		if len(lists) == 0 {
			lists = directChildren(v, "LEDGERENTRIES.LIST")
		}
		for _, e := range lists {
			name := directChild(e, "LEDGERNAME")
			raw := directChild(e, "AMOUNT")
			if name == "" || raw == "" {
				continue
			}
			if gstLikeLedgerRE.MatchString(name) {
				gstLike = true
			}
			entries = append(entries, [2]any{name, round2(parseAmount(raw))})
		}
		// journals, payments and credit notes matter only when they carry GST
		if (kind == "jv" || kind == "pay" || kind == "cn") && !gstLike {
			continue
		}
		inv := 0.0
		invLists := directChildren(v, "ALLINVENTORYENTRIES.LIST")
		if len(invLists) == 0 {
			invLists = directChildren(v, "INVENTORYENTRIES.LIST")
		}
		for _, e := range invLists {
			if raw := directChild(e, "AMOUNT"); raw != "" {
				inv += parseAmount(raw)
			}
		}
		party := directChild(v, "PARTYLEDGERNAME")
		if party == "" {
			party = directChild(v, "PARTYNAME")
		}
		out = append(out, map[string]any{
			"mid": mid, "date": date, "number": number, "vtype": vtype, "kind": kind,
			"ref": directChild(v, "REFERENCE"), "refdate": normISODate(directChild(v, "REFERENCEDATE")),
			"party": party, "gstin": strings.ToUpper(strings.TrimSpace(directChild(v, "PARTYGSTIN"))),
			"inv": round2(inv), "e": entries,
		})
	}
	return out
}

func guessGstVoucherKind(vtype string) string {
	t := strings.ToLower(vtype)
	switch {
	case strings.Contains(t, "debit"):
		return "dn"
	case strings.Contains(t, "credit"):
		return "cn"
	case strings.Contains(t, "journal"):
		return "jv"
	case strings.Contains(t, "payment"):
		return "pay"
	}
	return "inv"
}

func directChildren(n *xnode, name string) []*xnode {
	want := strings.ToUpper(name)
	var out []*xnode
	for _, c := range n.Children {
		if c.Name == want {
			out = append(out, c)
		}
	}
	return out
}

func round2(x float64) float64 { return math.Round(x*100) / 100 }
