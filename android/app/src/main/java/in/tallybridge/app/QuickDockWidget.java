package in.tallybridge.app;

import android.app.PendingIntent;
import android.appwidget.AppWidgetManager;
import android.appwidget.AppWidgetProvider;
import android.content.Context;
import android.content.Intent;
import android.os.Bundle;
import android.widget.RemoteViews;

/**
 * Quick actions bar: New order, Voice order, Scan bill, Search, Proforma and GSM.
 * When the widget is narrow it keeps the four most used actions so every button stays easy to tap.
 */
public final class QuickDockWidget extends AppWidgetProvider {
 /** Below this width (dp) the six-button bar gets cramped, so the four-button version is used. */
 static final int FULL_MIN_WIDTH_DP=320;

 private static PendingIntent open(Context c,int request,Intent i){return PendingIntent.getActivity(c,request,i,PendingIntent.FLAG_UPDATE_CURRENT|PendingIntent.FLAG_IMMUTABLE);}

 static boolean full(Bundle options){return options==null||options.getInt(AppWidgetManager.OPTION_APPWIDGET_MIN_WIDTH,FULL_MIN_WIDTH_DP)>=FULL_MIN_WIDTH_DP;}

 private static void paint(Context c,AppWidgetManager m,int id){
  boolean full=full(m.getAppWidgetOptions(id));
  RemoteViews v=new RemoteViews(c.getPackageName(),full?R.layout.quick_dock_widget:R.layout.quick_dock_widget_small);
  v.setOnClickPendingIntent(R.id.quick_dock_brand,open(c,7100+id,new Intent(c,MainActivity.class).setAction(MainActivity.ACTION_HOME)));
  v.setOnClickPendingIntent(R.id.quick_dock_order,open(c,7200+id,new Intent(c,OrdersActivity.class).setAction(OrdersActivity.ACTION_NEW)));
  v.setOnClickPendingIntent(R.id.quick_dock_voice,open(c,7250+id,new Intent(c,OrdersActivity.class).setAction(OrdersActivity.ACTION_VOICE)));
  v.setOnClickPendingIntent(R.id.quick_dock_bill,open(c,7275+id,new Intent(c,BillScanActivity.class).setAction(BillScanActivity.ACTION_CAMERA)));
  v.setOnClickPendingIntent(R.id.quick_dock_search,open(c,7290+id,new Intent(c,UniversalSearchActivity.class)));
  if(full){
   v.setOnClickPendingIntent(R.id.quick_dock_proforma,open(c,7400+id,new Intent(c,MainActivity.class).setAction(MainActivity.ACTION_NEW_PROFORMA)));
   v.setOnClickPendingIntent(R.id.quick_dock_gsm,open(c,7300+id,new Intent(c,GsmCalculatorActivity.class)));
  }
  m.updateAppWidget(id,v);
 }

 @Override public void onUpdate(Context c,AppWidgetManager m,int[] ids){for(int id:ids)paint(c,m,id);}
 @Override public void onAppWidgetOptionsChanged(Context c,AppWidgetManager m,int id,Bundle options){paint(c,m,id);}
}
