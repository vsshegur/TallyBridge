package in.tallybridge.app;

import android.content.Context;
import androidx.test.ext.junit.runners.AndroidJUnit4;
import androidx.test.platform.app.InstrumentationRegistry;
import org.json.*;
import org.junit.*;
import org.junit.runner.RunWith;
import static org.junit.Assert.*;

@RunWith(AndroidJUnit4.class)
public class OrdersOfflineStoreTest {
 @Test public void localSaveEditAndDispatchStayAvailableWithoutPc() throws Exception {
  Context c=InstrumentationRegistry.getInstrumentation().getTargetContext();
  c.getSharedPreferences("identity",Context.MODE_PRIVATE).edit().clear().commit();
  CryptoStore secure=new CryptoStore(c);
  OfflineOrdersStore store=new OfflineOrdersStore(secure);
  JSONObject payload=new JSONObject().put("company","Vijayalaxmi Textiles").put("buyer","Offline Customer").put("items",new JSONArray().put(new JSONObject().put("name","Bath Towel").put("quantity",4).put("unit","PCS"))).put("notes","offline fixture");
  JSONObject created=store.saveLocal(payload,null);
  assertTrue(created.optString("id").startsWith("local-"));
  assertEquals(1,store.list().optInt("pendingCount"));
  payload.put("items",new JSONArray().put(new JSONObject().put("name","Bath Towel").put("quantity",6).put("unit","PCS")));
  JSONObject edited=store.saveLocal(payload,created);
  assertEquals(6.0,edited.optJSONArray("items").optJSONObject(0).optDouble("quantity"),0.001);
  store.dispatchLocal(edited);
  JSONObject list=store.list();
  assertEquals(0,list.optInt("pendingCount"));
  assertEquals(1,list.optInt("dispatchedCount"));
 }
}
