package in.tallybridge.app;

import android.content.ContentResolver;
import android.database.Cursor;
import android.graphics.Bitmap;
import android.graphics.BitmapFactory;
import android.graphics.Canvas;
import android.graphics.Color;
import android.graphics.Matrix;
import android.graphics.pdf.PdfRenderer;
import android.media.ExifInterface;
import android.net.Uri;
import android.os.ParcelFileDescriptor;
import android.provider.OpenableColumns;
import java.io.ByteArrayOutputStream;
import java.io.File;
import java.io.FileOutputStream;
import java.io.InputStream;
import java.util.ArrayList;
import java.util.List;

/**
 * Turns photos and PDFs into the page images InvoiceTotally reads: a JPEG of at
 * most 1800 px and 1.4 MB, plus a 900 px preview used to find where each bill
 * starts. Same sizes as the InvoiceTotally web upload.
 */
final class BillPages {
 static final int MAX_SIDE=1800, THUMB_SIDE=900, MAX_BYTES=1_350_000, MAX_THUMB_BYTES=380_000, MAX_PAGES=40;
 private BillPages(){}

 /** One prepared page on disk. */
 static final class Page {
  final File full, thumb; final String group;
  Page(File full,File thumb,String group){this.full=full;this.thumb=thumb;this.group=group;}
 }

 static String displayName(ContentResolver cr,Uri uri){
  try(Cursor c=cr.query(uri,new String[]{OpenableColumns.DISPLAY_NAME},null,null,null)){
   if(c!=null&&c.moveToFirst()){String n=c.getString(0);if(n!=null&&!n.trim().isEmpty())return n.trim();}
  }catch(Exception ignored){}
  String last=uri.getLastPathSegment();return last==null?"Bill":last;
 }

 static boolean isPdf(ContentResolver cr,Uri uri){
  String type=cr.getType(uri);
  if(type!=null)return type.equals("application/pdf");
  return displayName(cr,uri).toLowerCase(java.util.Locale.ROOT).endsWith(".pdf");
 }

 /** Encodes bmp as JPEG, lowering quality and then size until it fits. */
 static byte[] encode(Bitmap src,int maxSide,int quality,int maxBytes)throws Exception{
  int side=maxSide,q=quality;
  while(true){
   Bitmap b=scaled(src,side);
   ByteArrayOutputStream out=new ByteArrayOutputStream();
   if(!b.compress(Bitmap.CompressFormat.JPEG,q,out))throw new Exception("Could not prepare the page image");
   if(b!=src)b.recycle();
   if(out.size()<=maxBytes||side<700)return out.toByteArray();
   if(q>62)q-=12;else side=Math.round(side*0.8f);
  }
 }

 private static Bitmap scaled(Bitmap src,int maxSide){
  int w=src.getWidth(),h=src.getHeight();float k=Math.min(1f,maxSide/(float)Math.max(w,h));
  if(k>=1f)return src;
  return Bitmap.createScaledBitmap(src,Math.max(1,Math.round(w*k)),Math.max(1,Math.round(h*k)),true);
 }

 private static Page save(Bitmap bmp,File dir,String group)throws Exception{
  if(!dir.exists()&&!dir.mkdirs())throw new Exception("Could not create private bill storage");
  String id=java.util.UUID.randomUUID().toString();
  File full=new File(dir,"p-"+id+".jpg"),thumb=new File(dir,"t-"+id+".jpg");
  write(full,encode(bmp,MAX_SIDE,86,MAX_BYTES));
  write(thumb,encode(bmp,THUMB_SIDE,60,MAX_THUMB_BYTES));
  return new Page(full,thumb,group);
 }

 private static void write(File f,byte[] data)throws Exception{
  try(FileOutputStream out=new FileOutputStream(f)){out.write(data);out.getFD().sync();}
 }

 /** A photo (from the camera, gallery or a share) as one page. */
 static Page fromImage(ContentResolver cr,Uri uri,File dir,String group)throws Exception{
  BitmapFactory.Options bounds=new BitmapFactory.Options();bounds.inJustDecodeBounds=true;
  try(InputStream in=cr.openInputStream(uri)){if(in==null)throw new Exception("Could not open the photo");BitmapFactory.decodeStream(in,null,bounds);}
  if(bounds.outWidth<=0||bounds.outHeight<=0)throw new Exception("This file is not a readable photo");
  int sample=1;while(Math.max(bounds.outWidth/(sample*2),bounds.outHeight/(sample*2))>=MAX_SIDE)sample*=2;
  BitmapFactory.Options opt=new BitmapFactory.Options();opt.inSampleSize=sample;
  Bitmap bmp;try(InputStream in=cr.openInputStream(uri)){bmp=BitmapFactory.decodeStream(in,null,opt);}
  if(bmp==null)throw new Exception("Could not read the photo");
  bmp=rotate(cr,uri,bmp);
  try{return save(bmp,dir,group);}finally{bmp.recycle();}
 }

 private static Bitmap rotate(ContentResolver cr,Uri uri,Bitmap src){
  int o=ExifInterface.ORIENTATION_NORMAL;
  try(InputStream in=cr.openInputStream(uri)){if(in!=null)o=new ExifInterface(in).getAttributeInt(ExifInterface.TAG_ORIENTATION,ExifInterface.ORIENTATION_NORMAL);}catch(Exception ignored){}
  float deg=o==ExifInterface.ORIENTATION_ROTATE_90?90:o==ExifInterface.ORIENTATION_ROTATE_180?180:o==ExifInterface.ORIENTATION_ROTATE_270?270:0;
  if(deg==0)return src;
  Matrix m=new Matrix();m.postRotate(deg);
  Bitmap out=Bitmap.createBitmap(src,0,0,src.getWidth(),src.getHeight(),m,true);
  if(out!=src)src.recycle();
  return out;
 }

 /** Every page of a PDF (up to limit), rendered on white like the web app does. */
 static List<Page> fromPdf(ContentResolver cr,Uri uri,File dir,String group,int limit)throws Exception{
  List<Page> out=new ArrayList<>();
  ParcelFileDescriptor fd=cr.openFileDescriptor(uri,"r");
  if(fd==null)throw new Exception("Could not open the PDF");
  PdfRenderer r;
  try{r=new PdfRenderer(fd);}catch(SecurityException e){fd.close();throw new Exception(group+": the PDF is password-protected — remove the password and try again");}
  catch(Exception e){fd.close();throw new Exception(group+": not a readable PDF");}
  try{
   int n=Math.min(r.getPageCount(),limit);
   for(int i=0;i<n;i++){
    try(PdfRenderer.Page page=r.openPage(i)){
     float k=Math.min(3f,MAX_SIDE/(float)Math.max(page.getWidth(),page.getHeight()));
     Bitmap bmp=Bitmap.createBitmap(Math.max(1,Math.round(page.getWidth()*k)),Math.max(1,Math.round(page.getHeight()*k)),Bitmap.Config.ARGB_8888);
     new Canvas(bmp).drawColor(Color.WHITE);
     page.render(bmp,null,null,PdfRenderer.Page.RENDER_MODE_FOR_DISPLAY);
     try{out.add(save(bmp,dir,group));}finally{bmp.recycle();}
    }
   }
  }finally{r.close();fd.close();}
  if(out.isEmpty())throw new Exception(group+": the PDF has no pages");
  return out;
 }

 static Bitmap preview(File f,int maxSide){
  BitmapFactory.Options b=new BitmapFactory.Options();b.inJustDecodeBounds=true;BitmapFactory.decodeFile(f.getAbsolutePath(),b);
  int s=1;while(Math.max(b.outWidth/s,b.outHeight/s)>maxSide)s*=2;
  BitmapFactory.Options o=new BitmapFactory.Options();o.inSampleSize=s;return BitmapFactory.decodeFile(f.getAbsolutePath(),o);
 }
}
