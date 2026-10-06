package in.tallybridge.app;

import android.app.*;
import android.content.*;
import android.widget.Toast;

/** One consistent share chooser for TallyBridge business text. */
final class ShareCenter {
 private static final String WA="com.whatsapp", WAB="com.whatsapp.w4b";
 private ShareCenter(){}
 static void shareText(Activity a,String title,String text){
  String[] choices={"WhatsApp Business","WhatsApp","Other apps"};
  new AlertDialog.Builder(a).setTitle("Share with").setItems(choices,(d,which)->{
   if(which==0)send(a,title,text,WAB,"WhatsApp Business is not installed");
   else if(which==1)send(a,title,text,WA,"WhatsApp is not installed");
   else send(a,title,text,null,"");
  }).setNegativeButton("Cancel",null).show();
 }
 private static void send(Activity a,String title,String text,String pkg,String missing){
  Intent i=new Intent(Intent.ACTION_SEND);i.setType("text/plain");i.putExtra(Intent.EXTRA_TEXT,text);if(pkg!=null)i.setPackage(pkg);
  try{if(pkg==null)a.startActivity(Intent.createChooser(i,title));else a.startActivity(i);}catch(Exception e){
   if(pkg!=null){Toast.makeText(a,missing,Toast.LENGTH_SHORT).show();Intent any=new Intent(Intent.ACTION_SEND);any.setType("text/plain");any.putExtra(Intent.EXTRA_TEXT,text);try{a.startActivity(Intent.createChooser(any,title));}catch(Exception ignored){Toast.makeText(a,"No sharing app is available",Toast.LENGTH_SHORT).show();}}
  }
 }
}
