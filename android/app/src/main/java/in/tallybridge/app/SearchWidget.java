package in.tallybridge.app;
import android.app.PendingIntent;import android.appwidget.*;import android.content.*;import android.widget.RemoteViews;
/** Dedicated Universal Search home-screen widget. */
public final class SearchWidget extends AppWidgetProvider{
 @Override public void onUpdate(Context c,AppWidgetManager m,int[] ids){for(int id:ids){RemoteViews v=new RemoteViews(c.getPackageName(),R.layout.search_widget);Intent i=new Intent(c,UniversalSearchActivity.class);v.setOnClickPendingIntent(R.id.search_widget_root,PendingIntent.getActivity(c,5202,i,PendingIntent.FLAG_UPDATE_CURRENT|PendingIntent.FLAG_IMMUTABLE));m.updateAppWidget(id,v);}}
}
