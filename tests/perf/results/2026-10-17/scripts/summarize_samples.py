import csv, re, sys
for lab in sys.argv[1:]:
    st = next(r for r in csv.DictReader(open(f"{lab}_stats.csv")) if r["Name"] == "Aggregated")
    n, f = int(st["Request Count"]), int(st["Failure Count"])
    print(f"{lab}: reqs={n} rps={float(st['Requests/s']):.1f} fail%={100*f/max(n,1):.2f} avg={float(st['Average Response Time']):.1f} "
          f"p50={st['50%']} p95={st['95%']} p99={st['99%']} max={float(st['Max Response Time']):.0f}")
    meta = open(f"{lab}_meta.txt").read()
    a = float(re.search(r"(?<!_)locust_start=([\d.]+)", meta).group(1)); b = float(re.search(r"(?<!_)locust_end=([\d.]+)", meta).group(1))
    rows = [r for r in csv.DictReader(open(f"{lab}_samples.csv")) if r["ledger_outbox_unpublished"]]
    first, = rows[:1]
    during = [r for r in rows if a <= float(r["epoch"]) <= b]
    last_in = during[-1]
    lag = lambda r: int(r["ledger_tx_total"]) - int(r["settlement_tx_total"])
    print(f"   start: ledger_tx={first['ledger_tx_total']}  at locust end: outbox_unpublished={last_in['ledger_outbox_unpublished']} scoring_lag={lag(last_in)} "
          f"max_unpublished={max(int(r['ledger_outbox_unpublished']) for r in rows)} max_lag={max(lag(r) for r in rows)} "
          f"ledger_pg_conns max={max(int(r['ledger_pg_conns']) for r in rows)} settlement_pg_conns max={max(int(r['settlement_pg_conns']) for r in rows)}")
    # published rate during run: delta(total - unpublished)/dt
    pub = lambda r: int(r["ledger_tx_total"]) - int(r["ledger_outbox_unpublished"])
    d0, d1 = during[len(during)//3], during[-1]
    dt = float(d1["epoch"]) - float(d0["epoch"])
    print(f"   steady window {dt:.0f}s: tx inserted/s={(int(d1['ledger_tx_total'])-int(d0['ledger_tx_total']))/dt:.1f} "
          f"outbox published/s={(pub(d1)-pub(d0))/dt:.1f} scored/s={(int(d1['settlement_tx_total'])-int(d0['settlement_tx_total']))/dt:.1f}")
    print("   " + [l for l in meta.splitlines() if l.startswith("drain")][0])
