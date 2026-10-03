package in.tallybridge.app;

import android.app.Activity;
import android.content.*;
import android.net.Uri;
import android.os.Build;
import android.provider.Settings;
import android.widget.Toast;
import java.io.*;
import java.net.*;
import java.util.concurrent.*;

final class UpdateInstaller {
 private static final ExecutorService work=Executors.newSingleThreadExecutor();
 private UpdateInstaller(){}

 static void install(Activity a,String url){
  if(url==null||!url.startsWith("https://github.com/vsshegur/TallyBridge/releases/download/")){Toast.makeText(a,"Update link is not an official TallyBridge release",Toast.LENGTH_LONG).show();return;}
  if(Build.VERSION.SDK_INT>=26&&!a.getPackageManager().canRequestPackageInstalls()){
   Toast.makeText(a,"Allow TallyBridge to install its verified update, then tap Install update again.",Toast.LENGTH_LONG).show();
   try{a.startActivity(new Intent(Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES,Uri.parse("package:"+a.getPackageName())));}catch(Exception ignored){}
   return;
  }
  Toast.makeText(a,"Downloading signed update…",Toast.LENGTH_SHORT).show();
  work.execute(()->{
   File file=null;
   try{
    URLConnection c=new URL(url).openConnection();c.setConnectTimeout(10000);c.setReadTimeout(30000);c.setRequestProperty("User-Agent","TallyBridge Android updater");
    if(c instanceof HttpURLConnection){HttpURLConnection h=(HttpURLConnection)c;h.setInstanceFollowRedirects(true);int code=h.getResponseCode();if(code<200||code>=300)throw new IOException("Update download returned HTTP "+code);}
    long declared=c.getContentLengthLong();if(declared>0&&declared>80L*1024*1024)throw new IOException("Update file is unexpectedly large");
    File dir=new File(a.getCacheDir(),"pdf");if(!dir.exists()&&!dir.mkdirs())throw new IOException("Could not prepare update storage");
    file=new File(dir,"TallyBridge-update.apk");long total=0;try(InputStream in=c.getInputStream();OutputStream out=new FileOutputStream(file)){byte[] b=new byte[32768];int n;while((n=in.read(b))!=-1){total+=n;if(total>80L*1024*1024)throw new IOException("Update file is unexpectedly large");out.write(b,0,n);}}
    if(total<100000)throw new IOException("Downloaded update is incomplete");
    if(!AppIdentity.archiveMatches(a,file.getAbsolutePath()))throw new SecurityException("Update signature does not match the permanent TallyBridge certificate");
    File ready=file;a.runOnUiThread(()->openInstaller(a,ready));
   }catch(Exception e){if(file!=null)file.delete();String m=e.getMessage()==null?"Update failed":e.getMessage();a.runOnUiThread(()->Toast.makeText(a,m,Toast.LENGTH_LONG).show());}
  });
 }
 private static void openInstaller(Activity a,File f){
  try{Uri uri=Uri.parse("content://in.tallybridge.app.pdf/"+Uri.encode(f.getName()));Intent i=new Intent(Intent.ACTION_VIEW).setDataAndType(uri,"application/vnd.android.package-archive").addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION);a.startActivity(i);}catch(Exception e){Toast.makeText(a,"Android installer could not open the verified APK",Toast.LENGTH_LONG).show();}
 }
}
