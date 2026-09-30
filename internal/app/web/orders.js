(()=>{
  I.orders='<path d="M5 4h14v16H5z"/><path d="M8 8h8M8 12h8M8 16h5"/>';
  let orderTab='pending', orderData={pending:[],dispatched:[],pendingCount:0,dispatchedCount:0}, orderLoading=false;

  function installOrdersNav(){
    const side=document.querySelector('.side-nav');
    if(!side||side.querySelector('[data-route="orders"]'))return;
    const b=document.createElement('button');
    b.className='side-btn'+(route==='orders'?' active':'');b.dataset.route='orders';
    b.innerHTML=icon('orders')+'<span>Orders</span>';
    const companies=side.querySelector('[data-route="companies"]');
    if(companies)companies.insertAdjacentElement('afterend',b);else side.appendChild(b);
  }

  const baseRender=render;
  render=function(){
    installOrdersNav();
    if(route!=='orders'){baseRender();return;}
    $$('.side-btn').forEach(b=>b.classList.toggle('active',b.dataset.route===route));
    renderOrders();lastRenderKey='orders';
  };

  function orderWhen(v){if(!v||String(v).startsWith('0001'))return '—';return new Date(v).toLocaleString();}
  function orderQty(v){const n=Number(v||0);return Number.isInteger(n)?String(n):String(n.toFixed(2)).replace(/0+$/,'').replace(/\.$/,'')}
  function orderItems(o){return (o.items||[]).map((x,i)=>`<div class="list-row"><div><div class="title">${i+1}. ${esc(x.name)}</div><div class="sub">Qty ${esc(orderQty(x.quantity))}${x.unit?' '+esc(x.unit):''}${x.rate!=null?' • Rate '+money(x.rate):''}</div></div>${x.rate!=null?`<b>${money(Number(x.quantity||0)*Number(x.rate||0))}</b>`:''}</div>`).join('')}
  function orderCard(o,pending){
    const details=o.buyerDetails||{},phone=details.MOBILENUMBER||details.MOBILE||details.PHONENUMBER||details.PHONE||'';
    return `<div class="card" style="margin-bottom:14px"><div class="list-row"><div><div class="title">${esc(o.number||'Order')} • ${esc(o.buyer)}</div><div class="sub">${esc(o.company)} • ${pending?'Taken':'Dispatched'} ${orderWhen(pending?o.createdAt:o.dispatchedAt)}${phone?' • '+esc(phone):''}</div></div><span class="status-pill ${pending?'warn':'ok'}">${pending?'PENDING':'DISPATCHED'}</span></div>${orderItems(o)}${o.notes?`<div class="notice info" style="margin-top:10px"><b>Notes:</b> ${esc(o.notes)}</div>`:''}${pending?`<div class="toolbar" style="margin-top:14px"><button class="btn secondary" data-order-action="copy" data-order-id="${esc(o.id)}">Copy details</button><button class="btn" data-order-action="dispatch" data-order-id="${esc(o.id)}">Mark dispatched</button></div>`:`<div class="sub" style="margin-top:12px">This order is view-only and will be removed automatically 7 days after dispatch.</div>`}</div>`;
  }

  function renderOrders(){
    const p=$('#page');if(!p)return;
    p.innerHTML=head('Orders','Orders taken from your phone are stored on this PC. Pending orders stay until dispatched; dispatched orders are permanently removed after 7 days.')+
      `<div class="grid dashboard-grid"><div class="card span6"><div class="metric-label">Pending orders</div><div class="metric-value" id="orderPendingMetric">${orderData.pendingCount||0}</div><div class="metric-sub">Waiting for dispatch</div></div><div class="card span6"><div class="metric-label">Dispatched (7 days)</div><div class="metric-value" id="orderDispatchedMetric">${orderData.dispatchedCount||0}</div><div class="metric-sub">Automatically removed after 7 days</div></div></div>`+
      `<div class="toolbar" style="margin:18px 0"><button class="btn ${orderTab==='pending'?'':'secondary'}" data-order-tab="pending">Pending</button><button class="btn ${orderTab==='dispatched'?'':'secondary'}" data-order-tab="dispatched">Dispatched</button><button class="btn secondary" data-order-action="refresh">Refresh</button></div><div id="ordersDesktopBody"><div class="screen-loader"><span class="spinner"></span>Loading orders…</div></div>`;
    if(!orderLoading)loadOrdersDesktop();else paintOrders();
  }

  function paintOrders(){
    if(route!=='orders')return;
    const p=$('#orderPendingMetric'),d=$('#orderDispatchedMetric');if(p)p.textContent=orderData.pendingCount||0;if(d)d.textContent=orderData.dispatchedCount||0;
    const box=$('#ordersDesktopBody');if(!box)return;
    const rows=orderTab==='pending'?(orderData.pending||[]):(orderData.dispatched||[]);
    box.innerHTML=rows.length?rows.map(o=>orderCard(o,orderTab==='pending')).join(''):`<div class="empty">${orderTab==='pending'?'No pending orders. New phone orders will appear here automatically.':'No dispatched orders from the last 7 days.'}</div>`;
  }

  async function loadOrdersDesktop(){
    orderLoading=true;
    try{orderData=await api('/api/v1/admin/orders');if(route==='orders')paintOrders()}
    catch(e){if(route==='orders')$('#ordersDesktopBody').innerHTML=`<div class="notice err">${esc(e.message)}</div>`}
    finally{orderLoading=false}
  }

  document.getElementById('adminApp')?.addEventListener('click',async e=>{
    const tab=e.target.closest('[data-order-tab]');if(tab){orderTab=tab.dataset.orderTab;renderOrders();return}
    const b=e.target.closest('[data-order-action]');if(!b)return;
    const action=b.dataset.orderAction,id=b.dataset.orderId;
    if(action==='refresh'){await loadOrdersDesktop();return}
    const all=[...(orderData.pending||[]),...(orderData.dispatched||[])],o=all.find(x=>x.id===id);if(!o)return;
    if(action==='copy'){
      if(!o.shareText){toast('Details are available only while the order is pending',true);return}
      try{await navigator.clipboard.writeText(o.shareText);toast('Order details copied')}catch{toast('Could not copy order details',true)}
      return;
    }
    if(action==='dispatch'){
      if(!confirm('Mark '+(o.number||'this order')+' for '+o.buyer+' as dispatched? It will stay visible for 7 days, then be permanently removed.'))return;
      b.disabled=true;
      try{await api('/api/v1/admin/orders/dispatch',{method:'POST',body:JSON.stringify({id})});toast('Order marked dispatched');await loadOrdersDesktop()}catch(err){toast(err.message,true);b.disabled=false}
    }
  });

  installOrdersNav();
  if(route==='orders')render();
})();
