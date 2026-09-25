const VERSION='tb-v160-desktop-bank-pdf';
self.addEventListener('install',e=>self.skipWaiting());
self.addEventListener('activate',e=>e.waitUntil(caches.keys().then(keys=>Promise.all(keys.map(k=>caches.delete(k)))).then(()=>self.clients.claim())));
self.addEventListener('fetch',e=>{ if(e.request.method==='GET') e.respondWith(fetch(e.request,{cache:'no-store'})); });
