package in.tallybridge.app;

import android.graphics.Bitmap;
import android.view.WindowManager;
import android.widget.*;
import androidx.test.ext.junit.runners.AndroidJUnit4;
import androidx.test.platform.app.InstrumentationRegistry;
import androidx.test.rule.ActivityTestRule;
import org.junit.*;
import org.junit.runner.RunWith;
import org.json.JSONObject;
import org.json.JSONArray;
import java.io.File;
import java.io.FileOutputStream;
import java.lang.reflect.*;
import static org.junit.Assert.*;

/** Fixture-only visual check. No server requests, accounts or production data. */
@RunWith(AndroidJUnit4.class)
public class DesignTest {
 @Rule public ActivityTestRule<MainActivity> rule=new ActivityTestRule<>(MainActivity.class);
 private void call(String name)throws Exception {Method m=MainActivity.class.getDeclaredMethod(name);m.setAccessible(true);m.invoke(rule.getActivity());}
 private void field(String name,Object value)throws Exception {Field f=MainActivity.class.getDeclaredField(name);f.setAccessible(true);f.set(rule.getActivity(),value);}
 private static boolean hasText(android.view.View v,String text){if(v instanceof TextView&&text.contentEquals(((TextView)v).getText()))return true;if(v instanceof android.view.ViewGroup){android.view.ViewGroup g=(android.view.ViewGroup)v;for(int i=0;i<g.getChildCount();i++)if(hasText(g.getChildAt(i),text))return true;}return false;}
 private void ui(Runnable r){InstrumentationRegistry.getInstrumentation().runOnMainSync(r);InstrumentationRegistry.getInstrumentation().waitForIdleSync();}
 private java.util.List<android.view.accessibility.AccessibilityNodeInfo> cancelNodes(android.app.UiAutomation automation){
  android.view.accessibility.AccessibilityNodeInfo root=automation.getRootInActiveWindow();
  if(root==null)return java.util.Collections.emptyList();
  java.util.List<android.view.accessibility.AccessibilityNodeInfo> nodes=root.findAccessibilityNodeInfosByText("CANCEL");
  if(nodes==null||nodes.isEmpty())nodes=root.findAccessibilityNodeInfosByText("Cancel");
  return nodes==null?java.util.Collections.emptyList():nodes;
 }
 private void cancelDialog()throws Exception {
  android.app.UiAutomation automation=InstrumentationRegistry.getInstrumentation().getUiAutomation();
  for(int attempt=0;attempt<5;attempt++){
   java.util.List<android.view.accessibility.AccessibilityNodeInfo> nodes=cancelNodes(automation);
   if(nodes.isEmpty()){Thread.sleep(200);if(attempt>0)return;continue;}
   android.view.accessibility.AccessibilityNodeInfo node=nodes.get(0),target=node;
   while(target!=null&&!target.isClickable())target=target.getParent();
   if(target!=null)target.performAction(android.view.accessibility.AccessibilityNodeInfo.ACTION_CLICK);
   Thread.sleep(250);InstrumentationRegistry.getInstrumentation().waitForIdleSync();
   if(cancelNodes(automation).isEmpty())return;
   android.graphics.Rect bounds=new android.graphics.Rect();(target!=null?target:node).getBoundsInScreen(bounds);
   if(!bounds.isEmpty()){
    android.os.ParcelFileDescriptor command=automation.executeShellCommand("input tap "+bounds.centerX()+" "+bounds.centerY());
    if(command!=null)command.close();
   }
   Thread.sleep(350);InstrumentationRegistry.getInstrumentation().waitForIdleSync();
   if(cancelNodes(automation).isEmpty())return;
  }
  fail("Cancel dialog remained visible after accessibility and coordinate dismissal attempts");
 }
 private void capture(String name)throws Exception {
  Thread.sleep(400);
  Bitmap b=InstrumentationRegistry.getInstrumentation().getUiAutomation().takeScreenshot();assertNotNull(b);
  File dir=new File(rule.getActivity().getFilesDir(),"ui");assertTrue(dir.exists()||dir.mkdirs());
  try(FileOutputStream out=new FileOutputStream(new File(dir,name+".png"))){assertTrue(b.compress(Bitmap.CompressFormat.PNG,100,out));}b.recycle();
 }
 @Test public void screensRenderWithLongCompanyAndLargeAmounts()throws Exception {
  ui(()->rule.getActivity().getWindow().clearFlags(WindowManager.LayoutParams.FLAG_SECURE));
  capture("01-unlock");
  ui(()->{try{
   field("company","Vijayalaxmi Textiles • Solapur");
   JSONObject summary=new JSONObject().put("receivable",12345678.50).put("payable",245800.00).put("sales",1834500.75).put("rFetchedAt","2026-09-26T04:00:00Z").put("pFetchedAt","2026-09-26T04:00:00Z").put("sFetchedAt","2026-09-26T04:00:00Z");
   field("boot",new JSONObject().put("tallyOnline",true).put("autoSync",new JSONObject().put("lastOk","2026-09-27T08:00:00Z").put("lastError","Previous report warning")).put("lastSuccess","2026-09-27T08:00:00Z").put("lastUpdated","2026-09-27T08:02:00Z").put("summaries",new JSONObject().put("Vijayalaxmi Textiles • Solapur",summary)));
   Field st=MainActivity.class.getDeclaredField("store");st.setAccessible(true);((CryptoStore)st.get(rule.getActivity())).put("favourites:Vijayalaxmi Textiles • Solapur","[\"ABC Textiles & Trading Company\",\"Raj Fabrics\"]");
   call("renderHome");
   android.view.View root=rule.getActivity().getWindow().getDecorView();
   for(String section:SectionNav.LABELS)assertTrue("bottom bar shows "+section,hasText(root,section));
   assertTrue(hasText(root,"Voice order"));assertTrue(hasText(root,"YOUR PARTIES"));
   Field status=MainActivity.class.getDeclaredField("statusView");status.setAccessible(true);TextView statusText=(TextView)status.get(rule.getActivity());assertNotNull(statusText);assertTrue(statusText.getText().toString().startsWith("Connected"));assertFalse(statusText.getText().toString().contains("needs attention"));
  }catch(Exception e){throw new RuntimeException(e);}});
  capture("02-overview");
  ui(()->{try{Field b=MainActivity.class.getDeclaredField("body");b.setAccessible(true);LinearLayout body=(LinearLayout)b.get(rule.getActivity());((ScrollView)body.getChildAt(0)).fullScroll(android.view.View.FOCUS_DOWN);}catch(Exception e){throw new RuntimeException(e);}});
  capture("41-overview-bottom");
  ui(()->{try{field("tab","Parties");call("parties");}catch(Exception e){throw new RuntimeException(e);}});
  capture("39-parties");
  ui(()->assertTrue("favourite party listed",hasText(rule.getActivity().getWindow().getDecorView(),"Raj Fabrics")));
  ui(()->{try{field("tab","More");call("widgetsScreen");}catch(Exception e){throw new RuntimeException(e);}});
  capture("40-widgets");
  ui(()->assertTrue("widget list shown",hasText(rule.getActivity().getWindow().getDecorView(),"Order desk")));
  ui(()->{try{field("tab","Reports");call("reports");}catch(Exception e){throw new RuntimeException(e);}});
  capture("03-reports");
  ui(()->{try{call("connectScreen");}catch(Exception e){throw new RuntimeException(e);}});
  capture("04-connect");
  ui(()->{try{
   rule.getActivity().getSharedPreferences("appearance",0).edit().putString("mode","Dark").commit();
   call("applyAppearance");field("tab","Home");call("renderHome");
   Field f=MainActivity.class.getDeclaredField("dark");f.setAccessible(true);assertTrue(f.getBoolean(rule.getActivity()));
  }catch(Exception e){throw new RuntimeException(e);}});
  capture("05-dark-overview");
  ui(()->{try{field("tab","Reports");call("reports");}catch(Exception e){throw new RuntimeException(e);}});
  capture("06-dark-reports");
  ui(()->{try{call("connectScreen");}catch(Exception e){throw new RuntimeException(e);}});
  capture("07-dark-connect");
  ui(()->{try{field("tab","Parties");call("parties");}catch(Exception e){throw new RuntimeException(e);}});
  capture("44-dark-parties");
  ui(()->{try{field("tab","More");call("more");}catch(Exception e){throw new RuntimeException(e);}});
  capture("45-dark-more");
  ui(()->{try{field("tab","More");call("widgetsScreen");}catch(Exception e){throw new RuntimeException(e);}});
  capture("46-dark-widgets");
  ui(()->{try{
   JSONObject report=new JSONObject("{\"fetchedAt\":\"2026-09-27T08:00:00Z\",\"total\":24500,\"drCr\":\"Dr\",\"bills\":[{\"id\":\"fixture1\",\"billNumber\":\"INV-2026-001\",\"date\":\"2026-06-01\",\"invoiceDays\":118,\"pendingAmount\":24500,\"drCr\":\"Dr\"}]}");
   Method m=MainActivity.class.getDeclaredMethod("renderOutstanding",String.class,String.class,JSONObject.class);m.setAccessible(true);m.invoke(rule.getActivity(),"ABC Textiles & Trading Company","",report);
  }catch(Exception e){throw new RuntimeException(e);}});
  capture("09-outstanding");
  ui(()->{try{
   Method m=MainActivity.class.getDeclaredMethod("outstandingFilters",String.class,String.class,JSONObject.class);m.setAccessible(true);m.invoke(rule.getActivity(),"ABC Textiles","",new JSONObject());
  }catch(Exception e){throw new RuntimeException(e);}});
  capture("10-filters");
  ui(()->{try{call("syncStatus");}catch(Exception e){throw new RuntimeException(e);}});
  capture("11-sync");
  ui(()->{try{field("tab","More");call("more");call("chooseAppearance");}catch(Exception e){throw new RuntimeException(e);}});
  capture("08-dark-appearance");
  cancelDialog();
  ui(()->{try{
   rule.getActivity().getSharedPreferences("appearance",0).edit().putString("mode","Light").commit();call("applyAppearance");
   field("tab","Payments");field("company","Vijayalaxmi Textiles • Solapur");
   JSONObject bank=new JSONObject().put("bankName","State Bank").put("accountNumber","001234567890").put("ifsc","SBIN0000123").put("upi","demo@upi");
   JSONObject health=new JSONObject().put("ready",true).put("readyForPayments",true).put("score",100).put("tallyOnline",true).put("bank",bank)
    .put("items",new JSONArray()
     .put(new JSONObject().put("label","GSTIN").put("ok",true).put("value","27ABCDE1234F1Z5").put("required",true))
     .put(new JSONObject().put("label","Address").put("ok",true).put("value","Market Road, Solapur 413001").put("required",true))
     .put(new JSONObject().put("label","Mobile number").put("ok",true).put("value","9876543210").put("required",true))
     .put(new JSONObject().put("label","UPI ID").put("ok",true).put("value","demo@upi")));
   Method p=MainActivity.class.getDeclaredMethod("renderPayments",JSONObject.class);p.setAccessible(true);p.invoke(rule.getActivity(),health);
  }catch(Exception e){throw new RuntimeException(e);}});
  capture("15-payments");
  ui(()->{try{
   JSONObject report=new JSONObject("{\"fetchedAt\":\"2026-09-27T08:00:00Z\",\"total\":24500,\"drCr\":\"Dr\",\"bills\":[{\"id\":\"fixture1\",\"billNumber\":\"INV-2026-001\",\"date\":\"2026-06-01\",\"invoiceDays\":118,\"pendingAmount\":24500,\"drCr\":\"Dr\"}]}");
   Method m=MainActivity.class.getDeclaredMethod("renderOutstanding",String.class,String.class,JSONObject.class);m.setAccessible(true);m.invoke(rule.getActivity(),"ABC Textiles & Trading Company","",report);
  }catch(Exception e){throw new RuntimeException(e);}});
  capture("47-outstanding-light");
  ui(()->{try{call("syncStatus");}catch(Exception e){throw new RuntimeException(e);}});
  capture("48-sync-light");
  ui(()->{try{
   field("tab","More");
   JSONObject health=new JSONObject().put("ready",false).put("score",71).put("tallyOnline",true)
    .put("items",new JSONArray()
     .put(new JSONObject().put("label","GSTIN").put("ok",true).put("value","27ABCDE1234F1Z5").put("required",true))
     .put(new JSONObject().put("label","Address").put("ok",true).put("value","Market Road, Solapur").put("required",true))
     .put(new JSONObject().put("label","Mobile number").put("ok",false).put("hint","Add company mobile number in Tally.").put("required",true))
     .put(new JSONObject().put("label","UPI ID").put("ok",false).put("hint","Optional payment QR setting.")))
    .put("issues",new JSONArray().put("Mobile number is missing"));
   Method m=MainActivity.class.getDeclaredMethod("renderHealth",JSONObject.class);m.setAccessible(true);m.invoke(rule.getActivity(),health);
  }catch(Exception e){throw new RuntimeException(e);}});
  capture("16-health");
  ui(()->{try{
   JSONObject diag=new JSONObject().put("version","2.1.0").put("tallyOnline",true).put("lastSuccess","2026-09-28T03:00:00Z").put("gateState","READY")
    .put("cache",new JSONObject().put("entries",42).put("latestAt","2026-09-28T03:00:00Z"))
    .put("autoSync",new JSONObject().put("running",false).put("lastOk","2026-09-28T03:00:00Z"))
    .put("recentReads",new JSONArray().put(new JSONObject().put("report","Sales register").put("company","Vijayalaxmi Textiles").put("durationMs",220).put("success",true)));
   Method m=MainActivity.class.getDeclaredMethod("renderDiagnostics",JSONObject.class);m.setAccessible(true);m.invoke(rule.getActivity(),diag);
  }catch(Exception e){throw new RuntimeException(e);}});
  capture("17-diagnostics");
  ui(()->{try{field("tab","More");call("more");}catch(Exception e){throw new RuntimeException(e);}});
  capture("18-more");
  ui(()->{try{
   rule.getActivity().getSharedPreferences("appearance",0).edit().putString("mode","Light").commit();call("applyAppearance");
   Field f=MainActivity.class.getDeclaredField("dark");f.setAccessible(true);assertFalse(f.getBoolean(rule.getActivity()));
   rule.getActivity().getSharedPreferences("appearance",0).edit().putString("mode","System").commit();call("applyAppearance");
   Field st=MainActivity.class.getDeclaredField("store");st.setAccessible(true);((CryptoStore)st.get(rule.getActivity())).put("favourites:Vijayalaxmi Textiles • Solapur","[]");
   boolean systemDark=(rule.getActivity().getResources().getConfiguration().uiMode&android.content.res.Configuration.UI_MODE_NIGHT_MASK)==android.content.res.Configuration.UI_MODE_NIGHT_YES;
   assertEquals(systemDark,f.getBoolean(rule.getActivity()));
  }catch(Exception e){throw new RuntimeException(e);}});
 }
}
