package in.tallybridge.app;

import android.app.Activity;
import android.graphics.Typeface;
import android.util.TypedValue;
import android.view.Gravity;
import android.view.View;
import android.widget.FrameLayout;
import android.widget.LinearLayout;
import android.widget.Spinner;
import android.widget.TextView;

/**
 * One header for every screen opened from somewhere else (search, customer, GSM, bills, photo order):
 * a back arrow, the TALLYBRIDGE line, the title and a short subtitle. Matches the main screens' header.
 */
final class ScreenHeader {
 private ScreenHeader(){}

 static LinearLayout bar(Activity a,String brand,String title,String subtitle,int navy,int teal,int muted){
  LinearLayout top=new LinearLayout(a);top.setGravity(Gravity.CENTER_VERTICAL);top.setPadding(dp(a,6),dp(a,12),dp(a,18),dp(a,8));
  FrameLayout back=new FrameLayout(a);back.setContentDescription("Back");back.setClickable(true);back.setFocusable(true);back.setOnClickListener(v->a.finish());
  TypedValue ripple=new TypedValue();if(a.getTheme().resolveAttribute(android.R.attr.selectableItemBackgroundBorderless,ripple,true))back.setBackgroundResource(ripple.resourceId);
  back.addView(new UiIcon(a,"back",navy),new FrameLayout.LayoutParams(dp(a,24),dp(a,24),Gravity.CENTER));
  top.addView(back,new LinearLayout.LayoutParams(dp(a,48),dp(a,48)));
  LinearLayout titles=new LinearLayout(a);titles.setOrientation(LinearLayout.VERTICAL);titles.setPadding(dp(a,4),0,0,0);
  TextView b=new TextView(a);b.setText(brand==null||brand.isEmpty()?"TALLYBRIDGE":brand);b.setTextSize(10);b.setTextColor(teal);b.setTypeface(null,Typeface.BOLD);b.setLetterSpacing(.12f);titles.addView(b);
  TextView h=new TextView(a);h.setText(title);h.setTextSize(24);h.setTextColor(navy);h.setTypeface(null,Typeface.BOLD);h.setMaxLines(2);h.setEllipsize(android.text.TextUtils.TruncateAt.END);
  h.setAccessibilityHeading(true);titles.addView(h);
  if(subtitle!=null&&!subtitle.isEmpty()){TextView s=new TextView(a);s.setText(subtitle);s.setTextSize(12);s.setTextColor(muted);s.setLineSpacing(dp(a,2),1f);s.setPadding(0,dp(a,2),0,0);titles.addView(s);}
  top.addView(titles,new LinearLayout.LayoutParams(0,-2,1));
  return top;
 }

 /** Adds a view (a refresh or share action) at the right end of a header made by {@link #bar}. */
 static void action(LinearLayout header,Activity a,String icon,String label,int color,View.OnClickListener click){
  FrameLayout f=new FrameLayout(a);f.setContentDescription(label);f.setClickable(true);f.setFocusable(true);f.setOnClickListener(click);
  TypedValue ripple=new TypedValue();if(a.getTheme().resolveAttribute(android.R.attr.selectableItemBackgroundBorderless,ripple,true))f.setBackgroundResource(ripple.resourceId);
  f.addView(new UiIcon(a,icon,color),new FrameLayout.LayoutParams(dp(a,24),dp(a,24),Gravity.CENTER));
  header.addView(f,new LinearLayout.LayoutParams(dp(a,48),dp(a,48)));
 }

 /** Drop-downs look like the text fields (rounded card, border) and keep a chevron so they read as a choice. */
 static void spinner(Spinner s,boolean dark){
  s.setBackgroundResource(dark?R.drawable.spinner_bg_dark:R.drawable.spinner_bg_light);
  s.setPopupBackgroundResource(dark?R.drawable.popup_bg_dark:R.drawable.popup_bg_light);
  s.setPadding(dp(s,6),dp(s,2),dp(s,40),dp(s,2));s.setMinimumHeight(dp(s,52));
 }

 private static int dp(Activity a,float x){return Math.round(x*a.getResources().getDisplayMetrics().density);}
 private static int dp(View v,float x){return Math.round(x*v.getResources().getDisplayMetrics().density);}
}
