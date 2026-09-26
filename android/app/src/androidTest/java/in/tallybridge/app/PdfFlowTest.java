package in.tallybridge.app;
import android.graphics.pdf.PdfDocument;
import android.graphics.pdf.PdfRenderer;
import android.net.Uri;
import android.os.ParcelFileDescriptor;
import android.database.Cursor;
import android.provider.OpenableColumns;
import androidx.test.ext.junit.runners.AndroidJUnit4;
import androidx.test.platform.app.InstrumentationRegistry;
import org.junit.Test;
import org.junit.runner.RunWith;
import java.io.*;
import static org.junit.Assert.*;

@RunWith(AndroidJUnit4.class)
public class PdfFlowTest {
 @Test public void sharedPdfCanBeReadAndRendered()throws Exception {
  android.content.Context context=InstrumentationRegistry.getInstrumentation().getTargetContext();
  File dir=new File(context.getCacheDir(),"pdf");assertTrue(dir.exists()||dir.mkdirs());
  File file=new File(dir,"Outstanding-test.pdf");
  PdfDocument doc=new PdfDocument();try{
   PdfDocument.Page page=doc.startPage(new PdfDocument.PageInfo.Builder(595,842,1).create());
   page.getCanvas().drawText("Outstanding bill TEST-1",40,60,new android.graphics.Paint());doc.finishPage(page);
   try(OutputStream out=new FileOutputStream(file)){doc.writeTo(out);}
  }finally{doc.close();}
  Uri uri=Uri.parse("content://in.tallybridge.app.pdf/Outstanding-test.pdf");
  assertEquals("application/pdf",context.getContentResolver().getType(uri));
  try(Cursor c=context.getContentResolver().query(uri,null,null,null,null)){assertNotNull(c);assertTrue(c.moveToFirst());assertEquals(file.length(),c.getLong(c.getColumnIndexOrThrow(OpenableColumns.SIZE)));}
  try(ParcelFileDescriptor fd=context.getContentResolver().openFileDescriptor(uri,"r");PdfRenderer renderer=new PdfRenderer(fd)){assertEquals(1,renderer.getPageCount());try(PdfRenderer.Page page=renderer.openPage(0)){assertEquals(595,page.getWidth());}}
  assertTrue(file.delete());
 }
}
