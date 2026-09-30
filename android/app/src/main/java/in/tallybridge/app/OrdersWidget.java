package in.tallybridge.app;

import android.app.*;
import android.appwidget.*;
import android.content.*;
import android.widget.RemoteViews;
import org.json.*;
import java.util.concurrent.*;

/** Home-screen shortcut for quick order capture and pending-order visibility. */
public final class OrdersWidget extends AppWidgetProvider {
 private static final String ACTION_REFRESH="in.tallybridge.app.ORDERS_WIDGET_REFRESH";
 private static final String PREFS="orders_widget";

 @Override public void onUpdate(Context context,AppWidgetManager manager,int[] ids){for(int id:ids)paintCached(context,manager,id);refreshAll(context);}
 @Override public void onReceive(Context context,Intent intent){if(ACTION_REFRESH.equals(intent.getAction())){PendingResult result=goAsync();refreshAsync(context,result);return;}super.onReceive(context,intent);}

 static void refreshAll(Context context){Intent i=new Intent(context,OrdersWidget.class).setAction(ACTION_REFRESH);context.sendBroadcast(i);}

 private static PendingIntent activity(Context c,String action,int code){Intent i=new Intent(c,OrdersActivity.class);if(action!=null)i.setAction(action);return PendingIntent.getActivity(c,code,i,PendingIntent.FLAG_UPDATE_CURRENT|PendingIntent.FLAG_IMMUTABLE);}
 private static PendingIntent refresh(Context c){Intent i=new Intent(c,OrdersWidget.class).setAction(ACTION_REFRESH);return PendingIntent.getBroadcast(c,33,i,PendingIntent.FLAG_UPDATE_CURRENT|PendingIntent.FLAG_IMMUTABLE);}

 private static void paintCached(Context c,AppWidgetManager m,int id){android.content.SharedPreferences p=c.getSharedPreferences(PREFS,Context.MODE_PRIVATE);RemoteViews v=new RemoteViews(c.getPackageName(),R.layout.orders_widget);int count=p.getInt("pending",0);String names=p.getString("names","");v.setTextViewText(R.id.widget_pending,"Pending orders: "+count);v.setTextViewText(R.id.widget_names,names.isEmpty()?(count==0?"No pending orders":"Tap to view pending orders"):names);v.setOnClickPendingIntent(R.id.widget_add,activity(c,OrdersActivity.ACTION_NEW,31));v.setOnClickPendingIntent(R.id.widget_open,activity(c,null,32));v.setOnClickPendingIntent(R.id.widget_refresh,refresh(c));v.setOnClickPendingIntent(R.id.widget_names,activity(c,null,34));m.updateAppWidget(id,v);}

 private static void refreshAsync(Context context,PendingResult pending){ExecutorService work=Executors.newSingleThreadExecutor();work.execute(()->{try{CryptoStore store=new CryptoStore(context);if(store.get("base").isEmpty()||store.get("device").isEmpty())return;BridgeApi api=new BridgeApi(store);JSONObject data=api.get("/api/v1/mobile/orders");JSONArray a=data.optJSONArray("pending");StringBuilder names=new StringBuilder();if(a!=null){for(int i=0;i<Math.min(2,a.length());i++){JSONObject o=a.optJSONObject(i);if(o==null)continue;if(names.length()>0)names.append(" • ");names.append(o.optString("buyer",o.optString("number")));}}context.getSharedPreferences(PREFS,Context.MODE_PRIVATE).edit().putInt("pending",data.optInt("pendingCount")).putString("names",names.toString()).apply();}catch(Exception ignored){}finally{try{AppWidgetManager m=AppWidgetManager.getInstance(context);int[] ids=m.getAppWidgetIds(new ComponentName(context,OrdersWidget.class));for(int id:ids)paintCached(context,m,id);}finally{work.shutdown();if(pending!=null)pending.finish();}}});}
}
