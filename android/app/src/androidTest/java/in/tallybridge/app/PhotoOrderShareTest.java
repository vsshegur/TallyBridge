package in.tallybridge.app;

import android.content.*;
import android.graphics.*;
import android.net.Uri;
import android.view.WindowManager;
import androidx.test.ext.junit.runners.AndroidJUnit4;
import androidx.test.platform.app.InstrumentationRegistry;
import androidx.test.rule.ActivityTestRule;
import org.junit.*;
import org.junit.runner.RunWith;
import org.json.*;
import java.io.*;
import static org.junit.Assert.*;

/** Proves WhatsApp/Gallery-style image sharing opens the offline Photo Order screen. */
@RunWith(AndroidJUnit4.class)
public class PhotoOrderShareTest {
 @Rule public ActivityTestRule<PhotoOrderActivity> rule=new ActivityTestRule<>(PhotoOrderActivity.class,false,false);
 private void ui(Runnable r){InstrumentationRegistry.getInstrumentation().runOnMainSync(r);InstrumentationRegistry.getInstrumentation().waitForIdleSync();}
 private void capture(String name)throws Exception {Thread.sleep(600);Bitmap b=InstrumentationRegistry.getInstrumentation().getUiAutomation().takeScreenshot();assertNotNull(b);File dir=new File(rule.getActivity().getFilesDir(),"ui");assertTrue(dir.exists()||dir.mkdirs());try(FileOutputStream out=new FileOutputStream(new File(dir,name+".png"))){assertTrue(b.compress(Bitmap.CompressFormat.PNG,100,out));}b.recycle();}
 private File waitForCopiedPhoto(File dir,long timeoutMs)throws InterruptedException {long deadline=System.currentTimeMillis()+timeoutMs;while(System.currentTimeMillis()<deadline){File[] files=dir.listFiles((d,name)->name.endsWith(".jpg"));if(files!=null)for(File f:files)if(f.length()>0)return f;Thread.sleep(100);}return null;}

 @Test public void imageShareOpensPhotoOrderAndCopiesImagePrivately()throws Exception {
  Context c=InstrumentationRegistry.getInstrumentation().getTargetContext();
  c.getSharedPreferences("identity",Context.MODE_PRIVATE).edit().clear().commit();
  CryptoStore secure=new CryptoStore(c);secure.put("base","http://127.0.0.1:9");secure.put("device","offline-photo-fixture");secure.put("company","Vijayalaxmi Textiles");
  OfflineOrdersStore local=new OfflineOrdersStore(secure);
  local.cacheBootstrap(new JSONObject().put("companies",new JSONArray().put(new JSONObject().put("name","Vijayalaxmi Textiles"))));
  local.cacheMasters("Vijayalaxmi Textiles",new JSONObject().put("parties",new JSONArray().put(new JSONObject().put("name","Mahesh Textiles")).put(new JSONObject().put("name","Raj Fabrics"))).put("stock",new JSONObject().put("items",new JSONArray())));

  File privateDir=new File(c.getFilesDir(),"order-images");File[] stale=privateDir.listFiles();if(stale!=null)for(File f:stale)assertTrue("could not clear stale photo-order fixture",f.delete());
  File pdfDir=new File(c.getCacheDir(),"pdf");assertTrue(pdfDir.exists()||pdfDir.mkdirs());File incoming=new File(pdfDir,"customer-order.png");Bitmap img=Bitmap.createBitmap(900,1200,Bitmap.Config.ARGB_8888);Canvas canvas=new Canvas(img);canvas.drawColor(Color.WHITE);Paint p=new Paint(Paint.ANTI_ALIAS_FLAG);p.setColor(Color.BLACK);p.setTextSize(58);canvas.drawText("CUSTOMER ORDER",90,170,p);p.setTextSize(40);canvas.drawText("Bath Towel - 24 pcs",90,280,p);canvas.drawText("Kitchen Napkin - 48 pcs",90,350,p);try(FileOutputStream out=new FileOutputStream(incoming)){assertTrue(img.compress(Bitmap.CompressFormat.PNG,100,out));}img.recycle();
  Uri uri=Uri.parse("content://in.tallybridge.app.pdf/"+incoming.getName());Intent intent=new Intent(Intent.ACTION_SEND).setType("image/png").putExtra(Intent.EXTRA_STREAM,uri).addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION|Intent.FLAG_ACTIVITY_NEW_TASK);
  rule.launchActivity(intent);ui(()->rule.getActivity().getWindow().clearFlags(WindowManager.LayoutParams.FLAG_SECURE));
  File copied=waitForCopiedPhoto(privateDir,20000);assertNotNull("shared photo was not copied into private app storage within 20 seconds",copied);assertTrue(copied.length()>0);
  capture("22-photo-order-share");
 }
}
