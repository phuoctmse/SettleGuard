import re, sys
from datetime import datetime
LOG = r"C:/Users/ADMIN/AppData/Local/Temp/claude/d--SettleGuard/80be41dd-9ed2-417a-bac1-773413d93787/scratchpad/logs/ledger-service.log"
rows = []
for line in open(LOG, encoding="utf-8", errors="replace"):
    line = line.rstrip()
    if "POST http://localhost:8080/transactions" not in line:
        continue
    m = re.search(r" in ([\d.]+)(\S+)$", line)
    v, u = float(m.group(1)), m.group(2)
    ms = v / 1000 if u.endswith("µs") or u == "us" else v if u == "ms" else v * 1000
    rows.append((datetime.strptime(line[:19], "%Y/%m/%d %H:%M:%S").timestamp(), ms))
for lab in sys.argv[1:]:
    m = dict(re.findall(r"(?<!_)(locust_start|locust_end)=([\d.]+)", open(f"{lab}_meta.txt").read()))
    a, b = float(m["locust_start"]), float(m["locust_end"])
    d = sorted(x for t, x in rows if a - 1 <= t <= b + 1)
    p = lambda q: d[min(len(d) - 1, int(q * len(d)))]
    print(f"{lab}: server-side n={len(d)} avg={sum(d)/len(d):.1f}ms p50={p(.5):.1f} p95={p(.95):.1f} p99={p(.99):.1f} max={d[-1]:.1f}")
