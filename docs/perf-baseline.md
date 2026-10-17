# Performance baseline — settlement scoring path

Recorded 2026-10-17. Raw Locust CSV/HTML, per-second backlog samples and
`docker stats` for every run are in `tests/perf/results/2026-10-17/`.

## What is measured

`tests/perf/locustfile_settlement_scoring.py`: each simulated user creates
one client and two accounts on accounts-service in `on_start`, then loops
`POST /transactions` (a balanced 2-entry, 100-dong transaction) on
ledger-service with a 0.1–0.5 s wait between requests. Only that POST is in
Locust's stats. Every POST also feeds the async path: ledger outbox →
`ledger.entry-recorded` on NATS → settlement-engine risk scoring →
notification-service.

Because each user keeps posting on the same two accounts, the velocity rule
(more than 5 transactions in 5 minutes, SETTLEMENT-01/02) holds nearly
every transaction after the first few: 40,237 of the 43,047 scored
transactions were `held` at the end of the session. The async load is
therefore mostly hold decisions plus one notification row each.

## Environment

Everything ran on one host:

| | |
|---|---|
| CPU | AMD Ryzen 7 5800H, 8 cores / 16 threads |
| RAM | 15.9 GB |
| OS | Windows 11 Pro 10.0.26200 |
| Docker | Docker Desktop, engine 29.4.1 (VM: 16 CPUs, 5.8 GiB) |
| Infra | `infra/docker/docker-compose.yml`: Postgres 18 (one per service), NATS 2.10 with JetStream. Fresh volumes before the first run, **not reset between runs**; each run's starting row count is the first line of its `*_samples.csv` |
| Services | Go 1.26.2 binaries from `go build ./cmd/server`; notification-service on Python 3.14.3. Env vars as in `tests/README.md`, including `SETTLEMENT_BATCH_INTERVAL_SECONDS=5`. Gateway not running (Locust calls ledger/accounts directly). Every service logs each request to a file |
| Load generator | Locust 2.46.4 on Python 3.14.3, single process, same host |
| Other load | An unrelated kind cluster container (`foodfund-control-plane`) was running throughout |

## Results

Spawn rate 5 users/s for every run, so the 100-user run spends its first
20 s ramping. Numbers come from Locust's final console summary
(`*_console.txt`). The `*_stats.csv` files are written before the last
second of requests completes and undercount slightly (e.g. 13,001 vs
13,154); the console totals match ledger-service's own request log.

### Baseline (locustfile with a shared setup client)

| Users | Duration | Requests | req/s | Failures | Avg ms | p50 ms | p95 ms | p99 ms |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 20 | 30 s | 1,785 | 60.8 | 0.00% | 11 | 9 | 30 | 38 |
| 20 | 60 s | 3,615 | 61.1 | 0.00% | 13 | 10 | 35 | 52 |
| 50 | 60 s | 8,383 | 142.2 | 0.00% | 23 | 15 | 59 | 110 |
| 100 | 60 s | 13,154 | 222.2 | 0.00% | 68 | 48 | 200 | 290 |

Server-side durations from ledger-service's request log
(`scripts/serverside_latency.py`) track these closely. For example, at 20
users / 60 s: avg 10.6 ms and p99 44.2 ms server-side, against 13 ms and
52 ms in Locust.

### Original locustfile (as of PR #16), for comparison

PR #16 recorded 641 requests in 30 s at 20 users, 0% failures, with no
latency figures. The same locustfile on this host:

| Users | Duration | Requests | req/s | Failures | Avg ms | p50 ms | p95 ms | p99 ms |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 20 | 30 s | 835 | 28.2 | 0.00% | 21 | 10 | 38 | 120 |
| 20 | 60 s | 2,632 | 44.4 | 0.00% | 15 | 9 | 38 | 90 |
| 50 | 60 s | 2,235 | 37.5 | 0.00% | 134 | 15 | 270 | 2,900 |
| 100 | 60 s | 94 | 1.5 | 0.00% | 5,282 | 3,700 | 16,000 | 19,000 |

These numbers measure the load generator, not SettleGuard. See finding 1.

## Findings

### 1. The original locustfile throttled itself (fixed in this PR)

- **Symptom.** Throughput fell as users rose (44 → 37 → 1.5 req/s), and
  Locust logged `CPU usage above 90%` during the 50- and 100-user runs.
- **Locust's numbers diverged from the server's.** At 50 users,
  ledger-service's own log shows p99 195 ms while Locust reported 2.9 s.
  At 100 users, only 110 requests reached the server in 60 s, at a
  server-side average of 40 ms.
- **Cause.** `on_start` made three module-level `httpx.post()` calls. Each
  one builds a new client and SSL context: 211 ms and 430 ms of CPU per
  call in two measurements, against 1.6 ms or less on a shared client
  (`diagnostics/httpx_client_cost.txt`). Every Locust user shares one
  gevent hub, so that CPU blocked all in-flight requests. At 100 users the
  ramp did not finish inside 60 s: the user count reached 100 at 61 s
  (`original-locustfile/u100_60s_stats_history.csv`).
- **Control run.** The same Locust install on ledger-service's
  `GET /health` sustained 309 req/s at 100 users, avg 2 ms, with no CPU
  warning (`diagnostics/health_u100.txt`).
- **Fix.** One module-level `httpx.Client` serves all setup calls. Setup
  stays out of the stats as before. No service code changed.

### 2. The ledger outbox relay caps the async scoring path at ~40 events/s

ledger-service's relay (`internal/outbox/relay.go`) publishes one batch of
at most 20 events per 500 ms tick: a ceiling of 40 `ledger.entry-recorded`
events per second. Each `POST /transactions` writes one event. Measured
over the steady part of each run (`scripts/summarize_samples.py` on
`*_samples.csv`):

| Users | Inserted tx/s | Published/s | Scored/s | Backlog at Locust end | Drain after end |
|---:|---:|---:|---:|---:|---:|
| 20 (60 s) | 62.5 | 39.6 | 39.9 | 1,288 | 33 s |
| 50 | 153.7 | 40.2 | 40.2 | 5,898 | 153 s |
| 100 | 254.1 | 35.8 | 35.4 | 10,703 | > 180 s (sampling stopped) |

- settlement-engine scores at exactly the published rate, so the queue
  sits in ledger's `outbox_events`, not in NATS or the consumer.
- From 20 users up, the HTTP path accepts more than the relay publishes.
  Scoring lag therefore grows linearly for as long as the load lasts.
- Nothing is lost: the backlog drains at ~40/s afterwards.
- PR #16's run (641 requests in 30 s ≈ 21/s) stayed under the cap, so the
  effect never showed there.
- **Impact.** Risk holds and the alerts that follow them lag behind writes
  by backlog ÷ 40 seconds. That was over 4 minutes after 60 s at 100 users.
- No settings were changed. Batch size and poll interval are constants in
  the relay.

### 3. HTTP latency degrades between 50 and 100 users, with ledger's Postgres as the hot spot

From 50 to 100 users, p95 rose from 59 to 200 ms and server-side average
from 20 to 61 ms. Container and process CPU during each run (from
`*_docker_stats.csv` and `*_meta.txt`):

| Users | ledger Postgres CPU avg / max | ledger-service CPU (of one core) | NATS CPU avg |
|---:|---:|---:|---:|
| 20 | 25% / 42% | 16% | 12% |
| 50 | 129% / 266% | 51% | 15% |
| 100 | 333% / 556% | 97% | 20% |

The services call `sql.Open` without pool settings, so Go's default of
**2 idle connections** applies. Under concurrency, connections are closed
and reopened, and each new Postgres session costs a backend process plus
SCRAM authentication. New ledger sessions per HTTP request, from
`pg_stat_database.sessions` (`diagnostics/pg_sessions.txt`):

| Users | New sessions per request |
|---:|---:|
| 20 | 0.15 |
| 50 | 0.34 |
| 100 | 0.49 |

Open ledger connections peaked at 17 at 100 users.

This makes connection churn a likely contributor to ledger Postgres CPU.
How much of the latency it explains was **not isolated**: that would mean
changing pool settings, which this baseline deliberately did not do. The
outbox relay's polling on the same database is a second, smaller load.

## Smoke thresholds

`tests/perf/locustfile_settlement_scoring.py` sets a non-zero exit code
when Locust quits if any of these hold:

- no requests were made;
- the failure ratio is above `PERF_MAX_FAIL_RATIO` (default **0.01 = 1%**);
- p95 is above `PERF_MAX_P95_MS` (default **100 ms**).

How the defaults were chosen, from the 20-user / 60 s baseline above:

- **Failure ratio, 1%.** The baseline measured 0.00% (0 failures across
  all 8 runs). A broken stack fails close to 100% of requests, so 1% still
  catches it while tolerating a single transient error.
- **p95, 100 ms.** The baseline measured 35 ms, so this is about 3×. The
  headroom covers run-to-run variation (20-user p95 was 30, 35 and 48 ms
  in three runs here) and slower CI hardware: a 4-vCPU runner with
  everything on one host, against 16 threads here.

These limits are for 20 users only. The 100-user baseline (p95 200 ms)
exceeds them by design, so raise `PERF_MAX_P95_MS` for heavier runs.
Verified: a 20-user run exits 0; `PERF_MAX_P95_MS=1` exits 1; a dead host
exits 1 (`diagnostics/thr_*.txt`).

The **Perf smoke** workflow (`.github/workflows/perf-smoke.yml`,
`workflow_dispatch` only) runs 20 users for 30 s with these limits and
uploads the CSV/HTML as an artifact.

## Reproduce

From the repo root, with Docker running and each service's `.env` copied
from `.env.example`:

```bash
docker compose -f infra/docker/docker-compose.yml up -d \
  ledger-postgres accounts-postgres settlement-postgres notification-postgres nats
```

Start ledger-service, accounts-service, settlement-engine (with
`SETTLEMENT_BATCH_INTERVAL_SECONDS=5`) and notification-service exactly as
in `tests/README.md`. Then:

```bash
pip install -r tests/requirements.txt
OUT=tests/perf/results/$(date +%F)
F=tests/perf/locustfile_settlement_scoring.py
locust -f $F --host=http://localhost:8080 --headless -u 20  -r 5 -t 30s --csv $OUT/u20_30s  --html $OUT/u20_30s.html  --only-summary
locust -f $F --host=http://localhost:8080 --headless -u 20  -r 5 -t 60s --csv $OUT/u20_60s  --html $OUT/u20_60s.html  --only-summary
PERF_MAX_P95_MS=1000 \
locust -f $F --host=http://localhost:8080 --headless -u 50  -r 5 -t 60s --csv $OUT/u50_60s  --html $OUT/u50_60s.html  --only-summary
PERF_MAX_P95_MS=1000 \
locust -f $F --host=http://localhost:8080 --headless -u 100 -r 5 -t 60s --csv $OUT/u100_60s --html $OUT/u100_60s.html --only-summary
```

`tests/perf/results/2026-10-17/scripts/run_level.sh` is the wrapper that
produced these results. It runs one level and samples outbox backlog,
scoring lag, Postgres connections and `docker stats` every second, then
waits up to 180 s for the backlog to drain. It was written for Git Bash on
Windows (it reads service CPU through `powershell.exe`). The sampling
queries:

```sql
-- ledger DB
SELECT count(*), count(*) FILTER (WHERE published_at IS NULL),
       (SELECT count(*) FROM pg_stat_activity WHERE datname = 'ledger')
FROM outbox_events;
-- settlement DB
SELECT count(*), (SELECT count(*) FROM pg_stat_activity WHERE datname = 'settlement')
FROM transactions;
```
