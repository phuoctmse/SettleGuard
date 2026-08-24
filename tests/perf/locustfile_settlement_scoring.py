import os
import uuid

from locust import HttpUser, between, task

ACCOUNTS_API_URL = os.environ.get("ACCOUNTS_API_URL", "http://localhost:8081")


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
        import httpx

        client = httpx.post(f"{ACCOUNTS_API_URL}/clients", json={"name": f"perf-{uuid.uuid4()}"}).json()
        self.account_1 = httpx.post(f"{ACCOUNTS_API_URL}/accounts", json={"client_id": client["id"]}).json()["id"]
        self.account_2 = httpx.post(f"{ACCOUNTS_API_URL}/accounts", json={"client_id": client["id"]}).json()["id"]

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
