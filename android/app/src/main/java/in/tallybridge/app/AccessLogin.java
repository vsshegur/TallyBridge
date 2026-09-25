package in.tallybridge.app;

import android.app.Activity;
import android.content.Intent;
import android.net.Uri;
import org.json.*;
import java.io.*;
import java.net.*;
import java.nio.charset.StandardCharsets;
import java.util.*;

/** Cloudflare Managed OAuth public client: system browser, PKCE S256, loopback callback.
 * No WebView, Google password handling, service token or embedded client secret. */
final class AccessLogin {
 private final CryptoStore store;
 private volatile ServerSocket listener;
 private volatile int attempt=0;
 AccessLogin(CryptoStore store){this.store=store;}
 static String baseURL(String text)throws Exception{return Protocol.baseURL(text);}
 private String safeEndpoint(JSONObject meta,String name,String base)throws Exception{
  String endpoint=meta.getString(name);URI u=new URI(endpoint);String host=u.getHost();String own=new URI(base).getHost();
  if(!"https".equals(u.getScheme())||host==null||u.getUserInfo()!=null||u.getPort()!=-1||!(host.equals(own)||host.endsWith(".cloudflareaccess.com")))throw new Exception("Unexpected Cloudflare authentication endpoint");
  return endpoint;
 }
 void cancel(){attempt++;try{if(listener!=null)listener.close();}catch(Exception ignored){}}
 void login(Activity activity,String base,Runnable browserStarting)throws Exception{
  cancel();final int session=attempt;JSONObject meta=Http.json(base+"/.well-known/oauth-authorization-server","GET",null,Http.pairs("Accept","application/json"));
  String authorize=safeEndpoint(meta,"authorization_endpoint",base),token=safeEndpoint(meta,"token_endpoint",base),register=safeEndpoint(meta,"registration_endpoint",base);
  String state=CryptoStore.random(),verifier=CryptoStore.random(),challenge=CryptoStore.url64(CryptoStore.sha(verifier.getBytes(StandardCharsets.US_ASCII)));
  try(ServerSocket server=new ServerSocket(0,2,InetAddress.getByName("127.0.0.1"))){
   listener=server;server.setSoTimeout(1000);
   String redirect="http://127.0.0.1:"+server.getLocalPort()+"/oauth/callback";
   JSONObject registration=new JSONObject().put("client_name","TallyBridge Android").put("redirect_uris",new JSONArray().put(redirect)).put("grant_types",new JSONArray().put("authorization_code").put("refresh_token")).put("response_types",new JSONArray().put("code")).put("token_endpoint_auth_method","none");
   JSONObject client=Http.json(register,"POST",registration.toString(),Http.pairs("Content-Type","application/json","Accept","application/json"));
   String clientId=client.getString("client_id");
   Map<String,String> query=Http.pairs("client_id",clientId,"redirect_uri",redirect,"response_type","code","state",state,"code_challenge",challenge,"code_challenge_method","S256","resource",base);
   String scope="";JSONArray supported=meta.optJSONArray("scopes_supported");if(supported!=null){for(int i=0;i<supported.length();i++){String v=supported.optString(i);if(v.equals("openid")||v.equals("email")||v.equals("offline_access"))scope+=(scope.isEmpty()?"":" ")+v;}}if(!scope.isEmpty())query.put("scope",scope);
   if(session!=attempt)throw new IOException("Sign-in cancelled");
   String link=authorize+(authorize.contains("?")?"&":"?")+Http.form(query);
   activity.runOnUiThread(()->{if(session!=attempt)return;browserStarting.run();try{activity.startActivity(new Intent(Intent.ACTION_VIEW,Uri.parse(link)));}catch(Exception e){cancel();}});
   long deadline=System.currentTimeMillis()+180000;String code="";
   while(System.currentTimeMillis()<deadline){
    if(session!=attempt)throw new IOException("Sign-in cancelled");
    Socket socket;try{socket=server.accept();}catch(SocketTimeoutException e){continue;}
    try(Socket accepted=socket){
     accepted.setSoTimeout(3000);BufferedReader reader=new BufferedReader(new InputStreamReader(accepted.getInputStream(),StandardCharsets.US_ASCII));String line=reader.readLine();
     if(line==null||line.length()>8192||!line.startsWith("GET /oauth/callback?"))continue;
     String target=line.split(" ")[1];Uri callback=Uri.parse("http://127.0.0.1"+target);
     if(!state.equals(callback.getQueryParameter("state")))continue;
     String error=callback.getQueryParameter("error");code=callback.getQueryParameter("code");
     byte[] response="<!doctype html><meta name=viewport content='width=device-width'><h2>Sign-in received</h2><p>Return to the TallyBridge app to continue.</p>".getBytes(StandardCharsets.UTF_8);
     OutputStream out=accepted.getOutputStream();out.write(("HTTP/1.1 200 OK\r\nContent-Type: text/html; charset=utf-8\r\nCache-Control: no-store\r\nContent-Security-Policy: default-src 'none'\r\nConnection: close\r\nContent-Length: "+response.length+"\r\n\r\n").getBytes(StandardCharsets.US_ASCII));out.write(response);out.flush();
     if(error!=null)throw new Exception("Google sign-in was declined: "+error);if(code!=null&&!code.isEmpty())break;
    }
   }
   if(code==null||code.isEmpty())throw new Exception("Sign-in timed out. Return to the app and try again.");
   JSONObject result=Http.json(token,"POST",Http.form(Http.pairs("grant_type","authorization_code","client_id",clientId,"code",code,"redirect_uri",redirect,"code_verifier",verifier,"resource",base)),Http.pairs("Content-Type","application/x-www-form-urlencoded","Accept","application/json"));
   store.putSecret("refresh", "");store.put("base",base);store.put("tokenEndpoint",token);store.put("clientId",clientId);saveTokens(result);
  }finally{listener=null;}
 }
 private synchronized void saveTokens(JSONObject result)throws Exception{
  String access=result.getString("access_token");if(access.isEmpty())throw new Exception("Cloudflare did not return an access token");
  store.putSecret("access",access);if(result.has("refresh_token"))store.putSecret("refresh",result.getString("refresh_token"));
  store.put("expires",Long.toString(System.currentTimeMillis()+Math.max(1,result.optLong("expires_in",300))*1000));
 }
 synchronized String token()throws Exception{
  String access=store.secret("access");if(access.isEmpty())throw new Http.Failure(401,"Sign in with your allowed Google account");
  long expiry;try{expiry=Long.parseLong(store.get("expires"));}catch(Exception e){expiry=0;}
  if(System.currentTimeMillis()<expiry-45000)return access;
  String refresh=store.secret("refresh");if(refresh.isEmpty())throw new Http.Failure(401,"Your sign-in session expired. Sign in again; your phone approval is kept.");
  JSONObject result;try{result=Http.json(store.get("tokenEndpoint"),"POST",Http.form(Http.pairs("grant_type","refresh_token","refresh_token",refresh,"client_id",store.get("clientId"),"resource",store.get("base"))),Http.pairs("Content-Type","application/x-www-form-urlencoded","Accept","application/json"));}catch(Http.Failure e){if(e.status==400||e.status==401)throw new Http.Failure(401,"Your Google sign-in session expired. Sign in again; your phone approval is kept.");throw e;}
  saveTokens(result);return store.secret("access");
 }
}
