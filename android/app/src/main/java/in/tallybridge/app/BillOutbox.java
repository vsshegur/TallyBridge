package in.tallybridge.app;

import android.content.Context;
import org.json.JSONArray;
import org.json.JSONObject;
import java.io.File;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.security.SecureRandom;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Base64;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Locale;
import java.util.Map;

/**
 * Bills waiting on the phone until the Windows PC has them. Each bill is a
 * folder of page images plus meta.json; it is sent page by page and removed
 * from the phone once the PC confirms it. If the PC or the internet is not
 * available the bill simply waits and is sent next time.
 */
final class BillOutbox {
 private final File root;
 BillOutbox(Context c){root=new File(c.getFilesDir(),"bills-out");}

 static String newId(){byte[] b=new byte[12];new SecureRandom().nextBytes(b);StringBuilder s=new StringBuilder("ph-");for(byte x:b)s.append(String.format(Locale.ROOT,"%02x",x&255));return s.toString();}

 private File dir(String id){return new File(root,id);}
 private File meta(String id){return new File(dir(id),"meta.json");}

 private static void writeAtomic(File f,String text)throws Exception{
  File tmp=new File(f.getPath()+".tmp");
  try(java.io.FileOutputStream out=new java.io.FileOutputStream(tmp)){out.write(text.getBytes(StandardCharsets.UTF_8));out.getFD().sync();}
  if(!tmp.renameTo(f))throw new Exception("Could not save the bill on this phone");
 }
 private JSONObject read(String id)throws Exception{return new JSONObject(new String(Files.readAllBytes(meta(id).toPath()),StandardCharsets.UTF_8));}
 private void save(JSONObject m)throws Exception{writeAtomic(meta(m.getString("id")),m.toString());}

 /** Moves prepared pages into a new outbox entry. Pages keep their order; each source (camera, a PDF…) becomes one file. */
 synchronized JSONObject add(String connection,String company,String voucherType,List<BillPages.Page> pages)throws Exception{
  if(pages.isEmpty())throw new Exception("Add at least one page");
  if(pages.size()>BillPages.MAX_PAGES)throw new Exception("Up to "+BillPages.MAX_PAGES+" pages can be sent at once");
  String id=newId();File d=dir(id);if(!d.mkdirs())throw new Exception("Could not create private bill storage");
  Map<String,Integer> files=new LinkedHashMap<>();JSONArray pageList=new JSONArray();
  for(BillPages.Page p:pages){
   String group=p.group==null||p.group.trim().isEmpty()?"Bill photos":p.group.trim();
   if(!files.containsKey(group))files.put(group,0);
   int fileIdx=new ArrayList<>(files.keySet()).indexOf(group),pageIdx=files.get(group);files.put(group,pageIdx+1);
   String name=String.format(Locale.ROOT,"%02d-%03d",fileIdx,pageIdx);
   File full=new File(d,"p"+name+".jpg"),thumb=new File(d,"t"+name+".jpg");
   if(!p.full.renameTo(full))Files.copy(p.full.toPath(),full.toPath());
   if(p.thumb!=null&&p.thumb.exists()&&!p.thumb.renameTo(thumb))Files.copy(p.thumb.toPath(),thumb.toPath());
   pageList.put(new JSONObject().put("file",fileIdx).put("page",pageIdx).put("name",name).put("sent",false));
  }
  if(files.size()>20)throw new Exception("Up to 20 separate files can be sent at once");
  JSONArray fileList=new JSONArray();for(Map.Entry<String,Integer> e:files.entrySet())fileList.put(new JSONObject().put("name",e.getKey()).put("pages",e.getValue()));
  JSONObject m=new JSONObject().put("id",id).put("connection",connection).put("company",company).put("voucherType",voucherType)
   .put("files",fileList).put("pages",pageList).put("createdAt",Instant.now().toString()).put("state","waiting").put("error","");
  save(m);
  return m;
 }

 synchronized List<JSONObject> list(){
  List<JSONObject> out=new ArrayList<>();File[] ds=root.listFiles();if(ds==null)return out;
  for(File d:ds){try{if(new File(d,"meta.json").exists())out.add(read(d.getName()));}catch(Exception ignored){}}
  out.sort((a,b)->b.optString("createdAt").compareTo(a.optString("createdAt")));
  return out;
 }

 synchronized void delete(String id){File d=dir(id);File[] fs=d.listFiles();if(fs!=null)for(File f:fs)f.delete();d.delete();}

 synchronized int waiting(){return list().size();}

 interface Progress{void onPage(int done,int total);}

 /** Sends one bill to the PC. Returns null when the PC has it, or the reason it is still waiting. */
 String send(BridgeApi api,String id,Progress progress){
  JSONObject m;
  synchronized(this){try{m=read(id);}catch(Exception e){return "Bill not found on this phone";}}
  try{
   JSONArray pages=m.getJSONArray("pages");int total=pages.length(),done=0;
   for(int i=0;i<total;i++){
    JSONObject p=pages.getJSONObject(i);
    if(p.optBoolean("sent")){done++;continue;}
    File d=dir(id),full=new File(d,"p"+p.getString("name")+".jpg"),thumb=new File(d,"t"+p.getString("name")+".jpg");
    JSONObject body=new JSONObject().put("upload",id).put("file",p.getInt("file")).put("page",p.getInt("page"))
     .put("image",Base64.getEncoder().encodeToString(Files.readAllBytes(full.toPath())));
    if(thumb.exists())body.put("thumb",Base64.getEncoder().encodeToString(Files.readAllBytes(thumb.toPath())));
    api.call("/api/v1/mobile/bills/page","POST",body.toString());
    p.put("sent",true);done++;
    synchronized(this){save(m);}
    if(progress!=null)progress.onPage(done,total);
   }
   JSONObject submit=new JSONObject().put("upload",id).put("connection",m.getString("connection")).put("voucherType",m.optString("voucherType","Purchase")).put("files",m.getJSONArray("files"));
   api.call("/api/v1/mobile/bills/submit","POST",submit.toString());
   delete(id);
   return null;
  }catch(Http.Failure f){
   String why=f.getMessage()==null?"The PC refused the bill":f.getMessage();
   // A rejected upload (for example, the PC lost some pages after a restart) starts again from page one.
   if(f.status==400&&why.contains("send again"))try{JSONArray pages=m.getJSONArray("pages");for(int i=0;i<pages.length();i++)pages.getJSONObject(i).put("sent",false);}catch(Exception ignored){}
   mark(m,f.status==409||f.status==400?"failed":"waiting",why);
   return why;
  }catch(Exception e){
   String why="Not sent yet — the PC is not reachable. It will be sent when TallyBridge is connected.";
   mark(m,"waiting",why);
   return why;
  }
 }

 private synchronized void mark(JSONObject m,String state,String error){try{m.put("state",state).put("error",error);save(m);}catch(Exception ignored){}}

 /** Sends every waiting bill; returns how many are still on the phone. */
 int flush(BridgeApi api){for(JSONObject m:list())send(api,m.optString("id"),null);return list().size();}
}
