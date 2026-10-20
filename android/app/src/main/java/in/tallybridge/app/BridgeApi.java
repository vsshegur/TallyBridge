package in.tallybridge.app;
import org.json.JSONObject;
import java.nio.charset.StandardCharsets;
import java.time.Instant;
import java.util.Map;

final class BridgeApi {
 static final String APP_VERSION="2.10.0";
 final CryptoStore store;final AccessLogin login;
 BridgeApi(CryptoStore store){this.store=store;login=new AccessLogin(store);}
 private Map<String,String> headers(String path,String method,String body,boolean signed)throws Exception{
  String session=signed?store.secret("session"):"";
  Map<String,String> h=Http.pairs("Authorization",session.isEmpty()?"Bearer "+login.token():"TallyBridge "+session,"Accept","application/json","Content-Type","application/json");
  h.put("X-TB-Public-Key",store.publicKey());
  {String date=Instant.now().toString(),nonce=CryptoStore.random();String message=Protocol.canonical(method,path,date,nonce,body);
   h.put("X-TB-Device",store.get("device"));h.put("X-TB-Date",date);h.put("X-TB-Nonce",nonce);h.put("X-TB-Signature",store.sign(message));}
  return h;
 }
 private JSONObject rawCall(String path,String method,String body)throws Exception{return Http.json(store.get("base")+path,method,method.equals("GET")?null:body,headers(path,method,body,true));}
 JSONObject call(String path,String method,String body)throws Exception{
  try{return rawCall(path,method,body);}catch(Http.Failure first){
   if(first.status!=404||!isWorkPath(path))throw first;
   String version="";
   try{
    JSONObject status=rawCall("/api/v1/mobile/status","GET","");
    version=status.optString("version","").trim();
    if(!version.isEmpty()&&compareVersion(version,"2.4.2")>=0){
     try{return rawCall(path,method,body);}catch(Http.Failure retry){
      if(retry.status!=404)throw retry;
      throw new Exception("Windows TallyBridge "+version+" is connected, but this Work request is unavailable. Compatibility mode will be used where possible.");
     }
    }
   }catch(Http.Failure probeFailure){if(probeFailure.status!=404)throw probeFailure;}
   if(!version.isEmpty()&&compareVersion(version,"2.4.2")>=0)throw new Exception("Windows TallyBridge "+version+" is connected, but this Work request is unavailable. Compatibility mode will be used where possible.");
   throw first;
  }
 }
 JSONObject get(String path)throws Exception{JSONObject data=call(path,"GET","");if(path.equals("/api/v2/native/status")&&data.optBoolean("approved")&&!data.optString("sessionToken").isEmpty())store.putSecret("session",data.getString("sessionToken"));return data;}
 private static boolean isWorkPath(String path){
  if(path==null||!path.startsWith("/api/v1/mobile/"))return false;
  String p=path.substring("/api/v1/mobile/".length());int q=p.indexOf('?');if(q>=0)p=p.substring(0,q);
  return p.equals("workspace")||p.equals("alerts")||p.equals("followups")||p.equals("global-search")||p.equals("customer-workspace")||p.equals("tally-performance");
 }
 static int compareVersion(String a,String b){
  String[] x=(a==null?"":a).replaceAll("[^0-9.]","").split("\\.");String[] y=(b==null?"":b).replaceAll("[^0-9.]","").split("\\.");int n=Math.max(x.length,y.length);
  for(int i=0;i<n;i++){int xi=part(x,i),yi=part(y,i);if(xi!=yi)return xi<yi?-1:1;}return 0;
 }
 private static int part(String[] p,int i){if(i>=p.length||p[i].isEmpty())return 0;try{return Integer.parseInt(p[i]);}catch(Exception ignored){return 0;}}
 JSONObject identity()throws Exception{String p="/api/v2/native/identity";return Http.json(store.get("base")+p,"GET",null,headers(p,"GET","",false));}
 JSONObject pair(String code,String name)throws Exception{
  String path="/api/v2/native/pair";String body=new JSONObject().put("code",code).put("name",name).put("publicKey",store.publicKey()).toString();
  JSONObject result=Http.json(store.get("base")+path,"POST",body,headers(path,"POST",body,false));store.put("device",result.getString("deviceId"));store.put("fingerprint",result.getString("fingerprint"));store.put("email",result.getString("email"));return result;
 }
 byte[] pdf(String path)throws Exception{
  byte[] data=Http.request(store.get("base")+path,"GET",null,headers(path,"GET","",true),32*1024*1024);
  if(data.length<5||!new String(data,0,5,StandardCharsets.US_ASCII).equals("%PDF-"))throw new Exception("The bridge did not return a valid PDF");return data;
 }
 byte[] pdfPost(String path,String body)throws Exception{
  byte[] data=Http.request(store.get("base")+path,"POST",body.getBytes(StandardCharsets.UTF_8),headers(path,"POST",body,true),32*1024*1024);
  if(data.length<5||!new String(data,0,5,StandardCharsets.US_ASCII).equals("%PDF-"))throw new Exception("The bridge did not return a valid PDF");return data;
 }
 byte[] bytes(String path,int max)throws Exception{return Http.request(store.get("base")+path,"GET",null,headers(path,"GET","",true),max);}

 static String path(String endpoint,String company,String...params){StringBuilder p=new StringBuilder("/api/v1/mobile/").append(endpoint).append("?company=").append(Http.enc(company));for(int i=0;i<params.length;i+=2)p.append('&').append(Http.enc(params[i])).append('=').append(Http.enc(params[i+1]));return p.toString();}
}
