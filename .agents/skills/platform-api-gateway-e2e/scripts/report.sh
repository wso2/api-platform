#!/usr/bin/env bash
# report.sh  -> markdown summary of $WORK/results.tsv: one row per scenario, one column per version,
# then every failed check, then platform-api translation warnings and failed deployments.
set -uo pipefail
. "$(dirname "$0")/lib.sh"
need_work
R="$WORK/results.tsv"; [ -s "$R" ] || die "no results in $R"
python3 - "$R" <<'PY'
import sys,collections
rows=[l.rstrip("\n").split("\t") for l in open(sys.argv[1]) if l.strip()]
vers=sorted({r[1] for r in rows}); scen=list(dict.fromkeys(r[0] for r in rows))
agg=collections.defaultdict(lambda:[0,0])
for s,v,c,res,detail in rows:
    agg[(s,v)][0 if res=="PASS" else 1]+=1
print("| Scenario | "+" | ".join(vers)+" |"); print("|---|"+"---|"*len(vers))
for s in scen:
    cells=[]
    for v in vers:
        p,f=agg.get((s,v),[0,0])
        cells.append("—" if p+f==0 else ("PASS (%d)"%p if f==0 else "**FAIL** %d/%d"%(f,p+f)))
    print("| %s | %s |"%(s," | ".join(cells)))
fails=[r for r in rows if r[3]!="PASS"]
if fails:
    print("\nFailed checks:")
    for s,v,c,res,d in fails: print("- %s @ %s: %s — %s"%(s,v,c,d))
PY
w=$(grep -c "adapted for older gateway" "$LOGS/platform-api.log" 2>/dev/null || true)
echo; echo "platform-api translation warnings: ${w:-0}"
grep "adapted for older gateway" "$LOGS/platform-api.log" 2>/dev/null | sed -E 's/.*(kind=[^ ]+).*(field=[^ ]+).*(gatewayVersion=[^ ]+).*/- \1 \2 \3/' | sort | uniq -c | head -20
