package in.tallybridge.app;
import android.app.PendingIntent;import android.appwidget.*;import android.content.*;import android.widget.RemoteViews;
/** Dedicated one-tap voice order home-screen widget. */
public final class VoiceOrderWidget extends AppWidgetProvider{
 @Override public void onUpdate(Context c,AppWidgetManager m,int[] ids){for(int id:ids){RemoteViews v=new RemoteViews(c.getPackageName(),R.layout.voice_order_widget);Intent i=new Intent(c,OrdersActivity.class).setAction(OrdersActivity.ACTION_VOICE);v.setOnClickPendingIntent(R.id.voice_widget_root,PendingIntent.getActivity(c,5203,i,PendingIntent.FLAG_UPDATE_CURRENT|PendingIntent.FLAG_IMMUTABLE));m.updateAppWidget(id,v);}}
}
