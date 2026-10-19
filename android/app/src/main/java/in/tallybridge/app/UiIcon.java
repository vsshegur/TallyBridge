package in.tallybridge.app;

import android.content.Context;
import android.graphics.Canvas;
import android.graphics.Paint;
import android.graphics.Path;
import android.view.View;

/** Resolution-independent native line icons; no font or emoji dependency. */
final class UiIcon extends View {
 private final String name;
 private final Paint paint=new Paint(Paint.ANTI_ALIAS_FLAG);
 UiIcon(Context context,String name,int color){super(context);this.name=name;paint.setColor(color);paint.setStyle(Paint.Style.STROKE);paint.setStrokeWidth(1.8f);paint.setStrokeCap(Paint.Cap.ROUND);paint.setStrokeJoin(Paint.Join.ROUND);setImportantForAccessibility(IMPORTANT_FOR_ACCESSIBILITY_NO);}
 private void line(Canvas c,float... xy){Path p=new Path();p.moveTo(xy[0],xy[1]);for(int i=2;i<xy.length;i+=2)p.lineTo(xy[i],xy[i+1]);c.drawPath(p,paint);}
 @Override protected void onDraw(Canvas c){super.onDraw(c);c.save();float size=Math.min(getWidth(),getHeight());c.translate((getWidth()-size)/2,(getHeight()-size)/2);c.scale(size/24,size/24);
 switch(name){
 case "Home":line(c,3,10,12,3,21,10);line(c,5,9,5,21,10,21,10,14,14,14,14,21,19,21,19,9);break;
 case "Orders":c.drawRoundRect(5,4,19,21,2,2,paint);line(c,9,4,9,2,15,2,15,4);line(c,9,9,16,9);line(c,9,13,16,13);line(c,9,17,14,17);break;
 case "Ledgers":c.drawRoundRect(4,3,20,21,2,2,paint);line(c,8,3,8,21);line(c,12,8,16,8);line(c,12,12,16,12);break;
 case "Due":c.drawCircle(12,12,9,paint);line(c,12,7,12,12,16,14);break;
 case "Reports":line(c,4,3,4,21,21,21);line(c,8,17,8,12);line(c,13,17,13,8);line(c,18,17,18,5);break;
 case "Tools":c.drawCircle(7,17,2.5f,paint);line(c,9,15,18,6);c.drawArc(14,2,22,10,35,220,false,paint);line(c,17,7,21,3);break;
 case "System":c.drawCircle(12,12,4,paint);c.drawCircle(12,12,9,paint);line(c,12,1,12,4);line(c,12,20,12,23);line(c,1,12,4,12);line(c,20,12,23,12);line(c,4,4,6,6);line(c,18,18,20,20);line(c,18,6,20,4);line(c,4,20,6,18);break;
 case "bill":line(c,6,3,18,3,18,21,15.5f,19.5f,13,21,10.5f,19.5f,8,21,6,19.5f,6,3);line(c,9,8,15,8);line(c,9,12,15,12);line(c,9,16,12,16);break;
 case "Proforma":c.drawRoundRect(5,3,19,21,2,2,paint);line(c,8,8,16,8);line(c,8,12,16,12);line(c,8,16,13,16);break;
 case "Payments":c.drawRoundRect(3,6,21,18,2,2,paint);line(c,3,10,21,10);c.drawCircle(17,14,1.2f,paint);break;
 case "health":c.drawCircle(12,12,9,paint);line(c,8,12,11,15,17,8);break;
 case "diag":line(c,4,18,8,14,11,16,16,8,20,10);line(c,4,21,20,21);break;
 case "qr":c.drawRect(4,4,10,10,paint);c.drawRect(14,4,20,10,paint);c.drawRect(4,14,10,20,paint);line(c,14,14,17,14,17,17,20,17,20,20,14,20);break;
 case "More":c.drawCircle(5,12,1,paint);c.drawCircle(12,12,1,paint);c.drawCircle(19,12,1,paint);break;
 case "Parties":c.drawCircle(9,8,3.5f,paint);c.drawArc(2.5f,14,15.5f,26,180,180,false,paint);c.drawCircle(17,9,2.5f,paint);c.drawArc(14,15,22,23,250,110,false,paint);break;
 case "plus":c.drawCircle(12,12,9,paint);line(c,12,8,12,16);line(c,8,12,16,12);break;
 case "mic":c.drawRoundRect(9,3,15,14,3,3,paint);c.drawArc(5,6,19,18,0,180,false,paint);line(c,12,18,12,21);line(c,9,21,15,21);break;
 case "work":c.drawRoundRect(5,4,19,21,2,2,paint);line(c,9,4,9,2.5f,15,2.5f,15,4);line(c,8.5f,13,11,15.5f,15.5f,10);break;
 case "widget":c.drawRoundRect(4,4,11,11,2,2,paint);c.drawRoundRect(13,4,20,11,2,2,paint);c.drawRoundRect(4,13,11,20,2,2,paint);c.drawRoundRect(13,13,20,20,2,2,paint);break;
 case "camera":c.drawRoundRect(3,7,21,19,2,2,paint);c.drawCircle(12,13,3.5f,paint);line(c,8,7,9.5f,4.5f,14.5f,4.5f,16,7);break;
 case "truck":line(c,3,6,14,6,14,17,3,17,3,6);line(c,14,10,18,10,21,13,21,17,14,17);c.drawCircle(7,18,1.8f,paint);c.drawCircle(17.5f,18,1.8f,paint);break;
 case "back":line(c,14,5,7,12,14,19);break;
 case "next":line(c,9,5,16,12,9,19);break;
 case "refresh":c.drawArc(4,4,20,20,45,290,false,paint);line(c,20,4,20,10,14,10);break;
 case "search":c.drawCircle(10,10,6,paint);line(c,15,15,21,21);break;
 case "calculator":c.drawRoundRect(5,3,19,21,2,2,paint);line(c,8,7,16,7);line(c,8,11,9,11);line(c,12,11,13,11);line(c,16,11,17,11);line(c,8,15,9,15);line(c,12,15,13,15);line(c,16,15,17,15);break;
 case "down":line(c,6,9,12,15,18,9);break;
 case "receive":line(c,18,6,6,18,6,9);line(c,6,18,15,18);break;
 case "pay":line(c,6,18,18,6,9,6);line(c,18,6,18,15);break;
 case "star":line(c,12,3,15,9,22,10,17,15,18,22,12,18,6,22,7,15,2,10,9,9,12,3);break;
 case "lock":c.drawRoundRect(5,10,19,21,2,2,paint);c.drawArc(8,2,16,15,180,180,false,paint);line(c,12,14,12,17);break;
 case "share":c.drawCircle(5,12,3,paint);c.drawCircle(19,5,3,paint);c.drawCircle(19,19,3,paint);line(c,8,11,16,6);line(c,8,13,16,18);break;
 default:c.drawRoundRect(5,3,19,21,2,2,paint);line(c,9,8,15,8);line(c,9,12,15,12);line(c,9,16,13,16);
 }c.restore();}
}
