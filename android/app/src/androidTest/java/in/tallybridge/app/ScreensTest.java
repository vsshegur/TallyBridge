package in.tallybridge.app;

import android.app.Activity;
import android.content.Context;
import android.content.Intent;
import android.graphics.Bitmap;
import android.view.View;
import android.view.ViewGroup;
import android.view.WindowManager;
import android.widget.TextView;
import androidx.test.ext.junit.runners.AndroidJUnit4;
import androidx.test.platform.app.InstrumentationRegistry;
import androidx.test.rule.ActivityTestRule;
import org.json.JSONArray;
import org.json.JSONObject;
import org.junit.*;
import org.junit.runner.RunWith;
import java.io.File;
import java.io.FileOutputStream;
import java.lang.reflect.Field;
import java.lang.reflect.Method;
import static org.junit.Assert.*;

/**
 * Light and dark screenshots of the screens opened from Home, More and widgets: Search, Customer and GSM calculator.
 * Fixture data only; the PC address is unreachable on purpose, so each screen also proves it copes with the PC being off.
 */
@RunWith(AndroidJUnit4.class)
public class ScreensTest {
 @Rule public ActivityTestRule<UniversalSearchActivity> search=new ActivityTestRule<>(UniversalSearchActivity.class,false,false);
 @Rule public ActivityTestRule<Customer360Activity> customer=new ActivityTestRule<>(Customer360Activity.class,false,false);
 @Rule public ActivityTestRule<GsmCalculatorActivity> gsm=new ActivityTestRule<>(GsmCalculatorActivity.class,false,false);

 private Context ctx(){return InstrumentationRegistry.getInstrumentation().getTargetContext();}
 private void ui(Runnable r){InstrumentationRegistry.getInstrumentation().runOnMainSync(r);InstrumentationRegistry.getInstrumentation().waitForIdleSync();}
 private void mode(String m){ctx().getSharedPreferences("appearance",Context.MODE_PRIVATE).edit().putString("mode",m).commit();}
 private void unsecure(Activity a){ui(()->a.getWindow().clearFlags(WindowManager.LayoutParams.FLAG_SECURE));}
 private void capture(Activity a,String name)throws Exception{
  Thread.sleep(450);Bitmap b=InstrumentationRegistry.getInstrumentation().getUiAutomation().takeScreenshot();assertNotNull(b);
  File dir=new File(a.getFilesDir(),"ui");assertTrue(dir.exists()||dir.mkdirs());
  try(FileOutputStream out=new FileOutputStream(new File(dir,name+".png"))){assertTrue(b.compress(Bitmap.CompressFormat.PNG,100,out));}b.recycle();
 }
 private static Object invoke(Object target,String name,Class<?>[] types,Object...args){
  try{Method m=target.getClass().getDeclaredMethod(name,types);m.setAccessible(true);return m.invoke(target,args);}catch(Exception e){throw new RuntimeException(e);}
 }
 private static void set(Object target,String name,Object value){
  try{Field f=target.getClass().getDeclaredField(name);f.setAccessible(true);f.set(target,value);}catch(Exception e){throw new RuntimeException(e);}
 }
 private static boolean hasText(View v,String text){
  if(v instanceof TextView&&((TextView)v).getText().toString().contains(text))return true;
  if(v instanceof ViewGroup){ViewGroup g=(ViewGroup)v;for(int i=0;i<g.getChildCount();i++)if(hasText(g.getChildAt(i),text))return true;}
  return false;
 }

 @Before public void seed()throws Exception{
  InstrumentationRegistry.getInstrumentation().setInTouchMode(true); // as after a tap: no keyboard-focus ring on the back arrow
  CryptoStore s=new CryptoStore(ctx());s.put("base","https://127.0.0.1:9");s.put("device","fixture-device");s.put("company","Vijayalaxmi Textiles");
  s.put("billCompanies","[{\"id\":\"c1\",\"name\":\"Vijayalaxmi Textiles\"}]");
 }
 @After public void reset(){mode("System");}

 private JSONArray searchResults()throws Exception{
  return new JSONArray()
   .put(new JSONObject().put("kind","ledger").put("title","ABC Textiles & Trading Company").put("subtitle","Bengaluru • 27ABCDE1234F1Z5").put("party","ABC Textiles & Trading Company").put("amount",24500))
   .put(new JSONObject().put("kind","ledger").put("title","Raj Fabrics").put("subtitle","Solapur • 9000000002").put("party","Raj Fabrics"))
   .put(new JSONObject().put("kind","sales").put("title","Sales INV-2026-014").put("subtitle","ABC Textiles & Trading Company • 20 Sep 2026").put("number","INV-2026-014").put("party","ABC Textiles & Trading Company").put("amount",18500))
   .put(new JSONObject().put("kind","order").put("title","ORD-20261019-001").put("subtitle","Mahesh Textiles • 12 PCS Bath Towel 70x140").put("number","ORD-20261019-001"));
 }
 private void showSearch(String name,String query)throws Exception{
  UniversalSearchActivity a=search.launchActivity(new Intent(ctx(),UniversalSearchActivity.class));unsecure(a);
  Thread.sleep(1500);
  if(name.endsWith("pc-off")){ui(()->assertTrue("search explains the PC is needed",hasText(a.getWindow().getDecorView(),"Search needs the PC")));capture(a,name);ui(a::finish);Thread.sleep(300);return;}
  JSONArray results=searchResults();
  ui(()->{set(a,"all",results);try{Field q=UniversalSearchActivity.class.getDeclaredField("query");q.setAccessible(true);((TextView)q.get(a)).setText(query);}catch(Exception e){throw new RuntimeException(e);}invoke(a,"paint",new Class[]{});});
  ui(()->assertTrue(hasText(a.getWindow().getDecorView(),"ABC Textiles & Trading Company")));
  capture(a,name);
  ui(a::finish);Thread.sleep(300);
 }
 @Test public void searchLightDarkAndPcOff()throws Exception{
  mode("Light");showSearch("49-search-pc-off","");
  showSearch("50-search","abc");
  mode("Dark");showSearch("51-search-dark","");
 }

 private JSONObject customerFixture()throws Exception{
  JSONObject c=new JSONObject().put("outstanding",24500).put("overdue",12000).put("pendingBillCount",2)
   .put("lastPayment",new JSONObject().put("amount",10000).put("date","2026-09-12"))
   .put("latestInvoices",new JSONArray().put(new JSONObject().put("number","INV-2026-014").put("date","2026-09-20").put("voucherType","Sales").put("amount",18500)));
  return new JSONObject().put("customer",c).put("lastPurchaseDate","2026-09-20")
   .put("frequentItems",new JSONArray().put(new JSONObject().put("name","Bath Towel 70x140").put("count",6)))
   .put("pendingOrders",new JSONArray().put(new JSONObject().put("number","ORD-20261019-001").put("items",new JSONArray().put(new JSONObject().put("name","Bath Towel 70x140").put("quantity",12).put("unit","PCS")))));
 }
 private void showCustomer(String name)throws Exception{
  Customer360Activity a=customer.launchActivity(new Intent(ctx(),Customer360Activity.class).putExtra(Customer360Activity.EXTRA_PARTY,"ABC Textiles & Trading Company"));unsecure(a);
  Thread.sleep(1500);JSONObject x=customerFixture();
  ui(()->invoke(a,"paint",new Class[]{JSONObject.class},x));
  ui(()->assertTrue(hasText(a.getWindow().getDecorView(),"Pending orders")));
  capture(a,name);ui(a::finish);Thread.sleep(300);
 }
 @Test public void customerLightAndDark()throws Exception{
  mode("Light");showCustomer("52-customer");
  mode("Dark");showCustomer("53-customer-dark");
 }

 private void showGsm(String name)throws Exception{
  GsmCalculatorActivity a=gsm.launchActivity(new Intent(ctx(),GsmCalculatorActivity.class));unsecure(a);
  ui(()->{for(String[] f:new String[][]{{"item","Bath Towel"},{"length","30"},{"breadth","60"},{"weight","500"}}){try{Field fl=GsmCalculatorActivity.class.getDeclaredField(f[0]);fl.setAccessible(true);((TextView)fl.get(a)).setText(f[1]);}catch(Exception e){throw new RuntimeException(e);}}
   invoke(a,"calculate",new Class[]{boolean.class},true);});
  capture(a,name);ui(a::finish);Thread.sleep(300);
 }
 @Test public void gsmLightAndDark()throws Exception{
  mode("Light");showGsm("54-gsm");
  mode("Dark");showGsm("55-gsm-dark");
 }

}
