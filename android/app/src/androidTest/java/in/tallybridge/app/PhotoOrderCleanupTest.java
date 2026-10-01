package in.tallybridge.app;

import android.content.*;
import android.graphics.*;
import android.net.Uri;
import androidx.test.ext.junit.runners.AndroidJUnit4;
import androidx.test.platform.app.InstrumentationRegistry;
import androidx.test.rule.ActivityTestRule;
import org.junit.*;
import org.junit.runner.RunWith;
import org.json.*;
import java.io.*;
import java.util.*;
import static org.junit.Assert.*;

/** Proves cancel/back does not leave unsaved customer images in private storage. */
@RunWith(AndroidJUnit4.class)
public class PhotoOrderCleanupTest {
 @Rule public ActivityTestRule<PhotoOrderActivity> rule=new ActivityTestRule<>(PhotoOrderActivity.class,false,false);

 @Test public void closingUnsavedPhotoOrderDeletesNewPrivateCopy()throws Exception {
  Context c=InstrumentationRegistry.getInstrumentation().getTargetContext();
  c.getSharedPreferences("identity",Context.MODE_PRIVATE).edit().clear().commit();
  CryptoStore secure=new CryptoStore(c);secure.put("base","http://127.0.0.1:9");secure.put("device","cleanup-fixture");secure.put("company","Cleanup Firm");
  OfflineOrdersStore local=new OfflineOrdersStore(secure);local.cacheBootstrap(new JSONObject().put("companies",new JSONArray().put(new JSONObject().put("name","Cleanup Firm"))));
  File privateDir=new File(c.getFilesDir(),"order-images");Set<String> before=new HashSet<>();File[] old=privateDir.listFiles();if(old!=null)for(File f:old)before.add(f.getName());

  File pdfDir=new File(c.getCacheDir(),"pdf");assertTrue(pdfDir.exists()||pdfDir.mkdirs());File incoming=new File(pdfDir,"cleanup-order.png");Bitmap img=Bitmap.createBitmap(500,700,Bitmap.Config.ARGB_8888);new Canvas(img).drawColor(Color.WHITE);try(FileOutputStream out=new FileOutputStream(incoming)){assertTrue(img.compress(Bitmap.CompressFormat.PNG,100,out));}img.recycle();
  Uri uri=Uri.parse("content://in.tallybridge.app.pdf/"+incoming.getName());Intent intent=new Intent(Intent.ACTION_SEND).setType("image/png").putExtra(Intent.EXTRA_STREAM,uri).addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION|Intent.FLAG_ACTIVITY_NEW_TASK);
  rule.launchActivity(intent);

  File created=null;long deadline=System.currentTimeMillis()+8000;while(System.currentTimeMillis()<deadline&&created==null){File[] files=privateDir.listFiles();if(files!=null)for(File f:files)if(!before.contains(f.getName())){created=f;break;}if(created==null)Thread.sleep(100);}
  assertNotNull("photo share did not create a private working copy",created);assertTrue(created.exists());
  final File working=created;
  InstrumentationRegistry.getInstrumentation().runOnMainSync(()->rule.getActivity().finish());
  deadline=System.currentTimeMillis()+5000;while(System.currentTimeMillis()<deadline&&working.exists())Thread.sleep(100);
  assertFalse("unsaved shared photo was left behind after closing",working.exists());
 }
}
