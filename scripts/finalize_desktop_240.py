from pathlib import Path
p=Path('internal/app/web/operations.js');s=p.read_text(encoding='utf-8')
old="for(const [k,t] of [['work','Today'],['global-search','Search'],['followups','Follow-ups']])"
new="for(const [k,t] of [['work','Today'],['customers','Customers'],['global-search','Search'],['followups','Follow-ups']])"
if old not in s: raise SystemExit('operations install anchor missing')
s=s.replace(old,new,1)
old="render=function(){install();if(!['work','global-search','followups'].includes(route)){baseRender();return}"
new="function organize(){const side=document.querySelector('.side-nav');if(!side||side.dataset.organized==='1')return;const groups=[['DAILY WORK',['home','work','orders','followups']],['CUSTOMERS & FIND',['customers','global-search','companies']],['BUSINESS SETUP',['pdfbank','health']],['SYSTEM',['diag','devices','cloudflare','live','backup','updates','settings','about']]];for(const [label,keys] of groups){const h=document.createElement('div');h.textContent=label;h.style.cssText='padding:14px 14px 6px;font-size:10px;font-weight:800;letter-spacing:.14em;opacity:.62';side.appendChild(h);for(const key of keys){const b=side.querySelector(`[data-route=\"${key}\"]`);if(b)side.appendChild(b)}}side.dataset.organized='1'}\n render=function(){install();organize();if(!['work','customers','global-search','followups'].includes(route)){baseRender();return}"
if old not in s: raise SystemExit('operations render anchor missing')
s=s.replace(old,new,1)
old="if(route==='work')renderWork();else if(route==='global-search')renderSearch();else renderFollowups();"
new="if(route==='work')renderWork();else if(route==='customers')renderCustomers();else if(route==='global-search')renderSearch();else renderFollowups();"
if old not in s: raise SystemExit('route anchor missing')
s=s.replace(old,new,1)
marker=' async function loadSearch()'
if marker not in s: raise SystemExit('search insertion marker missing')
insert="""
 function renderCustomers(){const p=$('#page');if(!p)return;p.innerHTML=head('Customers','Open Customer 360 from the synchronized Tally ledger list.')+companyBar()+`<div class=\"card\"><input id=\"customerQ\" class=\"input\" placeholder=\"Search customer / ledger\" style=\"width:100%\"></div><div id=\"customerResults\"></div>`;const paint=()=>{const box=$('#customerResults');if(!box)return;const q=($('#customerQ')?.value||'').trim().toLowerCase(),xs=(searchData?.results||[]).filter(x=>x.kind==='ledger'&&(!q||(x.title+' '+(x.subtitle||'')).toLowerCase().includes(q))).slice(0,100);box.innerHTML=xs.length?`<div class=\"card\" style=\"margin-top:14px\">${xs.map(x=>`<div class=\"list-row clickable\" data-customer=\"${esc(x.party||x.ledger||x.title)}\"><div><div class=\"title\">${esc(x.title)}</div><div class=\"sub\">${esc(x.subtitle||'Customer ledger')}</div></div><b>Open</b></div>`).join('')}</div>`:'<div class=\"empty\" style=\"margin-top:14px\">No matching customer.</div>'};$('#customerQ')?.addEventListener('input',paint);if(searchData)paint();else loadSearch().then(paint)}
"""
s=s.replace(marker,insert+marker,1)
old="if(['work','global-search','followups'].includes(route))render();"
new="if(['work','customers','global-search','followups'].includes(route))render();"
if old not in s: raise SystemExit('initial route anchor missing')
s=s.replace(old,new,1)
# Customer 360 desktop: last purchase + frequent items.
old="<div class=\"section-title\">Latest invoices</div><div class=\"card\">${(c.latestInvoices||[]).map(v=>"
if old in s:
    extra="${x.lastPurchaseDate?`<div class=\"section-title\">Purchase pattern</div><div class=\"card\"><div class=\"list-row\"><div><div class=\"title\">Last purchase</div><div class=\"sub\">${esc(x.lastPurchaseDate)}</div></div></div>${(x.frequentItems||[]).map(i=>`<div class=\"list-row\"><div><div class=\"title\">${esc(i.name)}</div><div class=\"sub\">${i.count} saved invoice(s)</div></div></div>`).join('')}</div>`:''}<div class=\"section-title\">Latest invoices</div><div class=\"card\">${(c.latestInvoices||[]).map(v=>"
    s=s.replace(old,extra,1)
p.write_text(s,encoding='utf-8')
print('desktop organization applied')
