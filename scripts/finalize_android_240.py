from pathlib import Path


def rep(path, old, new, count=1):
    p=Path(path); s=p.read_text(encoding='utf-8')
    if s.count(old) < count:
        raise SystemExit(f'{path}: anchor missing: {old[:100]!r}')
    s=s.replace(old,new,count); p.write_text(s,encoding='utf-8')

orders='android/app/src/main/java/in/tallybridge/app/OrdersActivity.java'
rep(orders,' static final String ACTION_NEW="in.tallybridge.app.ORDER_NEW";',' static final String ACTION_NEW="in.tallybridge.app.ORDER_NEW", ACTION_QUICK="in.tallybridge.app.ORDER_QUICK";')
rep(orders,' private boolean dispatchedTab=false,destroyed=false,mastersLoading=false,dark=false;',' private boolean dispatchedTab=false,destroyed=false,mastersLoading=false,dark=false,quickMode=false;')
rep(orders,'  if(ACTION_NEW.equals(getIntent().getAction()))openForm(null);else showOrders();','  String action=getIntent().getAction();quickMode=ACTION_QUICK.equals(action);if(ACTION_NEW.equals(action)||quickMode)openForm(null);else showOrders();')
rep(orders,'  frame(order==null?"Add order":"Edit order","Offline-ready • saved buyer/item lists from Tally • rate is optional.");','  frame(order==null?(quickMode?"Quick order":"Add order"):"Edit order",quickMode&&order==null?"Party • items • quantity • dispatch • save. Works offline.":"Offline-ready • saved buyer/item lists from Tally • rate is optional.");')
old='''  body.addView(txt("NOTES",11,MUTED),full(16));EditText notes=input("Colours, packing, delivery instructions…",InputType.TYPE_CLASS_TEXT|InputType.TYPE_TEXT_FLAG_MULTI_LINE);notes.setMinLines(2);if(order!=null)notes.setText(order.optString("notes"));body.addView(notes,full(6));if(order==null)body.addView(btn("🎙 Speak order",v->startVoiceOrder(buyer,notes,itemsBox)),full(8));body.addView(primary(order==null?"Save pending order":"Update pending order",v->saveForm(order,companySpin,buyer,dispatchedThrough,notes)),full(18));
'''
new='''  EditText notes=input("Colours, packing, delivery instructions…",InputType.TYPE_CLASS_TEXT|InputType.TYPE_TEXT_FLAG_MULTI_LINE);notes.setMinLines(2);if(order!=null)notes.setText(order.optString("notes"));if(!quickMode||order!=null){body.addView(txt("NOTES",11,MUTED),full(16));body.addView(notes,full(6));}if(order==null)body.addView(btn("🎙 Speak order",v->startVoiceOrder(buyer,notes,itemsBox)),full(8));body.addView(primary(order==null?(quickMode?"Save quick order":"Save pending order"):"Update pending order",v->saveForm(order,companySpin,buyer,dispatchedThrough,notes)),full(18));
'''
rep(orders,old,new)

p=Path(orders);s=p.read_text(encoding='utf-8');start=s.find(' private void applyVoiceOrder(String spoken){\n');end=s.find(' private void refreshBootstrapSilently(){',start)
if start<0 or end<0: raise SystemExit('voice order method anchors missing')
voice=r''' private double voiceQty(String lower,String item){
  String q=java.util.regex.Pattern.quote(item.toLowerCase(Locale.ROOT));
  java.util.regex.Pattern[] ps=new java.util.regex.Pattern[]{java.util.regex.Pattern.compile("([0-9]+(?:[.][0-9]+)?)\\s*(?:dozen|dz|pcs?|pieces?)?\\s+(?:of\\s+)?"+q),java.util.regex.Pattern.compile(q+"\\s*[-,:]?\\s*([0-9]+(?:[.][0-9]+)?)\\s*(?:dozen|dz|pcs?|pieces?)?")};
  for(java.util.regex.Pattern p:ps){java.util.regex.Matcher m=p.matcher(lower);if(m.find())try{return Double.parseDouble(m.group(1));}catch(Exception ignored){}}
  return 1;
 }
 private void applyVoiceOrder(String spoken){
  if(voiceNotes!=null&&!quickMode)voiceNotes.setText(spoken);String lower=spoken.toLowerCase(Locale.ROOT);
  if(voiceBuyer!=null&&parties!=null){String best="";for(int i=0;i<parties.length();i++){JSONObject p=parties.optJSONObject(i);String n=p==null?parties.optString(i):p.optString("name");if(!n.isEmpty()&&lower.contains(n.toLowerCase(Locale.ROOT))&&n.length()>best.length())best=n;}if(!best.isEmpty())voiceBuyer.setText(best,false);}
  List<JSONObject> matches=new ArrayList<>();Set<String> used=new HashSet<>();if(stock!=null){List<JSONObject> candidates=new ArrayList<>();for(int i=0;i<stock.length();i++){JSONObject x=stock.optJSONObject(i);if(x!=null&&!x.optString("name").isEmpty()&&lower.contains(x.optString("name").toLowerCase(Locale.ROOT)))candidates.add(x);}candidates.sort((x,y)->Integer.compare(y.optString("name").length(),x.optString("name").length()));for(JSONObject x:candidates){String name=x.optString("name");if(used.add(name.toLowerCase(Locale.ROOT)))matches.add(x);}}
  int added=0;for(JSONObject matched:matches){String name=matched.optString("name");double q=voiceQty(lower,name);JSONObject existing=new JSONObject();try{existing.put("name",name).put("quantity",q).put("unit",matched.optString("unit"));}catch(Exception ignored){}if(added==0&&itemRows.size()==1&&itemRows.get(0).item.getText().toString().trim().isEmpty()){ItemRow r=itemRows.get(0);r.item.setText(name,false);r.qty.setText(qty(q));r.unit.setText(matched.optString("unit"));}else if(voiceItemsBox!=null)addItemRow(voiceItemsBox,existing);added++;}
  toast(added>0?("Voice order captured • "+added+" item"+(added==1?"":"s")+" found • review before saving"):"Voice captured • choose item and quantity before saving");
 }
'''
s=s[:start]+voice+s[end:];p.write_text(s,encoding='utf-8')

store='android/app/src/main/java/in/tallybridge/app/OfflineOrdersStore.java'
rep(store,' int waitingCount()throws Exception {JSONObject s=state();JSONArray a=s.getJSONArray("orders");int n=0;for(int i=0;i<a.length();i++){if(hasLocalMutation(a.optJSONObject(i)))n++;}return n;}',' int waitingCount()throws Exception {JSONObject s=state();JSONArray a=s.getJSONArray("orders");int n=0;for(int i=0;i<a.length();i++){if(hasLocalMutation(a.optJSONObject(i)))n++;}return n;}\n int pendingPhotoUploadCount()throws Exception {JSONObject s=state();JSONArray a=s.getJSONArray("orders");int n=0;for(int i=0;i<a.length();i++){JSONObject o=a.optJSONObject(i);if(o==null||!hasLocalMutation(o))continue;JSONArray photos=o.optJSONArray("attachments");if(photos==null)continue;for(int j=0;j<photos.length();j++){JSONObject p=photos.optJSONObject(j);if(p!=null&&!p.optString("localPath").isEmpty())n++;}}return n;}')

work='android/app/src/main/java/in/tallybridge/app/WorkActivity.java'
rep(work,'Button q=primary("+ Quick order",v->{Intent i=new Intent(this,OrdersActivity.class);i.setAction(OrdersActivity.ACTION_NEW);startActivity(i);});','Button q=primary("+ Quick order",v->{Intent i=new Intent(this,OrdersActivity.class);i.setAction(OrdersActivity.ACTION_QUICK);startActivity(i);});')
p=Path(work);s=p.read_text(encoding='utf-8')
marker='body.addView(primary("+ Add follow-up",v->followUpDialog(party,null)),full(12));'
if marker not in s: raise SystemExit('customer 360 UI marker missing')
customer_extra='String last=x.optString("lastPurchaseDate");if(!last.isEmpty())card("Last purchase",last,"",null);JSONArray frequent=x.optJSONArray("frequentItems");heading("Frequently purchased",frequent==null||frequent.length()==0?"Saved invoice details will build this list.":"From saved Sales invoice details");if(frequent!=null)for(int i=0;i<frequent.length();i++){JSONObject f=frequent.optJSONObject(i);if(f!=null)card(f.optString("name"),f.optInt("count")+" saved invoice(s)","",null);}'
s=s.replace(marker,marker+customer_extra,1)
anchor='card("SHA-256",AppIdentity.sha256(this),"",null);'
if anchor not in s: raise SystemExit('system diagnostics marker missing')
system_extra='card("SHA-256",AppIdentity.sha256(this),"",null);card("Package",getPackageName(),"",null);card("Google access / phone key",store.get("device").isEmpty()?"Not approved":"Approved phone key present","",null);JSONObject cache=d.optJSONObject("cache");if(cache!=null)card("Saved cache",cache.optInt("entries")+" entries • "+cache.optString("latestAt"),"",null);try{OfflineOrdersStore os=new OfflineOrdersStore(store);int waiting=os.waitingCount(),photos=os.pendingPhotoUploadCount();card("Offline order queue",waiting+" unsynced change(s) • "+photos+" photo(s) waiting",waiting==0&&photos==0?"CLEAR":"WAITING",null);}catch(Exception e){card("Offline order queue","Could not read phone queue","CHECK",null);}card("PC bridge",d.optString("version").isEmpty()?"Unavailable":"Connected • "+d.optString("version"),company,null);'
s=s.replace(anchor,system_extra,1);p.write_text(s,encoding='utf-8')
print('android hardening applied')
