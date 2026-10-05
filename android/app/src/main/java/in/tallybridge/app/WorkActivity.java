package in.tallybridge.app;

import android.app.*;
import android.content.*;
import android.graphics.*;
import android.graphics.drawable.GradientDrawable;
import android.os.*;
import android.text.*;
import android.view.*;
import android.widget.*;
import org.json.*;
import java.text.NumberFormat;
import java.time.*;
import java.util.*;
import java.util.concurrent.*;

/** Daily operations center. Uses only authenticated bridge APIs and saved/read-only Tally data. */
public final class WorkActivity extends Activity {
 private final ExecutorService work=Executors.newSingleThreadExecutor();
 private final Handler main=new Handler(Looper.getMainLooper());
 private CryptoStore store; private BridgeApi api; private OfflineOrdersStore offline; private String company=""; private boolean dark=false,destroyed=false;
 private LinearLayout root,body; private ProgressBar progress; private int NAVY,TEAL,MUTED,BG,SURFACE,BORDER,TINT,AMBER,RED,INK;
 private final NumberFormat money=NumberFormat.getCurrencyInstance(new Locale("en","IN"));

 @Override public void onCreate(Bundle b){applyTheme();super.onCreate(b);getWindow().addFlags(WindowManager.LayoutParams.FLAG_SECURE);styleWindow();try{store=new CryptoStore(this);api=new BridgeApi(store);offline=new OfflineOrdersStore(store);company=store.get("company");}catch(Exception e){fatal(e);return;}if(store.get("base").isEmpty()||store.get("device").isEmpty()){finish();startActivity(new Intent(this,MainActivity.class));return;}showToday();}
 @Override protected void onDestroy(){destroyed=true;work.shutdownNow();main.removeCallbacksAndMessages(null);super.onDestroy();}
  private String appearance(){return getSharedPreferences("appearance",MODE_PRIVATE).getString("mode","System");}
 private void applyTheme(){String m=appearance();dark=m.equals("Dark")||(m.equals("System")&&(getResources().getConfiguration().uiMode&android.content.res.Configuration.UI_MODE_NIGHT_MASK)==android.content.res.Configuration.UI_MODE_NIGHT_YES);setTheme(dark?R.style.AppThemeDark:R.style.AppThemeLight);NAVY=dark?0xffe6edf3:0xff102d44;TEAL=dark?0xff7bd8bd:0xff176b5b;MUTED=dark?0xffa7b8c5:0xff596c79;BG=dark?0xff101820:0xfff5f7fa;SURFACE=dark?0xff1b2732:Color.WHITE;BORDER=dark?0xff344552:0xffe4eaef;TINT=dark?0xff203f3a:0xffe5f2ee;AMBER=dark?0xffffc857:0xff8a5a00;RED=dark?0xffff9c9c:0xffa12626;INK=dark?0xff102d26:Color.WHITE;}
 private void styleWindow(){getWindow().getDecorView().setSystemUiVisibility(dark?0:(View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR|View.SYSTEM_UI_FLAG_LIGHT_NAVIGATION_BAR));getWindow().setStatusBarColor(BG);getWindow().setNavigationBarColor(SURFACE);}
 private int dp(float x){return Math.round(x*getResources().getDisplayMetrics().density);}
 private LinearLayout col(){LinearLayout l=new LinearLayout(this);l.setOrientation(LinearLayout.VERTICAL);return l;}
 private TextView text(String s,int z,int c){TextView v=new TextView(this);v.setText(s);v.setTextSize(z);v.setTextColor(c);v.setLineSpacing(dp(2),1f);v.setPadding(0,dp(3),0,dp(3));return v;}
 private GradientDrawable shape(int c,int r){GradientDrawable d=new GradientDrawable();d.setColor(c);d.setCornerRadius(dp(r));d.setStroke(dp(1),BORDER);return d;}
 private Button button(String s,View.OnClickListener l){Button b=new Button(this);b.setText(s);b.setAllCaps(false);b.setTextColor(TEAL);b.setTextSize(14);b.setBackground(shape(SURFACE,12));b.setMinHeight(dp(48));b.setOnClickListener(l);return b;}
 private Button primary(String s,View.OnClickListener l){Button b=button(s,l);b.setTextColor(INK);b.setBackground(shape(TEAL,12));return b;}
 private LinearLayout.LayoutParams full(int top){LinearLayout.LayoutParams p=new LinearLayout.LayoutParams(-1,-2);p.setMargins(0,dp(top),0,0);return p;}
 private void gap(LinearLayout l,int h){l.addView(new View(this),new LinearLayout.LayoutParams(1,dp(h)));}
 private void fatal(Exception e){TextView v=text("Today could not start.\n"+(e.getMessage()==null?"Unknown error":e.getMessage()),16,NAVY);v.setPadding(dp(24),dp(60),dp(24),dp(24));v.setBackgroundColor(BG);setContentView(v);}
 private String amount(double v){return money.format(v);}
 private void toast(String s){Toast.makeText(this,s,Toast.LENGTH_LONG).show();}

 private void frame(String title,String subtitle){
  root=col();root.setBackgroundColor(BG);
  LinearLayout top=new LinearLayout(this);top.setGravity(Gravity.CENTER_VERTICAL);top.setPadding(dp(18),dp(14),dp(18),dp(8));
  LinearLayout titles=col();TextView brand=text("TALLYBRIDGE",10,TEAL);brand.setTypeface(null,Typeface.BOLD);brand.setLetterSpacing(.14f);titles.addView(brand);
  TextView t=text(title,25,NAVY);t.setTypeface(null,Typeface.BOLD);titles.addView(t);titles.addView(text(subtitle,12,MUTED));
  top.addView(titles,new LinearLayout.LayoutParams(0,-2,1));Button home=button("Home",v->openMain(MainActivity.ACTION_HOME));top.addView(home,new LinearLayout.LayoutParams(dp(82),-2));root.addView(top);
  progress=new ProgressBar(this,null,android.R.attr.progressBarStyleHorizontal);progress.setIndeterminate(true);progress.setVisibility(View.GONE);progress.setIndeterminateTintList(android.content.res.ColorStateList.valueOf(TEAL));root.addView(progress,new LinearLayout.LayoutParams(-1,dp(3)));
  LinearLayout tabs=new LinearLayout(this);String[] names={"Today","Search","Customers","Tools","System"};
  View.OnClickListener[] clicks={v->showToday(),v->openMain(MainActivity.ACTION_SEARCH),v->openMain(MainActivity.ACTION_CUSTOMERS),v->showTools(),v->openMain(MainActivity.ACTION_SYSTEM)};
  for(int i=0;i<names.length;i++){Button b=button(names[i],clicks[i]);LinearLayout.LayoutParams lp=new LinearLayout.LayoutParams(0,-2,1);lp.setMargins(dp(2),dp(7),dp(2),dp(4));tabs.addView(b,lp);}
  HorizontalScrollView hs=new HorizontalScrollView(this);hs.setHorizontalScrollBarEnabled(false);hs.setPadding(dp(12),0,dp(12),0);hs.addView(tabs,new HorizontalScrollView.LayoutParams(dp(610),-2));root.addView(hs);
  ScrollView sv=new ScrollView(this);body=col();body.setPadding(dp(18),dp(7),dp(18),dp(26));sv.addView(body);root.addView(sv,new LinearLayout.LayoutParams(-1,0,1));
  root.addView(SectionNav.bar(this,"Work",SURFACE,TINT,MUTED,TEAL));setContentView(root);
 }
 private void openMain(String action){Intent i=new Intent(this,MainActivity.class).setAction(action).addFlags(Intent.FLAG_ACTIVITY_CLEAR_TOP|Intent.FLAG_ACTIVITY_SINGLE_TOP);startActivity(i);finish();overridePendingTransition(0,0);}
 private void busy(boolean yes){if(progress!=null)progress.setVisibility(yes?View.VISIBLE:View.GONE);}
 private void heading(String t,String d){TextView a=text(t,19,NAVY);a.setTypeface(null,Typeface.BOLD);body.addView(a,full(14));if(d!=null&&!d.isEmpty())body.addView(text(d,12,MUTED));}
 private void card(String title,String detail,String value,View.OnClickListener click){LinearLayout c=col();c.setPadding(dp(15),dp(13),dp(15),dp(13));c.setBackground(shape(SURFACE,14));TextView t=text(title,15,NAVY);t.setTypeface(null,Typeface.BOLD);c.addView(t);if(detail!=null&&!detail.isEmpty())c.addView(text(detail,12,MUTED));if(value!=null&&!value.isEmpty()){TextView v=text(value,19,TEAL);v.setTypeface(null,Typeface.BOLD);c.addView(v,full(4));}if(click!=null){c.setOnClickListener(click);c.setFocusable(true);}body.addView(c,full(8));}
 private String path(String ep,String...p){return BridgeApi.path(ep,company,p);}
 private void get(String title,String url,java.util.function.Consumer<JSONObject> render){busy(true);work.execute(()->{try{JSONObject x=api.get(url);main.post(()->{if(destroyed)return;busy(false);render.accept(x);});}catch(Exception e){main.post(()->{if(destroyed)return;busy(false);boolean oldBridge=e instanceof Http.Failure&&((Http.Failure)e).status==404;card(oldBridge?"Update Windows TallyBridge":"Could not load "+title,oldBridge?"The connected Windows bridge does not provide this advanced Work screen yet. Core reports and offline tools remain available.":(e.getMessage()==null?"Connection unavailable":e.getMessage()),"",v->get(title,url,render));});}});}
 private void post(String url,JSONObject payload,Runnable done){busy(true);work.execute(()->{try{api.call(url,"POST",payload.toString());main.post(()->{if(destroyed)return;busy(false);done.run();});}catch(Exception e){main.post(()->{if(destroyed)return;busy(false);toast(e instanceof Http.Failure&&((Http.Failure)e).status==404?"This action needs the matching Windows TallyBridge release.":e.getMessage());});}});}

 private void showToday(){frame("Today","Daily work • "+company);loadToday();}
 private void loadToday(){busy(true);work.execute(()->{try{JSONObject x=api.get(path("workspace"));main.post(()->{if(destroyed)return;busy(false);renderToday(x);});}catch(Exception advanced){try{JSONObject x=compatibilityToday();main.post(()->{if(destroyed)return;busy(false);renderToday(x);});}catch(Exception fallback){main.post(()->{if(destroyed)return;busy(false);card("Could not load Today",fallback.getMessage()==null?"PC connection unavailable":fallback.getMessage(),"",v->loadToday());});}}});}
 private JSONObject compatibilityToday()throws Exception{
  JSONObject boot=api.get("/api/v1/mobile/bootstrap"),x=new JSONObject();x.put("_compat",true);x.put("tallyOnline",boot.optBoolean("tallyOnline"));
  JSONObject sums=boot.optJSONObject("summaries"),sm=sums==null?null:sums.optJSONObject(company);x.put("receivable",sm==null?0:sm.optDouble("receivable"));x.put("payable",sm==null?0:sm.optDouble("payable"));
  try{JSONObject d=api.get(path("today-dispatch"));x.put("salesToday",d.optDouble("total"));x.put("salesCount",d.optInt("count"));}catch(Exception ignored){}
  int pending=0,old=0,dispatchedToday=0;try{JSONObject o=offline.list();pending=o.optInt("pendingCount");JSONArray pa=o.optJSONArray("pending");Instant cutoff=Instant.now().minus(Duration.ofHours(48));if(pa!=null)for(int i=0;i<pa.length();i++){JSONObject a=pa.optJSONObject(i);if(a==null)continue;try{String c=a.optString("createdAt");if(!c.isEmpty()&&Instant.parse(c).isBefore(cutoff))old++;}catch(Exception ignored){}}JSONArray da=o.optJSONArray("dispatched");LocalDate today=LocalDate.now();if(da!=null)for(int i=0;i<da.length();i++){JSONObject a=da.optJSONObject(i);if(a==null)continue;try{String d=a.optString("dispatchedAt");if(!d.isEmpty()&&Instant.parse(d).atZone(ZoneId.systemDefault()).toLocalDate().equals(today))dispatchedToday++;}catch(Exception ignored){}}}catch(Exception ignored){}
  x.put("pendingOrders",pending);x.put("oldPendingOrders",old);x.put("dispatchedToday",dispatchedToday);
  JSONArray alerts=new JSONArray();alerts.put(new JSONObject().put("id","compat").put("severity","info").put("title","Compatibility mode").put("detail","Today is using core bridge data because the advanced Work endpoint is unavailable. Orders, dispatch and main account figures remain usable.").put("action","tools"));x.put("alerts",alerts);return x;
 }
 private void renderToday(JSONObject x){
  body.removeAllViews();boolean compat=x.optBoolean("_compat");heading(x.optBoolean("tallyOnline")?"Business ready":"Saved mode",compat?"Compatibility mode • using core bridge data and the phone’s saved Orders.":x.optBoolean("tallyOnline")?"Tally is connected. Figures below use the latest synchronized data.":"Tally is unavailable. Last-known-good figures remain visible.");
  LinearLayout quick=new LinearLayout(this);Button q=primary("+ Quick order",v->{Intent i=new Intent(this,OrdersActivity.class);i.setAction(OrdersActivity.ACTION_QUICK);startActivity(i);finish();overridePendingTransition(0,0);});Button o=button("Orders",v->SectionNav.go(this,"Orders"));quick.addView(q,new LinearLayout.LayoutParams(0,-2,1));LinearLayout.LayoutParams op=new LinearLayout.LayoutParams(0,-2,1);op.setMargins(dp(8),0,0,0);quick.addView(o,op);body.addView(quick,full(12));
  LinearLayout tools=new LinearLayout(this);Button gsm=button("GSM calculator",v->startActivity(new Intent(this,GsmCalculatorActivity.class)));Button pi=button("Proforma",v->{Intent i=new Intent(this,MainActivity.class);i.setAction(MainActivity.ACTION_NEW_PROFORMA);startActivity(i);});tools.addView(gsm,new LinearLayout.LayoutParams(0,-2,1));LinearLayout.LayoutParams pp=new LinearLayout.LayoutParams(0,-2,1);pp.setMargins(dp(8),0,0,0);tools.addView(pi,pp);body.addView(tools,full(8));
  card("Pending orders",x.optInt("oldPendingOrders")+" pending for 2+ days",String.valueOf(x.optInt("pendingOrders")),v->SectionNav.go(this,"Orders"));
  card("Dispatched today","Orders marked dispatched today",String.valueOf(x.optInt("dispatchedToday")),v->SectionNav.go(this,"Orders"));
  card("Sales today",x.optInt("salesCount")+" posted invoices",amount(x.optDouble("salesToday")),null);
  card("Receivable","Saved outstanding balance",amount(x.optDouble("receivable")),null);card("Payable","Saved outstanding balance",amount(x.optDouble("payable")),null);
  JSONArray alerts=x.optJSONArray("alerts");int visibleAlerts=0;if(alerts!=null)for(int i=0;i<alerts.length();i++){JSONObject a=alerts.optJSONObject(i);if(a!=null&&!"followups".equals(a.optString("action"))&&!(x.optBoolean("tallyOnline")&&"sync-error".equals(a.optString("id"))))visibleAlerts++;}heading("Needs attention",visibleAlerts==0?"Nothing urgent detected.":visibleAlerts+" useful alert(s)");if(alerts!=null)for(int i=0;i<alerts.length();i++){JSONObject a=alerts.optJSONObject(i);if(a==null||"followups".equals(a.optString("action"))||(x.optBoolean("tallyOnline")&&"sync-error".equals(a.optString("id"))))continue;String sev=a.optString("severity");int c="critical".equals(sev)?RED:"warning".equals(sev)?AMBER:TEAL;LinearLayout box=col();box.setPadding(dp(14),dp(12),dp(14),dp(12));box.setBackground(shape(SURFACE,13));TextView t=text(a.optString("title"),14,c);t.setTypeface(null,Typeface.BOLD);box.addView(t);box.addView(text(a.optString("detail"),12,MUTED));String action=a.optString("action");box.setOnClickListener(v->{if("orders".equals(action))SectionNav.go(this,"Orders");else if("tools".equals(action))showTools();else openMain(MainActivity.ACTION_SYSTEM);});body.addView(box,full(7));}
 }

 private void showTools(){frame("Tools","Fast everyday actions • one consistent TallyBridge workflow");heading("Business tools","Common actions in one place");card("GSM Calculator","Calculate towel GSM from length, breadth and weight • fully offline","",v->startActivity(new Intent(this,GsmCalculatorActivity.class)));card("Quick order","Create an order quickly • saved on phone when PC is unavailable","",v->{Intent i=new Intent(this,OrdersActivity.class);i.setAction(OrdersActivity.ACTION_QUICK);startActivity(i);finish();overridePendingTransition(0,0);});card("Proforma invoice","Create and share a proforma invoice","",v->{Intent i=new Intent(this,MainActivity.class);i.setAction(MainActivity.ACTION_NEW_PROFORMA);startActivity(i);});card("Orders","Review pending and dispatched orders","",v->SectionNav.go(this,"Orders"));card("Search","Find invoices, parties, LR/RR and saved activity","",v->openMain(MainActivity.ACTION_SEARCH));}

}
