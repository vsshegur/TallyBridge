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

/** Home-screen shortcut for quick order capture and pending-order visibility, even while the PC is offline. */
public final class OrdersWidget extends AppWidgetProvider {
 private static final String ACTION_REFRESH="in.tallybridge.app.ORDERS_WIDGET_REFRESH";
 private static final String PREFS="orders_widget";
 private static final int COMPACT_HEIGHT_DP=190;
 private static final int TALL_HEIGHT_DP=260;
 private static final int NARROW_WIDTH_DP=300;

 @Override public void onUpdate(Context context,AppWidgetManager manager,int[] ids){for(int id:ids)paintCached(context,manager,id);refreshAll(context);}
 @Override public void onAppWidgetOptionsChanged(Context context,AppWidgetManager manager,int id,Bundle options){paintCached(context,manager,id);}
 @Override public void onReceive(Context context,Intent intent){if(ACTION_REFRESH.equals(intent.getAction())){PendingResult result=goAsync();refreshAsync(context,result);return;}super.onReceive(context,intent);}
 static void refreshAll(Context context){Intent i=new Intent(context,OrdersWidget.class).setAction(ACTION_REFRESH);context.sendBroadcast(i);}

 private static boolean dark(Context c){String mode=c.getSharedPreferences("appearance",Context.MODE_PRIVATE).getString("mode","System");return "Dark".equals(mode)||("System".equals(mode)&&(c.getResources().getConfiguration().uiMode&Configuration.UI_MODE_NIGHT_MASK)==Configuration.UI_MODE_NIGHT_YES);}
 static int previewLayout(Context c){return dark(c)?R.layout.orders_widget_dark:R.layout.orders_widget;}
 static int layoutFor(Context c,int heightDp){boolean compact=heightDp>0&&heightDp<COMPACT_HEIGHT_DP;if(compact)return dark(c)?R.layout.orders_widget_compact_dark:R.layout.orders_widget_compact;return previewLayout(c);}
 private static PendingIntent activity(Context c,String action,int code){Intent i=new Intent(c,OrdersActivity.class);if(action!=null)i.setAction(action);return PendingIntent.getActivity(c,code,i,PendingIntent.FLAG_UPDATE_CURRENT|PendingIntent.FLAG_IMMUTABLE);}
 private static PendingIntent refresh(Context c){Intent i=new Intent(c,OrdersWidget.class).setAction(ACTION_REFRESH);return PendingIntent.getBroadcast(c,33,i,PendingIntent.FLAG_UPDATE_CURRENT|PendingIntent.FLAG_IMMUTABLE);}
 private static PendingIntent gsm(Context c){Intent i=new Intent(c,GsmCalculatorActivity.class);return PendingIntent.getActivity(c,38,i,PendingIntent.FLAG_UPDATE_CURRENT|PendingIntent.FLAG_IMMUTABLE);}
 private static void size(RemoteViews v,int id,float sp){v.setTextViewTextSize(id,TypedValue.COMPLEX_UNIT_SP,sp);}

 private static void paintCached(Context c,AppWidgetManager m,int id){
  android.content.SharedPreferences p=c.getSharedPreferences(PREFS,Context.MODE_PRIVATE);
  Bundle options=m.getAppWidgetOptions(id);
  int height=options==null?0:options.getInt(AppWidgetManager.OPTION_APPWIDGET_MIN_HEIGHT,0);
  int width=options==null?0:options.getInt(AppWidgetManager.OPTION_APPWIDGET_MIN_WIDTH,0);
  boolean compact=height>0&&height<COMPACT_HEIGHT_DP;
  boolean tall=height==0||height>=TALL_HEIGHT_DP;
  boolean narrow=width>0&&width<NARROW_WIDTH_DP;
  RemoteViews v=new RemoteViews(c.getPackageName(),layoutFor(c,height));

  int count=p.getInt("pending",0);String p1=p.getString("party1","");String p2=p.getString("party2","");String p3=p.getString("party3","");int more=p.getInt("moreParties",0);boolean offline=p.getBoolean("offline",false);
  String first=p1.isEmpty()?(count==0?"No pending orders":"Open pending orders"):p1;
  v.setTextViewText(R.id.widget_pending,String.valueOf(count));
  v.setTextViewText(R.id.widget_mode,offline?"PHONE OFFLINE":"PC SYNCED");
  v.setTextViewText(R.id.widget_party1,first);
  v.setTextViewText(R.id.widget_party2,p2);
  v.setTextViewText(R.id.widget_party3,p3);
  v.setTextViewText(R.id.widget_more,more>0?"+"+more+" more customer"+(more==1?"":"s"):"");
  v.setTextViewText(R.id.widget_add,narrow?"+ Order":"+ New order");
  v.setTextViewText(R.id.widget_open,narrow?"Pending":"View pending");

  size(v,R.id.widget_pending,compact?28f:31f);
  size(v,R.id.widget_party1,compact?14f:14f);
  size(v,R.id.widget_party2,14f);
  size(v,R.id.widget_party3,14f);
  size(v,R.id.widget_add,narrow?12f:13f);
  size(v,R.id.widget_open,narrow?11f:12f);
  size(v,R.id.widget_gsm,narrow?11f:12f);

  v.setViewVisibility(R.id.widget_mode,!compact&&!narrow?View.VISIBLE:View.GONE);
  v.setViewVisibility(R.id.widget_party_label,compact?View.GONE:View.VISIBLE);
  v.setViewVisibility(R.id.widget_party2,!compact&&!p2.isEmpty()?View.VISIBLE:View.GONE);
  v.setViewVisibility(R.id.widget_party3,!compact&&tall&&!p3.isEmpty()?View.VISIBLE:View.GONE);
  v.setViewVisibility(R.id.widget_more,!compact&&more>0?View.VISIBLE:View.GONE);

  v.setOnClickPendingIntent(R.id.widget_add,activity(c,OrdersActivity.ACTION_NEW,31));
  v.setOnClickPendingIntent(R.id.widget_open,activity(c,null,32));
  v.setOnClickPendingIntent(R.id.widget_gsm,gsm(c));
  v.setOnClickPendingIntent(R.id.widget_refresh,refresh(c));
  v.setOnClickPendingIntent(R.id.widget_party1,activity(c,null,34));
  v.setOnClickPendingIntent(R.id.widget_party2,activity(c,null,36));
  v.setOnClickPendingIntent(R.id.widget_party3,activity(c,null,37));
  v.setOnClickPendingIntent(R.id.widget_pending,activity(c,null,35));
  m.updateAppWidget(id,v);
 }

 private static void cache(Context context,JSONObject data,boolean offline){JSONArray a=data.optJSONArray("pending");java.util.LinkedHashSet<String> parties=new java.util.LinkedHashSet<>();if(a!=null){for(int i=0;i<a.length();i++){JSONObject o=a.optJSONObject(i);if(o==null)continue;String name=o.optString("buyer",o.optString("number")).trim();if(!name.isEmpty())parties.add(name);}}java.util.ArrayList<String> list=new java.util.ArrayList<>(parties);String p1=list.size()>0?list.get(0):"",p2=list.size()>1?list.get(1):"",p3=list.size()>2?list.get(2):"";int more=Math.max(0,list.size()-3);context.getSharedPreferences(PREFS,Context.MODE_PRIVATE).edit().putInt("pending",data.optInt("pendingCount")).putString("party1",p1).putString("party2",p2).putString("party3",p3).putInt("moreParties",more).putBoolean("offline",offline).apply();}
 private static void refreshAsync(Context context,PendingResult pending){ExecutorService work=Executors.newSingleThreadExecutor();work.execute(()->{boolean offlineMode=true;try{CryptoStore secure=new CryptoStore(context);OfflineOrdersStore local=new OfflineOrdersStore(secure);JSONObject saved=local.list();cache(context,saved,true);if(!secure.get("base").isEmpty()&&!secure.get("device").isEmpty()){try{JSONObject synced=local.sync(new BridgeApi(secure));cache(context,synced,false);offlineMode=false;}catch(Exception ignored){offlineMode=true;}}}catch(Exception ignored){}finally{try{if(offlineMode){try{CryptoStore secure=new CryptoStore(context);cache(context,new OfflineOrdersStore(secure).list(),true);}catch(Exception ignored){}}AppWidgetManager m=AppWidgetManager.getInstance(context);int[] ids=m.getAppWidgetIds(new ComponentName(context,OrdersWidget.class));for(int id:ids)paintCached(context,m,id);}finally{work.shutdown();if(pending!=null)pending.finish();}}});}
}
