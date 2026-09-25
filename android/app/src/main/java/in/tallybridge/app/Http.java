package in.tallybridge.app;

import java.net.*;
import java.io.*;
import java.nio.charset.StandardCharsets;
import java.util.*;
import org.json.JSONObject;

final class Http {
 static final class Failure extends IOException {final int status;Failure(int status,String message){super(message);this.status=status;}}
 static byte[] request(String target,String method,byte[] body,Map<String,String> headers,int limit)throws Exception{
  URL url=new URL(target);if(!"https".equals(url.getProtocol()))throw new IOException("HTTPS is required");
  HttpURLConnection c=(HttpURLConnection)url.openConnection();
  c.setConnectTimeout(12000);c.setReadTimeout(60000);c.setInstanceFollowRedirects(false);c.setUseCaches(false);c.setRequestMethod(method);
  for(Map.Entry<String,String> h:headers.entrySet())c.setRequestProperty(h.getKey(),h.getValue());
  try{
   if(body!=null){c.setDoOutput(true);c.setFixedLengthStreamingMode(body.length);try(OutputStream out=c.getOutputStream()){out.write(body);}}
   int status=c.getResponseCode();InputStream input=status>=400?c.getErrorStream():c.getInputStream();
   byte[] data=input==null?new byte[0]:read(input,status>=400?65536:limit);
   if(status<200||status>=300){String message;
    try{message=new JSONObject(new String(data,StandardCharsets.UTF_8)).optString("error","HTTP "+status);}catch(Exception e){message="HTTP "+status;}
    if(status==302||status==301)message="Cloudflare sign-in is needed. Enable Managed OAuth in your Access application, then sign in again.";
    throw new Failure(status,message);
   }
   return data;
  }finally{c.disconnect();}
 }
 static byte[] read(InputStream input,int limit)throws IOException{
  try(InputStream in=input;ByteArrayOutputStream out=new ByteArrayOutputStream()){
   byte[] b=new byte[8192];int n,total=0;while((n=in.read(b))!=-1){total+=n;if(total>limit)throw new IOException("Response is too large");out.write(b,0,n);}return out.toByteArray();
  }
 }
 static JSONObject json(String target,String method,String body,Map<String,String> headers)throws Exception{
  byte[] data=request(target,method,body==null?null:body.getBytes(StandardCharsets.UTF_8),headers,16*1024*1024);
  try{return new JSONObject(new String(data,StandardCharsets.UTF_8));}catch(Exception e){throw new IOException("Expected report data but received a web page. Check Cloudflare Access settings.");}
 }
 static String enc(String value){try{return URLEncoder.encode(value,"UTF-8");}catch(Exception e){throw new IllegalArgumentException(e);}}
 static String form(Map<String,String> fields){StringBuilder s=new StringBuilder();for(Map.Entry<String,String> e:fields.entrySet()){if(s.length()>0)s.append('&');s.append(enc(e.getKey())).append('=').append(enc(e.getValue()));}return s.toString();}
 static Map<String,String> pairs(String...values){Map<String,String> m=new LinkedHashMap<>();for(int i=0;i<values.length;i+=2)m.put(values[i],values[i+1]);return m;}
}
