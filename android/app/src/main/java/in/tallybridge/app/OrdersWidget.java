package in.tallybridge.app;

import android.app.*;
import android.appwidget.*;
import android.content.*;
import android.content.res.Configuration;
import android.os.Bundle;
import android.util.TypedValue;
import android.view.View;
import android.widget.RemoteViews;
import org.json.*;
import java.util.concurrent.*;

/** Responsive home-screen shortcut for fast order capture and pending visibility, including offline mode. */
public final class OrdersWidget extends AppWidgetProvider {
 private static final String ACTION_REFRESH="in.tallybridge.app.ORDERS_WIDGET_REFRESH";
 private static final String PREFS="orders_widget";

 @Override public void onUpdate(Context context,AppWidgetManager manager,int[] ids){for(int id:ids)paintCached(context,manager,id,manager.getAppWidgetOptions(id));refreshAll(context);}
 @Override public void onAppWidgetOptionsChanged(Context context,AppWidgetManager manager,int id,Bundle options){paintCached(context,manager,id,options);}
 @Override public void onReceive(Context context,Intent intent){if(ACTION_REFRESH.equals(intent.getAction())){PendingResult result=goAsync();refreshAsync(context,result);return;}super.onReceive(context,intent);}
 static void refreshAll(Context context){Intent i=new Intent(context,OrdersWidget.class).setAction(ACTION_REFRESH);context.sendBroadcast(i);}

 private static boolean dark(Context c){String mode=c.getSharedPreferences("appearance",Context.MODE_PRIVATE).getString("mode","System");return "Dark".equals(mode)||("System".equals(mode)&&(c.getResources().getConfiguration().uiMode&Configuration.UI_MODE_NIGHT_MASK)==Configuration.UI_MODE_NIGHT_YES);}
 static int previewLayout(Context c){return dark(c)?R.layout.orders_widget_dark:R.layout.orders_widget;}
 private static PendingIntent activity(Context c,String action,int code){Intent i=new Intent(c,OrdersActivity.class);if(action!=null)i.setAction(action);return PendingIntent.getActivity(c,code,i,PendingIntent.FLAG_UPDATE_CURRENT|PendingIntent.FLAG_IMMUTABLE);}
 private static PendingIntent refresh(Context c){Intent i=new Intent(c,OrdersWidget.class).setAction(ACTION_REFRESH);return PendingIntent.getBroadcast(c,33,i,PendingIntent.FLAG_UPDATE_CURRENT|PendingIntent.FLAG_IMMUTABLE);}
 private static int option(Bundle b,String key,int fallback){if(b==null)return fallback;int v=b.getInt(key,0);return v>0?v:fallback;}

 private static RemoteViews buildViews(Context c,int widthDp,int heightDp){
  android.content.SharedPreferences p=c.getSharedPreferences(PREFS,Context.MODE_PRIVATE);
  RemoteViews v=new RemoteViews(c.getPackageName(),previewLayout(c));
  int count=p.getInt("pending",0);String names=p.getString("names","");boolean offline=p.getBoolean("offline",false);
  boolean compact=heightDp<165,narrow=widthDp<250,tall=heightDp>=225;
  v.setTextViewText(R.id.widget_pending,count+" pending order"+(count==1?"":"s"));
  v.setTextViewText(R.id.widget_mode,offline?"OFFLINE • PHONE COPY":"SYNCED WITH PC");
  v.setTextViewText(R.id.widget_names,names.isEmpty()?(count==0?"No pending orders. Add one when a customer calls.":"Tap Pending to review orders."):names);
  v.setViewVisibility(R.id.widget_header,compact?View.GONE:View.VISIBLE);
  v.setViewVisibility(R.id.widget_mode,compact?View.GONE:View.VISIBLE);
  v.setViewVisibility(R.id.widget_names,compact?View.GONE:View.VISIBLE);
  v.setTextViewText(R.id.widget_add,narrow?"+ Add":"+ Add order");
  v.setTextViewText(R.id.widget_open,narrow?"Pending":"Open pending");
  v.setTextViewTextSize(R.id.widget_pending,TypedValue.COMPLEX_UNIT_SP,compact?19:21);
  v.setTextViewTextSize(R.id.widget_names,TypedValue.COMPLEX_UNIT_SP,14);
  v.setTextViewTextSize(R.id.widget_add,TypedValue.COMPLEX_UNIT_SP,13);
  v.setTextViewTextSize(R.id.widget_open,TypedValue.COMPLEX_UNIT_SP,13);
  v.setInt(R.id.widget_names,"setMaxLines",tall?3:1);
  v.setOnClickPendingIntent(R.id.widget_add,activity(c,OrdersActivity.ACTION_NEW,31));
  v.setOnClickPendingIntent(R.id.widget_open,activity(c,null,32));
  v.setOnClickPendingIntent(R.id.widget_refresh,refresh(c));
  v.setOnClickPendingIntent(R.id.widget_names,activity(c,null,34));
  v.setOnClickPendingIntent(R.id.widget_pending,activity(c,null,35));
  return v;
 }
 static RemoteViews previewForSize(Context c,int widthDp,int heightDp){return buildViews(c,widthDp,heightDp);}
 private static void paintCached(Context c,AppWidgetManager m,int id,Bundle options){
  int width=option(options,AppWidgetManager.OPTION_APPWIDGET_MIN_WIDTH,250);
  int height=option(options,AppWidgetManager.OPTION_APPWIDGET_MIN_HEIGHT,180);
  m.updateAppWidget(id,buildViews(c,width,height));
 }

 private static void cache(Context context,JSONObject data,boolean offline){JSONArray a=data.optJSONArray("pending");StringBuilder names=new StringBuilder();if(a!=null){for(int i=0;i<Math.min(3,a.length());i++){JSONObject o=a.optJSONObject(i);if(o==null)continue;if(names.length()>0)names.append('\n');names.append(o.optString("buyer",o.optString("number")));}}context.getSharedPreferences(PREFS,Context.MODE_PRIVATE).edit().putInt("pending",data.optInt("pendingCount")).putString("names",names.toString()).putBoolean("offline",offline).apply();}
 private static void refreshAsync(Context context,PendingResult pending){ExecutorService work=Executors.newSingleThreadExecutor();work.execute(()->{boolean offlineMode=true;try{CryptoStore secure=new CryptoStore(context);OfflineOrdersStore local=new OfflineOrdersStore(secure);JSONObject saved=local.list();cache(context,saved,true);if(!secure.get("base").isEmpty()&&!secure.get("device").isEmpty()){try{JSONObject synced=local.sync(new BridgeApi(secure));cache(context,synced,false);offlineMode=false;}catch(Exception ignored){offlineMode=true;}}}catch(Exception ignored){}finally{try{if(offlineMode){try{CryptoStore secure=new CryptoStore(context);cache(context,new OfflineOrdersStore(secure).list(),true);}catch(Exception ignored){}}AppWidgetManager m=AppWidgetManager.getInstance(context);int[] ids=m.getAppWidgetIds(new ComponentName(context,OrdersWidget.class));for(int id:ids)paintCached(context,m,id,m.getAppWidgetOptions(id));}finally{work.shutdown();if(pending!=null)pending.finish();}}});}
}
