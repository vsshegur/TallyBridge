from pathlib import Path


def replace_once(path, old, new):
    p = Path(path)
    text = p.read_text(encoding="utf-8")
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{path}: expected one match, found {count}: {old[:120]!r}")
    p.write_text(text.replace(old, new, 1), encoding="utf-8")

photo = "android/app/src/main/java/in/tallybridge/app/PhotoOrderActivity.java"
replace_once(photo,
'''for(int i=0;i<Math.min(5,uris.size());i++)photos.add(copyAndCompress(uris.get(i),i));''',
'''for(int i=0;i<Math.min(5,uris.size());i++)addPreparedPhoto(copyAndCompress(uris.get(i),i));''')

replace_once(photo,
''' @Override protected void onDestroy(){destroyed=true;if(!saved)cleanupUnsavedPhotos();work.shutdownNow();main.removeCallbacksAndMessages(null);super.onDestroy();}\n private void cleanupUnsavedPhotos(){for(JSONObject p:photos){String path=p.optString("localPath");if(!path.isEmpty())try{new File(path).delete();}catch(Exception ignored){}}}\n private Bitmap previewBitmap(String path){''',
''' @Override protected void onDestroy(){destroyed=true;work.shutdownNow();if(!saved)cleanupUnsavedPhotos();main.removeCallbacksAndMessages(null);super.onDestroy();}\n private void deleteLocalPhoto(JSONObject p){if(p==null)return;String path=p.optString("localPath");if(!path.isEmpty())try{new File(path).delete();}catch(Exception ignored){}}\n private void cleanupUnsavedPhotos(){synchronized(photos){for(JSONObject p:photos)deleteLocalPhoto(p);photos.clear();}}\n private void ensurePhotoWorkActive()throws InterruptedException{if(destroyed||Thread.currentThread().isInterrupted())throw new InterruptedException("Photo order closed");}\n private void addPreparedPhoto(JSONObject p)throws Exception{synchronized(photos){if(destroyed||Thread.currentThread().isInterrupted()){deleteLocalPhoto(p);throw new InterruptedException("Photo order closed");}photos.add(p);}}\n private Bitmap previewBitmap(String path){''')

replace_once(photo,
''' private JSONObject copyAndCompress(Uri uri,int index)throws Exception{\n  BitmapFactory.Options bounds=new BitmapFactory.Options();''',
''' private JSONObject copyAndCompress(Uri uri,int index)throws Exception{\n  ensurePhotoWorkActive();\n  BitmapFactory.Options bounds=new BitmapFactory.Options();''')

replace_once(photo,
'''  ByteArrayOutputStream out=new ByteArrayOutputStream();if(!bmp.compress(Bitmap.CompressFormat.JPEG,84,out))throw new Exception("Could not prepare shared image");if(out.size()>2500000){out.reset();bmp.compress(Bitmap.CompressFormat.JPEG,70,out);}bmp.recycle();byte[] data=out.toByteArray();if(data.length==0||data.length>3*1024*1024)throw new Exception("One order photo is too large. Please share a smaller image.");\n  File dir=new File(getFilesDir(),"order-images");''',
'''  ByteArrayOutputStream out=new ByteArrayOutputStream();if(!bmp.compress(Bitmap.CompressFormat.JPEG,84,out))throw new Exception("Could not prepare shared image");if(out.size()>2500000){out.reset();bmp.compress(Bitmap.CompressFormat.JPEG,70,out);}bmp.recycle();byte[] data=out.toByteArray();if(data.length==0||data.length>3*1024*1024)throw new Exception("One order photo is too large. Please share a smaller image.");\n  ensurePhotoWorkActive();\n  File dir=new File(getFilesDir(),"order-images");''')

replace_once(photo,
'''  File dir=new File(getFilesDir(),"order-images");if(!dir.exists()&&!dir.mkdirs())throw new Exception("Could not create private order image storage");File file=new File(dir,"photo-"+UUID.randomUUID()+".jpg");try(FileOutputStream fos=new FileOutputStream(file)){fos.write(data);fos.getFD().sync();}\n  String sha=hex(MessageDigest.getInstance("SHA-256").digest(data));return new JSONObject().put("name","customer-order-"+(index+1)+".jpg").put("contentType","image/jpeg").put("size",data.length).put("sha256",sha).put("localPath",file.getAbsolutePath()).put("_uploadDirty",true);''',
'''  File dir=new File(getFilesDir(),"order-images");if(!dir.exists()&&!dir.mkdirs())throw new Exception("Could not create private order image storage");File file=new File(dir,"photo-"+UUID.randomUUID()+".jpg");try(FileOutputStream fos=new FileOutputStream(file)){fos.write(data);fos.getFD().sync();}if(destroyed||Thread.currentThread().isInterrupted()){file.delete();throw new InterruptedException("Photo order closed");}\n  String sha=hex(MessageDigest.getInstance("SHA-256").digest(data));return new JSONObject().put("name","customer-order-"+(index+1)+".jpg").put("contentType","image/jpeg").put("size",data.length).put("sha256",sha).put("localPath",file.getAbsolutePath()).put("_uploadDirty",true);''')

print("Final Orders 2.3.1 Android lifecycle patch applied")
