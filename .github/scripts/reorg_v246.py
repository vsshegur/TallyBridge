from pathlib import Path
import re


def read(path):
    return Path(path).read_text(encoding="utf-8")


def write(path, content):
    Path(path).write_text(content, encoding="utf-8")


def replace1(content, old, new, label):
    if old not in content:
        raise SystemExit("missing replacement: " + label)
    return content.replace(old, new, 1)


# Keep Android and Windows on the same release number.
p = "android/app/build.gradle"
s = read(p)
s = replace1(s, "versionCode 20405", "versionCode 20406", "android version code")
s = replace1(s, "versionName '2.4.5'", "versionName '2.4.6'", "android version name")
write(p, s)

p = "android/app/src/main/java/in/tallybridge/app/BridgeApi.java"
s = read(p)
s = replace1(s, 'static final String APP_VERSION="2.4.5";', 'static final String APP_VERSION="2.4.6";', "BridgeApi version")
write(p, s)

p = "internal/app/types.go"
s = read(p)
s, count = re.subn(r'const Version = "[^"]+"', 'const Version = "2.4.6"', s, count=1)
if count != 1:
    raise SystemExit("missing Windows Version")
write(p, s)


# Work and Orders keep their specialized logic but share the same visible navigation shell.
section_nav = '''package in.tallybridge.app;

import android.app.Activity;
import android.content.Intent;
import android.graphics.Typeface;
import android.view.Gravity;
import android.widget.LinearLayout;
import android.widget.TextView;

final class SectionNav {
 private SectionNav(){}
 static LinearLayout bar(Activity a,String selected,int surface,int tint,int muted,int teal){
  LinearLayout nav=new LinearLayout(a);
  nav.setPadding(dp(a,8),dp(a,6),dp(a,8),dp(a,7));
  nav.setBackgroundColor(surface);
  for(String label:new String[]{"Home","Work","Orders","Reports","More"}){
   boolean on=label.equals(selected);
   LinearLayout item=new LinearLayout(a);item.setOrientation(LinearLayout.VERTICAL);item.setGravity(Gravity.CENTER);
   item.setPadding(dp(a,4),dp(a,3),dp(a,4),dp(a,3));
   item.setBackground(round(a,on?tint:surface,15));
   item.setFocusable(true);item.setContentDescription(label+(on?", selected":""));
   item.addView(new UiIcon(a,label,on?teal:muted),new LinearLayout.LayoutParams(dp(a,22),dp(a,22)));
   TextView t=new TextView(a);t.setText(label);t.setTextSize(11);t.setGravity(Gravity.CENTER);t.setTextColor(on?teal:muted);
   if(on)t.setTypeface(null,Typeface.BOLD);
   item.addView(t);
   if(!on)item.setOnClickListener(v->go(a,label));
   LinearLayout.LayoutParams lp=new LinearLayout.LayoutParams(0,dp(a,57),1);lp.setMargins(dp(a,2),0,dp(a,2),0);nav.addView(item,lp);
  }
  return nav;
 }
 static void go(Activity a,String label){
  if("Work".equals(label)){
   Intent i=new Intent(a,WorkActivity.class).addFlags(Intent.FLAG_ACTIVITY_CLEAR_TOP|Intent.FLAG_ACTIVITY_SINGLE_TOP);
   a.startActivity(i);if(a instanceof OrdersActivity)a.finish();a.overridePendingTransition(0,0);return;
  }
  if("Orders".equals(label)){
   Intent i=new Intent(a,OrdersActivity.class).addFlags(Intent.FLAG_ACTIVITY_CLEAR_TOP|Intent.FLAG_ACTIVITY_SINGLE_TOP);
   a.startActivity(i);if(a instanceof WorkActivity)a.finish();a.overridePendingTransition(0,0);return;
  }
  String action="Reports".equals(label)?MainActivity.ACTION_REPORTS:"More".equals(label)?MainActivity.ACTION_MORE:MainActivity.ACTION_HOME;
  Intent i=new Intent(a,MainActivity.class).setAction(action).addFlags(Intent.FLAG_ACTIVITY_CLEAR_TOP|Intent.FLAG_ACTIVITY_SINGLE_TOP);
  a.startActivity(i);if(!(a instanceof MainActivity))a.finish();a.overridePendingTransition(0,0);
 }
 private static int dp(Activity a,float x){return Math.round(x*a.getResources().getDisplayMetrics().density);}
 private static android.graphics.drawable.Drawable round(Activity a,int color,int radius){
  android.graphics.drawable.GradientDrawable d=new android.graphics.drawable.GradientDrawable();
  d.setColor(color);d.setCornerRadius(dp(a,radius));return d;
 }
}
'''
write("android/app/src/main/java/in/tallybridge/app/SectionNav.java", section_nav)


# Route Work's common tasks through the mature MainActivity implementations.
p = "android/app/src/main/java/in/tallybridge/app/MainActivity.java"
s = read(p)
s = replace1(
    s,
    ' static final String ACTION_NEW_PROFORMA="in.tallybridge.app.NEW_PROFORMA";',
    ' static final String ACTION_NEW_PROFORMA="in.tallybridge.app.NEW_PROFORMA";\n'
    ' static final String ACTION_SEARCH="in.tallybridge.app.OPEN_SEARCH", ACTION_CUSTOMERS="in.tallybridge.app.OPEN_CUSTOMERS", ACTION_SYSTEM="in.tallybridge.app.OPEN_SYSTEM";\n'
    ' static final String ACTION_REPORTS="in.tallybridge.app.OPEN_REPORTS", ACTION_MORE="in.tallybridge.app.OPEN_MORE", ACTION_HOME="in.tallybridge.app.OPEN_HOME";',
    "MainActivity actions",
)
old = ' private void handlePendingLaunch(){String action=pendingLaunchAction;pendingLaunchAction="";if(ACTION_NEW_PROFORMA.equals(action)){rootTab("Proforma");return;}rootTab("Home");}'
new = ''' private void handlePendingLaunch(){String action=pendingLaunchAction;pendingLaunchAction="";
  if(ACTION_NEW_PROFORMA.equals(action)){rootTab("Proforma");return;}
  if(ACTION_SEARCH.equals(action)){history.clear();tab="Home";open(this::search);return;}
  if(ACTION_CUSTOMERS.equals(action)){history.clear();tab="Reports";open(()->ledgers(false));return;}
  if(ACTION_SYSTEM.equals(action)){rootTab("More");return;}
  if(ACTION_REPORTS.equals(action)){rootTab("Reports");return;}
  if(ACTION_MORE.equals(action)){rootTab("More");return;}
  rootTab("Home");
 }'''
s = replace1(s, old, new, "handlePendingLaunch")
old = ' private void rootTab(String name){history.clear();tab=name;switch(name){case "Home":home();break;case "Work":startActivity(new Intent(this,WorkActivity.class));break;case "Orders":startActivity(new Intent(this,OrdersActivity.class));break;case "Reports":reports();break;case "Proforma":proforma();break;case "Payments":payments();break;default:more();}}'
new = ' private void rootTab(String name){history.clear();tab=name;switch(name){case "Home":home();break;case "Work":startActivity(new Intent(this,WorkActivity.class));overridePendingTransition(0,0);break;case "Orders":startActivity(new Intent(this,OrdersActivity.class));overridePendingTransition(0,0);break;case "Reports":reports();break;case "Proforma":proforma();break;case "Payments":payments();break;default:more();}}'
s = replace1(s, old, new, "rootTab")
s = s.replace("Orders, dispatch, follow-ups and alerts", "Orders, dispatch, sales and alerts")
write(p, s)


# Work becomes a reliable daily hub. Search/Customers/System use core screens; Follow-ups are removed from visible workflow.
p = "android/app/src/main/java/in/tallybridge/app/WorkActivity.java"
s = read(p)
s = s.replace('@Override protected void onStop(){super.onStop();if(!isChangingConfigurations())finish();}\n', '')
frame_pattern = r' private void frame\(String title,String subtitle\)\{.*?\n private void busy\(boolean yes\)'
frame_replacement = ''' private void frame(String title,String subtitle){
  root=col();root.setBackgroundColor(BG);
  LinearLayout top=new LinearLayout(this);top.setGravity(Gravity.CENTER_VERTICAL);top.setPadding(dp(18),dp(14),dp(18),dp(8));
  LinearLayout titles=col();TextView brand=text("TALLYBRIDGE",10,TEAL);brand.setTypeface(null,Typeface.BOLD);brand.setLetterSpacing(.14f);titles.addView(brand);
  TextView t=text(title,25,NAVY);t.setTypeface(null,Typeface.BOLD);titles.addView(t);titles.addView(text(subtitle,12,MUTED));
  top.addView(titles,new LinearLayout.LayoutParams(0,-2,1));Button home=button("Home",v->openMain(MainActivity.ACTION_HOME));top.addView(home,new LinearLayout.LayoutParams(dp(82),-2));root.addView(top);
  progress=new ProgressBar(this,null,android.R.attr.progressBarStyleHorizontal);progress.setIndeterminate(true);progress.setVisibility(View.GONE);progress.setIndeterminateTintList(android.content.res.ColorStateList.valueOf(TEAL));root.addView(progress,new LinearLayout.LayoutParams(-1,dp(3)));
  LinearLayout tabs=new LinearLayout(this);String[] names={"Today","Search","Customers","Tools","System"};
  View.OnClickListener[] clicks={v->showToday(),v->openMain(MainActivity.ACTION_SEARCH),v->openMain(MainActivity.ACTION_CUSTOMERS),v->showTools(),v->openMain(MainActivity.ACTION_SYSTEM)};
  for(int i=0;i<names.length;i++){Button b=button(names[i],clicks[i]);LinearLayout.LayoutParams lp=new LinearLayout.LayoutParams(0,-2,1);lp.setMargins(dp(2),dp(7),dp(2),dp(4));tabs.addView(b,lp);}
  HorizontalScrollView hs=new HorizontalScrollView(this);hs.setHorizontalScrollBarEnabled(false);hs.setPadding(dp(12),0,dp(12),0);hs.addView(tabs,new HorizontalScrollView.LayoutParams(dp(610),-2));root.addView(hs);
  ScrollView sv=new ScrollView(this);body=col();body.setPadding(dp(18),dp(7),dp(18),dp(26));sv.addView(body);root.addView(sv,new LinearLayout.LayoutParams(-1,0,1));
  root.addView(SectionNav.bar(this,"Work",SURFACE,TINT,MUTED,TEAL));setContentView(root);
 }
 private void openMain(String action){Intent i=new Intent(this,MainActivity.class).setAction(action).addFlags(Intent.FLAG_ACTIVITY_CLEAR_TOP|Intent.FLAG_ACTIVITY_SINGLE_TOP);startActivity(i);finish();overridePendingTransition(0,0);}
 private void busy(boolean yes)'''
s, count = re.subn(frame_pattern, frame_replacement, s, count=1, flags=re.S)
if count != 1:
    raise SystemExit("Work frame replacement failed")
s = re.sub(r'\n\s*try\{JSONObject f=api\.get\(path\("followups"\)\);.*?catch\(Exception ignored\)\{\}', '', s, count=1, flags=re.S)
s = re.sub(r'\n\s*card\("Payment follow-ups".*?\);', '', s, count=1)
s = s.replace('Search","Invoices • parties • orders • LR/RR • GSTIN • follow-ups', 'Search","Invoices • parties • orders • LR/RR • GSTIN')
s = s.replace('Customer 360","Orders • invoices • outstanding • activity • follow-ups', 'Customer 360","Orders • invoices • outstanding • activity')
s = s.replace('Fast everyday actions • works with or without advanced Work data', 'Fast everyday actions • one consistent TallyBridge workflow')
s = s.replace('card("Search","Find invoices, parties, LR/RR and saved activity","",v->showSearch());', 'card("Search","Find invoices, parties, LR/RR and saved activity","",v->openMain(MainActivity.ACTION_SEARCH));')
s = s.replace('if(a!=null&&!(x.optBoolean("tallyOnline")&&"sync-error".equals(a.optString("id"))))visibleAlerts++;', 'if(a!=null&&!"followups".equals(a.optString("action"))&&!(x.optBoolean("tallyOnline")&&"sync-error".equals(a.optString("id"))))visibleAlerts++;')
s = s.replace('if(a==null||(x.optBoolean("tallyOnline")&&"sync-error".equals(a.optString("id"))))continue;', 'if(a==null||"followups".equals(a.optString("action"))||(x.optBoolean("tallyOnline")&&"sync-error".equals(a.optString("id"))))continue;')
write(p, s)


# Orders keeps all offline/sync behavior but is visually part of the same app.
p = "android/app/src/main/java/in/tallybridge/app/OrdersActivity.java"
s = read(p)
frame_replacement = ''' private void frame(String title,String subtitle){
  root=col();root.setBackgroundColor(BG);
  LinearLayout h=new LinearLayout(this);h.setGravity(Gravity.CENTER_VERTICAL);h.setPadding(dp(18),dp(14),dp(18),dp(8));
  LinearLayout titles=col();TextView brand=txt("TALLYBRIDGE",10,TEAL);brand.setTypeface(Typeface.DEFAULT_BOLD);brand.setLetterSpacing(.14f);titles.addView(brand);
  TextView t=txt(title,26,NAVY);t.setTypeface(Typeface.DEFAULT_BOLD);titles.addView(t);titles.addView(txt(subtitle,13,MUTED));
  h.addView(titles,new LinearLayout.LayoutParams(0,-2,1));Button home=btn("Home",v->SectionNav.go(this,"Home"));h.addView(home,new LinearLayout.LayoutParams(dp(84),-2));root.addView(h);
  progress=new ProgressBar(this,null,android.R.attr.progressBarStyleHorizontal);progress.setIndeterminate(true);progress.setVisibility(View.GONE);progress.setIndeterminateTintList(android.content.res.ColorStateList.valueOf(TEAL));root.addView(progress,new LinearLayout.LayoutParams(-1,dp(3)));
  ScrollView scroll=new ScrollView(this);body=col();body.setPadding(dp(18),dp(7),dp(18),dp(24));scroll.addView(body);root.addView(scroll,new LinearLayout.LayoutParams(-1,0,1));
  root.addView(SectionNav.bar(this,"Orders",SURFACE,TINT,MUTED,TEAL));setContentView(root);
 }
 private void busy(boolean yes)'''
s, count = re.subn(frame_pattern, frame_replacement, s, count=1, flags=re.S)
if count != 1:
    raise SystemExit("Orders frame replacement failed")
write(p, s)


# Main Orders widget: clearer information hierarchy plus direct GSM access.
p = "android/app/src/main/java/in/tallybridge/app/OrdersWidget.java"
s = read(p)
s = replace1(
    s,
    ' private static PendingIntent refresh(Context c){Intent i=new Intent(c,OrdersWidget.class).setAction(ACTION_REFRESH);return PendingIntent.getBroadcast(c,33,i,PendingIntent.FLAG_UPDATE_CURRENT|PendingIntent.FLAG_IMMUTABLE);}',
    ' private static PendingIntent refresh(Context c){Intent i=new Intent(c,OrdersWidget.class).setAction(ACTION_REFRESH);return PendingIntent.getBroadcast(c,33,i,PendingIntent.FLAG_UPDATE_CURRENT|PendingIntent.FLAG_IMMUTABLE);}\n private static PendingIntent gsm(Context c){Intent i=new Intent(c,GsmCalculatorActivity.class);return PendingIntent.getActivity(c,38,i,PendingIntent.FLAG_UPDATE_CURRENT|PendingIntent.FLAG_IMMUTABLE);}',
    "widget GSM intent",
)
s = replace1(s, '  size(v,R.id.widget_open,narrow?12f:13f);', '  size(v,R.id.widget_open,narrow?11f:12f);\n  size(v,R.id.widget_gsm,narrow?11f:12f);', "widget GSM size")
s = replace1(s, '  v.setOnClickPendingIntent(R.id.widget_open,activity(c,null,32));', '  v.setOnClickPendingIntent(R.id.widget_open,activity(c,null,32));\n  v.setOnClickPendingIntent(R.id.widget_gsm,gsm(c));', "widget GSM click")
write(p, s)


def full_widget(dark=False):
    panel = "orders_widget_panel_dark" if dark else "orders_widget_panel"
    primary = "orders_widget_primary_dark" if dark else "orders_widget_primary"
    secondary = "orders_widget_secondary_dark" if dark else "orders_widget_secondary"
    title = "#E6EDF3" if dark else "#102D44"
    teal = "#7BD8BD" if dark else "#176B5B"
    muted = "#A7B8C5" if dark else "#596C79"
    primary_text = "#102D26" if dark else "#FFFFFF"
    return f'''<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android" android:id="@+id/widget_root" android:layout_width="match_parent" android:layout_height="match_parent" android:orientation="vertical" android:padding="15dp" android:background="@drawable/{panel}">
 <LinearLayout android:layout_width="match_parent" android:layout_height="wrap_content" android:gravity="center_vertical">
  <LinearLayout android:layout_width="0dp" android:layout_height="wrap_content" android:layout_weight="1" android:orientation="vertical"><TextView android:layout_width="wrap_content" android:layout_height="wrap_content" android:text="TALLYBRIDGE" android:textColor="{teal}" android:textSize="9sp" android:textStyle="bold" android:letterSpacing="0.13"/><TextView android:layout_width="wrap_content" android:layout_height="wrap_content" android:layout_marginTop="2dp" android:text="Order Desk" android:textColor="{title}" android:textSize="19sp" android:textStyle="bold"/></LinearLayout>
  <TextView android:id="@+id/widget_mode" android:layout_width="wrap_content" android:layout_height="wrap_content" android:paddingLeft="8dp" android:paddingTop="4dp" android:paddingRight="8dp" android:paddingBottom="4dp" android:background="@drawable/{secondary}" android:text="PC SYNCED" android:textColor="{muted}" android:textSize="9sp" android:textStyle="bold"/>
  <TextView android:id="@+id/widget_refresh" android:layout_width="38dp" android:layout_height="38dp" android:layout_marginStart="7dp" android:gravity="center" android:background="@drawable/{secondary}" android:text="↻" android:textColor="{teal}" android:textSize="18sp" android:textStyle="bold" android:contentDescription="Refresh pending orders"/>
 </LinearLayout>
 <LinearLayout android:layout_width="match_parent" android:layout_height="wrap_content" android:layout_marginTop="8dp" android:gravity="bottom"><TextView android:id="@+id/widget_pending" android:layout_width="wrap_content" android:layout_height="wrap_content" android:text="0" android:textColor="{title}" android:textSize="32sp" android:textStyle="bold"/><LinearLayout android:layout_width="wrap_content" android:layout_height="wrap_content" android:layout_marginStart="9dp" android:layout_marginBottom="4dp" android:orientation="vertical"><TextView android:layout_width="wrap_content" android:layout_height="wrap_content" android:text="ORDERS WAITING" android:textColor="{teal}" android:textSize="10sp" android:textStyle="bold"/><TextView android:layout_width="wrap_content" android:layout_height="wrap_content" android:text="Tap a customer or open the desk" android:textColor="{muted}" android:textSize="10sp"/></LinearLayout></LinearLayout>
 <LinearLayout android:id="@+id/widget_party_area" android:layout_width="match_parent" android:layout_height="0dp" android:layout_weight="1" android:orientation="vertical" android:gravity="center_vertical">
  <TextView android:id="@+id/widget_party_label" android:layout_width="match_parent" android:layout_height="wrap_content" android:text="NEXT CUSTOMERS" android:textColor="{muted}" android:textSize="9sp" android:textStyle="bold" android:letterSpacing="0.08"/>
  <TextView android:id="@+id/widget_party1" android:layout_width="match_parent" android:layout_height="wrap_content" android:layout_marginTop="5dp" android:ellipsize="end" android:maxLines="1" android:text="No pending orders" android:textColor="{title}" android:textSize="14sp" android:textStyle="bold"/>
  <TextView android:id="@+id/widget_party2" android:layout_width="match_parent" android:layout_height="wrap_content" android:layout_marginTop="3dp" android:ellipsize="end" android:maxLines="1" android:textColor="{title}" android:textSize="13sp" android:textStyle="bold"/>
  <TextView android:id="@+id/widget_party3" android:layout_width="match_parent" android:layout_height="wrap_content" android:layout_marginTop="3dp" android:ellipsize="end" android:maxLines="1" android:textColor="{title}" android:textSize="13sp" android:textStyle="bold"/>
  <TextView android:id="@+id/widget_more" android:layout_width="match_parent" android:layout_height="wrap_content" android:layout_marginTop="3dp" android:textColor="{muted}" android:textSize="10sp" android:textStyle="bold"/>
 </LinearLayout>
 <LinearLayout android:layout_width="match_parent" android:layout_height="wrap_content" android:orientation="horizontal"><TextView android:id="@+id/widget_add" android:layout_width="0dp" android:layout_height="44dp" android:layout_weight="1.25" android:gravity="center" android:background="@drawable/{primary}" android:text="+ New order" android:textColor="{primary_text}" android:textSize="13sp" android:textStyle="bold"/><TextView android:id="@+id/widget_gsm" android:layout_width="0dp" android:layout_height="44dp" android:layout_weight="0.75" android:layout_marginStart="7dp" android:gravity="center" android:background="@drawable/{secondary}" android:text="GSM" android:textColor="{teal}" android:textSize="12sp" android:textStyle="bold"/><TextView android:id="@+id/widget_open" android:layout_width="0dp" android:layout_height="44dp" android:layout_weight="0.95" android:layout_marginStart="7dp" android:gravity="center" android:background="@drawable/{secondary}" android:text="Open desk" android:textColor="{teal}" android:textSize="12sp" android:textStyle="bold"/></LinearLayout>
</LinearLayout>'''


def compact_widget(dark=False):
    panel = "orders_widget_panel_dark" if dark else "orders_widget_panel"
    primary = "orders_widget_primary_dark" if dark else "orders_widget_primary"
    secondary = "orders_widget_secondary_dark" if dark else "orders_widget_secondary"
    title = "#E6EDF3" if dark else "#102D44"
    teal = "#7BD8BD" if dark else "#176B5B"
    muted = "#A7B8C5" if dark else "#596C79"
    primary_text = "#102D26" if dark else "#FFFFFF"
    return f'''<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android" android:id="@+id/widget_root" android:layout_width="match_parent" android:layout_height="match_parent" android:orientation="vertical" android:padding="12dp" android:background="@drawable/{panel}">
 <LinearLayout android:layout_width="match_parent" android:layout_height="wrap_content" android:gravity="center_vertical"><LinearLayout android:layout_width="0dp" android:layout_height="wrap_content" android:layout_weight="1" android:orientation="vertical"><TextView android:layout_width="wrap_content" android:layout_height="wrap_content" android:text="TALLYBRIDGE • ORDERS" android:textColor="{teal}" android:textSize="9sp" android:textStyle="bold" android:letterSpacing="0.10"/><LinearLayout android:layout_width="wrap_content" android:layout_height="wrap_content" android:gravity="bottom"><TextView android:id="@+id/widget_pending" android:layout_width="wrap_content" android:layout_height="wrap_content" android:text="0" android:textColor="{title}" android:textSize="28sp" android:textStyle="bold"/><TextView android:layout_width="wrap_content" android:layout_height="wrap_content" android:layout_marginStart="7dp" android:layout_marginBottom="4dp" android:text="pending" android:textColor="{muted}" android:textSize="11sp"/></LinearLayout></LinearLayout><TextView android:id="@+id/widget_refresh" android:layout_width="38dp" android:layout_height="38dp" android:gravity="center" android:background="@drawable/{secondary}" android:text="↻" android:textColor="{teal}" android:textSize="18sp" android:textStyle="bold" android:contentDescription="Refresh pending orders"/></LinearLayout>
 <TextView android:id="@+id/widget_mode" android:layout_width="wrap_content" android:layout_height="wrap_content" android:visibility="gone" android:text="PC SYNCED"/>
 <LinearLayout android:id="@+id/widget_party_area" android:layout_width="match_parent" android:layout_height="0dp" android:layout_weight="1" android:orientation="vertical" android:gravity="center_vertical"><TextView android:id="@+id/widget_party_label" android:layout_width="wrap_content" android:layout_height="wrap_content" android:visibility="gone"/><TextView android:id="@+id/widget_party1" android:layout_width="match_parent" android:layout_height="wrap_content" android:ellipsize="end" android:maxLines="1" android:text="No pending orders" android:textColor="{title}" android:textSize="14sp" android:textStyle="bold"/><TextView android:id="@+id/widget_party2" android:layout_width="match_parent" android:layout_height="wrap_content" android:visibility="gone"/><TextView android:id="@+id/widget_party3" android:layout_width="match_parent" android:layout_height="wrap_content" android:visibility="gone"/><TextView android:id="@+id/widget_more" android:layout_width="match_parent" android:layout_height="wrap_content" android:visibility="gone"/></LinearLayout>
 <LinearLayout android:layout_width="match_parent" android:layout_height="wrap_content" android:orientation="horizontal"><TextView android:id="@+id/widget_add" android:layout_width="0dp" android:layout_height="40dp" android:layout_weight="1.2" android:gravity="center" android:background="@drawable/{primary}" android:text="+ Order" android:textColor="{primary_text}" android:textSize="12sp" android:textStyle="bold"/><TextView android:id="@+id/widget_gsm" android:layout_width="0dp" android:layout_height="40dp" android:layout_weight="0.8" android:layout_marginStart="6dp" android:gravity="center" android:background="@drawable/{secondary}" android:text="GSM" android:textColor="{teal}" android:textSize="12sp" android:textStyle="bold"/><TextView android:id="@+id/widget_open" android:layout_width="0dp" android:layout_height="40dp" android:layout_weight="0.9" android:layout_marginStart="6dp" android:gravity="center" android:background="@drawable/{secondary}" android:text="Open" android:textColor="{teal}" android:textSize="12sp" android:textStyle="bold"/></LinearLayout>
</LinearLayout>'''


write("android/app/src/main/res/layout/orders_widget.xml", full_widget(False))
write("android/app/src/main/res/layout/orders_widget_dark.xml", full_widget(True))
write("android/app/src/main/res/layout/orders_widget_compact.xml", compact_widget(False))
write("android/app/src/main/res/layout/orders_widget_compact_dark.xml", compact_widget(True))


# Windows control center: dark desktop-only theme and navigation ordered around daily operation.
p = "internal/app/web/admin.js"
s = read(p)
s = replace1(
    s,
    "const nav=[['home','Overview'],['companies','Companies'],['pdfbank','Payments'],['health','Company Health'],['diag','Diagnostics'],['devices','Phones'],['cloudflare','Connection'],['live','Sync'],['backup','Backup & Restore'],['updates','Updates'],['settings','Settings'],['about','About']];",
    "const nav=[['home','Overview'],['companies','Companies'],['live','Sync'],['devices','Phones'],['cloudflare','Connection'],['health','Company Health'],['pdfbank','Payments'],['diag','Diagnostics'],['backup','Backup & Restore'],['updates','Updates'],['settings','Settings'],['about','About']];",
    "desktop nav",
)
s = s.replace("v2.1 • production workspace • read-only Tally access", "v2.4.6 • dark control center • read-only Tally access")
s = s.replace("TallyBridge runs quietly in the tray and keeps your Android app supplied with live or last-known-good read-only Tally data.", "One control center for Tally connection, Android sync, company health and troubleshooting.")
write(p, s)

p = "internal/app/web/styles.css"
s = read(p)
marker = "/* TALLYBRIDGE DESKTOP DARK 2.4.6 */"
if marker not in s:
    s += r'''
/* TALLYBRIDGE DESKTOP DARK 2.4.6 */
.desktop-body{--navy:#081827;--navy2:#0d2235;--ink:#e8eef5;--muted:#91a4b8;--blue:#69a9ff;--blue2:#142b43;--card:#111d29;--border:#263747;--shadow:0 14px 38px rgba(0,0,0,.25);--ok:#74ddb8;--okbg:#12352d;--err:#ff9c9c;--errbg:#3a2024;--warn:#ffd078;--warnbg:#3b3020;background:#08111b;color:var(--ink)}
.desktop-body .admin-shell{background:#08111b;grid-template-columns:238px minmax(0,1fr)}
.desktop-body .sidebar{background:linear-gradient(180deg,#081522 0%,#091d2e 58%,#07111b 100%);border-right:1px solid #1f3040;padding:20px 14px}
.desktop-body .brand{padding:7px 9px 18px;font-size:19px}.desktop-body .brandmark{background:#67d9b7;color:#09241e;box-shadow:0 8px 24px rgba(0,0,0,.28)}
.desktop-body .side-nav{gap:3px}.desktop-body .side-btn{color:#a9bbcc;padding:11px 12px;border-radius:11px}.desktop-body .side-btn:hover{background:#122a3f;color:#e8eef5}.desktop-body .side-btn.active{background:linear-gradient(135deg,#174f78,#176b5b);color:#fff;box-shadow:0 7px 18px rgba(0,0,0,.18)}
.desktop-body .sidebar-footer{border-top:1px solid #203242;margin-top:auto;padding:14px 9px 8px}.desktop-body .version-chip{color:#7890a5;line-height:1.5}
.desktop-body .main{background:radial-gradient(circle at 78% -12%,rgba(35,107,91,.22),transparent 32%),#08111b;padding:28px 32px 54px}
.desktop-body .page-head h1{color:#f3f7fb}.desktop-body .page-head p{color:var(--muted);max-width:760px}
.desktop-body .card,.desktop-body .hero-card,.desktop-body .action-card{background:linear-gradient(180deg,#13212f,#101b27);border-color:#27394a;box-shadow:var(--shadow);color:var(--ink)}
.desktop-body .hero-card{border:1px solid #2c4152}.desktop-body .hero-card h1,.desktop-body .title,.desktop-body .metric-value,.desktop-body .section-title{color:#eef4f9}
.desktop-body .metric-label,.desktop-body .metric-sub,.desktop-body .sub{color:var(--muted)}
.desktop-body .action-card:hover,.desktop-body .card:hover{border-color:#365369}.desktop-body .action-icon{background:#17364a;color:#7bd8bd}
.desktop-body .btn.secondary,.desktop-body .icon-btn{background:#192b3b;color:#d8e5ef;border:1px solid #2a4053}.desktop-body .btn.secondary:hover{background:#20374a}
.desktop-body .input,.desktop-body .select,.desktop-body input,.desktop-body select,.desktop-body textarea{background:#0c1823;color:#e8eef5;border-color:#2b3f50}
.desktop-body .input:focus,.desktop-body .select:focus{border-color:#5b9add;box-shadow:0 0 0 3px rgba(91,154,221,.13)}
.desktop-body .table-wrap,.desktop-body .empty{background:#0f1b27;border-color:#283a4a;color:var(--muted)}.desktop-body .table th{background:#101e2b;color:#91a4b8;border-color:#2a3b4a}.desktop-body .table td,.desktop-body .device-row,.desktop-body .list-row{border-color:#20313f}
.desktop-body .notice.info{background:#132d46;color:#9bc9ff}.desktop-body .notice.ok{background:#12352d;color:#8ee7c7}.desktop-body .notice.warn{background:#3a2f20;color:#ffd78b}.desktop-body .notice.err{background:#392126;color:#ffb0b0}
.desktop-body .live-pill.ok,.desktop-body .status-pill.ok{background:#12352d;color:#7fe0be}.desktop-body .live-pill.warn,.desktop-body .status-pill.warn{background:#3a2f20;color:#ffd078}.desktop-body .live-pill.err,.desktop-body .status-pill.err{background:#392126;color:#ff9c9c}
.desktop-body .toast{background:#1b3042;border:1px solid #335064}.desktop-body ::selection{background:#1f6f70;color:#fff}
@media(max-width:980px){.desktop-body .admin-shell{grid-template-columns:205px minmax(0,1fr)}.desktop-body .main{padding:22px 20px 44px}.desktop-body .sidebar{padding-left:10px;padding-right:10px}}
'''
write(p, s)


# Guard against silently applying only part of the redesign.
main = read("android/app/src/main/java/in/tallybridge/app/MainActivity.java")
work = read("android/app/src/main/java/in/tallybridge/app/WorkActivity.java")
orders = read("android/app/src/main/java/in/tallybridge/app/OrdersActivity.java")
checks = {
    "Main action routing": "ACTION_SEARCH" in main and "ACTION_CUSTOMERS" in main,
    "Work shared nav": 'SectionNav.bar(this,"Work"' in work,
    "Orders shared nav": 'SectionNav.bar(this,"Orders"' in orders,
    "No Follow-ups tab": 'String[] names={"Today","Search","Customers","Tools","System"}' in work,
    "Widget GSM": "widget_gsm" in read("android/app/src/main/res/layout/orders_widget.xml"),
    "Desktop dark": marker in read("internal/app/web/styles.css"),
    "Windows 2.4.6": 'const Version = "2.4.6"' in read("internal/app/types.go"),
}
failed = [name for name, ok in checks.items() if not ok]
if failed:
    raise SystemExit("sanity checks failed: " + ", ".join(failed))

print("2.4.6 reorganization complete")
