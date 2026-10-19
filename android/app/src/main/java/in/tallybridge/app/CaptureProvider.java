package in.tallybridge.app;

import android.content.ContentProvider;
import android.content.ContentValues;
import android.database.Cursor;
import android.net.Uri;
import android.os.ParcelFileDescriptor;
import java.io.File;
import java.io.FileNotFoundException;
import java.io.IOException;

/**
 * Gives the camera app a private place to save one bill photo
 * (content://in.tallybridge.app.capture/NAME.jpg → cache/capture/NAME.jpg).
 * Not exported; the camera only gets a one-time grant for that photo.
 */
public final class CaptureProvider extends ContentProvider {
 static final String AUTHORITY="in.tallybridge.app.capture";
 static Uri uriFor(File f){return Uri.parse("content://"+AUTHORITY+"/"+f.getName());}
 static File dir(android.content.Context c){File d=new File(c.getCacheDir(),"capture");d.mkdirs();return d;}

 @Override public boolean onCreate(){return true;}
 private File file(Uri uri)throws FileNotFoundException{
  try{
   File root=dir(getContext()).getCanonicalFile();String name=uri.getLastPathSegment();
   if(name==null||!name.matches("cap-[a-zA-Z0-9-]{8,64}\\.jpg"))throw new IOException();
   File f=new File(root,name).getCanonicalFile();
   if(!root.equals(f.getParentFile()))throw new IOException();
   return f;
  }catch(IOException e){throw new FileNotFoundException("Capture file unavailable");}
 }
 @Override public ParcelFileDescriptor openFile(Uri uri,String mode)throws FileNotFoundException{
  return ParcelFileDescriptor.open(file(uri),ParcelFileDescriptor.parseMode(mode));
 }
 @Override public String getType(Uri uri){return "image/jpeg";}
 @Override public Cursor query(Uri uri,String[] projection,String selection,String[] args,String sort){return null;}
 @Override public Uri insert(Uri u,ContentValues v){throw new UnsupportedOperationException();}
 @Override public int delete(Uri u,String s,String[] a){throw new UnsupportedOperationException();}
 @Override public int update(Uri u,ContentValues v,String s,String[] a){throw new UnsupportedOperationException();}
}
