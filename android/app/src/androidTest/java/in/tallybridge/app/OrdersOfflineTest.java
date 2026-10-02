package in.tallybridge.app;

import android.content.*;
import android.graphics.Bitmap;
import android.graphics.Color;
import android.view.*;
import android.widget.*;
import androidx.test.ext.junit.runners.AndroidJUnit4;
import androidx.test.platform.app.InstrumentationRegistry;
import androidx.test.rule.ActivityTestRule;
import org.junit.*;
import org.junit.runner.RunWith;
import org.json.*;
import java.io.*;
import java.lang.reflect.Method;
import static org.junit.Assert.*;

/** Proves Orders renders and shares from phone storage with no reachable PC. */
@RunWith(AndroidJUnit4.class)
public class OrdersOfflineTest {
 @Rule public ActivityTestRule<OrdersActivity> rule=new ActivityTestRule<>(OrdersActivity.class,false,false);
 private void ui(Runnable r){InstrumentationRegistry.getInstrumentation().runOnMainSync(r);InstrumentationRegistry.getInstrumentation().waitForIdleSync();}
 private void capture(String name)throws Exception {Thread.sleep(500);Bitmap b=InstrumentationRegistry.getInstrumentation().getUiAutomation().takeScreenshot();assertNotNull(b);File dir=new File(rule.getActivity().getFilesDir(),"ui");assertTrue(dir.exists()||dir.mkdirs());try(FileOutputStream out=new FileOutputStream(new File(dir,name+".png"))){assertTrue(b.compress(Bitmap.CompressFormat.PNG,100,out));}b.recycle();}
 private JSONObject item(String name,double qty,String unit,Double rate)throws Exception {JSONObject x=new JSONObject().put("name",name).put("quantity",qty).put("unit",unit);if(rate!=null)x.put("rate",rate);return x;}
 private JSONObject order(String buyer,String phone,String notes,JSONArray items)throws Exception {return new JSONObject().put("company","Vijayalaxmi Textiles").put("buyer",buyer).put("buyerDetails",new JSONObject().put("MOBILENUMBER",phone).put("ADDRESS","Market Road").put("CITY","Solapur").put("PARTYGSTIN","27ABCDE1234F1Z5")).put("items",items).put("notes",notes);}

 @Test public void ordersRemainUsableOfflineAndDetailsPreviewRenders()throws Exception {
  Context c=InstrumentationRegistry.getInstrumentation().getTargetContext();
  c.getSharedPreferences("identity",Context.MODE_PRIVATE).edit().clear().commit();
  c.getSharedPreferences("appearance",Context.MODE_PRIVATE).edit().putString("mode","Light").commit();
  CryptoStore secure=new CryptoStore(c);secure.put("base","http://127.0.0.1:9");secure.put("device","offline-fixture");secure.put("company","Vijayalaxmi Textiles");
  OfflineOrdersStore local=new OfflineOrdersStore(secure);
  local.cacheBootstrap(new JSONObject().put("companies",new JSONArray().put(new JSONObject().put("name","Vijayalaxmi Textiles"))));
  local.cacheMasters("Vijayalaxmi Textiles",new JSONObject().put("parties",new JSONArray().put(new JSONObject().put("name","Mahesh Textiles")).put(new JSONObject().put("name","Raj Fabrics"))).put("stock",new JSONObject().put("items",new JSONArray().put(new JSONObject().put("name","Bath Towel 70x140").put("unit","PCS")).put(new JSONObject().put("name","Microfiber Napkin 14x21").put("unit","PCS")))));
  JSONObject first=local.saveLocal(order("Mahesh Textiles","9876543210","Pink + Chocolate. Dispatch tomorrow.",new JSONArray().put(item("Bath Towel 70x140",12,"PCS",185.0)).put(item("Microfiber Napkin 14x21",24,"PCS",null))),null);
  local.saveLocal(order("Raj Fabrics","9988776655","Assorted colours.",new JSONArray().put(item("Microfiber Napkin 14x21",48,"PCS",null))),null);
  JSONObject old=local.saveLocal(order("Shree Stores","9765432100","Already sent.",new JSONArray().put(item("Bath Towel 70x140",6,"PCS",null))),null);
  local.dispatchLocal(old);
  assertEquals(2,local.list().optInt("pendingCount"));assertEquals(1,local.list().optInt("dispatchedCount"));

  rule.launchActivity(new Intent(c,OrdersActivity.class));
  ui(()->rule.getActivity().getWindow().clearFlags(WindowManager.LayoutParams.FLAG_SECURE));
  capture("19-orders-offline");

  JSONObject shareOrder=local.list().optJSONArray("pending").optJSONObject(1);
  if(shareOrder==null)shareOrder=first;
  JSONObject finalOrder=shareOrder;
  ui(()->{try{Method m=OrdersActivity.class.getDeclaredMethod("shareOrder",JSONObject.class);m.setAccessible(true);m.invoke(rule.getActivity(),finalOrder);}catch(Exception e){throw new RuntimeException(e);}});
  capture("20-orders-details-only");
  assertTrue(finalOrder.optString("shareText").contains("ORDER DETAILS"));
  assertTrue(finalOrder.optString("shareText").contains("Vijayalaxmi Textiles"));

  InstrumentationRegistry.getInstrumentation().sendKeyDownUpSync(android.view.KeyEvent.KEYCODE_BACK);
  InstrumentationRegistry.getInstrumentation().waitForIdleSync();
  ui(()->{
   OrdersActivity a=rule.getActivity();
   FrameLayout stage=new FrameLayout(a);stage.setBackgroundColor(Color.rgb(232,237,242));
   View widget=a.getLayoutInflater().inflate(R.layout.orders_widget,stage,false);
   ((TextView)widget.findViewById(R.id.widget_pending)).setText("2 pending orders");
   ((TextView)widget.findViewById(R.id.widget_mode)).setText("OFFLINE • PHONE COPY");
   ((TextView)widget.findViewById(R.id.widget_names)).setText("Raj Fabrics • Mahesh Textiles");
   int w=Math.round(360*a.getResources().getDisplayMetrics().density),h=Math.round(190*a.getResources().getDisplayMetrics().density);
   FrameLayout.LayoutParams lp=new FrameLayout.LayoutParams(w,h,Gravity.CENTER);stage.addView(widget,lp);a.setContentView(stage);
  });
  capture("21-orders-widget");

  ui(()->rule.getActivity().finish());
  c.getSharedPreferences("appearance",Context.MODE_PRIVATE).edit().putString("mode","Dark").commit();
  rule.launchActivity(new Intent(c,OrdersActivity.class));
  ui(()->rule.getActivity().getWindow().clearFlags(WindowManager.LayoutParams.FLAG_SECURE));
  capture("23-orders-dark");
  ui(()->{try{java.lang.reflect.Field f=OrdersActivity.class.getDeclaredField("dispatchedTab");f.setAccessible(true);f.setBoolean(rule.getActivity(),true);Method m=OrdersActivity.class.getDeclaredMethod("paintOrders");m.setAccessible(true);m.invoke(rule.getActivity());}catch(Exception e){throw new RuntimeException(e);}});
  capture("24-orders-dispatched-dark");
  ui(()->{OrdersActivity a=rule.getActivity();assertEquals(R.layout.orders_widget_dark,OrdersWidget.previewLayout(a));FrameLayout stage=new FrameLayout(a);stage.setBackgroundColor(Color.rgb(15,22,29));View widget=a.getLayoutInflater().inflate(R.layout.orders_widget_dark,stage,false);((TextView)widget.findViewById(R.id.widget_pending)).setText("2 pending orders");((TextView)widget.findViewById(R.id.widget_mode)).setText("OFFLINE • PHONE COPY");((TextView)widget.findViewById(R.id.widget_names)).setText("Raj Fabrics • Mahesh Textiles");int w=Math.round(360*a.getResources().getDisplayMetrics().density),h=Math.round(205*a.getResources().getDisplayMetrics().density);FrameLayout.LayoutParams lp=new FrameLayout.LayoutParams(w,h,Gravity.CENTER);stage.addView(widget,lp);a.setContentView(stage);});
  capture("25-orders-widget-dark");
  c.getSharedPreferences("appearance",Context.MODE_PRIVATE).edit().putString("mode","System").commit();
 }
}
