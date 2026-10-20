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
 private java.util.List<android.view.accessibility.AccessibilityNodeInfo> cancelNodes(android.app.UiAutomation automation){android.view.accessibility.AccessibilityNodeInfo root=automation.getRootInActiveWindow();if(root==null)return java.util.Collections.emptyList();java.util.List<android.view.accessibility.AccessibilityNodeInfo> nodes=root.findAccessibilityNodeInfosByText("CANCEL");if(nodes==null||nodes.isEmpty())nodes=root.findAccessibilityNodeInfosByText("Cancel");return nodes==null?java.util.Collections.emptyList():nodes;}
 private void cancelDialog()throws Exception {android.app.UiAutomation automation=InstrumentationRegistry.getInstrumentation().getUiAutomation();for(int attempt=0;attempt<5;attempt++){java.util.List<android.view.accessibility.AccessibilityNodeInfo> nodes=cancelNodes(automation);if(nodes.isEmpty()){Thread.sleep(200);if(attempt>0)return;continue;}android.view.accessibility.AccessibilityNodeInfo node=nodes.get(0),target=node;while(target!=null&&!target.isClickable())target=target.getParent();if(target!=null)target.performAction(android.view.accessibility.AccessibilityNodeInfo.ACTION_CLICK);Thread.sleep(250);InstrumentationRegistry.getInstrumentation().waitForIdleSync();if(cancelNodes(automation).isEmpty())return;android.graphics.Rect bounds=new android.graphics.Rect();(target!=null?target:node).getBoundsInScreen(bounds);if(!bounds.isEmpty()){android.os.ParcelFileDescriptor command=automation.executeShellCommand("input tap "+bounds.centerX()+" "+bounds.centerY());if(command!=null)command.close();}Thread.sleep(350);InstrumentationRegistry.getInstrumentation().waitForIdleSync();if(cancelNodes(automation).isEmpty())return;}fail("Cancel dialog remained visible after accessibility and coordinate dismissal attempts");}
 private void capture(String name)throws Exception {Thread.sleep(500);Bitmap b=InstrumentationRegistry.getInstrumentation().getUiAutomation().takeScreenshot();assertNotNull(b);File dir=new File(rule.getActivity().getFilesDir(),"ui");assertTrue(dir.exists()||dir.mkdirs());try(FileOutputStream out=new FileOutputStream(new File(dir,name+".png"))){assertTrue(b.compress(Bitmap.CompressFormat.PNG,100,out));}b.recycle();}
 private JSONObject item(String name,double qty,String unit,Double rate)throws Exception {JSONObject x=new JSONObject().put("name",name).put("quantity",qty).put("unit",unit);if(rate!=null)x.put("rate",rate);return x;}
 private JSONObject order(String buyer,String phone,String notes,JSONArray items)throws Exception {return new JSONObject().put("company","Vijayalaxmi Textiles").put("buyer",buyer).put("buyerDetails",new JSONObject().put("MOBILENUMBER",phone).put("ADDRESS","Fixture Market Road").put("CITY","Solapur").put("PARTYGSTIN","27AAAAA0000A1Z5")).put("items",items).put("notes",notes);}

 /** Fills a widget preview with real row layouts (a home-screen host fills the list through OrdersWidgetService). */
 private static void sampleRows(View widget,int rowLayout){
  ListView list=widget.findViewById(R.id.widget_orders_list);ViewGroup parent=(ViewGroup)list.getParent();list.setVisibility(View.GONE);widget.findViewById(R.id.widget_empty).setVisibility(View.GONE);
  LinearLayout rows=new LinearLayout(widget.getContext());rows.setOrientation(LinearLayout.VERTICAL);rows.setPadding(0,Math.round(3*widget.getResources().getDisplayMetrics().density),0,0);
  String[][] data={{"Raj Fabrics","19 Oct","09:32 PM","PHONE-20261019-2132-74B","48 PCS Microfiber Napkin 14x21"},{"Mahesh Textiles","18 Oct","06:15 PM","ORD-EDIT-001","12 PCS Bath Towel 70x140 • 24 PCS Microfiber Napkin 14x21"}};
  LayoutInflater in=LayoutInflater.from(widget.getContext());
  for(int i=0;i<data.length;i++){View r=in.inflate(rowLayout,rows,false);((TextView)r.findViewById(R.id.widget_row_party)).setText(data[i][0]);((TextView)r.findViewById(R.id.widget_row_date)).setText(data[i][1]);((TextView)r.findViewById(R.id.widget_row_time)).setText(data[i][2]);((TextView)r.findViewById(R.id.widget_row_number)).setText(data[i][3]);((TextView)r.findViewById(R.id.widget_row_items)).setText(data[i][4]);LinearLayout.LayoutParams lp=new LinearLayout.LayoutParams(-1,-2);if(i>0)lp.topMargin=Math.round(7*widget.getResources().getDisplayMetrics().density);rows.addView(r,lp);}
  parent.addView(rows,new FrameLayout.LayoutParams(-1,-2));
 }
 private void openForm()throws Exception{Method m=OrdersActivity.class.getDeclaredMethod("openForm",JSONObject.class);m.setAccessible(true);m.invoke(rule.getActivity(),(Object)null);}

 @Test public void ordersRemainUsableOfflineAndDetailsPreviewRenders()throws Exception {
  Context c=InstrumentationRegistry.getInstrumentation().getTargetContext();
  c.getSharedPreferences("identity",Context.MODE_PRIVATE).edit().clear().commit();
  c.getSharedPreferences("appearance",Context.MODE_PRIVATE).edit().putString("mode","Light").commit();
  CryptoStore secure=new CryptoStore(c);secure.put("base","http://127.0.0.1:9");secure.put("device","offline-fixture");secure.put("company","Vijayalaxmi Textiles");
  OfflineOrdersStore local=new OfflineOrdersStore(secure);
  local.cacheBootstrap(new JSONObject().put("companies",new JSONArray().put(new JSONObject().put("name","Vijayalaxmi Textiles"))));
  local.cacheMasters("Vijayalaxmi Textiles",new JSONObject().put("parties",new JSONArray().put(new JSONObject().put("name","Mahesh Textiles")).put(new JSONObject().put("name","Raj Fabrics"))).put("stock",new JSONObject().put("items",new JSONArray().put(new JSONObject().put("name","Bath Towel 70x140").put("unit","PCS")).put(new JSONObject().put("name","Microfiber Napkin 14x21").put("unit","PCS")))));
  JSONObject first=local.saveLocal(order("Mahesh Textiles","9000000001","Pink + Chocolate. Dispatch tomorrow.",new JSONArray().put(item("Bath Towel 70x140",12,"PCS",185.0)).put(item("Microfiber Napkin 14x21",24,"PCS",null))),null);
  JSONObject staleFirst=new JSONObject(first.toString());JSONObject remoteFirst=new JSONObject(first.toString()).put("id","server-fixture-1").put("number","ORD-EDIT-001").put("_localOnly",false);remoteFirst.optJSONObject("buyerDetails").put("__PHONE_ORDER_ID",staleFirst.optString("id"));local.replaceWithRemote(new JSONObject().put("pending",new JSONArray().put(remoteFirst)).put("dispatched",new JSONArray()));JSONObject edited=local.saveLocal(order("Mahesh Textiles","9000000001","Edited once, not duplicated.",new JSONArray().put(item("Bath Towel 70x140",12,"PCS",185.0))),staleFirst);assertEquals("server-fixture-1",edited.optString("id"));assertEquals(1,local.list().optInt("pendingCount"));assertTrue(edited.optString("shareText").contains("Rate: ₹185.00"));assertFalse(edited.optString("shareText").contains("Amount:"));assertFalse(edited.optString("shareText").contains("Order Total:"));first=edited;
  local.saveLocal(order("Raj Fabrics","9000000002","Assorted colours.",new JSONArray().put(item("Microfiber Napkin 14x21",48,"PCS",null))),null);
  JSONObject old=local.saveLocal(order("Shree Stores","9000000003","Already sent.",new JSONArray().put(item("Bath Towel 70x140",6,"PCS",null))),null);
  local.dispatchLocal(old,"VRL Logistics");
  assertEquals(2,local.list().optInt("pendingCount"));assertEquals(1,local.list().optInt("dispatchedCount"));assertEquals("VRL Logistics",local.list().optJSONArray("dispatched").optJSONObject(0).optString("dispatchedThrough"));

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

  cancelDialog();
  ui(()->{try{openForm();}catch(Exception e){throw new RuntimeException(e);}});
  capture("42-order-form");
  ui(()->{
   OrdersActivity a=rule.getActivity();FrameLayout stage=new FrameLayout(a);stage.setBackgroundColor(Color.rgb(232,237,242));
   View widget=a.getLayoutInflater().inflate(R.layout.orders_widget,stage,false);
   ((TextView)widget.findViewById(R.id.widget_pending)).setText("2");((TextView)widget.findViewById(R.id.widget_mode)).setText("Saved on phone • 10:42 AM");
   sampleRows(widget,R.layout.orders_widget_row);
   int w=Math.round(360*a.getResources().getDisplayMetrics().density),h=Math.round(300*a.getResources().getDisplayMetrics().density);stage.addView(widget,new FrameLayout.LayoutParams(w,h,Gravity.CENTER));a.setContentView(stage);
  });
  capture("21-orders-widget");
ui(()->{OrdersActivity a=rule.getActivity();FrameLayout stage=new FrameLayout(a);stage.setBackgroundColor(Color.rgb(232,237,242));View widget=a.getLayoutInflater().inflate(R.layout.orders_widget_compact,stage,false);((TextView)widget.findViewById(R.id.widget_pending)).setText("2");((TextView)widget.findViewById(R.id.widget_party1)).setText("Mahesh Textiles • 12 PCS Bath Towel 70x140");int w=Math.round(360*a.getResources().getDisplayMetrics().density),h=Math.round(150*a.getResources().getDisplayMetrics().density);stage.addView(widget,new FrameLayout.LayoutParams(w,h,Gravity.CENTER));a.setContentView(stage);});
capture("31-orders-widget-compact");
  ui(()->{OrdersActivity a=rule.getActivity();FrameLayout stage=new FrameLayout(a);stage.setBackgroundColor(Color.rgb(232,237,242));View widget=a.getLayoutInflater().inflate(R.layout.quick_pi_widget,stage,false);int size=Math.round(104*a.getResources().getDisplayMetrics().density);FrameLayout.LayoutParams lp=new FrameLayout.LayoutParams(size,size,Gravity.CENTER);stage.addView(widget,lp);a.setContentView(stage);});
  capture("29-widget-proforma-1x1");
  ui(()->{OrdersActivity a=rule.getActivity();FrameLayout stage=new FrameLayout(a);stage.setBackgroundColor(Color.rgb(232,237,242));View widget=a.getLayoutInflater().inflate(R.layout.quick_order_widget,stage,false);int size=Math.round(104*a.getResources().getDisplayMetrics().density);FrameLayout.LayoutParams lp=new FrameLayout.LayoutParams(size,size,Gravity.CENTER);stage.addView(widget,lp);a.setContentView(stage);});
  capture("30-widget-order-1x1");
  ui(()->{OrdersActivity a=rule.getActivity();FrameLayout stage=new FrameLayout(a);stage.setBackgroundColor(Color.rgb(232,237,242));View widget=a.getLayoutInflater().inflate(R.layout.gsm_widget,stage,false);int size=Math.round(104*a.getResources().getDisplayMetrics().density);stage.addView(widget,new FrameLayout.LayoutParams(size,size,Gravity.CENTER));a.setContentView(stage);});capture("33-widget-gsm-1x1");
  ui(()->{OrdersActivity a=rule.getActivity();FrameLayout stage=new FrameLayout(a);stage.setBackgroundColor(Color.rgb(232,237,242));View widget=a.getLayoutInflater().inflate(R.layout.search_widget,stage,false);int size=Math.round(104*a.getResources().getDisplayMetrics().density);stage.addView(widget,new FrameLayout.LayoutParams(size,size,Gravity.CENTER));a.setContentView(stage);});capture("34-widget-search-1x1");
  ui(()->{OrdersActivity a=rule.getActivity();FrameLayout stage=new FrameLayout(a);stage.setBackgroundColor(Color.rgb(232,237,242));View widget=a.getLayoutInflater().inflate(R.layout.voice_order_widget,stage,false);int size=Math.round(104*a.getResources().getDisplayMetrics().density);stage.addView(widget,new FrameLayout.LayoutParams(size,size,Gravity.CENTER));a.setContentView(stage);});capture("35-widget-voice-order-1x1");
  ui(()->{OrdersActivity a=rule.getActivity();FrameLayout stage=new FrameLayout(a);stage.setBackgroundColor(Color.rgb(232,237,242));View widget=a.getLayoutInflater().inflate(R.layout.quick_dock_widget,stage,false);int w=Math.round(390*a.getResources().getDisplayMetrics().density),h=Math.round(92*a.getResources().getDisplayMetrics().density);stage.addView(widget,new FrameLayout.LayoutParams(w,h,Gravity.CENTER));a.setContentView(stage);});capture("36-widget-quick-dock-5x1");
  ui(()->{OrdersActivity a=rule.getActivity();FrameLayout stage=new FrameLayout(a);stage.setBackgroundColor(Color.rgb(232,237,242));View widget=a.getLayoutInflater().inflate(R.layout.quick_dock_widget_small,stage,false);int w=Math.round(270*a.getResources().getDisplayMetrics().density),h=Math.round(80*a.getResources().getDisplayMetrics().density);stage.addView(widget,new FrameLayout.LayoutParams(w,h,Gravity.CENTER));a.setContentView(stage);});capture("37-widget-quick-dock-small");

  ui(()->rule.getActivity().finish());
  c.getSharedPreferences("appearance",Context.MODE_PRIVATE).edit().putString("mode","Dark").commit();
  rule.launchActivity(new Intent(c,OrdersActivity.class));
  ui(()->rule.getActivity().getWindow().clearFlags(WindowManager.LayoutParams.FLAG_SECURE));
  capture("23-orders-dark");
  ui(()->{try{java.lang.reflect.Field f=OrdersActivity.class.getDeclaredField("dispatchedTab");f.setAccessible(true);f.setBoolean(rule.getActivity(),true);Method m=OrdersActivity.class.getDeclaredMethod("paintOrders");m.setAccessible(true);m.invoke(rule.getActivity());}catch(Exception e){throw new RuntimeException(e);}});
  capture("24-orders-dispatched-dark");
  ui(()->{try{openForm();}catch(Exception e){throw new RuntimeException(e);}});
  capture("43-order-form-dark");
  ui(()->{OrdersActivity a=rule.getActivity();FrameLayout stage=new FrameLayout(a);stage.setBackgroundColor(Color.rgb(15,22,29));View widget=a.getLayoutInflater().inflate(R.layout.orders_widget_dark,stage,false);((TextView)widget.findViewById(R.id.widget_pending)).setText("2");((TextView)widget.findViewById(R.id.widget_mode)).setText("Saved on phone • 10:42 AM");sampleRows(widget,R.layout.orders_widget_row_dark);int w=Math.round(360*a.getResources().getDisplayMetrics().density),h=Math.round(300*a.getResources().getDisplayMetrics().density);stage.addView(widget,new FrameLayout.LayoutParams(w,h,Gravity.CENTER));a.setContentView(stage);});
  capture("25-orders-widget-dark");
ui(()->{OrdersActivity a=rule.getActivity();FrameLayout stage=new FrameLayout(a);stage.setBackgroundColor(Color.rgb(15,22,29));View widget=a.getLayoutInflater().inflate(R.layout.orders_widget_compact_dark,stage,false);((TextView)widget.findViewById(R.id.widget_pending)).setText("2");((TextView)widget.findViewById(R.id.widget_party1)).setText("Mahesh Textiles • 12 PCS Bath Towel 70x140");int w=Math.round(360*a.getResources().getDisplayMetrics().density),h=Math.round(150*a.getResources().getDisplayMetrics().density);stage.addView(widget,new FrameLayout.LayoutParams(w,h,Gravity.CENTER));a.setContentView(stage);});
capture("32-orders-widget-compact-dark");
  c.getSharedPreferences("appearance",Context.MODE_PRIVATE).edit().putString("mode","System").commit();
 }
}
