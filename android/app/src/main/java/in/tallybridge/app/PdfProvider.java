package in.tallybridge.app;
import android.content.*;
import android.database.*;
import android.net.Uri;
import android.os.*;
import android.provider.OpenableColumns;
import java.io.*;

/** Read-only, temporary URI grants; no external-storage permissions. */
public final class PdfProvider extends ContentProvider {
 @Override public boolean onCreate(){return true;}
 private File file(Uri uri)throws FileNotFoundException{
  try{File root=new File(getContext().getCacheDir(),"pdf").getCanonicalFile();String name=uri.getLastPathSegment();if(name==null||!name.endsWith(".pdf"))throw new IOException();File f=new File(root,name).getCanonicalFile();if(!root.equals(f.getParentFile())||!f.isFile()||System.currentTimeMillis()-f.lastModified()>86400000L)throw new IOException();return f;}catch(IOException e){throw new FileNotFoundException("PDF unavailable");}
 }
 @Override public ParcelFileDescriptor openFile(Uri uri,String mode)throws FileNotFoundException{if(!mode.equals("r"))throw new FileNotFoundException("Read-only PDF");return ParcelFileDescriptor.open(file(uri),ParcelFileDescriptor.MODE_READ_ONLY);}
 @Override public String getType(Uri uri){return "application/pdf";}
 @Override public Cursor query(Uri uri,String[] projection,String selection,String[] args,String sort){try{File f=file(uri);String[] columns=projection==null?new String[]{OpenableColumns.DISPLAY_NAME,OpenableColumns.SIZE}:projection;MatrixCursor cursor=new MatrixCursor(columns);Object[] row=new Object[columns.length];for(int i=0;i<columns.length;i++)row[i]=columns[i].equals(OpenableColumns.DISPLAY_NAME)?f.getName():columns[i].equals(OpenableColumns.SIZE)?f.length():null;cursor.addRow(row);return cursor;}catch(Exception e){return null;}}
 @Override public Uri insert(Uri u,ContentValues v){throw new UnsupportedOperationException();}
 @Override public int delete(Uri u,String s,String[] a){throw new UnsupportedOperationException();}
 @Override public int update(Uri u,ContentValues v,String s,String[] a){throw new UnsupportedOperationException();}
}
