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
 case "Ledgers":c.drawRoundRect(4,3,20,21,2,2,paint);line(c,8,3,8,21);line(c,12,8,16,8);line(c,12,12,16,12);break;
 case "Due":c.drawCircle(12,12,9,paint);line(c,12,7,12,12,16,14);break;
 case "Reports":line(c,4,3,4,21,21,21);line(c,8,17,8,12);line(c,13,17,13,8);line(c,18,17,18,5);break;
 case "More":c.drawCircle(5,12,1,paint);c.drawCircle(12,12,1,paint);c.drawCircle(19,12,1,paint);break;
 case "back":line(c,14,5,7,12,14,19);break;
 case "next":line(c,9,5,16,12,9,19);break;
 case "refresh":c.drawArc(4,4,20,20,45,290,false,paint);line(c,20,4,20,10,14,10);break;
 case "search":c.drawCircle(10,10,6,paint);line(c,15,15,21,21);break;
 case "down":line(c,6,9,12,15,18,9);break;
 case "receive":line(c,18,6,6,18,6,9);line(c,6,18,15,18);break;
 case "pay":line(c,6,18,18,6,9,6);line(c,18,6,18,15);break;
 case "star":line(c,12,3,15,9,22,10,17,15,18,22,12,18,6,22,7,15,2,10,9,9,12,3);break;
 case "lock":c.drawRoundRect(5,10,19,21,2,2,paint);c.drawArc(8,2,16,15,180,180,false,paint);line(c,12,14,12,17);break;
 case "share":c.drawCircle(5,12,3,paint);c.drawCircle(19,5,3,paint);c.drawCircle(19,19,3,paint);line(c,8,11,16,6);line(c,8,13,16,18);break;
 default:c.drawRoundRect(5,3,19,21,2,2,paint);line(c,9,8,15,8);line(c,9,12,15,12);line(c,9,16,13,16);
 }c.restore();}
}
