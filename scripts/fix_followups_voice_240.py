from pathlib import Path


def rep(path, old, new, count=1):
    p=Path(path); s=p.read_text(encoding='utf-8')
    if s.count(old) < count:
        raise SystemExit(f'{path}: anchor missing: {old[:120]!r}')
    s=s.replace(old,new,count); p.write_text(s,encoding='utf-8')

# Android Voice Order: preserve spoken units and suppress overlapping product-name matches.
p=Path('android/app/src/main/java/in/tallybridge/app/OrdersActivity.java'); s=p.read_text(encoding='utf-8')
start=s.find(' private double voiceQty(String lower,String item){\n')
end=s.find(' private void refreshBootstrapSilently(){',start)
if start<0 or end<0: raise SystemExit('voice parser anchors missing')
voice=r''' private static final class VoiceAmount { final double quantity; final String unit; VoiceAmount(double q,String u){quantity=q;unit=u;} }
 private VoiceAmount voiceAmount(String lower,String item,String defaultUnit){
  String q=java.util.regex.Pattern.quote(item.toLowerCase(Locale.ROOT));
  java.util.regex.Pattern[] ps=new java.util.regex.Pattern[]{java.util.regex.Pattern.compile("([0-9]+(?:[.][0-9]+)?)\\s*(dozen|dz|pcs?|pieces?)?\\s+(?:of\\s+)?"+q),java.util.regex.Pattern.compile(q+"\\s*[-,:]?\\s*([0-9]+(?:[.][0-9]+)?)\\s*(dozen|dz|pcs?|pieces?)?")};
  for(java.util.regex.Pattern p:ps){java.util.regex.Matcher m=p.matcher(lower);if(m.find())try{double n=Double.parseDouble(m.group(1));String u=m.group(2);if(u==null||u.trim().isEmpty())u=defaultUnit;else{u=u.toLowerCase(Locale.ROOT);if(u.equals("dz")||u.startsWith("dozen"))u="dozen";else if(u.startsWith("pc")||u.startsWith("piece"))u="pcs";}return new VoiceAmount(n,u==null?"":u);}catch(Exception ignored){}}
  return new VoiceAmount(1,defaultUnit==null?"":defaultUnit);
 }
 private void applyVoiceOrder(String spoken){
  if(voiceNotes!=null&&!quickMode)voiceNotes.setText(spoken);String lower=spoken.toLowerCase(Locale.ROOT);
  if(voiceBuyer!=null&&parties!=null){String best="";for(int i=0;i<parties.length();i++){JSONObject p=parties.optJSONObject(i);String n=p==null?parties.optString(i):p.optString("name");if(!n.isEmpty()&&lower.contains(n.toLowerCase(Locale.ROOT))&&n.length()>best.length())best=n;}if(!best.isEmpty())voiceBuyer.setText(best,false);}
  List<JSONObject> matches=new ArrayList<>();if(stock!=null){List<JSONObject> candidates=new ArrayList<>();for(int i=0;i<stock.length();i++){JSONObject x=stock.optJSONObject(i);if(x!=null&&!x.optString("name").isEmpty()&&lower.contains(x.optString("name").toLowerCase(Locale.ROOT)))candidates.add(x);}candidates.sort((x,y)->Integer.compare(y.optString("name").length(),x.optString("name").length()));String remaining=lower;for(JSONObject x:candidates){String n=x.optString("name").toLowerCase(Locale.ROOT);int at=remaining.indexOf(n);if(at<0)continue;matches.add(x);remaining=remaining.substring(0,at)+String.join("",Collections.nCopies(n.length()," "))+remaining.substring(at+n.length());}}
  int added=0;for(JSONObject matched:matches){String name=matched.optString("name"),defaultUnit=matched.optString("unit");VoiceAmount va=voiceAmount(lower,name,defaultUnit);JSONObject existing=new JSONObject();try{existing.put("name",name).put("quantity",va.quantity).put("unit",va.unit);}catch(Exception ignored){}if(added==0&&itemRows.size()==1&&itemRows.get(0).item.getText().toString().trim().isEmpty()){ItemRow r=itemRows.get(0);r.item.setText(name,false);r.qty.setText(qty(va.quantity));r.unit.setText(va.unit);}else if(voiceItemsBox!=null)addItemRow(voiceItemsBox,existing);added++;}
  toast(added>0?("Voice order captured • "+added+" item"+(added==1?"":"s")+" found • review before saving"):"Voice captured • choose item and quantity before saving");
 }
'''
s=s[:start]+voice+s[end:]
s=s.replace('i.putExtra(RecognizerIntent.EXTRA_PROMPT,"Say party, item and quantity")','i.putExtra(RecognizerIntent.EXTRA_PROMPT,"Say party, items and quantities")',1)
p.write_text(s,encoding='utf-8')

# Android follow-ups: explicit edit/contact actions; customer-facing reminder excludes internal notes.
work='android/app/src/main/java/in/tallybridge/app/WorkActivity.java'
p=Path(work); s=p.read_text(encoding='utf-8')
old=''' private void followUpCard(JSONObject f){LinearLayout c=col();c.setPadding(dp(14),dp(12),dp(14),dp(12));c.setBackground(shape(SURFACE,13));TextView t=text(f.optString("party"),15,NAVY);t.setTypeface(null,Typeface.BOLD);c.addView(t);String detail=(f.optString("promiseDate").isEmpty()?"No promise date":"Promise "+f.optString("promiseDate"))+(f.optString("note").isEmpty()?"":" • "+f.optString("note"));c.addView(text(detail,12,MUTED));if(f.optDouble("amount")>0)c.addView(text(amount(f.optDouble("amount")),14,TEAL));LinearLayout actions=new LinearLayout(this);Button customer=button("Customer",v->showCustomer(f.optString("party")));Button share=button("Share reminder",v->shareReminder(f));Button done=button("Done",v->completeFollowUp(f));actions.addView(customer,new LinearLayout.LayoutParams(0,-2,1));actions.addView(share,new LinearLayout.LayoutParams(0,-2,1));actions.addView(done,new LinearLayout.LayoutParams(0,-2,1));c.addView(actions,full(7));body.addView(c,full(7));}
'''
new=''' private void followUpCard(JSONObject f){LinearLayout c=col();c.setPadding(dp(14),dp(12),dp(14),dp(12));c.setBackground(shape(SURFACE,13));c.setOnClickListener(v->showCustomer(f.optString("party")));TextView t=text(f.optString("party"),15,NAVY);t.setTypeface(null,Typeface.BOLD);c.addView(t);String last=f.optString("lastContacted"),lastDay=last.length()>=10?last.substring(0,10):last;String detail=(f.optString("promiseDate").isEmpty()?"No promise date":"Promise "+f.optString("promiseDate"))+(lastDay.isEmpty()?"":" • Last contacted "+lastDay)+(f.optString("note").isEmpty()?"":" • "+f.optString("note"));c.addView(text(detail,12,MUTED));if(f.optDouble("amount")>0)c.addView(text(amount(f.optDouble("amount")),14,TEAL));LinearLayout actions=new LinearLayout(this);Button edit=button("Edit",v->followUpDialog(f.optString("party"),f));Button share=button("Reminder",v->shareReminder(f));Button contacted=button("Contacted",v->markContacted(f));Button done=button("Done",v->completeFollowUp(f));for(Button b:new Button[]{edit,share,contacted,done})actions.addView(b,new LinearLayout.LayoutParams(0,-2,1));c.addView(actions,full(7));body.addView(c,full(7));}
'''
if old not in s: raise SystemExit('Android followUpCard anchor missing')
s=s.replace(old,new,1)
old=''' private void completeFollowUp(JSONObject f){try{JSONObject q=new JSONObject(f.toString()).put("status","completed");post("/api/v1/mobile/followups",q,this::showFollowUps);}catch(Exception e){toast(e.getMessage());}}
 private void shareReminder(JSONObject f){StringBuilder b=new StringBuilder("Payment follow-up\\nCustomer: ").append(f.optString("party"));if(f.optDouble("amount")>0)b.append("\\nAmount: ").append(amount(f.optDouble("amount")));if(!f.optString("promiseDate").isEmpty())b.append("\\nPromise date: ").append(f.optString("promiseDate"));if(!f.optString("note").isEmpty())b.append("\\nNote: ").append(f.optString("note"));try{startActivity(Intent.createChooser(new Intent(Intent.ACTION_SEND).setType("text/plain").putExtra(Intent.EXTRA_TEXT,b.toString()),"Share follow-up"));}catch(Exception e){toast("No sharing app available");}}
'''
new=''' private void markContacted(JSONObject f){try{JSONObject q=new JSONObject(f.toString()).put("lastContacted",java.time.Instant.now().toString()).put("status","open");post("/api/v1/mobile/followups",q,this::showFollowUps);}catch(Exception e){toast(e.getMessage());}}
 private void completeFollowUp(JSONObject f){try{JSONObject q=new JSONObject(f.toString()).put("status","completed");post("/api/v1/mobile/followups",q,this::showFollowUps);}catch(Exception e){toast(e.getMessage());}}
 private void shareReminder(JSONObject f){StringBuilder b=new StringBuilder("Hello ").append(f.optString("party")).append(",\\n\\nPayment reminder from ").append(company).append(".");if(f.optDouble("amount")>0)b.append("\\nPending amount: ").append(amount(f.optDouble("amount")));if(!f.optString("promiseDate").isEmpty())b.append("\\nPromised date: ").append(f.optString("promiseDate"));b.append("\\n\\nPlease let us know the payment status. Thank you.");try{startActivity(Intent.createChooser(new Intent(Intent.ACTION_SEND).setType("text/plain").putExtra(Intent.EXTRA_TEXT,b.toString()),"Share payment reminder"));}catch(Exception e){toast("No sharing app available");}}
'''
if old not in s: raise SystemExit('Android followup action anchor missing')
s=s.replace(old,new,1);p.write_text(s,encoding='utf-8')

# Desktop follow-ups: edit, mark contacted, copy customer-safe reminder.
p=Path('internal/app/web/operations.js'); s=p.read_text(encoding='utf-8')
s=s.replace("let opsCompany=localStorage.getItem('tbOpsCompany')||'', workData=null, searchData=null, followData=null;","let opsCompany=localStorage.getItem('tbOpsCompany')||'', workData=null, searchData=null, followData=null, followIndex={};",1)
old=''' function followRow(f){return `<div class="list-row"><div><div class="title">${esc(f.party)}</div><div class="sub">${esc(f.promiseDate?'Promise '+f.promiseDate:'No promise date')}${f.note?' • '+esc(f.note):''}</div></div><div>${f.amount?`<b>${money(f.amount)}</b>`:''}<button class="btn secondary" data-follow-done="${esc(f.id)}" style="margin-left:8px">Done</button></div></div>`}
'''
new=''' function followRow(f){followIndex[f.id]=f;const last=f.lastContacted?String(f.lastContacted).slice(0,10):'';return `<div class="list-row"><div><div class="title">${esc(f.party)}</div><div class="sub">${esc(f.promiseDate?'Promise '+f.promiseDate:'No promise date')}${last?' • Last contacted '+esc(last):''}${f.note?' • '+esc(f.note):''}</div>${f.amount?`<b>${money(f.amount)}</b>`:''}</div><div class="toolbar"><button class="btn secondary" data-follow-edit="${esc(f.id)}">Edit</button><button class="btn secondary" data-follow-share="${esc(f.id)}">Reminder</button><button class="btn secondary" data-follow-contact="${esc(f.id)}">Contacted</button><button class="btn secondary" data-follow-done="${esc(f.id)}">Done</button></div></div>`}
'''
if old not in s: raise SystemExit('desktop followRow anchor missing')
s=s.replace(old,new,1)
old=""" async function saveFollow(party=''){const p=prompt('Customer / ledger name',party);if(!p)return;const amt=prompt('Amount (optional)','')??'';const date=prompt('Promise date YYYY-MM-DD (optional)','')??'';const note=prompt('Note / what customer said','')??'';try{await api('/api/v1/admin/followups',{method:'POST',body:JSON.stringify({company:opsCompany,party:p.trim(),amount:Number(amt||0),promiseDate:date.trim(),note:note.trim(),status:'open'})});followData=null;toast('Follow-up saved');renderFollowups()}catch(e){toast(e.message,true)}}
 async function complete(id){const f=(followData?.open||[]).find(x=>x.id===id);if(!f)return;try{await api('/api/v1/admin/followups',{method:'POST',body:JSON.stringify({...f,status:'completed'})});followData=null;toast('Follow-up completed');renderFollowups()}catch(e){toast(e.message,true)}}
"""
new=""" function reminderText(f){let t=`Hello ${f.party},\\n\\nPayment reminder from ${opsCompany}.`;if(Number(f.amount||0)>0)t+=`\\nPending amount: ${money(f.amount)}`;if(f.promiseDate)t+=`\\nPromised date: ${f.promiseDate}`;return t+'\\n\\nPlease let us know the payment status. Thank you.'}
 async function saveFollow(party='',original=null){const p=prompt('Customer / ledger name',original?.party||party);if(!p)return;const amt=prompt('Amount (optional)',original?.amount||'')??'';const date=prompt('Promise date YYYY-MM-DD (optional)',original?.promiseDate||'')??'';const note=prompt('Internal note / what customer said',original?.note||'')??'';const payload={...(original||{}),company:opsCompany,party:p.trim(),amount:Number(amt||0),promiseDate:date.trim(),note:note.trim(),status:'open'};try{await api('/api/v1/admin/followups',{method:'POST',body:JSON.stringify(payload)});followData=null;toast(original?'Follow-up updated':'Follow-up saved');route='followups';location.hash='followups';renderFollowups()}catch(e){toast(e.message,true)}}
 async function contacted(id){const f=followIndex[id];if(!f)return;try{await api('/api/v1/admin/followups',{method:'POST',body:JSON.stringify({...f,status:'open',lastContacted:new Date().toISOString()})});followData=null;toast('Contact time updated');renderFollowups()}catch(e){toast(e.message,true)}}
 async function copyReminder(id){const f=followIndex[id];if(!f)return;try{await navigator.clipboard.writeText(reminderText(f));toast('Payment reminder copied')}catch(e){toast('Could not copy reminder',true)}}
 async function complete(id){const f=followIndex[id];if(!f)return;try{await api('/api/v1/admin/followups',{method:'POST',body:JSON.stringify({...f,status:'completed'})});followData=null;toast('Follow-up completed');renderFollowups()}catch(e){toast(e.message,true)}}
"""
if old not in s: raise SystemExit('desktop followup functions anchor missing')
s=s.replace(old,new,1)
old="""const d=e.target.closest('[data-follow-done]');if(d){complete(d.dataset.followDone);return}});"""
new="""const ed=e.target.closest('[data-follow-edit]');if(ed){saveFollow('',followIndex[ed.dataset.followEdit]);return}const sh=e.target.closest('[data-follow-share]');if(sh){copyReminder(sh.dataset.followShare);return}const ct=e.target.closest('[data-follow-contact]');if(ct){contacted(ct.dataset.followContact);return}const d=e.target.closest('[data-follow-done]');if(d){complete(d.dataset.followDone);return}});"""
if old not in s: raise SystemExit('desktop followup click anchor missing')
s=s.replace(old,new,1);p.write_text(s,encoding='utf-8')
print('follow-up and voice-order hardening applied')
