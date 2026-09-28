package in.tallybridge.app;

import android.content.*;
import android.net.Uri;
import androidx.test.ext.junit.runners.AndroidJUnit4;
import androidx.test.rule.ActivityTestRule;
import org.junit.*;
import org.junit.runner.RunWith;
import static org.junit.Assert.*;

@RunWith(AndroidJUnit4.class)
public class ShareIntentTest {
 @Rule public ActivityTestRule<MainActivity> rule=new ActivityTestRule<>(MainActivity.class);

 @Test public void pdfPlusDetailsCarriesPdfAndTextAcrossIntentAndClipData(){
  Uri uri=Uri.parse("content://in.tallybridge.app.pdf/Invoice-1.pdf");
  String details="Invoice 1\nCGST Amount - ₹90.00\nTotal Amount - ₹1,180.00";
  Intent i=rule.getActivity().buildPdfShareIntent(uri,details);
  assertEquals(Intent.ACTION_SEND,i.getAction());
  assertEquals("*/*",i.getType());
  assertEquals(uri,i.getParcelableExtra(Intent.EXTRA_STREAM));
  assertEquals(details,i.getStringExtra(Intent.EXTRA_TEXT));
  assertNotNull(i.getClipData());
  assertEquals(1,i.getClipData().getItemCount());
  assertEquals(uri,i.getClipData().getItemAt(0).getUri());
  assertEquals(details,String.valueOf(i.getClipData().getItemAt(0).getText()));
  assertTrue((i.getFlags()&Intent.FLAG_GRANT_READ_URI_PERMISSION)!=0);
 }

 @Test public void pdfOnlyKeepsPdfMimeAndNoText(){
  Uri uri=Uri.parse("content://in.tallybridge.app.pdf/Invoice-2.pdf");
  Intent i=rule.getActivity().buildPdfShareIntent(uri,"");
  assertEquals("application/pdf",i.getType());
  assertEquals(uri,i.getParcelableExtra(Intent.EXTRA_STREAM));
  assertNull(i.getStringExtra(Intent.EXTRA_TEXT));
  assertNotNull(i.getClipData());
 }
}
