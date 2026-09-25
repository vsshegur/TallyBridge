package in.tallybridge.app;
import java.net.URI;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.util.Locale;

/** Shared request contract; kept independent of Android so it can be checked on a JVM. */
final class Protocol {
 static String baseURL(String text)throws Exception{
  URI u=new URI(text.trim());
  if(!"https".equalsIgnoreCase(u.getScheme())||u.getHost()==null||u.getUserInfo()!=null||u.getPort()!=-1||u.getRawQuery()!=null||u.getFragment()!=null||!(u.getPath().isEmpty()||u.getPath().equals("/")))throw new Exception("Enter your HTTPS subdomain, such as https://tally.example.com");
  return "https://"+u.getHost().toLowerCase(Locale.ROOT);
 }
 static String canonical(String method,String path,String date,String nonce,String body)throws Exception{
  byte[] digest=MessageDigest.getInstance("SHA-256").digest(body.getBytes(StandardCharsets.UTF_8));StringBuilder hex=new StringBuilder();for(byte b:digest)hex.append(String.format(Locale.ROOT,"%02x",b&255));
  return method+"\n"+path+"\n"+date+"\n"+nonce+"\n"+hex;
 }
}
