from pathlib import Path


def rep(path, old, new, count=1):
    p=Path(path); s=p.read_text(encoding='utf-8')
    if s.count(old) < count:
        raise SystemExit(f'{path}: anchor missing: {old[:100]!r}')
    s=s.replace(old,new,count); p.write_text(s,encoding='utf-8')

# Customer 360: last purchase + frequent saved invoice items.
rep('internal/app/workspace.go', '''type CustomerWorkspace struct {
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
rep('internal/app/workspace.go', '''	if fs, e := s.listFollowUps(company, party); e == nil {
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
		if json.Unmarshal(d.Data, &v) != nil || v.Optional || !strings.EqualFold(strings.TrimSpace(v.CustomerName), strings.TrimSpace(party)) {
			continue
		}
		vt := strings.ToLower(strings.TrimSpace(v.VoucherType))
		if vt != "" && !strings.Contains(vt, "sales") { continue }
		if dt := normISODate(v.InvoiceDate); dt > out.LastPurchaseDate { out.LastPurchaseDate = dt }
		for _, item := range v.Items {
			name := strings.TrimSpace(item.Name); if name != "" { counts[name]++ }
		}
	}
	out.FrequentItems = []FrequentItem{}
	for name, count := range counts { out.FrequentItems = append(out.FrequentItems, FrequentItem{Name:name, Count:count}) }
	sort.Slice(out.FrequentItems, func(i,j int) bool { if out.FrequentItems[i].Count==out.FrequentItems[j].Count { return strings.ToLower(out.FrequentItems[i].Name)<strings.ToLower(out.FrequentItems[j].Name) }; return out.FrequentItems[i].Count>out.FrequentItems[j].Count })
	if len(out.FrequentItems)>5 { out.FrequentItems=out.FrequentItems[:5] }
	return out, err
}
''')

# Smart alerts: aggregate overdue customer bills from verified saved bill-wise data.
rep('internal/app/workspace.go', '''	h := s.companyHealth(company, false)
''', '''	overdueBills, overdueAmount := 0, 0.0
	overdueParties := map[string]bool{}
	today := time.Now().Format("2006-01-02")
	for _, d := range s.cache.Matching("party-outstanding", company) {
		var po PartyOutstandingData
		if json.Unmarshal(d.Data, &po) != nil { continue }
		for _, bill := range po.Bills {
			due := normISODate(firstNonBlank(bill.DueDate, bill.Date))
			if due != "" && due < today && abs(bill.PendingAmount) > 0 {
				overdueBills++; overdueAmount += abs(bill.PendingAmount); overdueParties[po.Party]=true
			}
		}
	}
	if overdueBills > 0 {
		alerts = append(alerts, SmartAlert{ID:"customer-overdue", Severity:"warning", Title:"Customer payments overdue", Detail:fmt.Sprintf("%d bill(s) • %s across %d customer(s)", overdueBills, indianNumber(overdueAmount), len(overdueParties)), Action:"followups"})
	}
	h := s.companyHealth(company, false)
''')
# workspace.go needs fmt.
rep('internal/app/workspace.go', 'import (\n\t"encoding/json"', 'import (\n\t"encoding/json"\n\t"fmt"')

# Global search: invoice products/HSN/descriptions are searchable too.
rep('internal/app/workspace.go', '''		if v.LRNumber == "" && v.GSTIN == "" {
			continue
		}
''', '''		if v.LRNumber == "" && v.GSTIN == "" && len(v.Items) == 0 {
			continue
		}
''')
rep('internal/app/workspace.go', '''		seen[key] = true
		out = append(out, GlobalSearchResult{Kind: "invoice-detail", ID: v.MasterID, Title: firstNonBlank(v.InvoiceNumber, v.CustomerName), Subtitle: detailSubtitle(v), Party: v.CustomerName, Ledger: v.CustomerName, Number: v.InvoiceNumber, Date: v.InvoiceDate, MasterID: v.MasterID, VoucherType: v.VoucherType, Amount: v.TotalAmount, Optional: v.Optional, SearchText: searchText(v.InvoiceNumber, v.CustomerName, v.LRNumber, v.GSTIN, v.CustomerCity, v.VehicleNumber, v.EWayBillNumber)})
''', '''		seen[key] = true
		itemText := []string{}
		for _, item := range v.Items { itemText=append(itemText,item.Name,item.Description,item.HSN) }
		out = append(out, GlobalSearchResult{Kind: "invoice-detail", ID: v.MasterID, Title: firstNonBlank(v.InvoiceNumber, v.CustomerName), Subtitle: detailSubtitle(v), Party: v.CustomerName, Ledger: v.CustomerName, Number: v.InvoiceNumber, Date: v.InvoiceDate, MasterID: v.MasterID, VoucherType: v.VoucherType, Amount: v.TotalAmount, Optional: v.Optional, SearchText: searchText(v.InvoiceNumber, v.CustomerName, v.LRNumber, v.GSTIN, v.CustomerCity, v.VehicleNumber, v.EWayBillNumber, strings.Join(itemText," "))})
''')

# Phone diagnostics: expose unsynced Orders/photo queue without exposing data.
rep('android/app/src/main/java/in/tallybridge/app/OfflineOrdersStore.java', ''' int waitingCount()throws Exception {JSONObject s=state();JSONArray a=s.getJSONArray("orders");int n=0;for(int i=0;i<a.length();i++){if(hasLocalMutation(a.optJSONObject(i)))n++;}return n;}
''', ''' int waitingCount()throws Exception {JSONObject s=state();JSONArray a=s.getJSONArray("orders");int n=0;for(int i=0;i<a.length();i++){if(hasLocalMutation(a.optJSONObject(i)))n++;}return n;}
 int pendingPhotoUploadCount()throws Exception {JSONObject s=state();JSONArray a=s.getJSONArray("orders");int n=0;for(int i=0;i<a.length();i++){JSONObject o=a.optJSONObject(i);if(o==null||!hasLocalMutation(o))continue;JSONArray photos=o.optJSONArray("attachments");if(photos==null)continue;for(int j=0;j<photos.length();j++){JSONObject p=photos.optJSONObject(j);if(p!=null&&!p.optString("localPath").isEmpty())n++;}}return n;}
''')

# Quick Order is a first-class compact entry mode.
rep('android/app/src/main/java/in/tallybridge/app/OrdersActivity.java', ' static final String ACTION_NEW="in.tallybridge.app.ORDER_NEW";', ' static final String ACTION_NEW="in.tallybridge.app.ORDER_NEW", ACTION_QUICK="in.tallybridge.app.ORDER_QUICK";')
rep('android/app/src/main/java/in/tallybridge/app/OrdersActivity.java', ' private boolean dispatchedTab=false,destroyed=false,mastersLoading=false,dark=false;', ' private boolean dispatchedTab=false,destroyed=false,mastersLoading=false,dark=false,quickMode=false;')
rep('android/app/src/main/java/in/tallybridge/app/OrdersActivity.java', '  if(ACTION_NEW.equals(getIntent().getAction()))openForm(null);else showOrders();', '  String action=getIntent().getAction();quickMode=ACTION_QUICK.equals(action);if(ACTION_NEW.equals(action)||quickMode)openForm(null);else showOrders();')
rep('android/app/src/main/java/in/tallybridge/app/OrdersActivity.java', '  frame(order==null?"Add order":"Edit order","Offline-ready • saved buyer/item lists from Tally • rate is optional.");', '  frame(order==null?(quickMode?"Quick order":"Add order"):"Edit order",quickMode&&order==null?"Party • items • quantity • dispatch • save. Works offline.":"Offline-ready • saved buyer/item lists from Tally • rate is optional.");')
rep('android/app/src/main/java/in/tallybridge/app/OrdersActivity.java', '''  body.addView(txt("NOTES",11,MUTED),full(16));EditText notes=input("Colours, packing, delivery instructions…",InputType.TYPE_CLASS_TEXT|InputType.TYPE_TEXT_FLAG_MULTI_LINE);notes.setMinLines(2);if(order!=null)notes.setText(order.optString("notes"));body.addView(notes,full(6));if(order==null)body.addView(btn("🎙 Speak order",v->startVoiceOrder(buyer,notes,itemsBox)),full(8));body.addView(primary(order==null?"Save pending order":"Update pending order",v->saveForm(order,companySpin,buyer,dispatchedThrough,notes)),full(18));
''', '''  EditText notes=input("Colours, packing, delivery instructions…",InputType.TYPE_CLASS_TEXT|InputType.TYPE_TEXT_FLAG_MULTI_LINE);notes.setMinLines(2);if(order!=null)notes.setText(order.optString("notes"));if(!quickMode||order!=null){body.addView(txt("NOTES",11,MUTED),full(16));body.addView(notes,full(6));}if(order==null)body.addView(btn("🎙 Speak order",v->startVoiceOrder(buyer,notes,itemsBox)),full(8));body.addView(primary(order==null?(quickMode?"Save quick order":"Save pending order"):"Update pending order",v->saveForm(order,companySpin,buyer,dispatchedThrough,notes)),full(18));
''')

# Voice Order: recognize multiple saved items and quantities on either side of the product name.
start=' private void applyVoiceOrder(String spoken){\n'
end=' private void refreshBootstrapSilently(){'
p=Path('android/app/src/main/java/in/tallybridge/app/OrdersActivity.java'); s=p.read_text(encoding='utf-8'); a=s.find(start); b=s.find(end,a)
if a<0 or b<0: raise SystemExit('voice method anchors missing')
voice=''' private double voiceQty(String lower,String item){
  String q=java.util.regex.Pattern.quote(item.toLowerCase(Locale.ROOT));
  java.util.regex.Pattern[] ps=new java.util.regex.Pattern[]{java.util.regex.Pattern.compile("([0-9]+(?:[.][0-9]+)?)\\\\s*(?:dozen|dz|pcs?|pieces?)?\\\\s+(?:of\\\\s+)?"+q),java.util.regex.Pattern.compile(q+"\\\\s*[-,:]?\\\\s*([0-9]+(?:[.][0-9]+)?)\\\\s*(?:dozen|dz|pcs?|pieces?)?")};
  for(java.util.regex.Pattern p:ps){java.util.regex.Matcher m=p.matcher(lower);if(m.find())try{return Double.parseDouble(m.group(1));}catch(Exception ignored){}}
  return 1;
 }
 private void applyVoiceOrder(String spoken){
  if(voiceNotes!=null&&!quickMode)voiceNotes.setText(spoken);String lower=spoken.toLowerCase(Locale.ROOT);
  if(voiceBuyer!=null&&parties!=null){String best="";for(int i=0;i<parties.length();i++){JSONObject p=parties.optJSONObject(i);String n=p==null?parties.optString(i):p.optString("name");if(!n.isEmpty()&&lower.contains(n.toLowerCase(Locale.ROOT))&&n.length()>best.length())best=n;}if(!best.isEmpty())voiceBuyer.setText(best,false);}
  List<JSONObject> matches=new ArrayList<>();Set<String> used=new HashSet<>();if(stock!=null){List<JSONObject> candidates=new ArrayList<>();for(int i=0;i<stock.length();i++){JSONObject x=stock.optJSONObject(i);if(x!=null&&!x.optString("name").isEmpty()&&lower.contains(x.optString("name").toLowerCase(Locale.ROOT)))candidates.add(x);}candidates.sort((x,y)->Integer.compare(y.optString("name").length(),x.optString("name").length()));for(JSONObject x:candidates){String name=x.optString("name");String key=name.toLowerCase(Locale.ROOT);if(used.add(key))matches.add(x);}}
  int added=0;for(JSONObject matched:matches){String name=matched.optString("name");double q=voiceQty(lower,name);JSONObject existing=new JSONObject();try{existing.put("name",name).put("quantity",q).put("unit",matched.optString("unit"));}catch(Exception ignored){}if(added==0&&itemRows.size()==1&&itemRows.get(0).item.getText().toString().trim().isEmpty()){ItemRow r=itemRows.get(0);r.item.setText(name,false);r.qty.setText(qty(q));r.unit.setText(matched.optString("unit"));}else if(voiceItemsBox!=null)addItemRow(voiceItemsBox,existing);added++;}
  toast(added>0?("Voice order captured • "+added+" item"+(added==1?"":"s")+" found • review before saving"):"Voice captured as notes • choose item and quantity before saving");
 }
'''
s=s[:a]+voice+s[b:];p.write_text(s,encoding='utf-8')

# Today screen launches the compact Quick Order mode.
rep('android/app/src/main/java/in/tallybridge/app/WorkActivity.java', 'Button q=primary("+ Quick order",v->{Intent i=new Intent(this,OrdersActivity.class);i.setAction(OrdersActivity.ACTION_NEW);startActivity(i);});', 'Button q=primary("+ Quick order",v->{Intent i=new Intent(this,OrdersActivity.class);i.setAction(OrdersActivity.ACTION_QUICK);startActivity(i);});')
# Customer 360 shows last purchase and frequent saved invoice items.
rep('android/app/src/main/java/in/tallybridge/app/WorkActivity.java', '''JSONArray fs=x.optJSONArray("followUps");heading("Payment follow-ups",fs==null?"0":fs.length()+" open");if(fs!=null)for(int i=0;i<fs.length();i++){JSONObject f=fs.optJSONObject(i);if(f!=null)followUpCard(f);}body.addView(primary("+ Add follow-up",v->followUpDialog(party,null)),full(12));JSONArray invoices=c.optJSONArray("latestInvoices");heading("Latest invoices",""));''', '''JSONArray fs=x.optJSONArray("followUps");heading("Payment follow-ups",fs==null?"0":fs.length()+" open");if(fs!=null)for(int i=0;i<fs.length();i++){JSONObject f=fs.optJSONObject(i);if(f!=null)followUpCard(f);}body.addView(primary("+ Add follow-up",v->followUpDialog(party,null)),full(12));String last=x.optString("lastPurchaseDate");if(!last.isEmpty())card("Last purchase",last,"",null);JSONArray frequent=x.optJSONArray("frequentItems");heading("Frequently purchased",frequent==null||frequent.length()==0?"Saved invoice details will build this list.":"From invoice details already synchronized/opened");if(frequent!=null)for(int i=0;i<frequent.length();i++){JSONObject f=frequent.optJSONObject(i);if(f!=null)card(f.optString("name"),f.optInt("count")+" saved invoice(s)","",null);}JSONArray invoices=c.optJSONArray("latestInvoices");heading("Latest invoices","");''')
# The exact source has a slightly different tail; patch defensively below if prior replacement didn't match.
p=Path('android/app/src/main/java/in/tallybridge/app/WorkActivity.java'); s=p.read_text(encoding='utf-8')
if 'Frequently purchased' not in s:
    old='body.addView(primary("+ Add follow-up",v->followUpDialog(party,null)),full(12));JSONArray invoices=c.optJSONArray("latestInvoices");heading("Latest invoices",""));'
    if old in s:
        s=s.replace(old,'body.addView(primary("+ Add follow-up",v->followUpDialog(party,null)),full(12));String last=x.optString("lastPurchaseDate");if(!last.isEmpty())card("Last purchase",last,"",null);JSONArray frequent=x.optJSONArray("frequentItems");heading("Frequently purchased",frequent==null||frequent.length()==0?"Saved invoice details will build this list.":"From invoice details already synchronized/opened");if(frequent!=null)for(int i=0;i<frequent.length();i++){JSONObject f=frequent.optJSONObject(i);if(f!=null)card(f.optString("name"),f.optInt("count")+" saved invoice(s)","",null);}JSONArray invoices=c.optJSONArray("latestInvoices");heading("Latest invoices","");',1)
    else:
        marker='body.addView(primary("+ Add follow-up",v->followUpDialog(party,null)),full(12));'
        if marker not in s: raise SystemExit('customer UI anchor missing')
        s=s.replace(marker,marker+'String last=x.optString("lastPurchaseDate");if(!last.isEmpty())card("Last purchase",last,"",null);JSONArray frequent=x.optJSONArray("frequentItems");heading("Frequently purchased",frequent==null||frequent.length()==0?"Saved invoice details will build this list.":"From invoice details already synchronized/opened");if(frequent!=null)for(int i=0;i<frequent.length();i++){JSONObject f=frequent.optJSONObject(i);if(f!=null)card(f.optString("name"),f.optInt("count")+" saved invoice(s)","",null);}',1)
    p.write_text(s,encoding='utf-8')
# System diagnostics includes package, OAuth/pairing, cache and offline queues.
p=Path('android/app/src/main/java/in/tallybridge/app/WorkActivity.java'); s=p.read_text(encoding='utf-8')
anchor='card("SHA-256",AppIdentity.sha256(this),"",null);'
extra='card("SHA-256",AppIdentity.sha256(this),"",null);card("Package",getPackageName(),"",null);card("Google access / phone key",store.get("device").isEmpty()?"Not approved":"Approved phone key present","",null);JSONObject cache=d.optJSONObject("cache");if(cache!=null)card("Saved cache",cache.optInt("entries")+" entries • "+cache.optString("latestAt"),"",null);try{OfflineOrdersStore os=new OfflineOrdersStore(store);int waiting=os.waitingCount(),photos=os.pendingPhotoUploadCount();card("Offline order queue",waiting+" unsynced change(s) • "+photos+" photo(s) waiting",waiting==0&&photos==0?"CLEAR":"WAITING",null);}catch(Exception e){card("Offline order queue","Could not read phone queue","CHECK",null);}card("PC bridge",d.optString("version").isEmpty()?"Unavailable":"Connected • "+d.optString("version"),company,null);'
if anchor not in s: raise SystemExit('system diagnostics anchor missing')
s=s.replace(anchor,extra,1);p.write_text(s,encoding='utf-8')

# Desktop: organize navigation groups and add a dedicated Customers entry.
p=Path('internal/app/web/operations.js'); s=p.read_text(encoding='utf-8')
s=s.replace("for(const [k,t] of [['work','Today'],['global-search','Search'],['followups','Follow-ups']])", "for(const [k,t] of [['work','Today'],['customers','Customers'],['global-search','Search'],['followups','Follow-ups']])",1)
s=s.replace("render=function(){install();if(!['work','global-search','followups'].includes(route)){baseRender();return}", "function organize(){const side=document.querySelector('.side-nav');if(!side||side.dataset.organized==='1')return;const groups=[['DAILY WORK',['work','orders','followups']],['CUSTOMERS & FIND',['customers','global-search','companies']],['BUSINESS SETUP',['pdfbank','health']],['SYSTEM',['diag','devices','cloudflare','live','backup','updates','settings','about']]];for(const [label,keys] of groups){const h=document.createElement('div');h.textContent=label;h.style.cssText='padding:14px 14px 6px;font-size:10px;font-weight:800;letter-spacing:.14em;opacity:.62';side.appendChild(h);for(const key of keys){const b=side.querySelector(`[data-route=\"${key}\"]`);if(b)side.appendChild(b)}}side.dataset.organized='1'}\n render=function(){install();organize();if(!['work','customers','global-search','followups'].includes(route)){baseRender();return}",1)
s=s.replace("if(route==='work')renderWork();else if(route==='global-search')renderSearch();else renderFollowups();", "if(route==='work')renderWork();else if(route==='customers')renderCustomers();else if(route==='global-search')renderSearch();else renderFollowups();",1)
insert="""
 function renderCustomers(){const p=$('#page');if(!p)return;p.innerHTML=head('Customers','Open Customer 360 from the synchronized Tally ledger list.')+companyBar()+`<div class=\"card\"><input id=\"customerQ\" class=\"input\" placeholder=\"Search customer / ledger\" style=\"width:100%\"></div><div id=\"customerResults\"></div>`;const paint=()=>{const box=$('#customerResults');if(!box)return;const q=($('#customerQ')?.value||'').trim().toLowerCase(),xs=(searchData?.results||[]).filter(x=>x.kind==='ledger'&&(!q||(x.title+' '+(x.subtitle||'')).toLowerCase().includes(q))).slice(0,100);box.innerHTML=xs.length?`<div class=\"card\" style=\"margin-top:14px\">${xs.map(x=>`<div class=\"list-row clickable\" data-customer=\"${esc(x.party||x.ledger||x.title)}\"><div><div class=\"title\">${esc(x.title)}</div><div class=\"sub\">${esc(x.subtitle||'Customer ledger')}</div></div><b>Open</b></div>`).join('')}</div>`:'<div class=\"empty\" style=\"margin-top:14px\">No matching customer.</div>'};$('#customerQ')?.addEventListener('input',paint);if(searchData)paint();else loadSearch().then(paint)}
"""
marker=' async function loadSearch()'
if marker not in s: raise SystemExit('desktop customer insertion anchor missing')
s=s.replace(marker,insert+marker,1)
s=s.replace("if(['work','global-search','followups'].includes(route))render();", "if(['work','customers','global-search','followups'].includes(route))render();",1)
p.write_text(s,encoding='utf-8')

# Regression coverage for frequent items, overdue alerts and invoice-item search.
p=Path('internal/app/operations_240_test.go'); s=p.read_text(encoding='utf-8')
append=r'''

func TestSmartAlertsIncludeOverdueSavedBills(t *testing.T) {
	s := newOrderTestServer(t)
	yesterday := time.Now().Add(-24*time.Hour).Format("2006-01-02")
	x := PartyOutstandingData{Party:"ABC Traders", Bills:[]BillRow{{BillNumber:"INV1",DueDate:yesterday,PendingAmount:2500}}}
	if err:=s.cache.Put("party-outstanding","Firm","ABC Traders",x,time.Now()); err!=nil { t.Fatal(err) }
	alerts:=s.smartAlerts("Firm")
	found:=false; for _,a:=range alerts { if a.ID=="customer-overdue" { found=true; if !strings.Contains(a.Detail,"2,500") { t.Fatalf("unexpected detail %q",a.Detail) } } }
	if !found { t.Fatal("expected overdue customer alert") }
}

func TestCustomerWorkspaceBuildsPurchaseSignalsFromSavedInvoiceDetails(t *testing.T) {
	s := newOrderTestServer(t)
	v:=VoucherDetail{Company:"Firm",CustomerName:"ABC Traders",InvoiceNumber:"101",InvoiceDate:"2026-10-01",VoucherType:"Sales",Items:[]InvoiceItem{{Name:"Ashoka"},{Name:"Ashoka"},{Name:"Shubhangi"}}}
	if err:=s.cache.Put("voucher-detail-v6","Firm","fixture",v,time.Now()); err!=nil { t.Fatal(err) }
	x,err:=s.customerWorkspace("Firm","ABC Traders",false); if err!=nil && !errors.Is(err,osErrNotExist) { t.Fatal(err) }
	if x.LastPurchaseDate!="2026-10-01" || len(x.FrequentItems)==0 || x.FrequentItems[0].Name!="Ashoka" { t.Fatalf("missing purchase signals: %#v",x) }
}

func TestGlobalSearchIndexesInvoiceProducts(t *testing.T) {
	s := newOrderTestServer(t)
	v:=VoucherDetail{Company:"Firm",CustomerName:"ABC Traders",InvoiceNumber:"102",InvoiceDate:"2026-10-02",VoucherType:"Sales",Items:[]InvoiceItem{{Name:"Ashoka Towel",HSN:"6302",Description:"Red border"}}}
	if err:=s.cache.Put("voucher-detail-v6","Firm","fixture2",v,time.Now()); err!=nil { t.Fatal(err) }
	x:=s.buildGlobalSearch("Firm"); found:=false; for _,r:=range x.Results { if strings.Contains(r.SearchText,"ashoka towel")&&strings.Contains(r.SearchText,"6302") { found=true } }
	if !found { t.Fatal("invoice product was not indexed") }
}
'''
if 'TestSmartAlertsIncludeOverdueSavedBills' not in s:
    s += append
p.write_text(s,encoding='utf-8')
# tests now need errors import.
p=Path('internal/app/operations_240_test.go'); s=p.read_text(encoding='utf-8')
if '"errors"' not in s.split(')')[0]: s=s.replace('import (\n', 'import (\n\t"errors"\n',1)
p.write_text(s,encoding='utf-8')

print('final Operations Center hardening applied')
