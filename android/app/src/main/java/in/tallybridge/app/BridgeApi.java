package in.tallybridge.app;
import org.json.JSONObject;
import java.nio.charset.StandardCharsets;
import java.time.Instant;
import java.util.Map;

final class BridgeApi {
 final CryptoStore store;final AccessLogin login;
 BridgeApi(CryptoStore store){this.store=store;login=new AccessLogin(store);}
 private Map<String,String> headers(String path,String method,String body,boolean signed)throws Exception{
  Map<String,String> h=Http.pairs("Authorization","Bearer "+login.token(),"Accept","application/json","Content-Type","application/json");
  if(signed){String date=Instant.now().toString(),nonce=CryptoStore.random();String message=Protocol.canonical(method,path,date,nonce,body);
   h.put("X-TB-Device",store.get("device"));h.put("X-TB-Date",date);h.put("X-TB-Nonce",nonce);h.put("X-TB-Signature",store.sign(message));}
  return h;
 }
 JSONObject call(String path,String method,String body)throws Exception{return Http.json(store.get("base")+path,method,method.equals("GET")?null:body,headers(path,method,body,true));}
 JSONObject get(String path)throws Exception{return call(path,"GET","");}
 JSONObject identity()throws Exception{String p="/api/v2/native/identity";return Http.json(store.get("base")+p,"GET",null,headers(p,"GET","",false));}
 JSONObject pair(String code,String name)throws Exception{
  String path="/api/v2/native/pair";String body=new JSONObject().put("code",code).put("name",name).put("publicKey",store.publicKey()).toString();
  JSONObject result=Http.json(store.get("base")+path,"POST",body,headers(path,"POST",body,false));store.put("device",result.getString("deviceId"));store.put("fingerprint",result.getString("fingerprint"));store.put("email",result.getString("email"));return result;
 }
 byte[] pdf(String path)throws Exception{
  byte[] data=Http.request(store.get("base")+path,"GET",null,headers(path,"GET","",true),32*1024*1024);
  if(data.length<5||!new String(data,0,5,StandardCharsets.US_ASCII).equals("%PDF-"))throw new Exception("The bridge did not return a valid PDF");return data;
 }
 static String path(String endpoint,String company,String...params){StringBuilder p=new StringBuilder("/api/v1/mobile/").append(endpoint).append("?company=").append(Http.enc(company));for(int i=0;i<params.length;i+=2)p.append('&').append(Http.enc(params[i])).append('=').append(Http.enc(params[i+1]));return p.toString();}
}
