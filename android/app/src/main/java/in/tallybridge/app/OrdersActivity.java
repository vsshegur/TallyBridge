package in.tallybridge.app;

import android.app.*;
import android.content.*;
import android.graphics.Color;
import android.graphics.Typeface;
import android.graphics.drawable.GradientDrawable;
import android.os.*;
import android.text.InputType;
import android.view.*;
import android.widget.*;
import org.json.*;
import java.text.NumberFormat;
import java.time.*;
import java.time.format.DateTimeFormatter;
import java.util.*;
import java.util.concurrent.*;

/** Fast native order notebook. Orders remain usable offline and sync to the Windows bridge later. */
public final class OrdersActivity extends Activity {
 static final String ACTION_NEW="in.tallybridge.app.ORDER_NEW";
 private final ExecutorService work=Executors.newSingleThreadExecutor();
 private final Handler main=new Handler(Looper.getMainLooper());
 private CryptoStore store; private BridgeApi api; private OfflineOrdersStore offline;
 private LinearLayout root,body; private ProgressBar progress; private TextView syncState;
 private JSONObject boot=new JSONObject(),orderData=new JSONObject(),editing;
 private JSONArray companies=new JSONArray(),parties=new JSONArray(),stock=new JSONArray();
 private final Map<String,String> unitByItem=new HashMap<>();
 private final List<ItemRow> itemRows=new ArrayList<>();
 private JSONObject buyerDetails=new JSONObject(); private String buyerDetailsFor="";
 private boolean dispatchedTab=false,destroyed=false,mastersLoading=false;
 private int ordersHeaderCount=0;
 private final int NAVY=0xff102d44,TEAL=0xff176b5b,MUTED=0xff596c79,BG=0xfff5f7fa,SURFACE=Color.WHITE,BORDER=0xffe4eaef,TINT=0xffe5f2ee,AMBER=0xff8a5a00;
 private final NumberFormat currency=NumberFormat.getCurrencyInstance(new Locale("en","IN"));

 static final class ItemRow {
  final LinearLayout box; final AutoCompleteTextView item; final EditText qty,rate; final TextView unit;
  ItemRow(LinearLayout b,AutoCompleteTextView i,EditText q,EditText r,TextView u){box=b;item=i;qty=q;rate=r;unit=u;}
 }

 @Override public void onCreate(Bundle state){
  super.onCreate(state);getWindow().addFlags(WindowManager.LayoutParams.FLAG_SECURE);currency.setCurrency(Currency.getInstance("INR"));
  try{store=new CryptoStore(this);api=new BridgeApi(store);offline=new OfflineOrdersStore(store);}catch(Exception e){fatal(e);return;}
  if(store.get("base").isEmpty()||store.get("device").isEmpty()){needsMain("Connect and approve this phone in TallyBridge once. After that, Orders works even when the PC is offline.");return;}
  if(ACTION_NEW.equals(getIntent().getAction()))openForm(null);else showOrders();
 }
 @Override protected void onDestroy(){destroyed=true;work.shutdownNow();main.removeCallbacksAndMessages(null);super.onDestroy();}

 private int dp(float v){return Math.round(v*getResources().getDisplayMetrics().density);}
 private GradientDrawable shape(int color,int radius){GradientDrawable d=new GradientDrawable();d.setColor(color);d.setCornerRadius(dp(radius));d.setStroke(dp(1),BORDER);return d;}
 private LinearLayout col(){LinearLayout l=new LinearLayout(this);l.setOrientation(LinearLayout.VERTICAL);return l;}
 private TextView txt(String s,int size,int color){TextView v=new TextView(this);v.setText(s);v.setTextSize(size);v.setTextColor(color);v.setPadding(0,dp(4),0,dp(4));v.setLineSpacing(dp(2),1f);return v;}
 private void gap(LinearLayout l,int h){l.addView(new View(this),new LinearLayout.LayoutParams(1,dp(h)));}
 private Button btn(String s,View.OnClickListener c){Button b=new Button(this);b.setText(s);b.setAllCaps(false);b.setTextSize(15);b.setTextColor(TEAL);b.setBackground(shape(SURFACE,13));b.setMinHeight(dp(50));b.setOnClickListener(c);return b;}
 private Button primary(String s,View.OnClickListener c){Button b=btn(s,c);b.setTextColor(Color.WHITE);b.setBackground(shape(TEAL,13));return b;}
 private EditText input(String hint,int type){EditText e=new EditText(this);e.setHint(hint);e.setTextSize(16);e.setTextColor(NAVY);e.setHintTextColor(MUTED);e.setInputType(type);e.setPadding(dp(14),dp(12),dp(14),dp(12));e.setBackground(shape(SURFACE,12));e.setMinHeight(dp(52));return e;}
 private AutoCompleteTextView auto(String hint){AutoCompleteTextView a=new AutoCompleteTextView(this);a.setHint(hint);a.setTextSize(16);a.setTextColor(NAVY);a.setHintTextColor(MUTED);a.setPadding(dp(14),dp(12),dp(14),dp(12));a.setBackground(shape(SURFACE,12));a.setThreshold(1);a.setSingleLine(true);return a;}
 private LinearLayout.LayoutParams full(int top){LinearLayout.LayoutParams p=new LinearLayout.LayoutParams(-1,-2);p.setMargins(0,dp(top),0,0);return p;}
 private void fatal(Exception e){TextView v=txt("Orders could not start.\n"+e.getMessage(),16,NAVY);v.setPadding(dp(24),dp(60),dp(24),dp(24));setContentView(v);}
 private void needsMain(String message){LinearLayout p=col();p.setPadding(dp(24),dp(60),dp(24),dp(24));p.addView(txt("TallyBridge Orders",28,NAVY));p.addView(txt(message,16,MUTED),full(16));p.addView(primary("Open TallyBridge",v->startActivity(new Intent(this,MainActivity.class))),full(24));setContentView(p);}
 private void frame(String title,String subtitle){root=col();root.setBackgroundColor(BG);root.setPadding(dp(18),dp(18),dp(18),dp(18));LinearLayout h=new LinearLayout(this);h.setGravity(Gravity.CENTER_VERTICAL);LinearLayout titles=col();TextView t=txt(title,26,NAVY);t.setTypeface(Typeface.DEFAULT_BOLD);titles.addView(t);titles.addView(txt(subtitle,13,MUTED));h.addView(titles,new LinearLayout.LayoutParams(0,-2,1));Button close=btn("Back",v->finish());h.addView(close,new LinearLayout.LayoutParams(dp(84),-2));root.addView(h);progress=new ProgressBar(this,null,android.R.attr.progressBarStyleHorizontal);progress.setIndeterminate(true);progress.setVisibility(View.GONE);progress.setIndeterminateTintList(android.content.res.ColorStateList.valueOf(TEAL));root.addView(progress,new LinearLayout.LayoutParams(-1,dp(3)));ScrollView scroll=new ScrollView(this);body=col();body.setPadding(0,dp(14),0,dp(26));scroll.addView(body);root.addView(scroll,new LinearLayout.LayoutParams(-1,0,1));setContentView(root);}
 private void busy(boolean yes){if(progress!=null)progress.setVisibility(yes?View.VISIBLE:View.GONE);}
 private void toast(String s){Toast.makeText(this,s,Toast.LENGTH_LONG).show();}
 private String shortError(Exception e){String m=e.getMessage();if(m==null||m.trim().isEmpty())return "PC unavailable";m=m.replace('\n',' ').trim();return m.length()>72?m.substring(0,72)+"…":m;}

 private List<String> companyNames(){List<String> out=new ArrayList<>();for(int i=0;i<companies.length();i++){JSONObject x=companies.optJSONObject(i);String n=x==null?companies.optString(i):x.optString("name");if(!n.isEmpty()&&!out.contains(n))out.add(n);}return out;}

 private void showOrders(){
  frame("Orders","Works offline • saved securely on this phone • syncs to your PC automatically.");
  syncState=txt("Phone copy ready",12,TEAL);syncState.setTypeface(Typeface.DEFAULT_BOLD);body.addView(syncState);
  body.addView(primary("+ Add order",v->openForm(null)),full(8));
  LinearLayout tabs=new LinearLayout(this);tabs.setPadding(0,dp(12),0,dp(8));Button pending=btn("Pending",v->{dispatchedTab=false;paintOrders();}),dispatched=btn("Dispatched",v->{dispatchedTab=true;paintOrders();});tabs.addView(pending,new LinearLayout.LayoutParams(0,-2,1));tabs.addView(dispatched,new LinearLayout.LayoutParams(0,-2,1));body.addView(tabs);
  body.addView(txt("Pending orders can be edited/shared offline. Dispatched orders stay view-only and are removed after 7 days.",12,MUTED));
  ordersHeaderCount=body.getChildCount();loadOrders();
 }
 private void loadOrders(){
  try{orderData=offline.list();paintOrders();OrdersWidget.refreshAll(this);int waiting=offline.waitingCount();syncState.setText(waiting>0?waiting+" change(s) saved on phone • checking PC…":"Saved on phone • checking PC…");syncState.setTextColor(waiting>0?AMBER:TEAL);}catch(Exception e){toast("Could not read saved Orders: "+shortError(e));return;}
  syncOrdersSilently();
 }
 private void syncOrdersSilently(){
  busy(true);work.execute(()->{try{JSONObject synced=offline.sync(api);main.post(()->{if(destroyed)return;busy(false);orderData=synced;if(syncState!=null){syncState.setText("Synced with PC • offline copy is up to date");syncState.setTextColor(TEAL);}paintOrders();OrdersWidget.refreshAll(this);});}catch(Exception e){main.post(()->{if(destroyed)return;busy(false);try{orderData=offline.list();}catch(Exception ignored){}if(syncState!=null){syncState.setText("OFFLINE • saved on phone • will sync automatically");syncState.setTextColor(AMBER);}paintOrders();});}});
 }
 private void paintOrders(){
  if(body==null)return;while(body.getChildCount()>ordersHeaderCount)body.removeViewAt(ordersHeaderCount);
  int count=dispatchedTab?orderData.optInt("dispatchedCount"):orderData.optInt("pendingCount");TextView head=txt((dispatchedTab?"Dispatched":"Pending")+" • "+count,20,NAVY);head.setTypeface(Typeface.DEFAULT_BOLD);body.addView(head,full(10));
  JSONArray a=orderData.optJSONArray(dispatchedTab?"dispatched":"pending");if(a==null||a.length()==0){body.addView(txt(dispatchedTab?"No dispatched orders from the last 7 days.":"No pending orders. Tap + Add order — it works without the PC.",15,MUTED),full(18));return;}
  for(int i=0;i<a.length();i++){JSONObject o=a.optJSONObject(i);if(o!=null)body.addView(orderCard(o,!dispatchedTab),full(12));}
 }
 private View orderCard(JSONObject o,boolean pending){
  LinearLayout card=col();card.setPadding(dp(16),dp(14),dp(16),dp(14));card.setBackground(shape(SURFACE,14));TextView title=txt(o.optString("number","Order")+" • "+o.optString("buyer"),18,NAVY);title.setTypeface(Typeface.DEFAULT_BOLD);card.addView(title);card.addView(txt(o.optString("company")+(pending?" • Pending":" • Dispatched "+dateTime(o.optString("dispatchedAt"))),12,MUTED));
  if(o.optBoolean("_localOnly")||o.optBoolean("_dirty")||o.optBoolean("_dispatchDirty")){TextView waiting=txt("● Saved on phone • waiting to sync",12,AMBER);waiting.setTypeface(Typeface.DEFAULT_BOLD);card.addView(waiting);}
  JSONArray items=o.optJSONArray("items");if(items!=null){for(int j=0;j<items.length();j++){JSONObject x=items.optJSONObject(j);if(x==null)continue;String line=(j+1)+". "+x.optString("name")+" — Qty "+qty(x.optDouble("quantity"))+(x.optString("unit").isEmpty()?"":" "+x.optString("unit"));if(x.has("rate")&&!x.isNull("rate"))line+=" • "+currency.format(x.optDouble("rate"));card.addView(txt(line,14,NAVY),full(5));}}
  if(!o.optString("notes").isEmpty())card.addView(txt("Notes: "+o.optString("notes"),13,MUTED),full(8));
  if(pending){LinearLayout actions=new LinearLayout(this);actions.setPadding(0,dp(12),0,0);actions.addView(btn("Details only",v->shareOrder(o)),new LinearLayout.LayoutParams(0,-2,1));actions.addView(btn("Edit",v->openForm(o)),new LinearLayout.LayoutParams(0,-2,1));actions.addView(primary("Dispatched",v->dispatch(o)),new LinearLayout.LayoutParams(0,-2,1));card.addView(actions);}else card.addView(txt("View only • no sharing after dispatch",12,MUTED),full(10));return card;
 }
 private String qty(double v){return Math.rint(v)==v?String.valueOf((long)v):String.valueOf(v);}
 private String dateTime(String v){try{return OffsetDateTime.parse(v).toLocalDateTime().format(DateTimeFormatter.ofPattern("dd-MM-yyyy hh:mm a"));}catch(Exception e){try{return Instant.parse(v).atZone(ZoneId.systemDefault()).format(DateTimeFormatter.ofPattern("dd-MM-yyyy hh:mm a"));}catch(Exception ignored){return "";}}}

 private void shareOrder(JSONObject o){
  String text=o.optString("shareText");if(text.isEmpty()){toast("Only pending orders can be shared");return;}
  ScrollView scroll=new ScrollView(this);TextView preview=txt(text,15,NAVY);preview.setTextIsSelectable(true);preview.setPadding(dp(20),dp(12),dp(20),dp(12));scroll.addView(preview);
  new AlertDialog.Builder(this).setTitle("Details only").setMessage("Review before sharing").setView(scroll).setNegativeButton("Cancel",null).setPositiveButton("Share",(d,w)->{Intent i=new Intent(Intent.ACTION_SEND).setType("text/plain").putExtra(Intent.EXTRA_TEXT,text);startActivity(Intent.createChooser(i,"Share order details"));}).show();
 }
 private void dispatch(JSONObject o){
  new AlertDialog.Builder(this).setTitle("Mark dispatched?").setMessage(o.optString("number")+" • "+o.optString("buyer")+"\n\nThis works offline. It will sync to the PC later, remain view-only for 7 days, then be removed.").setNegativeButton("Cancel",null).setPositiveButton("Dispatched",(d,w)->{try{offline.dispatchLocal(o);OrdersWidget.refreshAll(this);toast("Marked dispatched on phone");showOrders();}catch(Exception e){toast(e.getMessage());}}).show();
 }

 private void openForm(JSONObject order){
  editing=order;try{companies=offline.companies();}catch(Exception ignored){companies=new JSONArray();}
  if(companies.length()==0){String c=store.get("company");if(!c.isEmpty()){JSONObject fallback=new JSONObject();try{fallback.put("name",c);companies.put(fallback);}catch(Exception ignored){}}}
  if(companies.length()>0){buildForm(order);refreshBootstrapSilently();return;}
  frame(order==null?"Add order":"Edit order","Loading company list. After the first sync, this screen works offline.");body.addView(txt("No saved company list yet. Trying the PC once…",15,MUTED));busy(true);work.execute(()->{try{JSONObject x=api.get("/api/v1/mobile/bootstrap");offline.cacheBootstrap(x);JSONArray cs=offline.companies();main.post(()->{if(destroyed)return;busy(false);companies=cs;buildForm(order);});}catch(Exception e){main.post(()->{if(!destroyed){busy(false);body.addView(txt("PC is offline and this phone has no saved company list yet. Open Orders once while connected, then it will work offline.",15,AMBER),full(16));}});}});
 }
 private void refreshBootstrapSilently(){work.execute(()->{try{JSONObject x=api.get("/api/v1/mobile/bootstrap");offline.cacheBootstrap(x);}catch(Exception ignored){}});}
 private void buildForm(JSONObject order){
  frame(order==null?"Add order":"Edit order","Offline-ready • saved buyer/item lists from Tally • rate is optional.");itemRows.clear();buyerDetails=order==null?new JSONObject():order.optJSONObject("buyerDetails");if(buyerDetails==null)buyerDetails=new JSONObject();buyerDetailsFor=order==null?"":order.optString("buyer");List<String> names=companyNames();if(names.isEmpty()){body.addView(txt("No saved company is available yet.",16,AMBER));return;}
  TextView offlineNote=txt("Changes save immediately on this phone. PC connection is not required.",12,TEAL);offlineNote.setTypeface(Typeface.DEFAULT_BOLD);body.addView(offlineNote);
  body.addView(txt("COMPANY",11,MUTED),full(10));Spinner companySpin=new Spinner(this);companySpin.setAdapter(new ArrayAdapter<>(this,android.R.layout.simple_spinner_dropdown_item,names));String wanted=order==null?store.get("company"):order.optString("company");companySpin.setSelection(Math.max(0,names.indexOf(wanted)));body.addView(companySpin,full(6));
  body.addView(txt("BUYER",11,MUTED),full(14));AutoCompleteTextView buyer=auto("Search saved Tally buyer or type name");if(order!=null)buyer.setText(order.optString("buyer"),false);body.addView(buyer,full(6));TextView buyerInfo=txt(buyerSummary(buyerDetails),12,MUTED);body.addView(buyerInfo);
  LinearLayout itemsBox=col();body.addView(txt("ITEMS",11,MUTED),full(16));body.addView(itemsBox);JSONArray existing=order==null?null:order.optJSONArray("items");if(existing!=null&&existing.length()>0){for(int i=0;i<existing.length();i++)addItemRow(itemsBox,existing.optJSONObject(i));}else addItemRow(itemsBox,null);body.addView(btn("+ Add another item",v->addItemRow(itemsBox,null)),full(8));
  body.addView(txt("NOTES",11,MUTED),full(16));EditText notes=input("Colours, packing, delivery instructions…",InputType.TYPE_CLASS_TEXT|InputType.TYPE_TEXT_FLAG_MULTI_LINE);notes.setMinLines(2);if(order!=null)notes.setText(order.optString("notes"));body.addView(notes,full(6));body.addView(primary(order==null?"Save pending order":"Update pending order",v->saveForm(order,companySpin,buyer,notes)),full(18));
  final String[] loadedCompany={""};companySpin.setOnItemSelectedListener(new android.widget.AdapterView.OnItemSelectedListener(){public void onNothingSelected(android.widget.AdapterView<?> p){}public void onItemSelected(android.widget.AdapterView<?> p,View v,int pos,long id){String c=names.get(pos);store.put("company",c);if(!c.equals(loadedCompany[0])){if(!loadedCompany[0].isEmpty()){buyer.setText("",false);buyerDetails=new JSONObject();buyerDetailsFor="";buyerInfo.setText(buyerSummary(buyerDetails));}loadedCompany[0]=c;loadMasters(c,buyer,buyerInfo);}}});
  buyer.setOnItemClickListener((p,v,pos,id)->fetchBuyer(String.valueOf(companySpin.getSelectedItem()),buyer.getText().toString(),buyerInfo));buyer.setOnFocusChangeListener((v,has)->{if(!has&&!buyer.getText().toString().trim().isEmpty())fetchBuyer(String.valueOf(companySpin.getSelectedItem()),buyer.getText().toString(),buyerInfo);});
 }

 private void loadMasters(String company,AutoCompleteTextView buyer,TextView buyerInfo){
  try{JSONObject cached=offline.masters(company);if(cached.length()>0)applyMasters(cached,buyer,buyerInfo);}catch(Exception ignored){}
  if(mastersLoading)return;mastersLoading=true;work.execute(()->{try{JSONObject x=api.get(BridgeApi.path("orders/masters",company));offline.cacheMasters(company,x);main.post(()->{if(!destroyed){mastersLoading=false;applyMasters(x,buyer,buyerInfo);}});}catch(Exception e){main.post(()->{mastersLoading=false;});}});
 }
 private void applyMasters(JSONObject x,AutoCompleteTextView buyer,TextView buyerInfo){parties=x.optJSONArray("parties");if(parties==null)parties=new JSONArray();JSONObject st=x.optJSONObject("stock");stock=st==null?new JSONArray():st.optJSONArray("items");if(stock==null)stock=new JSONArray();List<String> buyers=new ArrayList<>();for(int i=0;i<parties.length();i++){JSONObject p=parties.optJSONObject(i);if(p!=null&&!p.optString("name").isEmpty())buyers.add(p.optString("name"));}buyer.setAdapter(new ArrayAdapter<>(this,android.R.layout.simple_dropdown_item_1line,buyers));unitByItem.clear();for(int i=0;i<stock.length();i++){JSONObject s=stock.optJSONObject(i);if(s!=null)unitByItem.put(s.optString("name"),s.optString("unit"));}updateItemAdapters();if(!buyer.getText().toString().trim().isEmpty()&&buyerDetails.length()==0)fetchBuyer(store.get("company"),buyer.getText().toString(),buyerInfo);}
 private void fetchBuyer(String company,String buyer,TextView info){
  String selectedBuyer=buyer.trim();if(selectedBuyer.isEmpty())return;try{JSONObject cached=offline.buyer(company,selectedBuyer);if(cached.length()>0){buyerDetails=cached;buyerDetailsFor=selectedBuyer;info.setText(buyerSummary(buyerDetails));}}catch(Exception ignored){}
  work.execute(()->{try{JSONObject x=api.get(BridgeApi.path("ledger-details",company,"ledger",selectedBuyer));JSONObject d=x.optJSONObject("details");if(d==null)d=new JSONObject();offline.cacheBuyer(company,selectedBuyer,d);JSONObject finalD=d;main.post(()->{if(!destroyed){buyerDetails=finalD;buyerDetailsFor=selectedBuyer;info.setText(buyerSummary(finalD));}});}catch(Exception ignored){}});
 }
 private String buyerSummary(JSONObject d){if(d==null||d.length()==0)return "Saved contact details appear here when available. Buyer name can still be typed offline.";String phone=first(d,"MOBILENUMBER","MOBILE","PHONENUMBER","PHONE"),address=first(d,"ADDRESS"),city=first(d,"CITY"),gst=first(d,"PARTYGSTIN","GSTIN");StringBuilder b=new StringBuilder();if(!phone.isEmpty())b.append("Mobile: ").append(phone);if(!address.isEmpty())b.append(b.length()>0?"\n":"").append(address);if(!city.isEmpty())b.append(address.isEmpty()?(b.length()>0?"\n":""):", ").append(city);if(!gst.isEmpty())b.append(b.length()>0?"\n":"").append("GSTIN: ").append(gst);return b.length()==0?"Buyer selected. No saved contact fields.":b.toString();}
 private String first(JSONObject d,String...keys){if(d==null)return "";for(String k:keys){String v=d.optString(k).trim();if(!v.isEmpty())return v;}return "";}

 private void addItemRow(LinearLayout parent,JSONObject existing){
  LinearLayout box=col();box.setPadding(dp(12),dp(12),dp(12),dp(12));box.setBackground(shape(SURFACE,12));AutoCompleteTextView item=auto("Search saved Tally item or type product");if(existing!=null)item.setText(existing.optString("name"),false);box.addView(item);TextView unit=txt(existing==null?"":existing.optString("unit"),12,MUTED);box.addView(unit);LinearLayout line=new LinearLayout(this);EditText qty=input("Qty",InputType.TYPE_CLASS_NUMBER|InputType.TYPE_NUMBER_FLAG_DECIMAL),rate=input("Rate (optional)",InputType.TYPE_CLASS_NUMBER|InputType.TYPE_NUMBER_FLAG_DECIMAL);if(existing!=null){qty.setText(qty(existing.optDouble("quantity")));if(existing.has("rate")&&!existing.isNull("rate"))rate.setText(String.valueOf(existing.optDouble("rate")));}line.addView(qty,new LinearLayout.LayoutParams(0,-2,1));LinearLayout.LayoutParams rp=new LinearLayout.LayoutParams(0,-2,1);rp.setMargins(dp(8),0,0,0);line.addView(rate,rp);box.addView(line,full(8));Button remove=btn("Remove item",v->{if(itemRows.size()<=1){toast("Keep at least one item");return;}ItemRow target=null;for(ItemRow r:itemRows)if(r.box==box)target=r;if(target!=null)itemRows.remove(target);parent.removeView(box);});box.addView(remove,full(6));parent.addView(box,full(8));ItemRow row=new ItemRow(box,item,qty,rate,unit);itemRows.add(row);setItemAdapter(row);item.setOnItemClickListener((p,v,pos,id)->unit.setText(unitByItem.getOrDefault(item.getText().toString(),"")));item.setOnFocusChangeListener((v,has)->{if(!has)unit.setText(unitByItem.getOrDefault(item.getText().toString(),unit.getText().toString()));});
 }
 private void updateItemAdapters(){for(ItemRow r:itemRows)setItemAdapter(r);}
 private void setItemAdapter(ItemRow row){List<String> names=new ArrayList<>(unitByItem.keySet());Collections.sort(names,String.CASE_INSENSITIVE_ORDER);row.item.setAdapter(new ArrayAdapter<>(this,android.R.layout.simple_dropdown_item_1line,names));String u=unitByItem.get(row.item.getText().toString());if(u!=null&&!u.isEmpty())row.unit.setText(u);}
 private double number(String s)throws Exception{try{return Double.parseDouble(s.trim());}catch(Exception e){throw new Exception("Enter a valid quantity / rate");}}
 private void saveForm(JSONObject original,Spinner companySpin,AutoCompleteTextView buyer,EditText notes){
  try{String c=String.valueOf(companySpin.getSelectedItem()).trim(),b=buyer.getText().toString().trim();if(c.isEmpty()||b.isEmpty())throw new Exception("Select company and buyer");JSONArray items=new JSONArray();for(ItemRow r:itemRows){String name=r.item.getText().toString().trim(),q=r.qty.getText().toString().trim(),rate=r.rate.getText().toString().trim();if(name.isEmpty()||q.isEmpty())throw new Exception("Every item needs a product and quantity");JSONObject x=new JSONObject().put("name",name).put("quantity",number(q));String u=r.unit.getText().toString().trim();if(!u.isEmpty())x.put("unit",u);if(!rate.isEmpty())x.put("rate",number(rate));items.put(x);}JSONObject safeBuyerDetails=b.equalsIgnoreCase(buyerDetailsFor)?buyerDetails:new JSONObject();JSONObject payload=new JSONObject().put("company",c).put("buyer",b).put("buyerDetails",safeBuyerDetails).put("items",items).put("notes",notes.getText().toString().trim());offline.saveLocal(payload,original);store.put("company",c);OrdersWidget.refreshAll(this);toast(original==null?"Order saved on phone":"Order updated on phone");showOrders();}catch(Exception e){toast(e.getMessage());}
 }
}
