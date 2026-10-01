package in.tallybridge.app;

import android.app.*;
import android.content.*;
import android.graphics.*;
import android.graphics.drawable.GradientDrawable;
import android.media.ExifInterface;
import android.net.Uri;
import android.os.*;
import android.text.InputType;
import android.view.*;
import android.widget.*;
import org.json.*;
import java.io.*;
import java.security.MessageDigest;
import java.util.*;
import java.util.concurrent.*;

/** Receives customer order photos from WhatsApp/Gallery and saves them as offline-first Orders. */
public final class PhotoOrderActivity extends Activity {
 private final ExecutorService work=Executors.newSingleThreadExecutor();
 private final Handler main=new Handler(Looper.getMainLooper());
 private CryptoStore store; private BridgeApi api; private OfflineOrdersStore offline;
 private JSONArray companies=new JSONArray(); private final List<JSONObject> photos=new ArrayList<>();
 private Spinner companySpin; private AutoCompleteTextView buyer; private TextView buyerInfo,status; private EditText notes;
 private JSONObject buyerDetails=new JSONObject(); private String buyerDetailsFor=""; private boolean destroyed=false,saved=false; private int sharedUriCount=0;
 private final int NAVY=0xff102d44,TEAL=0xff176b5b,MUTED=0xff596c79,BG=0xfff5f7fa,SURFACE=Color.WHITE,BORDER=0xffe4eaef,AMBER=0xff8a5a00;

 @Override public void onCreate(Bundle state){
  super.onCreate(state);getWindow().addFlags(WindowManager.LayoutParams.FLAG_SECURE);
  try{store=new CryptoStore(this);api=new BridgeApi(store);offline=new OfflineOrdersStore(store);}catch(Exception e){fatal("Could not open TallyBridge Orders: "+e.getMessage());return;}
  if(store.get("base").isEmpty()||store.get("device").isEmpty()){fatal("Open TallyBridge once and pair this phone before saving photo orders.");return;}
  try{companies=offline.companies();}catch(Exception ignored){}
  if(companies.length()==0){String c=store.get("company");if(!c.isEmpty())try{companies.put(new JSONObject().put("name",c));}catch(Exception ignored){}}
  buildLoading();
  work.execute(()->{try{List<Uri> uris=sharedUris(getIntent());if(uris.isEmpty())throw new Exception("No image was shared");sharedUriCount=uris.size();for(int i=0;i<Math.min(5,uris.size());i++)photos.add(copyAndCompress(uris.get(i),i));main.post(()->{if(!destroyed)buildForm();});}catch(Exception e){main.post(()->{if(!destroyed)fatal(e.getMessage());});}});
 }
 @Override protected void onDestroy(){destroyed=true;if(!saved)cleanupUnsavedPhotos();work.shutdownNow();main.removeCallbacksAndMessages(null);super.onDestroy();}
 private void cleanupUnsavedPhotos(){for(JSONObject p:photos){String path=p.optString("localPath");if(!path.isEmpty())try{new File(path).delete();}catch(Exception ignored){}}}
 private Bitmap previewBitmap(String path){BitmapFactory.Options bounds=new BitmapFactory.Options();bounds.inJustDecodeBounds=true;BitmapFactory.decodeFile(path,bounds);int sample=1;while(Math.max(bounds.outWidth/sample,bounds.outHeight/sample)>700)sample*=2;BitmapFactory.Options opt=new BitmapFactory.Options();opt.inSampleSize=Math.max(1,sample);return BitmapFactory.decodeFile(path,opt);}

 private int dp(float v){return Math.round(v*getResources().getDisplayMetrics().density);}
 private GradientDrawable shape(int color,int radius){GradientDrawable d=new GradientDrawable();d.setColor(color);d.setCornerRadius(dp(radius));d.setStroke(dp(1),BORDER);return d;}
 private LinearLayout col(){LinearLayout l=new LinearLayout(this);l.setOrientation(LinearLayout.VERTICAL);return l;}
 private TextView txt(String s,int size,int color){TextView v=new TextView(this);v.setText(s);v.setTextSize(size);v.setTextColor(color);v.setPadding(0,dp(4),0,dp(4));return v;}
 private Button primary(String s,View.OnClickListener c){Button b=new Button(this);b.setText(s);b.setAllCaps(false);b.setTextSize(15);b.setTextColor(Color.WHITE);b.setBackground(shape(TEAL,13));b.setMinHeight(dp(52));b.setOnClickListener(c);return b;}
 private AutoCompleteTextView auto(String hint){AutoCompleteTextView a=new AutoCompleteTextView(this);a.setHint(hint);a.setTextSize(16);a.setTextColor(NAVY);a.setHintTextColor(MUTED);a.setPadding(dp(14),dp(12),dp(14),dp(12));a.setBackground(shape(SURFACE,12));a.setThreshold(1);a.setSingleLine(true);return a;}
 private EditText input(String hint){EditText e=new EditText(this);e.setHint(hint);e.setTextSize(16);e.setTextColor(NAVY);e.setHintTextColor(MUTED);e.setInputType(InputType.TYPE_CLASS_TEXT|InputType.TYPE_TEXT_FLAG_MULTI_LINE);e.setPadding(dp(14),dp(12),dp(14),dp(12));e.setBackground(shape(SURFACE,12));e.setMinLines(2);return e;}
 private LinearLayout.LayoutParams full(int top){LinearLayout.LayoutParams p=new LinearLayout.LayoutParams(-1,-2);p.setMargins(0,dp(top),0,0);return p;}
 private void toast(String s){Toast.makeText(this,s,Toast.LENGTH_LONG).show();}
 private void fatal(String message){LinearLayout p=col();p.setBackgroundColor(BG);p.setPadding(dp(24),dp(60),dp(24),dp(24));TextView h=txt("Photo order",28,NAVY);h.setTypeface(Typeface.DEFAULT_BOLD);p.addView(h);p.addView(txt(message,16,MUTED),full(16));p.addView(primary("Close",v->finish()),full(24));setContentView(p);}
 private void buildLoading(){LinearLayout p=col();p.setBackgroundColor(BG);p.setGravity(Gravity.CENTER);p.setPadding(dp(24),dp(60),dp(24),dp(24));ProgressBar bar=new ProgressBar(this);p.addView(bar);p.addView(txt("Preparing customer order photo…",16,NAVY),full(18));setContentView(p);}

 private List<Uri> sharedUris(Intent intent){LinkedHashSet<Uri> out=new LinkedHashSet<>();if(intent==null)return new ArrayList<>();if(Intent.ACTION_SEND_MULTIPLE.equals(intent.getAction())){ArrayList<Uri> list=intent.getParcelableArrayListExtra(Intent.EXTRA_STREAM);if(list!=null)out.addAll(list);}else{Uri u=intent.getParcelableExtra(Intent.EXTRA_STREAM);if(u!=null)out.add(u);}ClipData clip=intent.getClipData();if(clip!=null)for(int i=0;i<clip.getItemCount();i++){Uri u=clip.getItemAt(i).getUri();if(u!=null)out.add(u);}return new ArrayList<>(out);}
 private static String hex(byte[] b){StringBuilder s=new StringBuilder();for(byte x:b)s.append(String.format(Locale.US,"%02x",x&255));return s.toString();}
 private Bitmap rotateIfNeeded(Bitmap src,Uri uri){int orientation=ExifInterface.ORIENTATION_NORMAL;try(InputStream in=getContentResolver().openInputStream(uri)){if(in!=null)orientation=new ExifInterface(in).getAttributeInt(ExifInterface.TAG_ORIENTATION,ExifInterface.ORIENTATION_NORMAL);}catch(Exception ignored){}float deg=0;if(orientation==ExifInterface.ORIENTATION_ROTATE_90)deg=90;else if(orientation==ExifInterface.ORIENTATION_ROTATE_180)deg=180;else if(orientation==ExifInterface.ORIENTATION_ROTATE_270)deg=270;if(deg==0)return src;Matrix m=new Matrix();m.postRotate(deg);Bitmap out=Bitmap.createBitmap(src,0,0,src.getWidth(),src.getHeight(),m,true);if(out!=src)src.recycle();return out;}
 private JSONObject copyAndCompress(Uri uri,int index)throws Exception{
  BitmapFactory.Options bounds=new BitmapFactory.Options();bounds.inJustDecodeBounds=true;try(InputStream in=getContentResolver().openInputStream(uri)){if(in==null)throw new Exception("Could not open shared image");BitmapFactory.decodeStream(in,null,bounds);}if(bounds.outWidth<=0||bounds.outHeight<=0)throw new Exception("Shared file is not a readable image");
  int sample=1;while(Math.max(bounds.outWidth/sample,bounds.outHeight/sample)>2200)sample*=2;BitmapFactory.Options opt=new BitmapFactory.Options();opt.inSampleSize=sample;Bitmap bmp;try(InputStream in=getContentResolver().openInputStream(uri)){bmp=BitmapFactory.decodeStream(in,null,opt);}if(bmp==null)throw new Exception("Could not decode shared image");bmp=rotateIfNeeded(bmp,uri);
  int w=bmp.getWidth(),h=bmp.getHeight();float scale=Math.min(1f,1600f/Math.max(w,h));if(scale<1f){Bitmap scaled=Bitmap.createScaledBitmap(bmp,Math.max(1,Math.round(w*scale)),Math.max(1,Math.round(h*scale)),true);if(scaled!=bmp)bmp.recycle();bmp=scaled;}
  ByteArrayOutputStream out=new ByteArrayOutputStream();if(!bmp.compress(Bitmap.CompressFormat.JPEG,84,out))throw new Exception("Could not prepare shared image");if(out.size()>2500000){out.reset();bmp.compress(Bitmap.CompressFormat.JPEG,70,out);}bmp.recycle();byte[] data=out.toByteArray();if(data.length==0||data.length>3*1024*1024)throw new Exception("One order photo is too large. Please share a smaller image.");
  File dir=new File(getFilesDir(),"order-images");if(!dir.exists()&&!dir.mkdirs())throw new Exception("Could not create private order image storage");File file=new File(dir,"photo-"+UUID.randomUUID()+".jpg");try(FileOutputStream fos=new FileOutputStream(file)){fos.write(data);fos.getFD().sync();}
  String sha=hex(MessageDigest.getInstance("SHA-256").digest(data));return new JSONObject().put("name","customer-order-"+(index+1)+".jpg").put("contentType","image/jpeg").put("size",data.length).put("sha256",sha).put("localPath",file.getAbsolutePath()).put("_uploadDirty",true);
 }

 private List<String> companyNames(){List<String> out=new ArrayList<>();for(int i=0;i<companies.length();i++){JSONObject x=companies.optJSONObject(i);String n=x==null?companies.optString(i):x.optString("name");if(!n.isEmpty()&&!out.contains(n))out.add(n);}return out;}
 private void buildForm(){
  LinearLayout root=col();root.setBackgroundColor(BG);root.setPadding(dp(18),dp(18),dp(18),dp(28));ScrollView scroll=new ScrollView(this);LinearLayout body=col();scroll.addView(body);root.addView(scroll,new LinearLayout.LayoutParams(-1,0,1));setContentView(root);
  TextView title=txt("Save photo order",27,NAVY);title.setTypeface(Typeface.DEFAULT_BOLD);body.addView(title);body.addView(txt("Customer image copied into TallyBridge private storage. It works offline and syncs to the PC later.",13,MUTED),full(4));
  HorizontalScrollView strip=new HorizontalScrollView(this);LinearLayout row=new LinearLayout(this);row.setPadding(0,dp(10),0,dp(4));strip.addView(row);for(JSONObject p:photos){ImageView iv=new ImageView(this);iv.setScaleType(ImageView.ScaleType.CENTER_CROP);iv.setBackground(shape(SURFACE,12));Bitmap b=previewBitmap(p.optString("localPath"));iv.setImageBitmap(b);LinearLayout.LayoutParams ip=new LinearLayout.LayoutParams(dp(132),dp(132));ip.setMargins(0,0,dp(10),0);row.addView(iv,ip);}body.addView(strip,full(8));String photoStatus=sharedUriCount>5?("Using first 5 of "+sharedUriCount+" shared photos • maximum 5"):(photos.size()+" photo(s) • maximum 5");body.addView(txt(photoStatus,12,sharedUriCount>5?AMBER:TEAL));
  List<String> names=companyNames();if(names.isEmpty()){body.addView(txt("No saved company is available. Open TallyBridge Orders once while connected, then try sharing the image again.",15,AMBER),full(18));return;}
  body.addView(txt("COMPANY",11,MUTED),full(16));companySpin=new Spinner(this);companySpin.setAdapter(new ArrayAdapter<>(this,android.R.layout.simple_spinner_dropdown_item,names));String selected=store.get("company");int idx=names.indexOf(selected);companySpin.setSelection(Math.max(0,idx));body.addView(companySpin,full(5));
  body.addView(txt("PARTY / CUSTOMER",11,MUTED),full(16));buyer=auto("Search saved Tally buyer or type party name");body.addView(buyer,full(5));buyerInfo=txt("Choose the customer who sent this order image.",12,MUTED);body.addView(buyerInfo);
  body.addView(txt("NOTES (OPTIONAL)",11,MUTED),full(16));notes=input("Colours, delivery date, packing instructions…");body.addView(notes,full(5));status=txt("Saved locally first • PC connection is not required",12,TEAL);status.setTypeface(Typeface.DEFAULT_BOLD);body.addView(status,full(12));body.addView(primary("Save pending photo order",v->savePhotoOrder()),full(12));
  final String[] loaded={""};companySpin.setOnItemSelectedListener(new android.widget.AdapterView.OnItemSelectedListener(){public void onNothingSelected(android.widget.AdapterView<?> p){}public void onItemSelected(android.widget.AdapterView<?> p,View v,int pos,long id){String c=names.get(pos);store.put("company",c);if(!c.equals(loaded[0])){loaded[0]=c;buyer.setText("",false);buyerDetails=new JSONObject();buyerDetailsFor="";buyerInfo.setText("Choose the customer who sent this order image.");loadParties(c);}}});
  buyer.setOnItemClickListener((p,v,pos,id)->loadBuyerDetails(String.valueOf(companySpin.getSelectedItem()),buyer.getText().toString()));buyer.setOnFocusChangeListener((v,has)->{if(!has)loadBuyerDetails(String.valueOf(companySpin.getSelectedItem()),buyer.getText().toString());});
 }
 private void loadParties(String company){
  try{JSONObject cached=offline.masters(company);applyParties(cached);}catch(Exception ignored){}
  work.execute(()->{try{JSONObject x=api.get(BridgeApi.path("orders/masters",company));offline.cacheMasters(company,x);main.post(()->{if(!destroyed)applyParties(x);});}catch(Exception ignored){}});
 }
 private void applyParties(JSONObject x){JSONArray a=x==null?null:x.optJSONArray("parties");List<String> names=new ArrayList<>();if(a!=null)for(int i=0;i<a.length();i++){JSONObject p=a.optJSONObject(i);if(p!=null&&!p.optString("name").isEmpty())names.add(p.optString("name"));}Collections.sort(names,String.CASE_INSENSITIVE_ORDER);buyer.setAdapter(new ArrayAdapter<>(this,android.R.layout.simple_dropdown_item_1line,names));}
 private void loadBuyerDetails(String company,String party){String p=party==null?"":party.trim();if(p.isEmpty())return;try{JSONObject d=offline.buyer(company,p);if(d.length()>0){buyerDetails=d;buyerDetailsFor=p;buyerInfo.setText(summary(d));}}catch(Exception ignored){}work.execute(()->{try{JSONObject x=api.get(BridgeApi.path("ledger-details",company,"ledger",p));JSONObject d=x.optJSONObject("details");if(d==null)d=new JSONObject();offline.cacheBuyer(company,p,d);JSONObject f=d;main.post(()->{if(!destroyed){buyerDetails=f;buyerDetailsFor=p;buyerInfo.setText(summary(f));}});}catch(Exception ignored){}});}
 private String first(JSONObject d,String...keys){if(d==null)return "";for(String k:keys){String v=d.optString(k).trim();if(!v.isEmpty())return v;}return "";}
 private String summary(JSONObject d){String phone=first(d,"MOBILENUMBER","MOBILE","PHONENUMBER","PHONE"),city=first(d,"CITY"),gst=first(d,"PARTYGSTIN","GSTIN");StringBuilder b=new StringBuilder();if(!phone.isEmpty())b.append("Mobile: ").append(phone);if(!city.isEmpty())b.append(b.length()>0?" • ":"").append(city);if(!gst.isEmpty())b.append(b.length()>0?"\n":"").append("GSTIN: ").append(gst);return b.length()==0?"Party selected. Saved contact details are not available.":b.toString();}
 private void savePhotoOrder(){
  try{String company=String.valueOf(companySpin.getSelectedItem()).trim(),party=buyer.getText().toString().trim();if(company.isEmpty()||party.isEmpty())throw new Exception("Select company and party name");if(photos.isEmpty())throw new Exception("No order photo is available");JSONObject details=party.equalsIgnoreCase(buyerDetailsFor)?buyerDetails:new JSONObject();JSONArray attachments=new JSONArray();for(JSONObject p:photos)attachments.put(new JSONObject(p.toString()));JSONObject payload=new JSONObject().put("company",company).put("buyer",party).put("buyerDetails",details).put("items",new JSONArray()).put("photoOrder",true).put("attachments",attachments).put("notes",notes.getText().toString().trim());offline.saveLocal(payload,null);saved=true;store.put("company",company);OrdersWidget.refreshAll(this);toast("Photo order saved on phone");Intent i=new Intent(this,OrdersActivity.class);i.addFlags(Intent.FLAG_ACTIVITY_CLEAR_TOP);startActivity(i);finish();}catch(Exception e){toast(e.getMessage());}
 }
}
