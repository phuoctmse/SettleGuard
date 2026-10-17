import logging
import os
import uuid

import httpx
from locust import HttpUser, between, events, task

ACCOUNTS_API_URL = os.environ.get("ACCOUNTS_API_URL", "http://localhost:8081")

# Pass/fail limits for the 20-user smoke, set from the 20-user/60s baseline
# in docs/perf-baseline.md (0.00% failures, p95 35 ms on one dev host):
# - failure ratio 1%: a broken stack fails ~100% of requests, so this still
#   catches it while tolerating a single transient error.
# - p95 100 ms: ~3x the measured 35 ms, to absorb run-to-run variation and
#   slower CI hardware (4 vCPU runner, everything on one host).
# Calibrated for 20 users only; heavier runs exceed the p95 limit by design
# (the 100-user baseline measured 200 ms), so raise it via env for those.
MAX_FAIL_RATIO = float(os.environ.get("PERF_MAX_FAIL_RATIO", "0.01"))
MAX_P95_MS = float(os.environ.get("PERF_MAX_P95_MS", "100"))

# One client shared by every simulated user's setup calls. Module-level
# httpx.post() builds a fresh client (and SSL context) per call, measured
# at 210-430 ms of CPU each; every Locust user runs on one gevent hub, so that
# CPU stalled all in-flight requests. With 3 calls per on_start and a spawn
# rate of 5/s, Locust sat above 90% CPU for the whole ramp, inflating the
# measured POST /transactions latency at 50+ users (server-side p99 195 ms
# vs. 2.9 s measured) and never finishing the 100-user ramp in 60 s.
# Setup calls stay out of Locust's stats, as before.
_accounts = httpx.Client(base_url=ACCOUNTS_API_URL)


@events.quitting.add_listener
def _enforce_thresholds(environment, **_kwargs):
    total = environment.stats.total
    p95 = total.get_response_time_percentile(0.95)
    if total.num_requests == 0:
        logging.error("threshold: no requests were made")
    elif total.fail_ratio > MAX_FAIL_RATIO:
        logging.error("threshold: failure ratio %.2f%% > %.2f%%", total.fail_ratio * 100, MAX_FAIL_RATIO * 100)
    elif p95 > MAX_P95_MS:
        logging.error("threshold: p95 %.0f ms > %.0f ms", p95, MAX_P95_MS)
    else:
        logging.info(
            "threshold: ok (failure ratio %.2f%% <= %.2f%%, p95 %.0f ms <= %.0f ms)",
            total.fail_ratio * 100,
            MAX_FAIL_RATIO * 100,
            p95,
            MAX_P95_MS,
        )
        return
    environment.process_exit_code = 1


class TransactionScoringUser(HttpUser):
    """
    Load-tests settlement-engine's real-time scoring path indirectly, by
    posting transactions to ledger-service (the actual entry point --
    settlement-engine has no HTTP endpoint that triggers scoring directly,
    it only consumes ledger.entry-recorded off NATS). host should be
    ledger-service's base URL, e.g.:

        locust -f locustfile_settlement_scoring.py --host=http://localhost:8080

    Note: batch settlement throughput is NOT covered here -- RunBatch is
    schedule-driven (SETTLEMENT_BATCH_INTERVAL_SECONDS), not
    HTTP-triggered, so there's no request to load-test for that path.
    Measuring it would mean sampling settlement-engine's DB/logs over a
    fixed wall-clock window instead of a request-based Locust task --
    left as a manual follow-up, not scripted here.
    """

    wait_time = between(0.1, 0.5)

    def on_start(self):
        client = _accounts.post("/clients", json={"name": f"perf-{uuid.uuid4()}"}).json()
        self.account_1 = _accounts.post("/accounts", json={"client_id": client["id"]}).json()["id"]
        self.account_2 = _accounts.post("/accounts", json={"client_id": client["id"]}).json()["id"]

    @task
    def post_transaction(self):
        self.client.post(
            "/transactions",
            json={
                "entries": [
                    {"account_id": self.account_1, "direction": "debit", "amount": 100, "reason": "perf"},
                    {"account_id": self.account_2, "direction": "credit", "amount": 100, "reason": "perf"},
                ]
            },
        )
