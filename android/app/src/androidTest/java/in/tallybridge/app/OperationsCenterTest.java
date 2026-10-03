package in.tallybridge.app;

import android.content.Context;
import android.graphics.Bitmap;
import android.view.WindowManager;
import androidx.test.ext.junit.runners.AndroidJUnit4;
import androidx.test.platform.app.InstrumentationRegistry;
import androidx.test.rule.ActivityTestRule;
import org.json.JSONArray;
import org.json.JSONObject;
import org.junit.*;
import org.junit.runner.RunWith;
import java.io.File;
import java.io.FileOutputStream;
import java.lang.reflect.Method;
import static org.junit.Assert.*;

@RunWith(AndroidJUnit4.class)
public class OperationsCenterTest {
 @Rule public ActivityTestRule<WorkActivity> rule=new ActivityTestRule<>(WorkActivity.class,true,false);
 private void ui(Runnable r){InstrumentationRegistry.getInstrumentation().runOnMainSync(r);InstrumentationRegistry.getInstrumentation().waitForIdleSync();}
 private Object call(String name,Class<?>[] types,Object...args)throws Exception{Method m=WorkActivity.class.getDeclaredMethod(name,types);m.setAccessible(true);return m.invoke(rule.getActivity(),args);}
 private void capture(String name)throws Exception{Thread.sleep(350);Bitmap b=InstrumentationRegistry.getInstrumentation().getUiAutomation().takeScreenshot();assertNotNull(b);File dir=new File(rule.getActivity().getFilesDir(),"ui");assertTrue(dir.exists()||dir.mkdirs());try(FileOutputStream out=new FileOutputStream(new File(dir,name+".png"))){assertTrue(b.compress(Bitmap.CompressFormat.PNG,100,out));}b.recycle();}

 @Before public void seed()throws Exception{
  Context c=InstrumentationRegistry.getInstrumentation().getTargetContext();
  CryptoStore s=new CryptoStore(c);s.put("base","https://127.0.0.1:9");s.put("device","fixture-device");s.put("company","Vijayalaxmi Textiles");
 }

 @Test public void operationsScreensRenderClearly()throws Exception{
  rule.launchActivity(null);Thread.sleep(800);
  ui(()->rule.getActivity().getWindow().clearFlags(WindowManager.LayoutParams.FLAG_SECURE));
  JSONObject today=new JSONObject()
   .put("tallyOnline",true).put("pendingOrders",12).put("oldPendingOrders",3).put("dispatchedToday",7)
   .put("salesToday",1834500.75).put("salesCount",18).put("receivable",12345678.50).put("payable",245800.00)
   .put("followUpsDue",2).put("followUpsOverdue",1)
   .put("alerts",new JSONArray()
    .put(new JSONObject().put("severity","critical").put("title","Payment follow-up overdue").put("detail","ABC Traders • promised yesterday").put("action","followups"))
    .put(new JSONObject().put("severity","warning").put("title","Order pending for 2+ days").put("detail","ORD-20261001-003 • Mahesh Textiles").put("action","orders")));
  ui(()->{try{call("frame",new Class[]{String.class,String.class},"Today","Daily work • Vijayalaxmi Textiles");call("renderToday",new Class[]{JSONObject.class},today);}catch(Exception e){throw new RuntimeException(e);}});
  capture("26-work-today");

  JSONObject customer=new JSONObject().put("customer",new JSONObject().put("pendingBillCount",4).put("outstanding",128500.0).put("overdue",42000.0)
   .put("latestInvoices",new JSONArray().put(new JSONObject().put("number","2145").put("date","2026-10-01").put("amount",56400.0))))
   .put("pendingOrders",new JSONArray().put(new JSONObject().put("number","ORD-20261002-001").put("buyer","ABC Traders").put("dispatchedThrough","VRL")))
   .put("dispatchedOrders",new JSONArray().put(new JSONObject().put("number","ORD-20260930-004").put("buyer","ABC Traders").put("dispatchedThrough","Maruti Courier")))
   .put("followUps",new JSONArray().put(new JSONObject().put("id","f1").put("party","ABC Traders").put("amount",25000).put("promiseDate","2026-10-03").put("note","Call after 11 AM")))
   .put("lastPurchaseDate","2026-10-01")
   .put("frequentItems",new JSONArray().put(new JSONObject().put("name","Ashoka Towel").put("count",5)).put(new JSONObject().put("name","Shubhangi Napkin").put("count",3)));
  ui(()->{try{call("frame",new Class[]{String.class,String.class},"Customer 360","ABC Traders • Vijayalaxmi Textiles");call("renderCustomer",new Class[]{String.class,JSONObject.class},"ABC Traders",customer);}catch(Exception e){throw new RuntimeException(e);}});
  capture("27-customer-360");

  JSONObject health=new JSONObject().put("score",91).put("ready",false).put("items",new JSONArray()
   .put(new JSONObject().put("label","GSTIN").put("ok",true).put("value","27ABCDE1234F1Z5"))
   .put(new JSONObject().put("label","Address").put("ok",true).put("value","Market Road, Solapur"))
   .put(new JSONObject().put("label","Bank account").put("ok",true).put("value","••••7890"))
   .put(new JSONObject().put("label","UPI ID").put("ok",false).put("hint","Add UPI ID on Windows")));
  JSONObject diag=new JSONObject().put("version","2.4.0").put("company","Vijayalaxmi Textiles").put("tallyOnline",true)
   .put("cache",new JSONObject().put("entries",68).put("latestAt","2026-10-03T03:00:00Z"));
  JSONObject perf=new JSONObject().put("gateState","READY").put("totalReads",86).put("successReads",84).put("failedReads",2).put("successRate",97).put("averageMs",284).put("p95Ms",820).put("slowReads",1);
  ui(()->{try{call("frame",new Class[]{String.class,String.class},"System","Health • diagnostics • Tally performance • signing");call("renderSystem",new Class[]{JSONObject.class,JSONObject.class,JSONObject.class},health,diag,perf);}catch(Exception e){throw new RuntimeException(e);}});
  capture("28-work-system");
 }
}
