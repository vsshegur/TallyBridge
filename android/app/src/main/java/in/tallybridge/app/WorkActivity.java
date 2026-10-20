package in.tallybridge.app;

import android.app.*;import android.content.*;import android.graphics.*;import android.graphics.drawable.GradientDrawable;import android.os.*;import android.view.*;import android.widget.*;import org.json.*;import java.text.NumberFormat;import java.time.*;import java.util.*;import java.util.concurrent.*;

/**
 * Today's work: pending orders, today's dispatch and sales, money owed both ways and anything that needs attention.
 * Quick actions live on Home, so this screen only shows figures; every figure that has a screen behind it opens it.
 * When the PC cannot be reached it still shows the orders saved on this phone.
 */
public final class WorkActivity extends Activity{
 private final ExecutorService work=Executors.newSingleThreadExecutor();private final Handler main=new Handler(Looper.getMainLooper());
 private CryptoStore store;private BridgeApi api;private OfflineOrdersStore offline;private String company="";private boolean dark,destroyed;private int generation;
 private LinearLayout root,body;private ProgressBar progress;
 private int NAVY,TEAL,MUTED,BG,SURFACE,BORDER,TINT,AMBER,RED;
 private final NumberFormat money=NumberFormat.getCurrencyInstance(new Locale("en","IN"));

 @Override public void onCreate(Bundle b){
  applyTheme();super.onCreate(b);getWindow().addFlags(WindowManager.LayoutParams.FLAG_SECURE);money.setCurrency(Currency.getInstance("INR"));styleWindow();
  try{store=new CryptoStore(this);api=new BridgeApi(store);offline=new OfflineOrdersStore(store);company=store.get("company");}catch(Exception e){fatal(e);return;}
  if(store.get("base").isEmpty()||store.get("device").isEmpty()){startActivity(new Intent(this,MainActivity.class));finish();return;}
  showToday();
 }
 @Override protected void onDestroy(){destroyed=true;work.shutdownNow();main.removeCallbacksAndMessages(null);super.onDestroy();}

 private String appearance(){return getSharedPreferences("appearance",MODE_PRIVATE).getString("mode","System");}
 private void applyTheme(){
  String m=appearance();dark=m.equals("Dark")||(m.equals("System")&&(getResources().getConfiguration().uiMode&android.content.res.Configuration.UI_MODE_NIGHT_MASK)==android.content.res.Configuration.UI_MODE_NIGHT_YES);
  setTheme(dark?R.style.AppThemeDark:R.style.AppThemeLight);
  NAVY=dark?0xffe6edf3:0xff102d44;TEAL=dark?0xff7bd8bd:0xff176b5b;MUTED=dark?0xffa7b8c5:0xff596c79;BG=dark?0xff101820:0xfff5f7fa;SURFACE=dark?0xff1b2732:Color.WHITE;
  BORDER=dark?0xff344552:0xffe4eaef;TINT=dark?0xff203f3a:0xffe5f2ee;AMBER=dark?0xffffd28a:0xff8a5a00;RED=dark?0xffff9c9c:0xffa12626;
 }
 private void styleWindow(){getWindow().getDecorView().setSystemUiVisibility(dark?0:(View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR|View.SYSTEM_UI_FLAG_LIGHT_NAVIGATION_BAR));getWindow().setStatusBarColor(BG);getWindow().setNavigationBarColor(SURFACE);}

 private int dp(float x){return Math.round(x*getResources().getDisplayMetrics().density);}
 private LinearLayout col(){LinearLayout l=new LinearLayout(this);l.setOrientation(LinearLayout.VERTICAL);return l;}
 private TextView text(String s,int z,int c){TextView v=new TextView(this);v.setText(s);v.setTextSize(z);v.setTextColor(c);v.setLineSpacing(dp(2),1f);v.setPadding(0,dp(3),0,dp(3));return v;}
 private GradientDrawable shape(int c,int r){GradientDrawable d=new GradientDrawable();d.setColor(c);d.setCornerRadius(dp(r));d.setStroke(dp(1),BORDER);return d;}
 private LinearLayout.LayoutParams full(int t){LinearLayout.LayoutParams p=new LinearLayout.LayoutParams(-1,-2);p.setMargins(0,dp(t),0,0);return p;}
 private Button button(String s,View.OnClickListener l){Button b=new Button(this);b.setText(s);b.setAllCaps(false);b.setTextSize(14);b.setTextColor(TEAL);b.setBackground(shape(SURFACE,12));b.setMinHeight(dp(48));b.setOnClickListener(l);return b;}
 private void fatal(Exception e){TextView v=text("Today's work could not open.\n"+(e.getMessage()==null?"Unknown error":e.getMessage()),16,NAVY);v.setPadding(dp(24),dp(60),dp(24),dp(24));v.setBackgroundColor(BG);setContentView(v);}
 private String amount(double v){return money.format(Math.abs(v));}

 /** Same header as the main screens: brand line, title, subtitle and a refresh button; the bottom bar keeps Home selected. */
 private void frame(String title,String subtitle){
  generation++;root=col();root.setBackgroundColor(BG);
  LinearLayout top=new LinearLayout(this);top.setGravity(Gravity.CENTER_VERTICAL);top.setPadding(dp(18),dp(15),dp(12),dp(10));
  LinearLayout titles=col();TextView brand=text("TALLYBRIDGE",10,TEAL);brand.setTypeface(null,Typeface.BOLD);brand.setLetterSpacing(.12f);titles.addView(brand);
  TextView h=text(title,26,NAVY);h.setTypeface(null,Typeface.BOLD);titles.addView(h);if(subtitle!=null&&!subtitle.isEmpty())titles.addView(text(subtitle,12,MUTED));
  top.addView(titles,new LinearLayout.LayoutParams(0,-2,1));
  FrameLayout refresh=new FrameLayout(this);refresh.setContentDescription("Refresh");refresh.setFocusable(true);refresh.setClickable(true);refresh.setOnClickListener(v->showToday());
  UiIcon icon=new UiIcon(this,"refresh",NAVY);FrameLayout.LayoutParams ip=new FrameLayout.LayoutParams(dp(24),dp(24),Gravity.CENTER);refresh.addView(icon,ip);
  top.addView(refresh,new LinearLayout.LayoutParams(dp(48),dp(48)));root.addView(top);
  progress=new ProgressBar(this,null,android.R.attr.progressBarStyleHorizontal);progress.setIndeterminate(true);progress.setVisibility(View.GONE);progress.setIndeterminateTintList(android.content.res.ColorStateList.valueOf(TEAL));root.addView(progress,new LinearLayout.LayoutParams(-1,dp(3)));
  ScrollView sv=new ScrollView(this);body=col();body.setPadding(dp(18),dp(3),dp(18),dp(26));sv.addView(body);root.addView(sv,new LinearLayout.LayoutParams(-1,0,1));
  root.addView(SectionNav.bar(this,"Home",SURFACE,TINT,MUTED,TEAL));setContentView(root);
 }
 private void heading(String h,String d){TextView x=text(h,19,NAVY);x.setTypeface(null,Typeface.BOLD);body.addView(x,full(17));if(d!=null&&!d.isEmpty())body.addView(text(d,12,MUTED));}
 private View tile(String title,String value,String detail,int accent,View.OnClickListener click){
  LinearLayout c=col();c.setPadding(dp(14),dp(12),dp(14),dp(12));c.setBackground(shape(SURFACE,14));
  TextView t=text(title,11,MUTED);t.setTypeface(null,Typeface.BOLD);c.addView(t);
  TextView v=text(value,19,accent);v.setTypeface(null,Typeface.BOLD);v.setSingleLine(true);v.setEllipsize(android.text.TextUtils.TruncateAt.END);c.addView(v);
  if(detail!=null&&!detail.isEmpty())c.addView(text(detail,11,MUTED));
  if(click!=null){c.setOnClickListener(click);c.setFocusable(true);c.setContentDescription(title+" "+value+(detail==null?"":", "+detail));}
  return c;
 }
 private void two(View a,View b){LinearLayout r=new LinearLayout(this);r.addView(a,new LinearLayout.LayoutParams(0,-1,1));LinearLayout.LayoutParams p=new LinearLayout.LayoutParams(0,-1,1);p.setMargins(dp(8),0,0,0);r.addView(b,p);body.addView(r,full(8));}

 private void showToday(){frame("Today's work",company);loadToday();}
 private void loadToday(){
  final int epoch=generation;progress.setVisibility(View.VISIBLE);
  work.execute(()->{
   JSONObject x;
   try{x=api.get(BridgeApi.path("workspace",company));}
   catch(Exception live){try{x=savedToday();}catch(Exception e){x=null;}}
   JSONObject result=x;
   main.post(()->{
    if(destroyed||epoch!=generation)return;progress.setVisibility(View.GONE);
    if(result!=null){renderToday(result);return;}
    heading("PC not reachable","Orders saved on this phone still work. Try again when the PC is on.");
    body.addView(button("Try again",v->showToday()),full(10));
    body.addView(button("Open orders",v->SectionNav.go(this,"Orders")),full(8));
   });
  });
 }
 /** Figures that do not need the workspace endpoint: phone orders always; money and sales only if the PC answers. */
 private JSONObject savedToday()throws Exception{
  JSONObject x=new JSONObject().put("_saved",true).put("alerts",new JSONArray());
  try{
   JSONObject boot=api.get("/api/v1/mobile/bootstrap");x.put("tallyOnline",boot.optBoolean("tallyOnline"));
   JSONObject sums=boot.optJSONObject("summaries"),s=sums==null?null:sums.optJSONObject(company);
   if(s!=null)x.put("receivable",s.optDouble("receivable")).put("payable",s.optDouble("payable"));
   try{JSONObject d=api.get(BridgeApi.path("today-dispatch",company));x.put("salesToday",d.optDouble("total")).put("salesCount",d.optInt("count"));}catch(Exception ignored){}
  }catch(Exception pcOffline){x.put("pcOffline",true);}
  JSONObject o=offline.list();x.put("pendingOrders",o.optInt("pendingCount"));
  int old=0,todayCount=0;Instant cutoff=Instant.now().minus(Duration.ofHours(48));
  JSONArray p=o.optJSONArray("pending");if(p!=null)for(int i=0;i<p.length();i++)try{JSONObject a=p.optJSONObject(i);if(a!=null&&Instant.parse(a.optString("createdAt")).isBefore(cutoff))old++;}catch(Exception ignored){}
  JSONArray d=o.optJSONArray("dispatched");if(d!=null)for(int i=0;i<d.length();i++)try{JSONObject a=d.optJSONObject(i);if(a!=null&&Instant.parse(a.optString("dispatchedAt")).atZone(ZoneId.systemDefault()).toLocalDate().equals(LocalDate.now()))todayCount++;}catch(Exception ignored){}
  x.put("oldPendingOrders",old).put("dispatchedToday",todayCount);
  return x;
 }
 private String rupees(JSONObject x,String key){return x.has(key)?amount(x.optDouble(key)):"—";}
 private void renderToday(JSONObject x){
  boolean saved=x.optBoolean("_saved"),online=x.optBoolean("tallyOnline"),pcOffline=x.optBoolean("pcOffline");
  String head=pcOffline?"Orders on this phone":saved?"Saved figures":online?"Live figures":"Saved Tally figures";
  String sub=pcOffline?"The PC is not reachable • money figures show when it is":saved?"From the PC's last read of Tally":online?"Tally is connected • figures are live":"Tally is closed on the PC • last read figures";
  heading(head,sub);
  int old=x.optInt("oldPendingOrders");
  two(tile("PENDING ORDERS",String.valueOf(x.optInt("pendingOrders")),old>0?old+" older than 2 days":"None older than 2 days",old>0?AMBER:TEAL,v->SectionNav.go(this,"Orders")),
   tile("DISPATCHED TODAY",String.valueOf(x.optInt("dispatchedToday")),"Orders sent today",TEAL,v->SectionNav.go(this,"Orders")));
  two(tile("SALES TODAY",rupees(x,"salesToday"),x.has("salesCount")?x.optInt("salesCount")+(x.optInt("salesCount")==1?" invoice":" invoices"):"",NAVY,v->SectionNav.go(this,"Reports")),
   tile("CUSTOMERS OWE",rupees(x,"receivable"),"Receivable",NAVY,v->SectionNav.go(this,"Reports")));
  two(tile("YOU OWE",rupees(x,"payable"),"Payable to suppliers",NAVY,v->SectionNav.go(this,"Reports")),
   tile("TALLY",pcOffline?"PC OFF":online?"LIVE":"SAVED",pcOffline?"Phone orders only":online?"Connected now":"Last read figures",online?TEAL:AMBER,v->openSystem()));
  JSONObject health=x.optJSONObject("health"),perf=x.optJSONObject("performance");
  if(health!=null||perf!=null){
   heading("Company and Tally","Details Tally needs for invoices, and how quickly it answers");
   int score=health==null?0:health.optInt("score");String rate=perf==null?"—":perf.optInt("successRate")+"%";
   two(tile("COMPANY DETAILS",health==null?"—":score+"%",health!=null&&health.optBoolean("ready")?"Ready":"Some details missing",score>=80?TEAL:AMBER,v->openSystem()),
    tile("TALLY READS",rate,perf==null?"No reads yet":perf.optInt("averageMs")+" ms average",perf!=null&&perf.optInt("successRate")>=90?TEAL:AMBER,v->openSystem()));
  }
  JSONArray alerts=x.optJSONArray("alerts");int visible=0;
  if(alerts!=null)for(int i=0;i<alerts.length();i++){JSONObject a=alerts.optJSONObject(i);if(a!=null&&!"followups".equals(a.optString("action")))visible++;}
  heading("Needs attention",visible==0?"Nothing urgent today.":visible==1?"1 thing worth checking":visible+" things worth checking");
  if(alerts!=null)for(int i=0;i<alerts.length();i++){
   JSONObject a=alerts.optJSONObject(i);if(a==null||"followups".equals(a.optString("action")))continue;
   String sev=a.optString("severity"),act=a.optString("action");int color="critical".equals(sev)?RED:"warning".equals(sev)?AMBER:TEAL;
   LinearLayout c=col();c.setPadding(dp(14),dp(11),dp(14),dp(11));c.setBackground(shape(SURFACE,13));c.setFocusable(true);
   TextView t=text(a.optString("title"),14,color);t.setTypeface(null,Typeface.BOLD);c.addView(t);c.addView(text(a.optString("detail"),12,MUTED));
   c.setOnClickListener(v->{if("orders".equals(act))SectionNav.go(this,"Orders");else openSystem();});body.addView(c,full(7));
  }
 }
 private void openSystem(){Intent i=new Intent(this,MainActivity.class).setAction(MainActivity.ACTION_SYSTEM).addFlags(Intent.FLAG_ACTIVITY_CLEAR_TOP|Intent.FLAG_ACTIVITY_SINGLE_TOP);startActivity(i);}
}
