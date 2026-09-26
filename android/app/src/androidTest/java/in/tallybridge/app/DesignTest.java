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
 private void ui(Runnable r){InstrumentationRegistry.getInstrumentation().runOnMainSync(r);InstrumentationRegistry.getInstrumentation().waitForIdleSync();}
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
   field("boot",new JSONObject().put("tallyOnline",true).put("summaries",new JSONObject().put("Vijayalaxmi Textiles • Solapur",summary)));
   call("renderHome");
  }catch(Exception e){throw new RuntimeException(e);}});
  capture("02-overview");
  ui(()->{try{call("reports");}catch(Exception e){throw new RuntimeException(e);}});
  capture("03-reports");
  ui(()->{try{call("connectScreen");}catch(Exception e){throw new RuntimeException(e);}});
  capture("04-connect");
 }
}
