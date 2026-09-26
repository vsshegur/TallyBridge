package in.tallybridge.app;

import android.app.Activity;
import android.os.CancellationSignal;
import android.util.Base64;
import androidx.credentials.CredentialManager;
import androidx.credentials.CredentialManagerCallback;
import androidx.credentials.CustomCredential;
import androidx.credentials.GetCredentialRequest;
import androidx.credentials.GetCredentialResponse;
import androidx.credentials.exceptions.GetCredentialException;
import com.google.android.libraries.identity.googleid.GetSignInWithGoogleOption;
import com.google.android.libraries.identity.googleid.GoogleIdTokenCredential;
import org.json.JSONObject;
import java.nio.charset.StandardCharsets;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.TimeUnit;

/** Native Google account selection. No Cloudflare Access or client secret. */
final class AccessLogin {
 private final CryptoStore store;
 private volatile CancellationSignal signal;
 private volatile CompletableFuture<String> pending;
 private volatile int attempt;
 AccessLogin(CryptoStore store){this.store=store;}
 static String baseURL(String text)throws Exception{return Protocol.baseURL(text);}
 synchronized void cancel(){attempt++;if(signal!=null)signal.cancel();if(pending!=null)pending.cancel(false);}
 void login(Activity activity,String base,Runnable starting)throws Exception{
  cancel();final int session=attempt;
  JSONObject config=Http.json(base+"/api/v2/native/config","GET",null,Http.pairs("Accept","application/json"));
  String client=config.getString("googleClientId");
  if(!base.equals(config.getString("server"))||!client.matches("[0-9]+-[a-zA-Z0-9_-]+\\.apps\\.googleusercontent\\.com"))throw new Exception("PC address or Google client ID does not match. Check desktop setup.");
  String binding=CryptoStore.hex(CryptoStore.sha(Base64.decode(store.publicKey(),Base64.NO_WRAP)));
  GetSignInWithGoogleOption option=new GetSignInWithGoogleOption.Builder(client).setNonce(binding).build();
  GetCredentialRequest request=new GetCredentialRequest.Builder().addCredentialOption(option).build();
  CompletableFuture<String> result=new CompletableFuture<>();CancellationSignal cancellation=new CancellationSignal();
  synchronized(this){if(session!=attempt)throw new Exception("Sign-in cancelled");pending=result;signal=cancellation;}
  activity.runOnUiThread(()->{
   if(session!=attempt)return;starting.run();
   try{CredentialManager.create(activity).getCredentialAsync(activity,request,cancellation,activity.getMainExecutor(),new CredentialManagerCallback<GetCredentialResponse,GetCredentialException>(){
    @Override public void onResult(GetCredentialResponse response){try{
     if(!(response.getCredential() instanceof CustomCredential))throw new Exception("Google credential was not returned");
     CustomCredential c=(CustomCredential)response.getCredential();
     if(!GoogleIdTokenCredential.TYPE_GOOGLE_ID_TOKEN_CREDENTIAL.equals(c.getType()))throw new Exception("Unexpected sign-in credential");
     result.complete(GoogleIdTokenCredential.createFrom(c.getData()).getIdToken());
    }catch(Exception e){result.completeExceptionally(e);}}
    @Override public void onError(GetCredentialException e){result.completeExceptionally(new Exception("Google sign-in could not finish. Check your Android OAuth client, APK SHA-1 and test-user email. "+e.getType()));}
   });}catch(Exception e){result.completeExceptionally(e);}
  });
  try{
   String token=result.get(120,TimeUnit.SECONDS);
   String[] parts=token.split("\\.");if(parts.length!=3)throw new Exception("Google returned an invalid ID token");
   JSONObject claims=new JSONObject(new String(Base64.decode(parts[1],Base64.URL_SAFE|Base64.NO_WRAP),StandardCharsets.UTF_8));
   long expiry=claims.getLong("exp")*1000;
   // Parsing here only controls refresh UX. The Windows verifier validates all claims and signature.
   synchronized(this){if(session!=attempt)throw new Exception("Sign-in cancelled");store.putSecret("access",token);store.putSecret("refresh","");store.put("base",base);store.put("expires",Long.toString(expiry));}
  }finally{cancellation.cancel();synchronized(this){if(session==attempt){pending=null;signal=null;}}}
 }
 synchronized String token()throws Exception{
  String token=store.secret("access");long expires;try{expires=Long.parseLong(store.get("expires"));}catch(Exception e){expires=0;}
  if(token.isEmpty()||System.currentTimeMillis()>=expires-30000)throw new Http.Failure(401,"Google sign-in expired. Tap Sign in again; your PC phone approval is kept.");
  return token;
 }
}
