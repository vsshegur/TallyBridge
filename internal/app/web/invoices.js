// Purchase bills: TallyBridge as the InvoiceTotally bridge + bills sent from the phone.
(()=>{
  I.bills='<path d="M6 3h12v18l-3-2-3 2-3-2-3 2z"/><path d="M9 8h6M9 12h6M9 16h3"/>';
  let data=null, loading=false, timer=null, adding=false, editUrl=false;

  function installNav(){
    const side=document.querySelector('.side-nav');
    if(!side||side.querySelector('[data-route="bills"]'))return;
    const b=document.createElement('button');
    b.className='side-btn'+(route==='bills'?' active':'');b.dataset.route='bills';
    b.innerHTML=icon('bills')+'<span>InvoiceTotally</span>';
    const after=side.querySelector('[data-route="orders"]')||side.querySelector('[data-route="companies"]');
    if(after)after.insertAdjacentElement('afterend',b);else side.appendChild(b);
  }
  function versionChip(){const chip=document.querySelector('.version-chip');if(chip&&snap?.version)chip.textContent='v'+snap.version+' • phone access is read-only'}

  const baseRender=render;
  render=function(){
    installNav();versionChip();
    if(route!=='bills'){stop();baseRender();return;}
    $$('.side-btn').forEach(b=>b.classList.toggle('active',b.dataset.route===route));
    page();lastRenderKey='bills';
  };
  function stop(){if(timer){clearInterval(timer);timer=null}}

  const ago=v=>fmtAgo(v);
  const pill=(kind,text)=>`<span class="status-pill ${kind}"><span class="dot"></span>${esc(text)}</span>`;

  function page(){
    const p=$('#page');if(!p)return;
    p.innerHTML=head('InvoiceTotally','Purchase bills from the phone, bank statements and the GSTR-2B check are reviewed in InvoiceTotally. This PC reads Tally for them and posts only what you approve.')+'<div id="billsBody"><div class="screen-loader"><span class="spinner"></span> Loading…</div></div>';
    load();
    stop();timer=setInterval(()=>{if(route==='bills'&&!adding&&!document.querySelector('#billsBody input:focus,#billsBody textarea:focus'))load()},4000);
  }

  async function load(){
    if(loading)return;loading=true;
    try{data=await api('/api/v1/admin/invoicetotally');paint()}
    catch(e){const b=$('#billsBody');if(b)b.innerHTML=`<div class="notice err">${esc(e.message)}</div>`}
    finally{loading=false}
  }

  function setupCard(){
    const L=data.legacy||{};
    return `${L.found?`<div class="notice info" style="margin-bottom:16px"><b>InvoiceTotally Bridge settings found on this PC</b> (${L.tokens} compan${L.tokens===1?'y':'ies'}). TallyBridge can take over from it — one app for your phone and for posting bills.<div style="margin-top:10px"><button class="btn" data-bills="import-legacy">Use these settings</button></div></div>`:''}
    <div class="card" style="max-width:820px">
      <h2 style="margin-top:0">Connect InvoiceTotally</h2>
      <ol class="sub" style="font-size:13px;line-height:1.9;padding-left:18px;margin:0 0 14px">
        <li>Open InvoiceTotally in your browser → your company → <b>Tally bridge</b>.</li>
        <li>Copy the <b>Web app URL</b> and the <b>Bridge token</b> into the boxes below. Several companies? Paste one token per line.</li>
        <li>Press <b>Test &amp; save</b>. Then switch on <b>posting</b> when you're ready for reviewed bills to go into Tally.</li></ol>
      <label><div class="metric-label">InvoiceTotally web address</div><input id="itUrl" class="input" style="width:100%;margin:7px 0 14px" placeholder="https://invoicetotally.yourname.workers.dev" autocomplete="off"></label>
      <label><div class="metric-label">Company bridge tokens (one per line)</div><textarea id="itTokens" class="input" rows="3" style="width:100%;margin:7px 0 14px;font-family:monospace" placeholder="itb_…" autocomplete="off" spellcheck="false"></textarea></label>
      <button class="btn" data-bills="save-new">Test &amp; save</button><span id="itStatus" class="sub" style="margin-left:12px" role="status"></span>
    </div>`;
  }

  function companyRow(c){
    const server=c.server||{};
    let tally='';
    if(c.companyOpen===true)tally=pill('ok','Open in Tally');
    else if(c.companyOpen===false)tally=pill('warn','Not open in Tally');
    const st=c.stats||{};
    return `<div class="list-row"><div><div class="title">${esc(c.company||'Company '+c.hint)}</div>
      <div class="sub">Tally company: <b>${esc(c.tallyCompany||'—')}</b> • token ${esc(c.hint)}${c.pending?` • <b>${c.pending} waiting</b>`:''}</div>
      <div class="sub">Today: ${st.postedToday||0} posted • ${st.mastersToday||0} masters created${st.failedToday?` • <span style="color:var(--err)">${st.failedToday} failed</span>`:''} • last activity ${ago(st.lastSync)}</div>
      ${server.status==='down'?`<div class="sub" style="color:var(--err)">${esc(server.detail)}</div>`:''}</div>
      <div class="toolbar">${server.status==='ok'?pill('ok','Web app connected'):server.status==='down'?pill('err','Not connected'):pill('warn','Connecting…')} ${tally}
      <button class="btn danger" data-bills="remove" data-id="${esc(c.id)}" title="Stop handling this company on this PC">Remove</button></div></div>`;
  }

  function uploadRow(u){
    const kind=u.state==='failed'?'err':u.state==='sent'?(u.status==='In Tally'?'ok':'ok'):'warn';
    const inv=(u.cloud?.invoices||[]).map(i=>`${esc(i.number||'?')} • ${esc(i.party||'')}${i.total!=null?' • '+money(i.total):''} — ${esc(i.state||'')}`).join('<br>');
    return `<div class="list-row"><div><div class="title">${esc(u.company||'—')} • ${u.pages} page${u.pages===1?'':'s'} ${u.voucherType==='Sales'?'(sales)':''}</div>
      <div class="sub">From ${esc(u.device||'phone')} • ${ago(u.createdAt)}${u.batch?' • '+esc(u.batch):''}</div>
      ${inv?`<div class="sub">${inv}</div>`:''}${u.error&&u.state!=='sent'?`<div class="sub" style="color:var(--err)">${esc(u.error)}</div>`:''}</div>
      <div class="toolbar">${pill(kind,u.status||u.state)}
      ${u.state==='failed'||u.state==='queued'?`<button class="btn secondary" data-bills="retry" data-id="${esc(u.id)}">Send now</button>`:''}
      ${u.state!=='sent'?`<button class="btn danger" data-bills="delete-upload" data-id="${esc(u.id)}">Delete</button>`:''}</div></div>`;
  }

  function paint(){
    const box=$('#billsBody');if(!box||!data)return;
    if(!data.configured){box.innerHTML=setupCard();return}
    const t=data.tally||{},conns=data.connections||[],ups=data.uploads||[];
    const posted=conns.reduce((n,c)=>n+(c.stats?.postedToday||0),0),failed=conns.reduce((n,c)=>n+(c.stats?.failedToday||0),0);
    const pending=conns.reduce((n,c)=>n+(c.pending||0),0);
    const L=data.legacy||{};
    box.innerHTML=`${L.running?`<div class="notice warn" style="margin-bottom:14px"><b>The old InvoiceTotally Bridge is still running.</b> Close it (tray icon → Quit) and remove it from startup, so only TallyBridge posts to Tally.</div>`:''}
    <div class="grid dashboard-grid">
      <div class="card span6"><div class="metric-label">Posting to Tally</div>
        <div class="toolbar" style="margin-top:10px">${data.postingEnabled?pill('ok','ON — reviewed bills are posted'):pill('warn','OFF — nothing is written to Tally')}
        <button class="btn ${data.postingEnabled?'secondary':''}" data-bills="posting" data-on="${data.postingEnabled?0:1}">${data.postingEnabled?'Turn off':'Turn on'}</button></div>
        <div class="metric-sub" style="line-height:1.5">Only what you push in InvoiceTotally is posted: purchase/sales bills, bank statement entries (Receipt, Payment, Contra) and new ledgers, stock items and units — never edits or deletions. The phone stays read-only.</div></div>
      <div class="card span3"><div class="metric-label">Tally</div><div class="metric-value metric-small">${t.status==='ok'?'Connected':t.status==='down'?'Not reachable':'Checking…'}</div><div class="metric-sub">${esc(t.status==='ok'?((t.companies||[]).join(', ')||'No company open'):t.detail||'')}</div></div>
      <div class="card span3"><div class="metric-label">Today</div><div class="metric-value">${posted}</div><div class="metric-sub">posted${failed?` • ${failed} failed`:''}${pending?` • ${pending} waiting`:''}</div></div>
    </div>
    <div class="section-title">Companies</div>
    <div class="card">${conns.map(companyRow).join('')}
      <div class="toolbar" style="margin-top:12px">${adding?`<input id="itAdd" class="input grow" placeholder="Paste another company's bridge token (itb_…)" autocomplete="off" spellcheck="false"><button class="btn" data-bills="add">Test &amp; add</button><button class="btn secondary" data-bills="cancel-add">Cancel</button>`
        :`<button class="btn secondary" data-bills="show-add">＋ Add company</button><button class="btn secondary" data-bills="open-web">Open InvoiceTotally</button><button class="btn secondary" data-bills="edit-url">Web address</button>`}</div>
      ${editUrl?`<div class="toolbar" style="margin-top:12px"><input id="itUrlEdit" class="input grow" value="${esc(data.serverUrl)}"><button class="btn" data-bills="save-url">Test &amp; save</button><button class="btn secondary" data-bills="cancel-url">Cancel</button></div>`:''}
      <div id="itStatus" class="sub" role="status" style="margin-top:8px"></div></div>
    <div class="section-title">Bills from the phone</div>
    <div class="card"><label class="check-row" style="padding-top:0"><input type="checkbox" data-bills="phone" ${data.phoneUploads?'checked':''}><span><b>Allow approved phones to send bills</b><small>In the TallyBridge app tap <b>Scan bill</b>: photos and PDFs come to this PC and go on to InvoiceTotally for reading. If the internet is down they wait here and are sent automatically.</small></span></label>
      ${ups.length?ups.map(uploadRow).join(''):`<div class="empty">No bills from the phone yet.</div>`}</div>
    <div class="section-title">Activity</div>
    <div class="card">${(data.activity||[]).slice(0,40).map(a=>`<div class="list-row"><div><div class="title" style="font-weight:600;${a.ok?'':'color:var(--err)'}">${esc(a.text)}</div><div class="sub">${a.company?esc(a.company)+' • ':''}${new Date(a.at).toLocaleString()}</div></div></div>`).join('')||'<div class="empty">Nothing yet.</div>'}</div>`;
  }

  function say(msg,err){const s=$('#itStatus');if(s){s.textContent=msg;s.style.color=err?'var(--err)':''}else toast(msg,err)}
  async function busyBtn(b,label,fn){const old=b.innerHTML;b.disabled=true;b.innerHTML='<span class="spinner"></span> '+label;try{await fn()}finally{b.disabled=false;b.innerHTML=old}}
  const post=(path,body)=>api('/api/v1/admin/invoicetotally'+path,{method:'POST',body:JSON.stringify(body||{})});

  document.getElementById('adminApp')?.addEventListener('click',async e=>{
    const b=e.target.closest('[data-bills]');if(!b||route!=='bills')return;
    const a=b.dataset.bills;
    try{
      if(a==='import-legacy'){await busyBtn(b,'Connecting',async()=>{const r=await post('/import-legacy');toast('Connected: '+r.results.map(x=>x.company).join(', '));await load()});return}
      if(a==='save-new'){const url=$('#itUrl').value.trim(),tokens=$('#itTokens').value;if(!url||!tokens.trim()){say('Enter the web address and at least one token',true);return}
        await busyBtn(b,'Testing',async()=>{try{const r=await post('/save',{serverUrl:url,add:[tokens]});toast('Connected: '+r.results.map(x=>x.company).join(', '));await load()}catch(err){say(err.message,true)}});return}
      if(a==='show-add'){adding=true;paint();$('#itAdd')?.focus();return}
      if(a==='cancel-add'){adding=false;paint();return}
      if(a==='add'){const t=$('#itAdd').value.trim();if(!t)return;await busyBtn(b,'Testing',async()=>{try{const r=await post('/save',{serverUrl:data.serverUrl,add:[t]});adding=false;toast('Added '+r.results.map(x=>x.company).join(', '));await load()}catch(err){say(err.message,true)}});return}
      if(a==='edit-url'){editUrl=true;paint();return}
      if(a==='cancel-url'){editUrl=false;paint();return}
      if(a==='save-url'){await busyBtn(b,'Testing',async()=>{try{await post('/save',{serverUrl:$('#itUrlEdit').value.trim()});editUrl=false;toast('Web address saved');await load()}catch(err){say(err.message,true)}});return}
      if(a==='open-web'){if(data?.serverUrl)window.open(data.serverUrl,'_blank','noopener');return}
      if(a==='remove'){const c=(data.connections||[]).find(x=>x.id===b.dataset.id);if(!confirm(`Stop handling “${c?.company||'this company'}” on this PC? Bills already in InvoiceTotally stay there.`))return;await post('/save',{serverUrl:data.serverUrl,remove:[b.dataset.id]});toast('Removed');await load();return}
      if(a==='posting'){const on=b.dataset.on==='1';
        if(on&&!confirm('Turn on posting?\n\nTallyBridge will post what you push in InvoiceTotally into Tally on this PC: purchase/sales bills, bank statement entries (Receipt/Payment/Contra), and new ledgers, stock items and units you approved. It never edits or deletes anything, and the phone stays read-only.'))return;
        await busyBtn(b,on?'Turning on':'Turning off',async()=>{data=Object.assign(data,await post('/options',{postingEnabled:on}));paint();toast(on?'Posting is on':'Posting is off')});return}
      if(a==='phone'){await post('/options',{phoneUploads:b.checked});toast(b.checked?'Phones can send bills':'Phones can no longer send bills');await load();return}
      if(a==='retry'){await post('/uploads/retry',{id:b.dataset.id});toast('Sending…');setTimeout(load,800);return}
      if(a==='delete-upload'){if(!confirm('Delete this bill from the PC? It has not reached InvoiceTotally.'))return;await post('/uploads/delete',{id:b.dataset.id});await load();return}
    }catch(err){toast(err.message,true)}
  });

  installNav();
  if(route==='bills')render();
})();
