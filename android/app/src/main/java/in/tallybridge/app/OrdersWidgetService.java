package in.tallybridge.app;

import android.content.*;import android.content.res.Configuration;import android.widget.*;import org.json.*;import java.time.*;import java.time.format.DateTimeFormatter;import java.util.*;
/** RemoteViews collection backing the scrollable Order Desk widget. */
public final class OrdersWidgetService extends RemoteViewsService{
 @Override public RemoteViewsFactory onGetViewFactory(Intent intent){return new Factory(getApplicationContext());}
 static final class Factory implements RemoteViewsFactory{
  private final Context c; private final List<JSONObject> rows=new ArrayList<>(); private boolean dark;
  Factory(Context c){this.c=c;}
  public void onCreate(){reload();} public void onDestroy(){rows.clear();} public int getCount(){return rows.size();}
  public RemoteViews getLoadingView(){return null;} public int getViewTypeCount(){return 1;} public long getItemId(int p){return p;} public boolean hasStableIds(){return true;}
  public void onDataSetChanged(){reload();}
  private void reload(){rows.clear();try{String raw=c.getSharedPreferences("orders_widget_state",MODE_PRIVATE).getString("rows","[]");JSONArray a=new JSONArray(raw);for(int i=0;i<a.length();i++){JSONObject x=a.optJSONObject(i);if(x!=null)rows.add(x);}}catch(Exception ignored){}String mode=c.getSharedPreferences("appearance",MODE_PRIVATE).getString("mode","System");dark=mode.equals("Dark")||(mode.equals("System")&&(c.getResources().getConfiguration().uiMode&Configuration.UI_MODE_NIGHT_MASK)==Configuration.UI_MODE_NIGHT_YES);}
  private String[] when(String raw){String date="",time="";try{Instant i=Instant.parse(raw);ZonedDateTime z=i.atZone(ZoneId.systemDefault());date=z.format(DateTimeFormatter.ofPattern("dd MMM"));time=z.format(DateTimeFormatter.ofPattern("hh:mm a"));}catch(Exception ignored){try{OffsetDateTime o=OffsetDateTime.parse(raw);ZonedDateTime z=o.atZoneSameInstant(ZoneId.systemDefault());date=z.format(DateTimeFormatter.ofPattern("dd MMM"));time=z.format(DateTimeFormatter.ofPattern("hh:mm a"));}catch(Exception ignored2){}}return new String[]{date,time};}
  public RemoteViews getViewAt(int p){if(p<0||p>=rows.size())return null;JSONObject x=rows.get(p);RemoteViews v=new RemoteViews(c.getPackageName(),dark?R.layout.orders_widget_row_dark:R.layout.orders_widget_row);String[] wt=when(x.optString("createdAt"));v.setTextViewText(R.id.widget_row_party,x.optString("party","Order"));v.setTextViewText(R.id.widget_row_items,x.optString("detail"));v.setTextViewText(R.id.widget_row_number,x.optString("number"));v.setTextViewText(R.id.widget_row_date,wt[0]);v.setTextViewText(R.id.widget_row_time,wt[1]);v.setOnClickFillInIntent(R.id.widget_row_root,new Intent().putExtra("orderId",x.optString("id")));return v;}
 }
}
