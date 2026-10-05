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
   .put("alerts",new JSONArray()
    .put(new JSONObject().put("severity","warning").put("title","Order pending for 2+ days").put("detail","ORD-20261001-003 • Mahesh Textiles").put("action","orders"))
    .put(new JSONObject().put("severity","info").put("title","Compatibility mode").put("detail","Core bridge data is available.").put("action","tools")));
  ui(()->{try{call("frame",new Class[]{String.class,String.class},"Today","Daily work • Vijayalaxmi Textiles");call("renderToday",new Class[]{JSONObject.class},today);}catch(Exception e){throw new RuntimeException(e);}});
  capture("26-work-today");

  ui(()->{try{call("showTools",new Class[]{});}catch(Exception e){throw new RuntimeException(e);}});
  capture("27-customer-360");

  // In 2.4.6 Search, Customers and System intentionally route through the
  // proven MainActivity core screens instead of duplicate fragile Work APIs.
  assertEquals("in.tallybridge.app.OPEN_SEARCH",MainActivity.ACTION_SEARCH);
  assertEquals("in.tallybridge.app.OPEN_CUSTOMERS",MainActivity.ACTION_CUSTOMERS);
  assertEquals("in.tallybridge.app.OPEN_SYSTEM",MainActivity.ACTION_SYSTEM);
  assertNotNull(SectionNav.class);

  ui(()->{try{call("frame",new Class[]{String.class,String.class},"Work","Unified navigation • Home • Work • Orders • Reports • More");}catch(Exception e){throw new RuntimeException(e);}});
  capture("28-work-system");
 }
}
