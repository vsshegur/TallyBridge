package in.tallybridge.app;

import android.content.Context;
import android.content.SharedPreferences;
import android.security.keystore.KeyGenParameterSpec;
import android.security.keystore.KeyProperties;
import android.util.Base64;
import java.nio.charset.StandardCharsets;
import java.security.*;
import java.security.spec.ECGenParameterSpec;
import javax.crypto.*;
import javax.crypto.spec.GCMParameterSpec;

/** Private signing key and token-encryption key never leave Android Keystore. */
final class CryptoStore {
    private static final String SIGN="tb_phone_p256_v2", SECRET="tb_tokens_aes_v2";
    private final SharedPreferences prefs;
    private final KeyStore keys;
    CryptoStore(Context context) throws Exception {
        prefs=context.getSharedPreferences("identity",Context.MODE_PRIVATE);
        keys=KeyStore.getInstance("AndroidKeyStore"); keys.load(null);
    }
    synchronized void ensureKeys() throws Exception {
        if (!keys.containsAlias(SIGN)) {
            KeyPairGenerator generator=KeyPairGenerator.getInstance(KeyProperties.KEY_ALGORITHM_EC,"AndroidKeyStore");
            generator.initialize(new KeyGenParameterSpec.Builder(SIGN,KeyProperties.PURPOSE_SIGN|KeyProperties.PURPOSE_VERIFY)
                .setAlgorithmParameterSpec(new ECGenParameterSpec("secp256r1"))
                .setDigests(KeyProperties.DIGEST_SHA256).build());
            generator.generateKeyPair();
        }
        if (!keys.containsAlias(SECRET)) {
            KeyGenerator generator=KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES,"AndroidKeyStore");
            generator.init(new KeyGenParameterSpec.Builder(SECRET,KeyProperties.PURPOSE_ENCRYPT|KeyProperties.PURPOSE_DECRYPT)
                .setBlockModes(KeyProperties.BLOCK_MODE_GCM).setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE).build());
            generator.generateKey();
        }
    }
    synchronized String publicKey() throws Exception { ensureKeys(); return b64(keys.getCertificate(SIGN).getPublicKey().getEncoded()); }
    synchronized String sign(String text) throws Exception {
        ensureKeys(); Signature s=Signature.getInstance("SHA256withECDSA");
        s.initSign((PrivateKey)keys.getKey(SIGN,null));s.update(text.getBytes(StandardCharsets.UTF_8));return b64(s.sign());
    }
    synchronized void putSecret(String name,String value) throws Exception {
        ensureKeys();Cipher c=Cipher.getInstance("AES/GCM/NoPadding");c.init(Cipher.ENCRYPT_MODE,keys.getKey(SECRET,null));
        String encrypted=b64(c.getIV())+":"+b64(c.doFinal(value.getBytes(StandardCharsets.UTF_8)));
        if(!prefs.edit().putString(name,encrypted).commit())throw new Exception("Could not save sign-in on this phone");
    }
    synchronized String secret(String name) throws Exception {
        String saved=prefs.getString(name,"");if(saved.isEmpty())return "";
        String[] parts=saved.split(":",2);if(parts.length!=2)throw new Exception("Saved sign-in is damaged. Reset this app's pairing.");
        Cipher c=Cipher.getInstance("AES/GCM/NoPadding");c.init(Cipher.DECRYPT_MODE,keys.getKey(SECRET,null),new GCMParameterSpec(128,Base64.decode(parts[0],Base64.NO_WRAP)));
        return new String(c.doFinal(Base64.decode(parts[1],Base64.NO_WRAP)),StandardCharsets.UTF_8);
    }
    String get(String key) {return prefs.getString(key,"");}
    void put(String key,String value) {prefs.edit().putString(key,value).apply();}
    synchronized void reset() throws Exception {prefs.edit().clear().commit();keys.deleteEntry(SIGN);keys.deleteEntry(SECRET);}
    static String b64(byte[] bytes){return Base64.encodeToString(bytes,Base64.NO_WRAP);}
    static String url64(byte[] bytes){return Base64.encodeToString(bytes,Base64.NO_WRAP|Base64.URL_SAFE|Base64.NO_PADDING);}
    static String random(){byte[] b=new byte[32];new SecureRandom().nextBytes(b);return url64(b);}
    static byte[] sha(byte[] b)throws Exception{return MessageDigest.getInstance("SHA-256").digest(b);}
    static String hex(byte[] b){StringBuilder s=new StringBuilder();for(byte v:b)s.append(String.format(java.util.Locale.ROOT,"%02x",v&255));return s.toString();}
}
