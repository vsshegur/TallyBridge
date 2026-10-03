from pathlib import Path


def rep(path, old, new, count=1):
    p=Path(path); s=p.read_text(encoding='utf-8')
    if s.count(old) < count:
        raise SystemExit(f'{path}: anchor missing: {old[:100]!r}')
    s=s.replace(old,new,count); p.write_text(s,encoding='utf-8')

p='internal/app/workspace.go'
rep(p, '''type CustomerWorkspace struct {
	Customer         Customer360Data `json:"customer"`
	PendingOrders    []SalesOrder    `json:"pendingOrders"`
	DispatchedOrders []SalesOrder    `json:"dispatchedOrders"`
	FollowUps        []FollowUp      `json:"followUps"`
}
''', '''type FrequentItem struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type CustomerWorkspace struct {
	Customer         Customer360Data `json:"customer"`
	PendingOrders    []SalesOrder    `json:"pendingOrders"`
	DispatchedOrders []SalesOrder    `json:"dispatchedOrders"`
	FollowUps        []FollowUp      `json:"followUps"`
	LastPurchaseDate string          `json:"lastPurchaseDate,omitempty"`
	FrequentItems    []FrequentItem  `json:"frequentItems"`
}
''')
rep(p, 'import (\n\t"encoding/json"', 'import (\n\t"encoding/json"\n\t"fmt"')
rep(p, '''	if fs, e := s.listFollowUps(company, party); e == nil {
		out.FollowUps = append(out.FollowUps, fs.Open...)
	}
	return out, err
}
''', '''	if fs, e := s.listFollowUps(company, party); e == nil {
		out.FollowUps = append(out.FollowUps, fs.Open...)
	}
	counts := map[string]int{}
	for _, d := range s.cache.Matching("voucher-detail-v6", company) {
		var v VoucherDetail
		if json.Unmarshal(d.Data, &v) != nil || v.Optional || !strings.EqualFold(strings.TrimSpace(v.CustomerName), strings.TrimSpace(party)) { continue }
		vt := strings.ToLower(strings.TrimSpace(v.VoucherType))
		if vt != "" && !strings.Contains(vt, "sales") { continue }
		if dt := normISODate(v.InvoiceDate); dt > out.LastPurchaseDate { out.LastPurchaseDate = dt }
		for _, item := range v.Items { name := strings.TrimSpace(item.Name); if name != "" { counts[name]++ } }
	}
	out.FrequentItems = []FrequentItem{}
	for name, count := range counts { out.FrequentItems = append(out.FrequentItems, FrequentItem{Name:name, Count:count}) }
	sort.Slice(out.FrequentItems, func(i,j int) bool { if out.FrequentItems[i].Count==out.FrequentItems[j].Count { return strings.ToLower(out.FrequentItems[i].Name)<strings.ToLower(out.FrequentItems[j].Name) }; return out.FrequentItems[i].Count>out.FrequentItems[j].Count })
	if len(out.FrequentItems)>5 { out.FrequentItems=out.FrequentItems[:5] }
	return out, err
}
''')
rep(p, '''	h := s.companyHealth(company, false)
''', '''	overdueBills, overdueAmount := 0, 0.0
	overdueParties := map[string]bool{}
	today := time.Now().Format("2006-01-02")
	for _, d := range s.cache.Matching("party-outstanding", company) {
		var po PartyOutstandingData
		if json.Unmarshal(d.Data, &po) != nil { continue }
		for _, bill := range po.Bills {
			due := normISODate(firstNonBlank(bill.DueDate, bill.Date))
			if due != "" && due < today && abs(bill.PendingAmount) > 0 { overdueBills++; overdueAmount += abs(bill.PendingAmount); overdueParties[po.Party]=true }
		}
	}
	if overdueBills > 0 { alerts = append(alerts, SmartAlert{ID:"customer-overdue", Severity:"warning", Title:"Customer payments overdue", Detail:fmt.Sprintf("%d bill(s) • INR %s across %d customer(s)", overdueBills, indianNumber(overdueAmount), len(overdueParties)), Action:"followups"}) }
	h := s.companyHealth(company, false)
''')
rep(p, '''		if v.LRNumber == "" && v.GSTIN == "" {
			continue
		}
''', '''		if v.LRNumber == "" && v.GSTIN == "" && len(v.Items) == 0 {
			continue
		}
''')
rep(p, '''		seen[key] = true
		out = append(out, GlobalSearchResult{Kind: "invoice-detail", ID: v.MasterID, Title: firstNonBlank(v.InvoiceNumber, v.CustomerName), Subtitle: detailSubtitle(v), Party: v.CustomerName, Ledger: v.CustomerName, Number: v.InvoiceNumber, Date: v.InvoiceDate, MasterID: v.MasterID, VoucherType: v.VoucherType, Amount: v.TotalAmount, Optional: v.Optional, SearchText: searchText(v.InvoiceNumber, v.CustomerName, v.LRNumber, v.GSTIN, v.CustomerCity, v.VehicleNumber, v.EWayBillNumber)})
''', '''		seen[key] = true
		itemText := []string{}
		for _, item := range v.Items { itemText=append(itemText,item.Name,item.Description,item.HSN) }
		out = append(out, GlobalSearchResult{Kind: "invoice-detail", ID: v.MasterID, Title: firstNonBlank(v.InvoiceNumber, v.CustomerName), Subtitle: detailSubtitle(v), Party: v.CustomerName, Ledger: v.CustomerName, Number: v.InvoiceNumber, Date: v.InvoiceDate, MasterID: v.MasterID, VoucherType: v.VoucherType, Amount: v.TotalAmount, Optional: v.Optional, SearchText: searchText(v.InvoiceNumber, v.CustomerName, v.LRNumber, v.GSTIN, v.CustomerCity, v.VehicleNumber, v.EWayBillNumber, strings.Join(itemText," "))})
''')
print('workspace hardening applied')
