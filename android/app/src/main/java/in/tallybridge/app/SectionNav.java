package in.tallybridge.app;

import android.app.Activity;
import android.content.Intent;
import android.graphics.Typeface;
import android.view.Gravity;
import android.widget.LinearLayout;
import android.widget.TextView;

/** Shared five-section navigation used across the native Android app. */
final class SectionNav {
 private SectionNav(){}
 static final String[] LABELS={"Home","Orders","Reports","Tools","System"};

 static LinearLayout bar(Activity a,String selected,int surface,int tint,int muted,int teal){
  LinearLayout nav=new LinearLayout(a);
  nav.setPadding(dp(a,8),dp(a,6),dp(a,8),dp(a,7));
  nav.setBackgroundColor(surface);
  for(String label:LABELS){
   boolean on=label.equals(selected);
   LinearLayout item=new LinearLayout(a);item.setOrientation(LinearLayout.VERTICAL);item.setGravity(Gravity.CENTER);
   item.setPadding(dp(a,4),dp(a,3),dp(a,4),dp(a,3));
   item.setBackground(round(a,on?tint:surface,15));
   item.setFocusable(true);item.setContentDescription(label+(on?", selected":""));
   item.addView(new UiIcon(a,label,on?teal:muted),new LinearLayout.LayoutParams(dp(a,22),dp(a,22)));
   TextView t=new TextView(a);t.setText(label);t.setTextSize(label.length()>6?10:11);t.setGravity(Gravity.CENTER);t.setTextColor(on?teal:muted);t.setSingleLine(true);
   if(on)t.setTypeface(null,Typeface.BOLD);
   item.addView(t);
   if(!on)item.setOnClickListener(v->go(a,label));
   LinearLayout.LayoutParams lp=new LinearLayout.LayoutParams(0,dp(a,57),1);lp.setMargins(dp(a,2),0,dp(a,2),0);nav.addView(item,lp);
  }
  return nav;
 }

 static void go(Activity a,String label){
  if("Orders".equals(label)){
   Intent i=new Intent(a,OrdersActivity.class).addFlags(Intent.FLAG_ACTIVITY_CLEAR_TOP|Intent.FLAG_ACTIVITY_SINGLE_TOP);
   a.startActivity(i);if(a instanceof WorkActivity)a.finish();a.overridePendingTransition(0,0);return;
  }
  String action;
  switch(label){
   case "Reports": action=MainActivity.ACTION_REPORTS; break;
   case "Tools": action=MainActivity.ACTION_TOOLS; break;
   case "System": action=MainActivity.ACTION_SYSTEM; break;
   default: action=MainActivity.ACTION_HOME;
  }
  Intent i=new Intent(a,MainActivity.class).setAction(action).addFlags(Intent.FLAG_ACTIVITY_CLEAR_TOP|Intent.FLAG_ACTIVITY_SINGLE_TOP);
  a.startActivity(i);if(!(a instanceof MainActivity))a.finish();a.overridePendingTransition(0,0);
 }

 private static int dp(Activity a,float x){return Math.round(x*a.getResources().getDisplayMetrics().density);}
 private static android.graphics.drawable.Drawable round(Activity a,int color,int radius){
  android.graphics.drawable.GradientDrawable d=new android.graphics.drawable.GradientDrawable();
  d.setColor(color);d.setCornerRadius(dp(a,radius));return d;
 }
}
