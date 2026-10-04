from pathlib import Path
root=Path('.')

p=root/'internal/app/server.go'; s=p.read_text()
old='''\tcfg := s.store.Settings()\n\tif cfg.AutoUpdate {\n\t\ts.maybeCheckUpdates()\n\t}\n\treturn map[string]any{"bridgeOnline": true, "tallyOnline": st.TallyOnline, "companies": cs, "lastSuccess": st.LastSuccess, "cacheLatestAt": s.cache.Stats().LatestAt, "summaries": s.summaries(cs), "autoSync": s.autoSyncSnapshot(), "version": Version, "pdfBanks": s.store.PDFBankNames(), "health": health, "pdfTemplate": validPDFTemplate(cfg.PDFTemplate), "autoUpdate": cfg.AutoUpdate, "update": s.updateSnapshot()}\n'''
new='''\tcfg := s.store.Settings()\n\tif cfg.AutoUpdate {\n\t\ts.maybeCheckUpdates()\n\t}\n\tcacheLatest := s.cache.Stats().LatestAt\n\tsync := s.autoSyncSnapshot()\n\tlastUpdated := st.LastSuccess\n\tfor _, at := range []time.Time{cacheLatest, sync.LastProgress, sync.LastOK} {\n\t\tif at.After(lastUpdated) {\n\t\t\tlastUpdated = at\n\t\t}\n\t}\n\treturn map[string]any{"bridgeOnline": true, "tallyOnline": st.TallyOnline, "companies": cs, "lastSuccess": st.LastSuccess, "cacheLatestAt": cacheLatest, "lastUpdated": lastUpdated, "summaries": s.summaries(cs), "autoSync": sync, "version": Version, "pdfBanks": s.store.PDFBankNames(), "health": health, "pdfTemplate": validPDFTemplate(cfg.PDFTemplate), "autoUpdate": cfg.AutoUpdate, "update": s.updateSnapshot()}\n'''
assert old in s
p.write_text(s.replace(old,new))

p=root/'android/app/src/main/java/in/tallybridge/app/MainActivity.java'; s=p.read_text()
old='''for(String value:new String[]{boot.optString("lastSuccess"),boot.optString("cacheLatestAt"),sync.optString("lastProgress"),sync.optString("lastOk")})'''
new='''for(String value:new String[]{boot.optString("lastUpdated"),boot.optString("lastSuccess"),boot.optString("cacheLatestAt"),sync.optString("lastProgress"),sync.optString("lastOk")})'''
assert old in s
s=s.replace(old,new)
p.write_text(s)

p=root/'android/app/src/androidTest/java/in/tallybridge/app/DesignTest.java'; s=p.read_text()
old='''.put("lastSuccess","2026-09-27T08:00:00Z").put("summaries"'''
new='''.put("lastSuccess","2026-09-27T08:00:00Z").put("lastUpdated","2026-09-27T08:02:00Z").put("summaries"'''
assert old in s
p.write_text(s.replace(old,new))
