package app

import "strings"

func voucherNode(root *xnode) *xnode {
	var nodes []*xnode
	nodesByName(root, "VOUCHER", &nodes)
	if len(nodes) == 1 {
		return nodes[0]
	}
	return root
}
func invoiceLedgers(root *xnode) []*xnode {
	var rows []*xnode
	for _, n := range voucherNode(root).Children {
		if n.Name == "ALLLEDGERENTRIES.LIST" || n.Name == "LEDGERENTRIES.LIST" {
			rows = append(rows, n)
		}
	}
	return rows
}
func invoicePartyTotal(root *xnode, party string) (float64, bool) {
	sum := 0.0
	found := false
	for _, n := range invoiceLedgers(root) {
		name := directChild(n, "LEDGERNAME")
		if (party != "" && strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(party))) || strings.EqualFold(directChild(n, "ISPARTYLEDGER"), "Yes") {
			if raw := directChild(n, "AMOUNT"); raw != "" {
				sum += parseAmount(raw)
				found = true
			}
		}
	}
	if found {
		return sum, true
	}
	if raw := directChild(voucherNode(root), "AMOUNT"); raw != "" {
		return parseAmount(raw), true
	}
	return 0, false
}
func applyInvoiceAmounts(x *VoucherDetail, root *xnode, hint VoucherRow) {
	party := firstNonBlank(firstDeep(root, "PARTYLEDGERNAME"), hint.Party, x.CustomerName)
	total, verified := invoicePartyTotal(root, party)
	if !verified {
		return
	}
	x.TotalsVersion = 1
	x.TotalAmount = abs(total)
	direction := 1.0
	if total > 0 {
		direction = -1
	}
	for _, n := range invoiceLedgers(root) {
		name := strings.ToUpper(directChild(n, "LEDGERNAME"))
		if strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(party)) || strings.EqualFold(directChild(n, "ISPARTYLEDGER"), "Yes") {
			continue
		}
		amount := parseAmount(directChild(n, "AMOUNT")) * direction
		switch {
		case strings.Contains(name, "DISCOUNT"):
			x.DiscountAmount += abs(amount)
		case strings.Contains(name, "CGST"):
			x.CGST += amount
		case strings.Contains(name, "SGST") || strings.Contains(name, "UTGST"):
			x.SGST += amount
		case strings.Contains(name, "IGST"):
			x.IGST += amount
		case strings.Contains(name, "FREIGHT") || strings.Contains(name, "TRANSPORT") || strings.Contains(name, "CARTAGE"):
			x.Freight += amount
		case strings.Contains(name, "ROUND"):
			x.RoundOff += amount
		}
	}
	// Inventory AMOUNT is the line's recorded net value. DISCOUNT is a percentage,
	// and any item discount is already included: neither is subtracted again.
	for _, it := range x.Items {
		x.TaxableAmount += it.Amount
	}
	x.TaxableAmount -= x.DiscountAmount
	if len(x.Items) == 0 {
		x.TaxableAmount = x.TotalAmount - x.CGST - x.SGST - x.IGST - x.Freight - x.RoundOff
	}
	x.OtherCharges = x.TotalAmount - (x.TaxableAmount + x.CGST + x.SGST + x.IGST + x.Freight + x.RoundOff)
	if abs(x.OtherCharges) < 0.005 {
		x.OtherCharges = 0
	}
}
