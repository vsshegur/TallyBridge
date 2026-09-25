package in.tallybridge.app;
import java.nio.charset.StandardCharsets;
import java.security.*;
import java.security.spec.ECGenParameterSpec;

public final class ProtocolCheck {
 public static void main(String[] args)throws Exception{
  if(!Protocol.baseURL(" https://TALLY.example.com/ ").equals("https://tally.example.com"))throw new AssertionError("normalization");
  for(String input:new String[]{"http://example.com","https://x@example.com","https://example.com:443","https://example.com/phone","https://example.com?x=y","https://example.com/#fragment","javascript:alert(1)"}){
   boolean rejected=false;try{Protocol.baseURL(input);}catch(Exception e){rejected=true;}if(!rejected)throw new AssertionError("Unsafe URL accepted: "+input);
  }
  String canonical=Protocol.canonical("GET","/api/v1/mobile/auth-check","2026-09-25T01:02:03Z","0123456789abcdef0123456789abcdef","");
  if(!canonical.endsWith("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"))throw new AssertionError("body hash");
  KeyPairGenerator g=KeyPairGenerator.getInstance("EC");g.initialize(new ECGenParameterSpec("secp256r1"));KeyPair k=g.generateKeyPair();Signature s=Signature.getInstance("SHA256withECDSA");s.initSign(k.getPrivate());s.update(canonical.getBytes(StandardCharsets.UTF_8));byte[] signature=s.sign();s.initVerify(k.getPublic());s.update(canonical.getBytes(StandardCharsets.UTF_8));if(!s.verify(signature))throw new AssertionError("signature");s.initVerify(k.getPublic());s.update((canonical+"tampered").getBytes(StandardCharsets.UTF_8));if(s.verify(signature))throw new AssertionError("tampering");
  System.out.println("PASS: HTTPS validation, canonical request hashing, P-256 signatures and tamper rejection");
 }
}
