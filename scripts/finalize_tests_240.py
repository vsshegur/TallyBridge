from pathlib import Path
p=Path('internal/app/operations_240_test.go')
s=p.read_text(encoding='utf-8')
append=r'''

func TestSmartAlertsIncludeOverdueSavedBills(t *testing.T) {
	s := newOrderTestServer(t)
	yesterday := time.Now().Add(-24*time.Hour).Format("2006-01-02")
	x := PartyOutstandingData{Party:"ABC Traders", Bills:[]BillRow{{BillNumber:"INV1",DueDate:yesterday,PendingAmount:2500}}}
	if err:=s.cache.Put("party-outstanding","Firm","ABC Traders",x,time.Now()); err!=nil { t.Fatal(err) }
	alerts:=s.smartAlerts("Firm")
	found:=false
	for _,a:=range alerts {
		if a.ID=="customer-overdue" {
			found=true
			if !strings.Contains(a.Detail,"2,500") { t.Fatalf("unexpected overdue detail %q",a.Detail) }
		}
	}
	if !found { t.Fatal("expected overdue customer alert") }
}

func TestCustomerWorkspaceBuildsPurchaseSignalsFromSavedInvoiceDetails(t *testing.T) {
	s := newOrderTestServer(t)
	if err:=s.cache.Put("customer360","Firm","ABC Traders",Customer360Data{Company:"Firm",Ledger:"ABC Traders"},time.Now()); err!=nil { t.Fatal(err) }
	v:=VoucherDetail{Company:"Firm",CustomerName:"ABC Traders",InvoiceNumber:"101",InvoiceDate:"2026-10-01",VoucherType:"Sales",Items:[]InvoiceItem{{Name:"Ashoka"},{Name:"Ashoka"},{Name:"Shubhangi"}}}
	if err:=s.cache.Put("voucher-detail-v6","Firm","fixture",v,time.Now()); err!=nil { t.Fatal(err) }
	x,err:=s.customerWorkspace("Firm","ABC Traders",false)
	if err!=nil { t.Fatal(err) }
	if x.LastPurchaseDate!="2026-10-01" { t.Fatalf("bad last purchase date %q",x.LastPurchaseDate) }
	if len(x.FrequentItems)==0 || x.FrequentItems[0].Name!="Ashoka" || x.FrequentItems[0].Count!=2 { t.Fatalf("missing purchase signals: %#v",x.FrequentItems) }
}

func TestGlobalSearchIndexesInvoiceProducts(t *testing.T) {
	s := newOrderTestServer(t)
	v:=VoucherDetail{Company:"Firm",CustomerName:"ABC Traders",InvoiceNumber:"102",InvoiceDate:"2026-10-02",VoucherType:"Sales",Items:[]InvoiceItem{{Name:"Ashoka Towel",HSN:"6302",Description:"Red border"}}}
	if err:=s.cache.Put("voucher-detail-v6","Firm","fixture2",v,time.Now()); err!=nil { t.Fatal(err) }
	x:=s.buildGlobalSearch("Firm")
	found:=false
	for _,r:=range x.Results { if strings.Contains(r.SearchText,"ashoka towel")&&strings.Contains(r.SearchText,"6302")&&strings.Contains(r.SearchText,"red border") { found=true } }
	if !found { t.Fatal("invoice product/HSN/description was not indexed") }
}
'''
if 'TestSmartAlertsIncludeOverdueSavedBills' not in s:
    s += append
p.write_text(s,encoding='utf-8')
print('operations regression tests added')
