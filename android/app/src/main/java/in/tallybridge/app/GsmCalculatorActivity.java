package in.tallybridge.app;

import android.app.*;
import android.content.*;
import android.graphics.*;
import android.graphics.drawable.GradientDrawable;
import android.os.*;
import android.text.*;
import android.view.*;
import android.widget.*;
import java.util.*;

/** Offline towel GSM calculator. No server or Tally connection is required. */
public final class GsmCalculatorActivity extends Activity {
 private boolean dark;
 private int NAVY,TEAL,MUTED,BG,SURFACE,BORDER,TINT,PRIMARY_INK;
 private EditText item,description,length,breadth,weight;
 private Spinner unit;
 private TextView result,formula;
 private long lastGsm=0;

 @Override public void onCreate(Bundle state){applyAppearance();super.onCreate(state);getWindow().addFlags(WindowManager.LayoutParams.FLAG_SECURE);styleWindow();render();}
 private String appearance(){return getSharedPreferences("appearance",MODE_PRIVATE).getString("mode","System");}
 private void applyAppearance(){String m=appearance();dark=m.equals("Dark")||(m.equals("System")&&(getResources().getConfiguration().uiMode&android.content.res.Configuration.UI_MODE_NIGHT_MASK)==android.content.res.Configuration.UI_MODE_NIGHT_YES);setTheme(dark?R.style.AppThemeDark:R.style.AppThemeLight);NAVY=dark?0xffe6edf3:0xff102d44;TEAL=dark?0xff7bd8bd:0xff176b5b;MUTED=dark?0xffa7b8c5:0xff596c79;BG=dark?0xff101820:0xfff5f7fa;SURFACE=dark?0xff1b2732:Color.WHITE;BORDER=dark?0xff344552:0xffe4eaef;TINT=dark?0xff203f3a:0xffe5f2ee;PRIMARY_INK=dark?0xff102d26:Color.WHITE;}
 private void styleWindow(){getWindow().getDecorView().setSystemUiVisibility(dark?0:(View.SYSTEM_UI_FLAG_LIGHT_STATUS_BAR|View.SYSTEM_UI_FLAG_LIGHT_NAVIGATION_BAR));getWindow().setStatusBarColor(BG);getWindow().setNavigationBarColor(SURFACE);}
 private int dp(float x){return Math.round(x*getResources().getDisplayMetrics().density);}
 private LinearLayout col(){LinearLayout l=new LinearLayout(this);l.setOrientation(LinearLayout.VERTICAL);return l;}
 private TextView text(String s,int z,int c){TextView v=new TextView(this);v.setText(s);v.setTextSize(z);v.setTextColor(c);v.setLineSpacing(dp(2),1f);v.setPadding(0,dp(3),0,dp(3));return v;}
 private GradientDrawable shape(int c,int r){GradientDrawable d=new GradientDrawable();d.setColor(c);d.setCornerRadius(dp(r));d.setStroke(dp(1),BORDER);return d;}
 private LinearLayout.LayoutParams full(int top){LinearLayout.LayoutParams p=new LinearLayout.LayoutParams(-1,-2);p.setMargins(0,dp(top),0,0);return p;}
 private EditText input(String hint,int type){EditText e=new EditText(this);e.setHint(hint);e.setTextSize(16);e.setTextColor(NAVY);e.setHintTextColor(MUTED);e.setInputType(type);e.setPadding(dp(14),dp(12),dp(14),dp(12));e.setBackground(shape(SURFACE,12));e.setMinHeight(dp(52));return e;}
 private Button button(String s,View.OnClickListener c){Button b=new Button(this);b.setText(s);b.setAllCaps(false);b.setTextSize(15);b.setTextColor(TEAL);b.setBackground(shape(SURFACE,12));b.setMinHeight(dp(50));b.setOnClickListener(c);return b;}
 private Button primary(String s,View.OnClickListener c){Button b=button(s,c);b.setTextColor(PRIMARY_INK);b.setBackground(shape(TEAL,12));return b;}
 private void label(LinearLayout box,String s){TextView v=text(s,11,MUTED);v.setTypeface(null,Typeface.BOLD);v.setLetterSpacing(.08f);box.addView(v,full(14));}

 private void render(){
  LinearLayout root=col();root.setBackgroundColor(BG);
  root.addView(ScreenHeader.bar(this,null,"GSM calculator","Towel weight ÷ fabric area • works offline",NAVY,TEAL,MUTED));
  ScrollView sv=new ScrollView(this);LinearLayout body=col();body.setPadding(dp(18),dp(4),dp(18),dp(30));sv.addView(body);root.addView(sv,new LinearLayout.LayoutParams(-1,0,1));setContentView(root);

  label(body,"ITEM");item=input("Item name, e.g. Bath Towel",android.text.InputType.TYPE_CLASS_TEXT|android.text.InputType.TYPE_TEXT_FLAG_CAP_WORDS);body.addView(item,full(5));
  description=input("Description (optional)",android.text.InputType.TYPE_CLASS_TEXT|android.text.InputType.TYPE_TEXT_FLAG_CAP_SENTENCES|android.text.InputType.TYPE_TEXT_FLAG_MULTI_LINE);description.setSingleLine(false);description.setMinLines(2);body.addView(description,full(8));
  label(body,"DIMENSIONS");
  LinearLayout dims=new LinearLayout(this);length=input("Length",android.text.InputType.TYPE_CLASS_NUMBER|android.text.InputType.TYPE_NUMBER_FLAG_DECIMAL);breadth=input("Breadth",android.text.InputType.TYPE_CLASS_NUMBER|android.text.InputType.TYPE_NUMBER_FLAG_DECIMAL);LinearLayout.LayoutParams a=new LinearLayout.LayoutParams(0,-2,1);a.setMargins(0,0,dp(5),0);LinearLayout.LayoutParams b=new LinearLayout.LayoutParams(0,-2,1);b.setMargins(dp(5),0,0,0);dims.addView(length,a);dims.addView(breadth,b);body.addView(dims,full(5));
  unit=new Spinner(this);ArrayAdapter<String> adapter=new ArrayAdapter<>(this,android.R.layout.simple_spinner_dropdown_item,new String[]{"Inches","Centimetres"});unit.setAdapter(adapter);ScreenHeader.spinner(unit,dark);body.addView(unit,full(8));
  label(body,"WEIGHT");weight=input("Weight in grams",android.text.InputType.TYPE_CLASS_NUMBER|android.text.InputType.TYPE_NUMBER_FLAG_DECIMAL);body.addView(weight,full(5));

  LinearLayout resultCard=col();resultCard.setPadding(dp(18),dp(17),dp(18),dp(17));resultCard.setBackground(shape(TINT,16));TextView rTitle=text("CALCULATED GSM",11,TEAL);rTitle.setTypeface(null,Typeface.BOLD);rTitle.setLetterSpacing(.08f);resultCard.addView(rTitle);result=text("—",34,NAVY);result.setTypeface(null,Typeface.BOLD);resultCard.addView(result,full(4));formula=text("Enter length, breadth and weight to calculate.",12,MUTED);resultCard.addView(formula,full(4));body.addView(resultCard,full(16));

  body.addView(primary("Calculate GSM",v->calculate(true)),full(12));body.addView(button("Share on WhatsApp",v->share()),full(8));body.addView(button("Clear",v->clear()),full(8));
  body.addView(text("Formula: GSM = weight in grams ÷ area in square metres. Dimensions are converted automatically.",12,MUTED),full(14));

  TextWatcher watcher=new TextWatcher(){public void beforeTextChanged(CharSequence s,int st,int c,int a){}public void onTextChanged(CharSequence s,int st,int b,int c){calculate(false);}public void afterTextChanged(Editable e){}};length.addTextChangedListener(watcher);breadth.addTextChangedListener(watcher);weight.addTextChangedListener(watcher);unit.setOnItemSelectedListener(new android.widget.AdapterView.OnItemSelectedListener(){public void onItemSelected(android.widget.AdapterView<?> p,View v,int pos,long id){calculate(false);}public void onNothingSelected(android.widget.AdapterView<?> p){}});
 }
 private double number(EditText e){try{return Double.parseDouble(e.getText().toString().trim());}catch(Exception x){return 0;}}
 private boolean calculate(boolean showError){double l=number(length),b=number(breadth),w=number(weight);if(l<=0||b<=0||w<=0){lastGsm=0;result.setText("—");formula.setText("Enter length, breadth and weight to calculate.");if(showError)Toast.makeText(this,"Enter valid length, breadth and weight",Toast.LENGTH_SHORT).show();return false;}String u=unit.getSelectedItemPosition()==1?"cm":"in";lastGsm=GsmCalculator.rounded(l,b,w,u);result.setText(lastGsm+" GSM");formula.setText(trim(l)+" × "+trim(b)+" "+unitLabel()+" • "+trim(w)+" g");return true;}
 private String trim(double n){if(Math.rint(n)==n)return String.valueOf((long)n);return String.format(Locale.US,"%.2f",n).replaceAll("0+$","").replaceAll("\\.$","");}
 private String unitLabel(){return unit.getSelectedItemPosition()==1?"cm":"in";}
 private void clear(){item.setText("");description.setText("");length.setText("");breadth.setText("");weight.setText("");unit.setSelection(0);lastGsm=0;result.setText("—");formula.setText("Enter length, breadth and weight to calculate.");item.requestFocus();}
 private void share(){if(!calculate(true))return;String name=item.getText().toString().trim();if(name.isEmpty()){Toast.makeText(this,"Enter item name before sharing",Toast.LENGTH_SHORT).show();item.requestFocus();return;}String desc=description.getText().toString().trim();StringBuilder s=new StringBuilder();s.append("Towel GSM Details\n");s.append("Item: ").append(name).append('\n');if(!desc.isEmpty())s.append("Description: ").append(desc).append('\n');s.append("Length: ").append(trim(number(length))).append(' ').append(unitLabel()).append('\n');s.append("Breadth: ").append(trim(number(breadth))).append(' ').append(unitLabel()).append('\n');s.append("Weight: ").append(trim(number(weight))).append(" g\n");s.append("GSM: ").append(lastGsm);ShareCenter.shareText(this,"Share GSM details",s.toString());}
}
