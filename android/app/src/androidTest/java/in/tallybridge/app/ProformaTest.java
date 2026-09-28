package in.tallybridge.app;
import android.graphics.Bitmap;
import android.view.WindowManager;
import android.widget.*;
import androidx.test.ext.junit.runners.AndroidJUnit4;
import androidx.test.platform.app.InstrumentationRegistry;
import androidx.test.rule.ActivityTestRule;
import org.junit.*;
import org.junit.runner.RunWith;
import org.json.*;
import java.io.*;
import java.lang.reflect.*;
import static org.junit.Assert.*;
@RunWith(AndroidJUnit4.class)
public class ProformaTest {
 @Rule public ActivityTestRule<MainActivity> rule=new ActivityTestRule<>(MainActivity.class);
 ProformaComposer composer;
 void ui(Runnable r){InstrumentationRegistry.getInstrumentation().runOnMainSync(r);InstrumentationRegistry.getInstrumentation().waitForIdleSync();}
 void capture(String name)throws Exception{Thread.sleep(300);Bitmap b=InstrumentationRegistry.getInstrumentation().getUiAutomation().takeScreenshot();assertNotNull(b);File dir=new File(rule.getActivity().getFilesDir(),"ui");assertTrue(dir.exists()||dir.mkdirs());try(FileOutputStream out=new FileOutputStream(new File(dir,name+".png"))){assertTrue(b.compress(Bitmap.CompressFormat.PNG,100,out));}b.recycle();}
 @Test public void proformaUsesDiscountBeforeTaxAndRenders()throws Exception{
 ui(()->{try{MainActivity a=rule.getActivity();a.getWindow().clearFlags(WindowManager.LayoutParams.FLAG_SECURE);Field c=MainActivity.class.getDeclaredField("company");c.setAccessible(true);c.set(a,"Vijayalaxmi Textiles");Field tab=MainActivity.class.getDeclaredField("tab");tab.setAccessible(true);tab.set(a,"Proforma");Method base=MainActivity.class.getDeclaredMethod("base",String.class,boolean.class);base.setAccessible(true);base.invoke(a,"Proforma invoice",true);Field f=MainActivity.class.getDeclaredField("body");f.setAccessible(true);LinearLayout body=(LinearLayout)f.get(a);composer=new ProformaComposer(a,null,null,"Vijayalaxmi Textiles",false,(x,p)->{});composer.masters=new JSONObject().put("stock",new JSONObject().put("items",new JSONArray()));ProformaComposer.put(composer.draft,"buyer","ABC Textiles and Trading Company");ProformaComposer.put(composer.draft,"number","PI-TEST-1");ProformaComposer.put(composer.draft,"date","2026-09-27");composer.lines.put(new JSONObject("{\"name\":\"Cotton towel with a long product name\",\"description\":\"Blue border • Soft cotton\",\"quantity\":\"10\",\"unit\":\"pcs\",\"rate\":\"100\",\"discount\":\"10\",\"gst\":\"5\"}"));composer.render(body);assertTrue(composer.summary.getText().toString().contains("945.00"));assertTrue(composer.summary.getText().toString().contains("CGST + SGST"));}catch(Exception e){throw new RuntimeException(e);}});
 capture("12-proforma");
 ui(()->{composer.setState("placeState","29");composer.refreshTotal();assertTrue(composer.summary.getText().toString().contains("IGST"));assertTrue(composer.summary.getText().toString().contains("945.00"));((ScrollView)composer.panel.getParent()).fullScroll(android.view.View.FOCUS_DOWN);});capture("13-proforma-totals");
 ui(()->{try{JSONObject x=new JSONObject("{\"number\":\"PI-TEST-1\",\"date\":\"2026-09-27\",\"buyer\":\"ABC Textiles and Trading Company\",\"city\":\"Bengaluru\",\"total\":945,\"discountAmount\":100,\"taxable\":900,\"igst\":45,\"items\":[],\"bank\":{\"bankName\":\"State Bank\",\"accountNumber\":\"001234567890\",\"ifsc\":\"SBIN0000123\"}}");Method m=MainActivity.class.getDeclaredMethod("proformaReview",JSONObject.class,String.class);m.setAccessible(true);m.invoke(rule.getActivity(),x,composer.draft.toString());}catch(Exception e){throw new RuntimeException(e);}});capture("14-proforma-review");
 }
}
