package app

import (
	"strings"
	"testing"
	"time"
)

func TestFollowUpsClassifyDueAndOverdue(t *testing.T) {
	s := newOrderTestServer(t)
	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().Add(-24 * time.Hour).Format("2006-01-02")
	if _, err := s.saveFollowUp(FollowUp{Company: "Firm", Party: "Today Customer", PromiseDate: today, Amount: 1200}); err != nil { t.Fatal(err) }
	if _, err := s.saveFollowUp(FollowUp{Company: "Firm", Party: "Late Customer", PromiseDate: yesterday, Amount: 500}); err != nil { t.Fatal(err) }
	x, err := s.listFollowUps("Firm", "")
	if err != nil { t.Fatal(err) }
	if x.OpenCount != 2 || x.DueCount != 1 || x.OverdueCount != 1 { t.Fatalf("unexpected follow-up buckets: %#v", x) }
}

func TestDailyWorkspaceIncludesOrdersAndFollowUpsWithoutTallyRead(t *testing.T) {
	s := newOrderTestServer(t)
	if _, err := s.saveOrder(SalesOrder{Company:"Firm", Buyer:"Buyer", Items:[]OrderItem{{Name:"Item",Quantity:2}}}); err != nil { t.Fatal(err) }
	if _, err := s.saveFollowUp(FollowUp{Company:"Firm",Party:"Buyer",PromiseDate:time.Now().Format("2006-01-02")}); err != nil { t.Fatal(err) }
	x := s.dailyWorkspace("Firm")
	if x.PendingOrders != 1 || x.FollowUpsDue != 1 { t.Fatalf("workspace did not combine local workflow data: %#v", x) }
	if len(x.Alerts) == 0 { t.Fatal("expected at least the offline Tally alert in test environment") }
}

func TestGlobalSearchIncludesOrdersAndFollowUps(t *testing.T) {
	s := newOrderTestServer(t)
	if _, err := s.saveOrder(SalesOrder{Company:"Firm",Buyer:"ABC Traders",Items:[]OrderItem{{Name:"Ashoka Towel",Quantity:5}},DispatchedThrough:"VRL"}); err != nil { t.Fatal(err) }
	if _, err := s.saveFollowUp(FollowUp{Company:"Firm",Party:"ABC Traders",Note:"Call about balance"}); err != nil { t.Fatal(err) }
	x := s.buildGlobalSearch("Firm")
	kinds := map[string]bool{}
	var text string
	for _,r := range x.Results { kinds[r.Kind]=true; text += " "+r.SearchText }
	if !kinds["order"] || !kinds["followup"] { t.Fatalf("search missing workflow records: %#v", kinds) }
	if !strings.Contains(strings.ToLower(text),"ashoka towel") || !strings.Contains(strings.ToLower(text),"call about balance") { t.Fatalf("search text is incomplete: %s", text) }
}

func TestTallyPerformanceSummarizesRecentReads(t *testing.T) {
	s := newOrderTestServer(t)
	s.tally.logs=[]RequestLog{{DurationMS:100,Success:true},{DurationMS:2500,Success:true},{DurationMS:300,Success:false,Error:"timeout"}}
	x:=s.tallyPerformance()
	if x.TotalReads!=3 || x.SuccessReads!=2 || x.FailedReads!=1 || x.SlowReads!=1 { t.Fatalf("bad performance summary: %#v",x) }
	if x.AverageMS!=966 { t.Fatalf("unexpected average %d",x.AverageMS) }
}

func TestAdaptiveInvoiceLayoutOnlyCompactsShortDocuments(t *testing.T) {
	short:=adaptiveInvoiceHTML("<html><style>.x{}</style><body>x</body></html>",3)
	if !strings.Contains(short,"@page{margin:10mm") || !strings.Contains(short,"bank-qr img") { t.Fatalf("short layout was not compacted: %s",short) }
	long:=adaptiveInvoiceHTML("<html><style>.x{}</style><body>x</body></html>",8)
	if strings.Contains(long,"@page{margin:10mm") { t.Fatalf("long invoice should retain normal pagination: %s",long) }
}
