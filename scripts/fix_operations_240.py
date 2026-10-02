from pathlib import Path
p=Path('internal/app/production_features.go')
s=p.read_text(encoding='utf-8')
s=s.replace('add(&items,"sales-sync","Sales sync",salesReady,"Run Sync now once while the company is open in Tally.",true)','add(&items,"sales-sync","Sales sync",salesReady,"Run Sync now once while the company is open in Tally.",false)')
s=s.replace('add(&items,"orders-sync","Orders",ordersReady,"Repair the local Orders store if it cannot be opened.",true)','add(&items,"orders-sync","Orders",ordersReady,"Repair the local Orders store if it cannot be opened.",false)')
old='''\tfor _, it := range items {\n\t\tif it.OK { okCount++ }\n'''
new='''\tfor _, it := range items {\n\t\tif it.OK && it.Key!="sales-sync" && it.Key!="proforma-sync" && it.Key!="orders-sync" { okCount++ }\n'''
if old not in s: raise SystemExit('health score loop anchor missing')
s=s.replace(old,new,1)
old='''\tscore := 0\n\tif len(items)>0 { score = int(math.Round(float64(okCount)*100/float64(len(items)))) }\n'''
new='''\tscore := 0\n\tscoreItems := len(items)-3\n\tif scoreItems>0 { score = int(math.Round(float64(okCount)*100/float64(scoreItems))) }\n'''
if old not in s: raise SystemExit('health score formula anchor missing')
s=s.replace(old,new,1)
p.write_text(s,encoding='utf-8')
print('operations health scoring fixed')
