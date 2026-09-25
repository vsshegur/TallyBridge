package in.tallybridge.app;
import android.content.Context;
import android.graphics.*;
import android.view.*;

/** Bounded pinch-to-zoom and drag for the actual PDF page rendered by Android. */
final class PdfPageView extends View {
 private Bitmap bitmap;private final Paint paint=new Paint(Paint.ANTI_ALIAS_FLAG|Paint.FILTER_BITMAP_FLAG);
 private float zoom=1,dx=0,dy=0,lastX,lastY;private final ScaleGestureDetector scale;
 PdfPageView(Context context,Bitmap b){super(context);bitmap=b;setContentDescription("PDF page. Pinch to zoom, drag to pan.");scale=new ScaleGestureDetector(context,new ScaleGestureDetector.SimpleOnScaleGestureListener(){@Override public boolean onScale(ScaleGestureDetector d){zoom=Math.max(1,Math.min(4,zoom*d.getScaleFactor()));clamp();invalidate();return true;}});}
 private float fit(){return bitmap==null?1:Math.min((float)getWidth()/bitmap.getWidth(),(float)getHeight()/bitmap.getHeight());}
 private void clamp(){if(bitmap==null)return;float s=fit()*zoom;float mx=Math.max(0,(bitmap.getWidth()*s-getWidth())/2),my=Math.max(0,(bitmap.getHeight()*s-getHeight())/2);dx=Math.max(-mx,Math.min(mx,dx));dy=Math.max(-my,Math.min(my,dy));}
 @Override protected void onDraw(Canvas c){super.onDraw(c);if(bitmap==null)return;c.drawColor(Color.rgb(220,227,232));clamp();float s=fit()*zoom;c.save();c.translate((getWidth()-bitmap.getWidth()*s)/2+dx,(getHeight()-bitmap.getHeight()*s)/2+dy);c.scale(s,s);c.drawBitmap(bitmap,0,0,paint);c.restore();}
 @Override public boolean onTouchEvent(MotionEvent e){scale.onTouchEvent(e);if(e.getActionMasked()==MotionEvent.ACTION_DOWN){lastX=e.getX();lastY=e.getY();return true;}if(e.getActionMasked()==MotionEvent.ACTION_MOVE){if(!scale.isInProgress()&&e.getPointerCount()==1){dx+=e.getX()-lastX;dy+=e.getY()-lastY;clamp();invalidate();}lastX=e.getX();lastY=e.getY();}if(e.getActionMasked()==MotionEvent.ACTION_UP)performClick();return true;}
 @Override public boolean performClick(){super.performClick();return true;}
 @Override protected void onDetachedFromWindow(){super.onDetachedFromWindow();if(bitmap!=null){bitmap.recycle();bitmap=null;}}
}
