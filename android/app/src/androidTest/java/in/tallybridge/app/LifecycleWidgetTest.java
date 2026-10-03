package in.tallybridge.app;

import android.app.Activity;
import android.content.*;
import android.graphics.*;
import android.view.*;
import android.widget.*;
import androidx.test.ext.junit.runners.AndroidJUnit4;
import androidx.test.platform.app.InstrumentationRegistry;
import androidx.test.rule.ActivityTestRule;
import org.junit.*;
import org.junit.runner.RunWith;
import java.io.*;
import java.lang.reflect.*;
import java.util.Map;
import static org.junit.Assert.*;

@RunWith(AndroidJUnit4.class)
public class LifecycleWidgetTest {
 @Rule public ActivityTestRule<MainActivity> rule=new ActivityTestRule<>(MainActivity.class);
 private void ui(Runnable r){InstrumentationRegistry.getInstrumentation().runOnMainSync(r);InstrumentationRegistry.getInstrumentation().waitForIdleSync();}
 private void capture(Activity a,String name)throws Exception{Thread.sleep(300);Bitmap b=InstrumentationRegistry.getInstrumentation().getUiAutomation().takeScreenshot();assertNotNull(b);File dir=new File(a.getFilesDir(),"ui");assertTrue(dir.exists()||dir.mkdirs());try(FileOutputStream out=new FileOutputStream(new File(dir,name+".png"))){assertTrue(b.compress(Bitmap.CompressFormat.PNG,100,out));}b.recycle();}
 private Field field(Class<?> c,String n)throws Exception{Field f=c.getDeclaredField(n);f.setAccessible(true);return f;}

 @Test public void appPreservesTaskAndWidgetAdaptsToSize()throws Exception{
  MainActivity a=rule.getActivity();
  ui(()->a.getWindow().clearFlags(WindowManager.LayoutParams.FLAG_SECURE));
  Field unlocked=field(MainActivity.class,"unlocked"),tab=field(MainActivity.class,"tab"),cacheF=field(MainActivity.class,"cache");
  ui(()->{try{unlocked.setBoolean(a,true);tab.set(a,"Reports");((Map)cacheF.get(a)).put("sentinel",new org.json.JSONObject().put("ok",true));Method stop=MainActivity.class.getDeclaredMethod("onStop");stop.setAccessible(true);stop.invoke(a);}catch(Exception e){throw new RuntimeException(e);}});
  assertTrue(unlocked.getBoolean(a));assertEquals("Reports",tab.get(a));assertTrue(((Map)cacheF.get(a)).containsKey("sentinel"));

  Context c=InstrumentationRegistry.getInstrumentation().getTargetContext();
  c.getSharedPreferences("orders_widget",Context.MODE_PRIVATE).edit().putInt("pending",3).putString("names","ABC Textiles\nMahesh Textiles\nRaj Fabrics").putBoolean("offline",false).commit();

  ui(()->{FrameLayout stage=new FrameLayout(a);stage.setBackgroundColor(Color.rgb(232,237,242));RemoteViews rv=OrdersWidget.previewForSize(a,210,100);View w=rv.apply(a,stage);int width=Math.round(210*a.getResources().getDisplayMetrics().density),height=Math.round(100*a.getResources().getDisplayMetrics().density);stage.addView(w,new FrameLayout.LayoutParams(width,height,Gravity.CENTER));a.setContentView(stage);assertEquals(View.VISIBLE,w.findViewById(R.id.widget_add).getVisibility());assertEquals(View.VISIBLE,w.findViewById(R.id.widget_open).getVisibility());});
  capture(a,"29-orders-widget-compact");

  ui(()->{FrameLayout stage=new FrameLayout(a);stage.setBackgroundColor(Color.rgb(232,237,242));RemoteViews rv=OrdersWidget.previewForSize(a,360,260);View w=rv.apply(a,stage);int width=Math.round(360*a.getResources().getDisplayMetrics().density),height=Math.round(260*a.getResources().getDisplayMetrics().density);stage.addView(w,new FrameLayout.LayoutParams(width,height,Gravity.CENTER));a.setContentView(stage);TextView names=w.findViewById(R.id.widget_names);assertEquals(View.VISIBLE,names.getVisibility());assertTrue(names.getText().toString().contains("Mahesh Textiles"));});
  capture(a,"30-orders-widget-tall");

  CryptoStore s=new CryptoStore(c);s.put("base","https://127.0.0.1:9");s.put("device","fixture-device");s.put("company","Vijayalaxmi Textiles");
  Intent wi=new Intent(c,WorkActivity.class).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK);
  Activity launched=InstrumentationRegistry.getInstrumentation().startActivitySync(wi);assertTrue(launched instanceof WorkActivity);WorkActivity work=(WorkActivity)launched;
  ui(()->{try{Method stop=WorkActivity.class.getDeclaredMethod("onStop");stop.setAccessible(true);stop.invoke(work);}catch(Exception e){throw new RuntimeException(e);}});
  assertFalse(work.isFinishing());ui(work::finish);
 }
}
