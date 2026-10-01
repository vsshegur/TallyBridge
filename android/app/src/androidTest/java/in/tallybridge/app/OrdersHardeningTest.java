package in.tallybridge.app;

import android.content.*;
import androidx.test.ext.junit.runners.AndroidJUnit4;
import androidx.test.platform.app.InstrumentationRegistry;
import org.junit.*;
import org.junit.runner.RunWith;
import org.json.*;
import static org.junit.Assert.*;

/** Regression tests for offline-first reconciliation safety. */
@RunWith(AndroidJUnit4.class)
public class OrdersHardeningTest {
 private JSONObject item(double qty)throws Exception{return new JSONObject().put("name","Bath Towel").put("quantity",qty).put("unit","PCS");}
 private JSONObject payload(double qty)throws Exception{return new JSONObject().put("company","Hardening Firm").put("buyer","Race Customer").put("buyerDetails",new JSONObject()).put("items",new JSONArray().put(item(qty))).put("notes","latest phone edit");}

 @Test public void staleRemoteRefreshCannotOverwriteNewerPhoneEdit()throws Exception {
  Context c=InstrumentationRegistry.getInstrumentation().getTargetContext();
  c.getSharedPreferences("identity",Context.MODE_PRIVATE).edit().clear().commit();
  CryptoStore secure=new CryptoStore(c);
  OfflineOrdersStore local=new OfflineOrdersStore(secure);
  String created="2026-10-01T06:00:00Z";
  JSONObject server=new JSONObject().put("id","srv-race-1").put("number","ORD-RACE-1").put("company","Hardening Firm").put("buyer","Race Customer").put("buyerDetails",new JSONObject()).put("items",new JSONArray().put(item(1))).put("notes","old server value").put("status","pending").put("createdAt",created).put("updatedAt",created);
  JSONObject remote=new JSONObject().put("pending",new JSONArray().put(server)).put("dispatched",new JSONArray());
  local.replaceWithRemote(remote);
  JSONObject original=local.list().getJSONArray("pending").getJSONObject(0);
  local.saveLocal(payload(9),original);
  assertEquals(1,local.waitingCount());

  // Simulate a stale GET that began before the phone edit was saved.
  local.replaceWithRemote(remote);

  JSONObject after=local.list().getJSONArray("pending").getJSONObject(0);
  assertEquals(9.0,after.getJSONArray("items").getJSONObject(0).getDouble("quantity"),0.001);
  assertEquals("latest phone edit",after.getString("notes"));
  assertEquals(1,local.waitingCount());
 }
}
