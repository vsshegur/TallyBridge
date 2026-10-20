package in.tallybridge.app;

import android.app.Activity;
import android.app.AlertDialog;
import android.content.ActivityNotFoundException;
import android.content.ClipData;
import android.content.Intent;
import android.graphics.Bitmap;
import android.graphics.Color;
import android.graphics.Typeface;
import android.graphics.drawable.GradientDrawable;
import android.net.Uri;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.provider.MediaStore;
import android.view.Gravity;
import android.view.View;
import android.view.WindowManager;
import android.widget.*;
import org.json.JSONArray;
import org.json.JSONObject;
import java.io.File;
import java.text.NumberFormat;
import java.time.Instant;
import java.time.ZoneId;
import java.time.format.DateTimeFormatter;
import java.util.*;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

/**
 * Scan purchase bill: photograph a bill (or pick photos / PDFs, or share them
 * from WhatsApp) and send it to the Windows PC, which passes it on to
 * InvoiceTotally. InvoiceTotally reads it; the bill is reviewed on the PC and
 * posted to Tally from there. This screen also shows how each bill is doing.
 */
public final class BillScanActivity extends Activity {
 static final String ACTION_CAMERA="in.tallybridge.app.BILL_CAMERA";
 private static final int REQ_CAMERA=901, REQ_PICK=902;
 private final ExecutorService work=Executors.newSingleThreadExecutor();
 private final ExecutorService net=Executors.newSingleThreadExecutor();
 private final Handler main=new Handler(Looper.getMainLooper());
 private final List<BillPages.Page> pages=new ArrayList<>();
 private CryptoStore store; private BridgeApi api; private BillOutbox outbox;
 private JSONArray companies=new JSONArray(); private JSONArray uploads=new JSONArray();
 private boolean enabled=true, pcReachable=true, destroyed=false, busy=false, dark=false;
 private String reason="", voucherType="Purchase", pendingCapture="";
 private LinearLayout body,pageStrip,listBox; private TextView status,pageCount; private Spinner companySpin; private Button sendBtn,purchaseBtn,salesBtn;
 private int NAVY,TEAL,MUTED,BG,SURFACE,BORDER,AMBER,RED,PRIMARY_INK,TINT;
 private final NumberFormat currency=NumberFormat.getCurrencyInstance(new Locale("en","IN"));

 // ------------------------------------------------------------------ look
 private void applyAppearance(){
  String mode=getSharedPreferences("appearance",MODE_PRIVATE).getString("mode","System");
  dark=mode.equals("Dark")||(mode.equals("System")&&(getResources().getConfiguration().uiMode&android.content.res.Configuration.UI_MODE_NIGHT_MASK)==android.content.res.Configuration.UI_MODE_NIGHT_YES);
  setTheme(dark?R.style.AppThemeDark:R.style.AppThemeLight);
  NAVY=dark?0xffe6edf3:0xff102d44;TEAL=dark?0xff7bd8bd:0xff176b5b;MUTED=dark?0xffa7b8c5:0xff596c79;BG=dark?0xff101820:0xfff5f7fa;
  SURFACE=dark?0xff1b2732:Color.WHITE;BORDER=dark?0xff344552:0xffe4eaef;AMBER=dark?0xffffc857:0xff8a5a00;RED=dark?0xffff9c9c:0xffb3261e;
  PRIMARY_INK=dark?0xff102d26:Color.WHITE;TINT=dark?0xff17352f:0xffe5f3ef;
 }
 private int dp(float v){return Math.round(v*getResources().getDisplayMetrics().density);}
 private GradientDrawable shape(int color,int radius){GradientDrawable d=new GradientDrawable();d.setColor(color);d.setCornerRadius(dp(radius));d.setStroke(dp(1),BORDER);return d;}
 private LinearLayout col(){LinearLayout l=new LinearLayout(this);l.setOrientation(LinearLayout.VERTICAL);return l;}
 private TextView txt(String s,int size,int color){TextView v=new TextView(this);v.setText(s);v.setTextSize(size);v.setTextColor(color);v.setPadding(0,dp(3),0,dp(3));return v;}
 private TextView label(String s){TextView t=txt(s,11,MUTED);t.setTypeface(null,Typeface.BOLD);t.setLetterSpacing(.08f);return t;}
 private Button primary(String s,View.OnClickListener c){Button b=new Button(this);b.setText(s);b.setAllCaps(false);b.setTextSize(15);b.setTypeface(Typeface.create("sans-serif-medium",Typeface.NORMAL));b.setTextColor(PRIMARY_INK);b.setBackground(shape(TEAL,13));b.setMinHeight(dp(52));b.setOnClickListener(c);return b;}
 private Button secondary(String s,View.OnClickListener c){Button b=new Button(this);b.setText(s);b.setAllCaps(false);b.setTextSize(14);b.setTextColor(NAVY);b.setBackground(shape(SURFACE,13));b.setMinHeight(dp(48));b.setOnClickListener(c);return b;}
 private LinearLayout.LayoutParams full(int top){LinearLayout.LayoutParams p=new LinearLayout.LayoutParams(-1,-2);p.setMargins(0,dp(top),0,0);return p;}
 private LinearLayout.LayoutParams half(int left){LinearLayout.LayoutParams p=new LinearLayout.LayoutParams(0,-2,1);p.setMargins(dp(left),0,0,0);return p;}
 private void toast(String s){Toast.makeText(this,s,Toast.LENGTH_LONG).show();}
 private File prepDir(){return new File(getFilesDir(),"bills-prep");}

 // ------------------------------------------------------------------ lifecycle
 @Override public void onCreate(Bundle state){
  applyAppearance();super.onCreate(state);
  getWindow().addFlags(WindowManager.LayoutParams.FLAG_SECURE);getWindow().setStatusBarColor(BG);getWindow().setNavigationBarColor(SURFACE);
  if(!dark)getWindow().getDecorView().setSystemUiVisibility(View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR|View.SYSTEM_UI_FLAG_LIGHT_NAVIGATION_BAR);
  try{store=new CryptoStore(this);api=new BridgeApi(store);outbox=new BillOutbox(this);}catch(Exception e){fatal("Could not open TallyBridge: "+e.getMessage());return;}
  if(store.get("base").isEmpty()||store.get("device").isEmpty()){fatal("Open TallyBridge once and pair this phone with your PC first.");return;}
  try{companies=new JSONArray(store.get("billCompanies").isEmpty()?"[]":store.get("billCompanies"));}catch(Exception ignored){}
  voucherType=store.get("billType").equals("Sales")?"Sales":"Purchase";
  if(state!=null){restore(state);}else cleanupPrep();
  build();
  refresh(true);
  Intent in=getIntent();
  if(state==null&&in!=null){
   List<Uri> shared=sharedUris(in);
   if(!shared.isEmpty())importUris(shared,"Shared");
   else if(ACTION_CAMERA.equals(in.getAction()))main.postDelayed(this::takePhoto,250);
  }
 }
 @Override protected void onNewIntent(Intent in){super.onNewIntent(in);List<Uri> shared=sharedUris(in);if(!shared.isEmpty())importUris(shared,"Shared");}
 @Override protected void onSaveInstanceState(Bundle out){
  super.onSaveInstanceState(out);
  ArrayList<String> full=new ArrayList<>(),thumb=new ArrayList<>(),group=new ArrayList<>();
  synchronized(pages){for(BillPages.Page p:pages){full.add(p.full.getAbsolutePath());thumb.add(p.thumb.getAbsolutePath());group.add(p.group);}}
  out.putStringArrayList("full",full);out.putStringArrayList("thumb",thumb);out.putStringArrayList("group",group);out.putString("capture",pendingCapture);
 }
 private void restore(Bundle s){
  ArrayList<String> full=s.getStringArrayList("full"),thumb=s.getStringArrayList("thumb"),group=s.getStringArrayList("group");
  if(full!=null&&thumb!=null&&group!=null)for(int i=0;i<full.size();i++){File f=new File(full.get(i));if(f.exists())pages.add(new BillPages.Page(f,new File(thumb.get(i)),group.get(i)));}
  pendingCapture=s.getString("capture","");
 }
 @Override protected void onDestroy(){
  destroyed=true;work.shutdownNow();net.shutdownNow();main.removeCallbacksAndMessages(null);
  if(isFinishing()){synchronized(pages){for(BillPages.Page p:pages){p.full.delete();p.thumb.delete();}pages.clear();}}
  super.onDestroy();
 }
 private void cleanupPrep(){
  // Leftovers from a closed screen. Pages of bills that were sent live in the outbox, not here.
  File[] fs=prepDir().listFiles();if(fs!=null)for(File f:fs)if(System.currentTimeMillis()-f.lastModified()>6*3600*1000L)f.delete();
  File[] cs=CaptureProvider.dir(this).listFiles();if(cs!=null)for(File f:cs)if(System.currentTimeMillis()-f.lastModified()>6*3600*1000L)f.delete();
 }

 private void fatal(String message){LinearLayout p=col();p.setBackgroundColor(BG);p.setPadding(dp(24),dp(60),dp(24),dp(24));TextView h=txt("Purchase bill",28,NAVY);h.setTypeface(Typeface.DEFAULT_BOLD);p.addView(h);p.addView(txt(message,16,MUTED),full(16));p.addView(primary("Close",v->finish()),full(24));setContentView(p);}

 // ------------------------------------------------------------------ screen
 private void build(){
  LinearLayout root=col();root.setBackgroundColor(BG);
  root.addView(ScreenHeader.bar(this,null,"Scan bill","Photograph the bill or pick a PDF. It goes to your PC, InvoiceTotally reads it, and you review it there before anything is posted to Tally.",NAVY,TEAL,MUTED));
  ScrollView scroll=new ScrollView(this);body=col();body.setPadding(dp(18),dp(2),dp(18),dp(32));scroll.addView(body);
  root.addView(scroll,new LinearLayout.LayoutParams(-1,0,1));setContentView(root);

  body.addView(label("COMPANY"),full(6));
  companySpin=new Spinner(this);ScreenHeader.spinner(companySpin,dark);body.addView(companySpin,full(6));
  fillCompanies();

  body.addView(label("BILL TYPE"),full(14));
  LinearLayout types=new LinearLayout(this);purchaseBtn=secondary("Purchase",v->setType("Purchase"));salesBtn=secondary("Sales",v->setType("Sales"));
  types.addView(purchaseBtn,half(0));types.addView(salesBtn,half(10));body.addView(types,full(6));setType(voucherType);

  LinearLayout head=new LinearLayout(this);head.setGravity(Gravity.CENTER_VERTICAL);
  TextView pl=label("PAGES");head.addView(pl,new LinearLayout.LayoutParams(0,-2,1));pageCount=txt("",12,MUTED);head.addView(pageCount);body.addView(head,full(16));
  HorizontalScrollView strip=new HorizontalScrollView(this);pageStrip=new LinearLayout(this);pageStrip.setPadding(0,dp(6),0,dp(6));strip.addView(pageStrip);body.addView(strip,full(4));

  LinearLayout add=new LinearLayout(this);
  add.addView(secondary("Take photo",v->takePhoto()),half(0));add.addView(secondary("Photos / PDF",v->pick()),half(10));body.addView(add,full(8));

  status=txt("",13,TEAL);status.setTypeface(null,Typeface.BOLD);body.addView(status,full(12));
  sendBtn=primary("Send to PC",v->send());body.addView(sendBtn,full(8));

  LinearLayout lh=new LinearLayout(this);lh.setGravity(Gravity.CENTER_VERTICAL);
  TextView sent=txt("Sent bills",19,NAVY);sent.setTypeface(Typeface.DEFAULT_BOLD);lh.addView(sent,new LinearLayout.LayoutParams(0,-2,1));
  TextView ref=txt("Refresh",14,TEAL);ref.setTypeface(null,Typeface.BOLD);ref.setPadding(dp(12),dp(8),0,dp(8));ref.setOnClickListener(v->refresh(true));lh.addView(ref);
  body.addView(lh,full(26));
  listBox=col();body.addView(listBox,full(4));
  paintPages();paintList();
 }

 private void setType(String t){
  voucherType=t;store.put("billType",t);
  if(purchaseBtn==null)return;
  purchaseBtn.setBackground(shape(t.equals("Purchase")?TINT:SURFACE,13));purchaseBtn.setTextColor(t.equals("Purchase")?TEAL:NAVY);
  salesBtn.setBackground(shape(t.equals("Sales")?TINT:SURFACE,13));salesBtn.setTextColor(t.equals("Sales")?TEAL:NAVY);
 }

 private void fillCompanies(){
  List<String> names=new ArrayList<>();for(int i=0;i<companies.length();i++)names.add(companies.optJSONObject(i)==null?"":companies.optJSONObject(i).optString("name"));
  if(names.isEmpty())names.add("Connect to the PC to load companies");
  companySpin.setAdapter(new ArrayAdapter<>(this,android.R.layout.simple_spinner_dropdown_item,names));
  String saved=store.get("billCompany");for(int i=0;i<companies.length();i++){JSONObject c=companies.optJSONObject(i);if(c!=null&&c.optString("id").equals(saved))companySpin.setSelection(i);}
  companySpin.setOnItemSelectedListener(new AdapterView.OnItemSelectedListener(){
   public void onItemSelected(AdapterView<?> p,View v,int pos,long id){JSONObject c=companies.optJSONObject(pos);if(c!=null)store.put("billCompany",c.optString("id"));}
   public void onNothingSelected(AdapterView<?> p){}
  });
 }

 private void paintPages(){
  if(pageStrip==null)return;pageStrip.removeAllViews();
  List<BillPages.Page> copy;synchronized(pages){copy=new ArrayList<>(pages);}
  for(int i=0;i<copy.size();i++){
   BillPages.Page p=copy.get(i);
   FrameLayout frame=new FrameLayout(this);
   ImageView iv=new ImageView(this);iv.setScaleType(ImageView.ScaleType.CENTER_CROP);iv.setBackground(shape(SURFACE,12));
   Bitmap b=BillPages.preview(p.thumb.exists()?p.thumb:p.full,360);if(b!=null)iv.setImageBitmap(b);
   iv.setContentDescription("Page "+(i+1));
   frame.addView(iv,new FrameLayout.LayoutParams(-1,-1));
   TextView n=txt(String.valueOf(i+1),12,Color.WHITE);n.setTypeface(null,Typeface.BOLD);n.setPadding(dp(8),dp(2),dp(8),dp(2));n.setBackground(shape(0xaa102d44,10));
   FrameLayout.LayoutParams np=new FrameLayout.LayoutParams(-2,-2,Gravity.TOP|Gravity.START);np.setMargins(dp(6),dp(6),0,0);frame.addView(n,np);
   TextView x=txt("✕",14,Color.WHITE);x.setGravity(Gravity.CENTER);x.setBackground(shape(0xcc7d1f18,14));x.setContentDescription("Remove page "+(i+1));
   FrameLayout.LayoutParams xp=new FrameLayout.LayoutParams(dp(30),dp(30),Gravity.TOP|Gravity.END);xp.setMargins(0,dp(6),dp(6),0);frame.addView(x,xp);
   final BillPages.Page page=p;x.setOnClickListener(v->{synchronized(pages){pages.remove(page);}page.full.delete();page.thumb.delete();paintPages();});
   LinearLayout.LayoutParams fp=new LinearLayout.LayoutParams(dp(112),dp(150));fp.setMargins(0,0,dp(10),0);pageStrip.addView(frame,fp);
  }
  if(copy.isEmpty()){TextView e=txt("No pages yet — take a photo of the bill or pick a PDF.",13,MUTED);pageStrip.addView(e);}
  pageCount.setText(copy.isEmpty()?"":copy.size()+" page"+(copy.size()==1?"":"s")+" • max "+BillPages.MAX_PAGES);
  updateSend();
 }

 private void updateSend(){
  if(sendBtn==null)return;
  int n;synchronized(pages){n=pages.size();}
  sendBtn.setEnabled(!busy&&n>0&&companies.length()>0&&enabled);
  sendBtn.setAlpha(sendBtn.isEnabled()?1f:.5f);
  sendBtn.setText(n==0?"Send to PC":"Send "+n+" page"+(n==1?"":"s")+" to PC");
  if(!busy&&status!=null){
   if(!enabled){status.setText(reason);status.setTextColor(AMBER);}
   else if(companies.length()==0){status.setText("Connect to the PC once to load your companies.");status.setTextColor(AMBER);}
   else if(!pcReachable){status.setText("PC not reachable — bills you send will wait on this phone and go automatically later.");status.setTextColor(AMBER);}
   else{status.setText(n==0?"":"Ready to send");status.setTextColor(TEAL);}
  }
 }

 // ------------------------------------------------------------------ add pages
 private void takePhoto(){
  if(remaining()<=0){toast("Up to "+BillPages.MAX_PAGES+" pages can be sent at once");return;}
  try{
   File f=new File(CaptureProvider.dir(this),"cap-"+UUID.randomUUID()+".jpg");pendingCapture=f.getAbsolutePath();
   Uri u=CaptureProvider.uriFor(f);
   Intent i=new Intent(MediaStore.ACTION_IMAGE_CAPTURE).putExtra(MediaStore.EXTRA_OUTPUT,u);
   i.setClipData(ClipData.newRawUri("bill",u));i.addFlags(Intent.FLAG_GRANT_WRITE_URI_PERMISSION|Intent.FLAG_GRANT_READ_URI_PERMISSION);
   startActivityForResult(i,REQ_CAMERA);
  }catch(ActivityNotFoundException e){toast("No camera app is available. Use Photos / PDF instead.");}
 }
 private void pick(){
  Intent i=new Intent(Intent.ACTION_OPEN_DOCUMENT).addCategory(Intent.CATEGORY_OPENABLE).setType("*/*");
  i.putExtra(Intent.EXTRA_MIME_TYPES,new String[]{"image/*","application/pdf"});i.putExtra(Intent.EXTRA_ALLOW_MULTIPLE,true);
  try{startActivityForResult(i,REQ_PICK);}catch(ActivityNotFoundException e){toast("No file picker is available on this phone");}
 }
 @Override protected void onActivityResult(int req,int result,Intent data){
  super.onActivityResult(req,result,data);
  if(req==REQ_CAMERA){
   File f=pendingCapture.isEmpty()?null:new File(pendingCapture);pendingCapture="";
   if(result!=RESULT_OK||f==null||!f.exists()||f.length()==0){if(f!=null)f.delete();return;}
   importUris(Collections.singletonList(Uri.fromFile(f)),"Camera photos");
   return;
  }
  if(req==REQ_PICK&&result==RESULT_OK&&data!=null){
   List<Uri> uris=new ArrayList<>();ClipData clip=data.getClipData();
   if(clip!=null)for(int i=0;i<clip.getItemCount();i++){Uri u=clip.getItemAt(i).getUri();if(u!=null)uris.add(u);}
   else if(data.getData()!=null)uris.add(data.getData());
   importUris(uris,"Photos");
  }
 }
 private int remaining(){synchronized(pages){return BillPages.MAX_PAGES-pages.size();}}

 private void importUris(List<Uri> uris,String photoGroup){
  if(uris.isEmpty())return;
  busy=true;updateSend();status.setTextColor(TEAL);status.setText("Preparing pages…");
  work.execute(()->{
   String problem=null;int added=0;
   for(Uri u:uris){
    if(destroyed)return;
    int left=remaining();if(left<=0){problem="Only the first "+BillPages.MAX_PAGES+" pages were added";break;}
    try{
     List<BillPages.Page> got;
     boolean camera="file".equals(u.getScheme());
     if(!camera&&BillPages.isPdf(getContentResolver(),u))got=BillPages.fromPdf(getContentResolver(),u,prepDir(),BillPages.displayName(getContentResolver(),u),left);
     else got=Collections.singletonList(BillPages.fromImage(getContentResolver(),u,prepDir(),photoGroup));
     synchronized(pages){pages.addAll(got);}added+=got.size();
     int a=added;main.post(()->{if(!destroyed){status.setText("Preparing pages… "+a);paintPages();}});
    }catch(Exception e){problem=e.getMessage();}
    finally{if("file".equals(u.getScheme()))new File(u.getPath()).delete();}
   }
   String why=problem;int total=added;
   main.post(()->{if(destroyed)return;busy=false;paintPages();if(why!=null){status.setText(why);status.setTextColor(AMBER);}else status.setText(total+" page"+(total==1?"":"s")+" added");});
  });
 }

 private List<Uri> sharedUris(Intent in){
  LinkedHashSet<Uri> out=new LinkedHashSet<>();if(in==null)return new ArrayList<>();
  if(Intent.ACTION_SEND_MULTIPLE.equals(in.getAction())){ArrayList<Uri> l=in.getParcelableArrayListExtra(Intent.EXTRA_STREAM);if(l!=null)out.addAll(l);}
  else if(Intent.ACTION_SEND.equals(in.getAction())){Uri u=in.getParcelableExtra(Intent.EXTRA_STREAM);if(u!=null)out.add(u);}
  if(Intent.ACTION_SEND.equals(in.getAction())||Intent.ACTION_SEND_MULTIPLE.equals(in.getAction())){ClipData c=in.getClipData();if(c!=null)for(int i=0;i<c.getItemCount();i++){Uri u=c.getItemAt(i).getUri();if(u!=null)out.add(u);}}
  return new ArrayList<>(out);
 }

 // ------------------------------------------------------------------ send
 private void send(){
  JSONObject c=companies.optJSONObject(companySpin.getSelectedItemPosition());
  if(c==null){toast("Choose a company");return;}
  List<BillPages.Page> copy;synchronized(pages){copy=new ArrayList<>(pages);}
  if(copy.isEmpty()){toast("Add at least one page");return;}
  busy=true;updateSend();status.setTextColor(TEAL);status.setText("Saving on this phone…");
  String type=voucherType;
  net.execute(()->{
   JSONObject m;
   try{m=outbox.add(c.optString("id"),c.optString("name"),type,copy);}
   catch(Exception e){main.post(()->{if(destroyed)return;busy=false;status.setText(e.getMessage());status.setTextColor(RED);updateSend();});return;}
   synchronized(pages){pages.removeAll(copy);}
   main.post(()->{if(!destroyed){paintPages();status.setText("Sending to PC…");}});
   String why=outbox.send(api,m.optString("id"),(done,total)->main.post(()->{if(!destroyed)status.setText("Sending page "+done+" of "+total+"…");}));
   main.post(()->{
    if(destroyed)return;busy=false;
    if(why==null){status.setText("Sent ✓ InvoiceTotally is reading it. Review it on the PC.");status.setTextColor(TEAL);toast("Bill sent to the PC");}
    else{status.setText(why);status.setTextColor(AMBER);}
    updateSend();refresh(true);
   });
  });
 }

 // ------------------------------------------------------------------ status list
 private void refresh(boolean fromPc){
  net.execute(()->{
   outbox.flush(api);
   try{
    JSONObject x=api.get("/api/v1/mobile/bills"+(fromPc?"?refresh=1":""));
    JSONArray cs=x.optJSONArray("companies");if(cs==null)cs=new JSONArray();
    store.put("billCompanies",cs.toString());
    JSONArray ups=x.optJSONArray("uploads");
    JSONArray finalCs=cs;
    main.post(()->{
     if(destroyed)return;pcReachable=true;enabled=x.optBoolean("enabled",true);reason=x.optString("reason");
     String before=companies.toString();companies=finalCs;if(!before.equals(finalCs.toString()))fillCompanies();
     uploads=ups==null?new JSONArray():ups;paintList();updateSend();
    });
   }catch(Http.Failure f){
    main.post(()->{if(destroyed)return;pcReachable=true;if(f.status==404){enabled=false;reason="Update TallyBridge on the PC to 2.6 or newer to send bills from the phone.";}else{reason=f.getMessage();}paintList();updateSend();});
   }catch(Exception e){
    main.post(()->{if(destroyed)return;pcReachable=false;paintList();updateSend();});
   }
  });
 }

 private String when(String iso){try{return Instant.parse(iso).atZone(ZoneId.systemDefault()).format(DateTimeFormatter.ofPattern("dd MMM, hh:mm a",Locale.ENGLISH));}catch(Exception e){return "";}}
 private TextView pill(String text,int fg){TextView t=txt(text,11,fg);t.setTypeface(null,Typeface.BOLD);t.setPadding(dp(9),dp(4),dp(9),dp(4));GradientDrawable d=new GradientDrawable();d.setCornerRadius(dp(10));d.setColor((fg&0x00ffffff)|0x22000000);t.setBackground(d);return t;}

 private void paintList(){
  if(listBox==null)return;listBox.removeAllViews();
  List<JSONObject> local=outbox.list();
  for(JSONObject m:local){
   LinearLayout card=card();
   int n=m.optJSONArray("pages")==null?0:m.optJSONArray("pages").length();
   row(card,m.optString("company")+" • "+n+" page"+(n==1?"":"s"),pill("On this phone",AMBER));
   card.addView(txt(when(m.optString("createdAt"))+(m.optString("error").isEmpty()?"":"\n"+m.optString("error")),12,MUTED));
   LinearLayout actions=new LinearLayout(this);
   actions.addView(secondary("Send now",v->{status.setText("Sending…");net.execute(()->{String why=outbox.send(api,m.optString("id"),null);main.post(()->{if(!destroyed){toast(why==null?"Bill sent to the PC":why);refresh(true);}});});}),half(0));
   actions.addView(secondary("Delete",v->new AlertDialog.Builder(this).setTitle("Delete this bill?").setMessage("It has not reached the PC yet.").setPositiveButton("Delete",(d,w)->{outbox.delete(m.optString("id"));paintList();}).setNegativeButton("Keep",null).show()),half(10));
   card.addView(actions,full(8));listBox.addView(card,full(8));
  }
  for(int i=0;i<uploads.length();i++){
   JSONObject u=uploads.optJSONObject(i);if(u==null)continue;
   LinearLayout card=card();
   String st=u.optString("status");int color=u.optString("state").equals("failed")?RED:st.equals("In Tally")?TEAL:st.startsWith("Ready")?TEAL:AMBER;
   int n=u.optInt("pages");
   row(card,u.optString("company")+" • "+n+" page"+(n==1?"":"s")+(u.optString("voucherType").equals("Sales")?" • Sales":""),pill(st.isEmpty()?u.optString("state"):st,color));
   card.addView(txt(when(u.optString("createdAt")),12,MUTED));
   JSONObject cloud=u.optJSONObject("cloud");JSONArray inv=cloud==null?null:cloud.optJSONArray("invoices");
   if(inv!=null)for(int k=0;k<inv.length();k++){
    JSONObject b=inv.optJSONObject(k);if(b==null)continue;
    String line=(b.optString("number").isEmpty()?"Bill":"Bill "+b.optString("number"))+(b.optString("party").isEmpty()?"":" • "+b.optString("party"))+(b.isNull("total")||!b.has("total")?"":" • "+currency.format(b.optDouble("total")));
    card.addView(txt(line+"\n"+b.optString("state")+(b.optString("error").isEmpty()?"":" — "+b.optString("error")),13,NAVY),full(6));
   }
   if(!u.optString("error").isEmpty()&&!u.optString("state").equals("sent"))card.addView(txt(u.optString("error"),12,RED));
   listBox.addView(card,full(8));
  }
  if(local.isEmpty()&&uploads.length()==0)listBox.addView(txt(pcReachable?"No bills sent from this phone yet.":"PC not reachable — the list will load when TallyBridge is connected.",13,MUTED),full(4));
 }
 private LinearLayout card(){LinearLayout c=col();c.setPadding(dp(14),dp(12),dp(14),dp(12));c.setBackground(shape(SURFACE,15));return c;}
 private void row(LinearLayout card,String title,View right){
  LinearLayout r=new LinearLayout(this);r.setGravity(Gravity.CENTER_VERTICAL);
  TextView t=txt(title,15,NAVY);t.setTypeface(null,Typeface.BOLD);r.addView(t,new LinearLayout.LayoutParams(0,-2,1));r.addView(right);card.addView(r);
 }
}
