#!/usr/bin/env bash
# Usage: perf_run.sh <label> <users> <spawn_rate> <duration> <outdir>
# Runs one headless Locust level and samples backlog/lag/connections/CPU
# around it. Sampling continues after Locust stops until the ledger outbox
# backlog and scoring lag drain (or 180 s), so drain time is measured too.
set -u
label=$1; users=$2; rate=$3; dur=$4; out=$5
mkdir -p "$out"
LOCUST=/d/SettleGuard/tests/.venv/Scripts/locust.exe
LOCUSTFILE=/d/SettleGuard/tests/perf/locustfile_settlement_scoring.py
samples="$out/${label}_samples.csv"
dstats="$out/${label}_docker_stats.csv"

sample_db() {
  local l s
  l=$(docker exec settleguard-ledger-dev psql -U ledger -d ledger -tAF, -c \
    "SELECT count(*), count(*) FILTER (WHERE published_at IS NULL), (SELECT count(*) FROM pg_stat_activity WHERE datname='ledger') FROM outbox_events" 2>/dev/null)
  s=$(docker exec settleguard-settlement-dev psql -U settlement -d settlement -tAF, -c \
    "SELECT count(*), (SELECT count(*) FROM pg_stat_activity WHERE datname='settlement') FROM transactions" 2>/dev/null)
  echo "$(date +%s.%N | cut -c1-14),$l,$s"
}

echo "epoch,ledger_tx_total,ledger_outbox_unpublished,ledger_pg_conns,settlement_tx_total,settlement_pg_conns" > "$samples"
echo "epoch,container,cpu_pct,mem" > "$dstats"

( while :; do sample_db >> "$samples"; sleep 1; done ) &
db_pid=$!
( while :; do
    ts=$(date +%s)
    docker stats --no-stream --format '{{.Name}},{{.CPUPerc}},{{.MemUsage}}' 2>/dev/null \
      | grep settleguard | sed "s/^/$ts,/" >> "$dstats"
  done ) &
ds_pid=$!

cpu_before=$(powershell.exe -NoProfile -Command "Get-Process ledger-service,accounts-service,settlement-engine | ForEach-Object { \$_.Name + '=' + [math]::Round(\$_.CPU,2) }" | tr -d '\r' | tr '\n' ' ')
start=$(date +%s.%N)
"$LOCUST" -f "$LOCUSTFILE" --host=http://localhost:8080 --headless \
  -u "$users" -r "$rate" -t "$dur" \
  --csv "$out/$label" --html "$out/$label.html" --only-summary \
  > "$out/${label}_console.txt" 2>&1
locust_rc=$?
end=$(date +%s.%N)
cpu_after=$(powershell.exe -NoProfile -Command "Get-Process ledger-service,accounts-service,settlement-engine | ForEach-Object { \$_.Name + '=' + [math]::Round(\$_.CPU,2) }" | tr -d '\r' | tr '\n' ' ')

# Drain: wait for outbox backlog 0 and settlement caught up with ledger.
drain_start=$(date +%s)
while :; do
  row=$(tail -n 1 "$samples")
  IFS=, read -r _ ltot lunpub _ stot _ <<< "$row"
  if [ "${lunpub:-1}" = 0 ] && [ -n "${stot:-}" ] && [ "$stot" -ge "${ltot:-0}" ]; then break; fi
  [ $(( $(date +%s) - drain_start )) -ge 180 ] && { echo "drain timeout" >> "$out/${label}_meta.txt"; break; }
  sleep 1
done
drain_end=$(date +%s)
kill $db_pid $ds_pid 2>/dev/null; wait $db_pid $ds_pid 2>/dev/null

{
  echo "label=$label users=$users spawn_rate=$rate duration=$dur locust_rc=$locust_rc"
  echo "locust_start=$start locust_end=$end"
  echo "drain_seconds_after_locust_end=$((drain_end - drain_start))"
  echo "service_cpu_seconds_before: $cpu_before"
  echo "service_cpu_seconds_after:  $cpu_after"
} >> "$out/${label}_meta.txt"
cat "$out/${label}_meta.txt"
