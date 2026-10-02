from pathlib import Path
p=Path('android/app/src/main/java/in/tallybridge/app/OrdersActivity.java')
s=p.read_text(encoding='utf-8')
old='java.util.regex.Pattern.compile("(?:^|\\s)(\\d+(?:\\.\\d+)?)\\s+(?:dozen\\s+|pcs?\\s+|pieces?\\s+)?"+java.util.regex.Pattern.quote(matchedName.toLowerCase(Locale.ROOT)))'
# The generated Java source from the guarded patch may contain single backslash escapes,
# so locate the whole Pattern.compile expression by stable surrounding text instead.
start='java.util.regex.Matcher m=java.util.regex.Pattern.compile('
pos=s.find(start)
if pos<0:
    raise SystemExit('voice regex start not found')
end=s.find(').matcher(lower);',pos)
if end<0:
    raise SystemExit('voice regex end not found')
replacement='java.util.regex.Matcher m=java.util.regex.Pattern.compile("([0-9]+(?:[.][0-9]+)?) +(dozen +|pcs? +|pieces? +)?"+java.util.regex.Pattern.quote(matchedName.toLowerCase(Locale.ROOT))).matcher(lower);'
s=s[:pos]+replacement+s[end+len(').matcher(lower);'):]
p.write_text(s,encoding='utf-8')
print('voice quantity parser fixed without Java escape ambiguity')
