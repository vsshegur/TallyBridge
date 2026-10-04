from pathlib import Path
root=Path('.')
p=root/'android/app/src/androidTest/java/in/tallybridge/app/OrdersOfflineTest.java';s=p.read_text()
s=s.replace('''   ((TextView)widget.findViewById(R.id.widget_pending)).setText("2 pending orders");
   ((TextView)widget.findViewById(R.id.widget_mode)).setText("OFFLINE • PHONE COPY");
   ((TextView)widget.findViewById(R.id.widget_names)).setText("Raj Fabrics • Mahesh Textiles");
   int w=Math.round(360*a.getResources().getDisplayMetrics().density),h=Math.round(190*a.getResources().getDisplayMetrics().density);''','''   ((TextView)widget.findViewById(R.id.widget_pending)).setText("2");
   ((TextView)widget.findViewById(R.id.widget_mode)).setText("PHONE OFFLINE");
   ((TextView)widget.findViewById(R.id.widget_party1)).setText("Raj Fabrics");
   ((TextView)widget.findViewById(R.id.widget_party2)).setText("Mahesh Textiles");
   ((TextView)widget.findViewById(R.id.widget_party3)).setVisibility(View.GONE);
   ((TextView)widget.findViewById(R.id.widget_more)).setVisibility(View.GONE);
   int w=Math.round(360*a.getResources().getDisplayMetrics().density),h=Math.round(220*a.getResources().getDisplayMetrics().density);''')
s=s.replace('''  capture("21-orders-widget");

  ui(()->rule.getActivity().finish());''','''  capture("21-orders-widget");
  ui(()->{OrdersActivity a=rule.getActivity();FrameLayout stage=new FrameLayout(a);stage.setBackgroundColor(Color.rgb(232,237,242));View widget=a.getLayoutInflater().inflate(R.layout.quick_pi_widget,stage,false);int size=Math.round(104*a.getResources().getDisplayMetrics().density);FrameLayout.LayoutParams lp=new FrameLayout.LayoutParams(size,size,Gravity.CENTER);stage.addView(widget,lp);a.setContentView(stage);});
  capture("29-widget-proforma-1x1");
  ui(()->{OrdersActivity a=rule.getActivity();FrameLayout stage=new FrameLayout(a);stage.setBackgroundColor(Color.rgb(232,237,242));View widget=a.getLayoutInflater().inflate(R.layout.quick_order_widget,stage,false);int size=Math.round(104*a.getResources().getDisplayMetrics().density);FrameLayout.LayoutParams lp=new FrameLayout.LayoutParams(size,size,Gravity.CENTER);stage.addView(widget,lp);a.setContentView(stage);});
  capture("30-widget-order-1x1");

  ui(()->rule.getActivity().finish());''')
old='''  ui(()->{OrdersActivity a=rule.getActivity();assertEquals(R.layout.orders_widget_dark,OrdersWidget.previewLayout(a));FrameLayout stage=new FrameLayout(a);stage.setBackgroundColor(Color.rgb(15,22,29));View widget=a.getLayoutInflater().inflate(R.layout.orders_widget_dark,stage,false);((TextView)widget.findViewById(R.id.widget_pending)).setText("2 pending orders");((TextView)widget.findViewById(R.id.widget_mode)).setText("OFFLINE • PHONE COPY");((TextView)widget.findViewById(R.id.widget_names)).setText("Raj Fabrics • Mahesh Textiles");int w=Math.round(360*a.getResources().getDisplayMetrics().density),h=Math.round(205*a.getResources().getDisplayMetrics().density);FrameLayout.LayoutParams lp=new FrameLayout.LayoutParams(w,h,Gravity.CENTER);stage.addView(widget,lp);a.setContentView(stage);});'''
new='''  ui(()->{OrdersActivity a=rule.getActivity();assertEquals(R.layout.orders_widget_dark,OrdersWidget.previewLayout(a));FrameLayout stage=new FrameLayout(a);stage.setBackgroundColor(Color.rgb(15,22,29));View widget=a.getLayoutInflater().inflate(R.layout.orders_widget_dark,stage,false);((TextView)widget.findViewById(R.id.widget_pending)).setText("2");((TextView)widget.findViewById(R.id.widget_mode)).setText("PHONE OFFLINE");((TextView)widget.findViewById(R.id.widget_party1)).setText("Raj Fabrics");((TextView)widget.findViewById(R.id.widget_party2)).setText("Mahesh Textiles");((TextView)widget.findViewById(R.id.widget_party3)).setVisibility(View.GONE);((TextView)widget.findViewById(R.id.widget_more)).setVisibility(View.GONE);int w=Math.round(360*a.getResources().getDisplayMetrics().density),h=Math.round(220*a.getResources().getDisplayMetrics().density);FrameLayout.LayoutParams lp=new FrameLayout.LayoutParams(w,h,Gravity.CENTER);stage.addView(widget,lp);a.setContentView(stage);});'''
assert old in s;s=s.replace(old,new);p.write_text(s)

p=root/'android/app/src/androidTest/java/in/tallybridge/app/DesignTest.java';s=p.read_text()
old='''   field("boot",new JSONObject().put("tallyOnline",true).put("autoSync",new JSONObject().put("lastOk","2026-09-27T08:00:00Z")).put("lastSuccess","2026-09-27T08:00:00Z").put("summaries",new JSONObject().put("Vijayalaxmi Textiles • Solapur",summary)));
   call("renderHome");'''
new='''   field("boot",new JSONObject().put("tallyOnline",true).put("autoSync",new JSONObject().put("lastOk","2026-09-27T08:00:00Z").put("lastError","Previous report warning")).put("lastSuccess","2026-09-27T08:00:00Z").put("summaries",new JSONObject().put("Vijayalaxmi Textiles • Solapur",summary)));
   call("renderHome");
   Field status=MainActivity.class.getDeclaredField("statusView");status.setAccessible(true);TextView statusText=(TextView)status.get(rule.getActivity());assertNotNull(statusText);assertTrue(statusText.getText().toString().startsWith("Connected"));assertFalse(statusText.getText().toString().contains("needs attention"));'''
assert old in s;s=s.replace(old,new);p.write_text(s)
