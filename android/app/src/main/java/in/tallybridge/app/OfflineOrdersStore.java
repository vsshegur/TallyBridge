package in.tallybridge.app;

import org.json.*;
import java.io.*;
import java.nio.file.*;
import java.time.*;
import java.time.format.DateTimeFormatter;
import java.util.*;

/** Encrypted phone-side Orders cache. Phone changes queue locally and reconcile with the PC later. */
final class OfflineOrdersStore {
 private static final String STATE="orders_offline_state_v1", BOOT="orders_offline_boot_v1", MASTERS="orders_offline_masters_v1", BUYERS="orders_offline_buyers_v1", PHONE_TOKEN="__PHONE_ORDER_ID";
 private static final long RETENTION_MS=7L*24*60*60*1000;
 private static final int MAX_PHOTO_BYTES=3*1024*1024;
 private final CryptoStore secure;
 OfflineOrdersStore(CryptoStore secure){this.secure=secure;}

 private JSONObject read(String key)throws Exception {String s=secure.secret(key);return s.isEmpty()?new JSONObject():new JSONObject(s);}
 private void write(String key,JSONObject value)throws Exception {secure.putSecret(key,value.toString());}
 private JSONObject clone(JSONObject x)throws Exception{return x==null?new JSONObject():new JSONObject(x.toString());}
 private JSONArray clone(JSONArray x)throws Exception{return x==null?new JSONArray():new JSONArray(x.toString());}
 private JSONObject state()throws Exception {JSONObject s=read(STATE);if(s.optJSONArray("orders")==null)s.put("orders",new JSONArray());prune(s);return s;}
 private void prune(JSONObject s)throws Exception {JSONArray src=s.optJSONArray("orders"),kept=new JSONArray();long now=System.currentTimeMillis();boolean changed=false;if(src==null)src=new JSONArray();for(int i=0;i<src.length();i++){JSONObject o=src.optJSONObject(i);if(o==null)continue;if("dispatched".equals(o.optString("status"))){long at=instantMs(o.optString("dispatchedAt"));if(at>0&&now-at>=RETENTION_MS){deleteLocalPhotos(o);changed=true;continue;}}kept.put(o);}if(changed)s.put("orders",kept);}
 private void deleteLocalPhotos(JSONObject o){JSONArray a=o.optJSONArray("attachments");if(a==null)return;for(int i=0;i<a.length();i++){JSONObject p=a.optJSONObject(i);if(p==null)continue;String path=p.optString("localPath");if(!path.isEmpty())try{new File(path).delete();}catch(Exception ignored){}}}
 private long instantMs(String value){try{return Instant.parse(value).toEpochMilli();}catch(Exception e){try{return OffsetDateTime.parse(value).toInstant().toEpochMilli();}catch(Exception ignored){return 0;}}}
 private String now(){return Instant.now().toString();}

 JSONObject list()throws Exception {synchronized(OfflineOrdersStore.class){JSONObject s=state();write(STATE,s);return listFrom(s.optJSONArray("orders"));}}
 private JSONObject listFrom(JSONArray orders)throws Exception {List<JSONObject> pending=new ArrayList<>(),dispatched=new ArrayList<>();for(int i=0;i<orders.length();i++){JSONObject raw=orders.optJSONObject(i);if(raw==null)continue;JSONObject o=decorate(clone(raw));if("dispatched".equals(o.optString("status")))dispatched.add(o);else pending.add(o);}pending.sort((a,b)->b.optString("createdAt").compareTo(a.optString("createdAt")));dispatched.sort((a,b)->b.optString("dispatchedAt").compareTo(a.optString("dispatchedAt")));JSONArray p=new JSONArray(),d=new JSONArray();for(JSONObject o:pending)p.put(o);for(JSONObject o:dispatched)d.put(o);return new JSONObject().put("pending",p).put("dispatched",d).put("pendingCount",p.length()).put("dispatchedCount",d.length());}

 JSONObject saveLocal(JSONObject payload,JSONObject original)throws Exception {synchronized(OfflineOrdersStore.class){JSONObject s=state();JSONArray a=s.getJSONArray("orders");JSONObject o=clone(payload);String ts=now();if(original==null){String shortId=UUID.randomUUID().toString().replace("-","").substring(0,8).toUpperCase(Locale.ROOT);o.put("id","local-"+UUID.randomUUID()).put("number","PHONE-"+LocalDateTime.now().format(DateTimeFormatter.ofPattern("yyyyMMdd-HHmm"))+"-"+shortId.substring(0,3)).put("createdAt",ts).put("_localOnly",true);}else{o.put("id",original.optString("id")).put("number",original.optString("number")).put("createdAt",original.optString("createdAt",ts));if(original.optBoolean("_localOnly"))o.put("_localOnly",true);else o.put("_dirty",true);}o.put("status","pending").put("updatedAt",ts).remove("dispatchedAt");o.remove("_dispatchDirty");o.put("shareText",shareText(o));replace(a,o);write(STATE,s);return decorate(clone(o));}}

 JSONObject dispatchLocal(JSONObject original)throws Exception {synchronized(OfflineOrdersStore.class){JSONObject s=state();JSONArray a=s.getJSONArray("orders");JSONObject o=clone(original);String ts=now();o.put("status","dispatched").put("dispatchedAt",ts).put("updatedAt",ts).put("shareText","").put("_dispatchDirty",true);replace(a,o);write(STATE,s);return decorate(clone(o));}}

 private void replace(JSONArray a,JSONObject order)throws Exception {String id=order.optString("id");for(int i=0;i<a.length();i++){JSONObject x=a.optJSONObject(i);if(x!=null&&id.equals(x.optString("id"))){a.put(i,order);return;}}a.put(order);}
 private JSONObject serverPayload(JSONObject o,boolean includeId)throws Exception {JSONObject details=clone(o.optJSONObject("buyerDetails"));if(o.optBoolean("_localOnly"))details.put(PHONE_TOKEN,o.optString("id"));JSONObject p=new JSONObject().put("company",o.optString("company")).put("buyer",o.optString("buyer")).put("buyerDetails",details).put("items",clone(o.optJSONArray("items"))).put("photoOrder",o.optBoolean("photoOrder")).put("notes",o.optString("notes"));if(includeId)p.put("id",o.optString("id"));return p;}
 private JSONObject findPhoneOrder(JSONObject remote,String localId){for(String key:new String[]{"pending","dispatched"}){JSONArray a=remote.optJSONArray(key);if(a==null)continue;for(int i=0;i<a.length();i++){JSONObject o=a.optJSONObject(i);if(o==null)continue;JSONObject d=o.optJSONObject("buyerDetails");if(d!=null&&localId.equals(d.optString(PHONE_TOKEN)))return o;}}return null;}
 private JSONObject attachmentBySHA(JSONArray a,String sha){if(a==null||sha==null||sha.isEmpty())return null;for(int i=0;i<a.length();i++){JSONObject p=a.optJSONObject(i);if(p!=null&&sha.equalsIgnoreCase(p.optString("sha256")))return p;}return null;}

 private JSONObject syncPhotos(BridgeApi api,JSONObject target,JSONObject saved)throws Exception {
  JSONArray local=target.optJSONArray("attachments");if(local==null||local.length()==0)return saved;
  JSONArray remote=saved.optJSONArray("attachments");JSONArray merged=new JSONArray();
  for(int i=0;i<local.length();i++){
   JSONObject p=local.optJSONObject(i);if(p==null)continue;String sha=p.optString("sha256"),localPath=p.optString("localPath");JSONObject server=attachmentBySHA(remote,sha);
   if(server==null){if(localPath.isEmpty())continue;byte[] bytes=Files.readAllBytes(Paths.get(localPath));if(bytes.length==0||bytes.length>MAX_PHOTO_BYTES)throw new Exception("Order photo is too large to sync");JSONObject q=new JSONObject().put("orderId",saved.optString("id")).put("name",p.optString("name","order-photo.jpg")).put("contentType",p.optString("contentType","image/jpeg")).put("sha256",sha).put("data",Base64.getEncoder().encodeToString(bytes));server=api.call("/api/v1/mobile/orders/photo","POST",q.toString());}
   JSONObject keep=clone(server);if(!localPath.isEmpty())keep.put("localPath",localPath);merged.put(keep);
  }
  saved.put("photoOrder",true).put("attachments",merged);return saved;
 }

 JSONObject sync(BridgeApi api)throws Exception {
  while(true){JSONObject target=null; synchronized(OfflineOrdersStore.class){JSONObject snapshot=state();JSONArray a=snapshot.getJSONArray("orders");for(int i=0;i<a.length();i++){JSONObject o=a.optJSONObject(i);if(o!=null&&(o.optBoolean("_localOnly")||o.optBoolean("_dirty")||o.optBoolean("_dispatchDirty"))){target=clone(o);break;}}}
   if(target==null)break;
   boolean localOnly=target.optBoolean("_localOnly"),dirty=target.optBoolean("_dirty"),dispatchDirty=target.optBoolean("_dispatchDirty");JSONObject saved=target;
   if(localOnly){JSONObject remote=api.get("/api/v1/mobile/orders");JSONObject existing=findPhoneOrder(remote,target.optString("id"));saved=existing!=null?clone(existing):api.call("/api/v1/mobile/orders","POST",serverPayload(target,false).toString());}
   else if(dirty){saved=api.call("/api/v1/mobile/orders","POST",serverPayload(target,true).toString());}
   if(target.optBoolean("photoOrder"))saved=syncPhotos(api,target,saved);
   if(dispatchDirty||"dispatched".equals(target.optString("status"))){JSONObject q=new JSONObject().put("id",saved.optString("id"));JSONObject dispatched=api.call("/api/v1/mobile/orders/dispatch","POST",q.toString());if(target.optBoolean("photoOrder")){dispatched.put("photoOrder",true).put("attachments",saved.optJSONArray("attachments"));}saved=dispatched;}
   synchronized(OfflineOrdersStore.class){JSONObject latest=state();JSONArray a=latest.getJSONArray("orders");String oldId=target.optString("id");int found=-1;for(int i=0;i<a.length();i++){JSONObject x=a.optJSONObject(i);if(x!=null&&oldId.equals(x.optString("id"))){found=i;break;}}if(found>=0){saved.remove("_localOnly");saved.remove("_dirty");saved.remove("_dispatchDirty");a.put(found,decorate(saved));write(STATE,latest);}}
  }
  JSONObject remote=api.get("/api/v1/mobile/orders");replaceWithRemote(remote);return list();
 }

 private Map<String,String> localPhotoPaths(JSONArray current){Map<String,String> out=new HashMap<>();if(current==null)return out;for(int i=0;i<current.length();i++){JSONObject o=current.optJSONObject(i);if(o==null)continue;JSONArray a=o.optJSONArray("attachments");if(a==null)continue;for(int j=0;j<a.length();j++){JSONObject p=a.optJSONObject(j);if(p==null)continue;String sha=p.optString("sha256"),path=p.optString("localPath");if(!sha.isEmpty()&&!path.isEmpty())out.put(sha,path);}}return out;}
 private JSONObject restoreLocalPaths(JSONObject o,Map<String,String> paths)throws Exception{JSONArray a=o.optJSONArray("attachments");if(a==null)return o;for(int i=0;i<a.length();i++){JSONObject p=a.optJSONObject(i);if(p==null)continue;String path=paths.get(p.optString("sha256"));if(path!=null&&!path.isEmpty()&&new File(path).exists())p.put("localPath",path);}return o;}
 void replaceWithRemote(JSONObject remote)throws Exception {synchronized(OfflineOrdersStore.class){JSONObject old=state();Map<String,String> paths=localPhotoPaths(old.optJSONArray("orders"));JSONArray all=new JSONArray(),p=remote.optJSONArray("pending"),d=remote.optJSONArray("dispatched");if(p!=null)for(int i=0;i<p.length();i++){JSONObject o=p.optJSONObject(i);if(o!=null)all.put(decorate(restoreLocalPaths(clone(o),paths)));}if(d!=null)for(int i=0;i<d.length();i++){JSONObject o=d.optJSONObject(i);if(o!=null)all.put(decorate(restoreLocalPaths(clone(o),paths)));}JSONObject s=new JSONObject().put("orders",all);write(STATE,s);}}

 private JSONObject decorate(JSONObject o)throws Exception {if(!"dispatched".equals(o.optString("status")))o.put("status","pending").put("shareText",shareText(o));else o.put("shareText","");return o;}
 private String shareText(JSONObject o){StringBuilder b=new StringBuilder("*ORDER DETAILS*\n");b.append("Firm: ").append(o.optString("company")).append('\n');b.append("Customer: ").append(o.optString("buyer")).append('\n');JSONObject bd=o.optJSONObject("buyerDetails");String phone=first(bd,"MOBILE","MOBILENO","MOBILENUMBER","PHONE","PHONENUMBER"),address=first(bd,"ADDRESS"),city=first(bd,"CITY");if(!phone.isEmpty())b.append("Mobile: ").append(phone).append('\n');if(!address.isEmpty()||!city.isEmpty())b.append("Address: ").append((address+(address.isEmpty()||city.isEmpty()?"":", ")+city).trim()).append('\n');b.append("Order No: ").append(o.optString("number","Phone order")).append('\n');String created=o.optString("createdAt");try{created=Instant.parse(created).atZone(ZoneId.systemDefault()).format(DateTimeFormatter.ofPattern("dd-MM-yyyy hh:mm a"));}catch(Exception ignored){}b.append("Order Date: ").append(created).append("\n\n");if(o.optBoolean("photoOrder")){JSONArray photos=o.optJSONArray("attachments");b.append("Photo order: ").append(photos==null?0:photos.length()).append(" image(s)").append('\n');}JSONArray items=o.optJSONArray("items");boolean allRated=items!=null&&items.length()>0;double total=0;if(items!=null)for(int i=0;i<items.length();i++){JSONObject x=items.optJSONObject(i);if(x==null)continue;double q=x.optDouble("quantity");b.append(i+1).append(". ").append(x.optString("name")).append(" — Qty: ").append(qty(q));if(!x.optString("unit").isEmpty())b.append(' ').append(x.optString("unit"));if(x.has("rate")&&!x.isNull("rate")){double r=x.optDouble("rate"),amount=q*r;total+=amount;b.append(String.format(Locale.US," — Rate: ₹%.2f — Amount: ₹%.2f",r,amount));}else allRated=false;b.append('\n');}if(allRated)b.append(String.format(Locale.US,"\nOrder Total: ₹%.2f\n",total));if(!o.optString("notes").isEmpty())b.append("\nNotes: ").append(o.optString("notes")).append('\n');b.append("Status: Pending");return b.toString();}
 private String qty(double v){return Math.rint(v)==v?String.valueOf((long)v):String.valueOf(v);}
 private String first(JSONObject d,String...keys){if(d==null)return "";for(String k:keys){String v=d.optString(k).trim();if(!v.isEmpty())return v;}return "";}

 void cacheBootstrap(JSONObject boot)throws Exception {JSONObject x=new JSONObject();x.put("companies",clone(boot.optJSONArray("companies")));x.put("savedAt",now());write(BOOT,x);}
 JSONArray companies()throws Exception {JSONObject x=read(BOOT);JSONArray a=clone(x.optJSONArray("companies"));String current=secure.get("company");boolean found=false;for(int i=0;i<a.length();i++){JSONObject c=a.optJSONObject(i);String n=c==null?a.optString(i):c.optString("name");if(n.equalsIgnoreCase(current))found=true;}if(!current.isEmpty()&&!found)a.put(new JSONObject().put("name",current));return a;}

 void cacheMasters(String company,JSONObject data)throws Exception {synchronized(OfflineOrdersStore.class){JSONObject all=read(MASTERS);all.put(company,new JSONObject().put("parties",clone(data.optJSONArray("parties"))).put("stock",clone(data.optJSONObject("stock"))).put("savedAt",now()));write(MASTERS,all);}}
 JSONObject masters(String company)throws Exception {return clone(read(MASTERS).optJSONObject(company));}
 void cacheBuyer(String company,String buyer,JSONObject details)throws Exception {synchronized(OfflineOrdersStore.class){JSONObject all=read(BUYERS),byCompany=all.optJSONObject(company);if(byCompany==null)byCompany=new JSONObject();byCompany.put(buyer,clone(details));all.put(company,byCompany);write(BUYERS,all);}}
 JSONObject buyer(String company,String buyer)throws Exception {JSONObject c=read(BUYERS).optJSONObject(company);return clone(c==null?null:c.optJSONObject(buyer));}

 int waitingCount()throws Exception {JSONObject s=state();JSONArray a=s.getJSONArray("orders");int n=0;for(int i=0;i<a.length();i++){JSONObject o=a.optJSONObject(i);if(o!=null&&(o.optBoolean("_localOnly")||o.optBoolean("_dirty")||o.optBoolean("_dispatchDirty")))n++;}return n;}
}
