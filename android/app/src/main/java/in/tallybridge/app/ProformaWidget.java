package in.tallybridge.app;

import android.app.PendingIntent;
import android.appwidget.AppWidgetManager;
import android.appwidget.AppWidgetProvider;
import android.content.Context;
import android.content.Intent;
import android.widget.RemoteViews;

/** One-tap 1x1 shortcut that opens a fresh proforma invoice draft. */
public final class ProformaWidget extends AppWidgetProvider {
 @Override public void onUpdate(Context context, AppWidgetManager manager, int[] ids){
  for(int id:ids){
   RemoteViews v=new RemoteViews(context.getPackageName(),R.layout.quick_pi_widget);
   Intent i=new Intent(context,MainActivity.class).setAction(MainActivity.ACTION_NEW_PROFORMA);
   PendingIntent p=PendingIntent.getActivity(context,4101,i,PendingIntent.FLAG_UPDATE_CURRENT|PendingIntent.FLAG_IMMUTABLE);
   v.setOnClickPendingIntent(R.id.quick_widget_root,p);
   manager.updateAppWidget(id,v);
  }
 }
}
