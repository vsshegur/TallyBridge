package in.tallybridge.app;

import android.app.PendingIntent;
import android.appwidget.AppWidgetManager;
import android.appwidget.AppWidgetProvider;
import android.content.Context;
import android.content.Intent;
import android.widget.RemoteViews;

/** One-tap 1x1 shortcut: opens the camera for a supplier (purchase) bill. */
public final class BillScanWidget extends AppWidgetProvider {
 @Override public void onUpdate(Context c,AppWidgetManager m,int[] ids){
  for(int id:ids){
   RemoteViews v=new RemoteViews(c.getPackageName(),R.layout.bill_scan_widget);
   Intent i=new Intent(c,BillScanActivity.class).setAction(BillScanActivity.ACTION_CAMERA);
   v.setOnClickPendingIntent(R.id.bill_widget_root,PendingIntent.getActivity(c,5204,i,PendingIntent.FLAG_UPDATE_CURRENT|PendingIntent.FLAG_IMMUTABLE));
   m.updateAppWidget(id,v);
  }
 }
}
