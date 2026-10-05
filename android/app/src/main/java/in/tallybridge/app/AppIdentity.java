package in.tallybridge.app;

import android.content.Context;
import android.content.pm.PackageInfo;
import android.content.pm.PackageManager;
import android.content.pm.Signature;
import android.os.Build;
import java.security.MessageDigest;
import java.util.Locale;

final class AppIdentity {
 static final String EXPECTED_SHA1="55:CC:4A:6D:74:68:51:43:A7:E1:E9:14:B7:39:CF:C6:FF:4F:2A:BE";
 static final String EXPECTED_SHA256="55:2C:7E:A3:B6:93:BE:84:19:59:24:83:CD:5A:26:1B:F3:AF:83:AD:5B:93:F7:EF:35:C4:17:27:64:B0:63:DF";
 private AppIdentity(){}

 static String sha1(Context c){try{return digest(current(c),"SHA-1");}catch(Exception e){return "Unavailable";}}
 static String sha256(Context c){try{return digest(current(c),"SHA-256");}catch(Exception e){return "Unavailable";}}
 static boolean permanent(Context c){return same(sha1(c),EXPECTED_SHA1)&&same(sha256(c),EXPECTED_SHA256);}
 static String status(Context c){return permanent(c)?"Permanent signing verified":"Signing differs from permanent release identity";}

 private static Signature current(Context c)throws Exception{
  PackageManager pm=c.getPackageManager();
  PackageInfo pi;
  if(Build.VERSION.SDK_INT>=28){
   pi=pm.getPackageInfo(c.getPackageName(),PackageManager.GET_SIGNING_CERTIFICATES);
   Signature[] x=pi.signingInfo.getApkContentsSigners();
   if(x==null||x.length==0)throw new Exception("No app signer");
   return x[0];
  }
  pi=pm.getPackageInfo(c.getPackageName(),PackageManager.GET_SIGNATURES);
  if(pi.signatures==null||pi.signatures.length==0)throw new Exception("No app signer");
  return pi.signatures[0];
 }
 private static String digest(Signature s,String algorithm)throws Exception{
  byte[] d=MessageDigest.getInstance(algorithm).digest(s.toByteArray());
  StringBuilder b=new StringBuilder();
  for(int i=0;i<d.length;i++){if(i>0)b.append(':');b.append(String.format(Locale.US,"%02X",d[i]&0xff));}
  return b.toString();
 }
 private static boolean same(String a,String b){return a!=null&&b!=null&&a.replace(":","").equalsIgnoreCase(b.replace(":",""));}
 static boolean archiveMatches(Context c,String apk)throws Exception{
  PackageManager pm=c.getPackageManager();PackageInfo pi;
  if(Build.VERSION.SDK_INT>=28){pi=pm.getPackageArchiveInfo(apk,PackageManager.GET_SIGNING_CERTIFICATES);if(pi==null||pi.signingInfo==null)return false;Signature[] x=pi.signingInfo.getApkContentsSigners();if(x==null||x.length==0)return false;return same(digest(x[0],"SHA-256"),EXPECTED_SHA256)&&"in.tallybridge.app".equals(pi.packageName);}
  pi=pm.getPackageArchiveInfo(apk,PackageManager.GET_SIGNATURES);if(pi==null||pi.signatures==null||pi.signatures.length==0)return false;return same(digest(pi.signatures[0],"SHA-256"),EXPECTED_SHA256)&&"in.tallybridge.app".equals(pi.packageName);
 }
}
