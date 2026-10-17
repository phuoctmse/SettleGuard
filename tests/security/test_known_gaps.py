# Documented gaps, kept as executable findings (spec §4.3). Each test asserts
# the SAFE behaviour; strict xfail flips red the day the gap is closed so the
# report gets updated in the same change.
import time
import uuid

import httpx
import pytest

from clients.gateway import GatewayClient


def _wait_until(predicate, timeout=20.0, interval=0.5):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        result = predicate()
        if result is not None:
            return result
        time.sleep(interval)
    raise AssertionError(f"condition not met within {timeout}s")


def _tenant(gateway_base_url, api_key_factory):
    key = api_key_factory()
    http = GatewayClient(gateway_base_url, key.raw_key).http
    accs = [http.post("/accounts/accounts", json={"client_id": key.client_id}).json()["id"] for _ in range(2)]
    return http, accs


# OWASP API8:2023 Security Misconfiguration
@pytest.mark.xfail(strict=True, reason="SG-PT-01: backend ports 8080-8083 accept unauthenticated requests")
def test_backend_ports_require_authentication(accounts_base_url):
    resp = httpx.post(f"{accounts_base_url}/clients", json={"name": f"Direct {uuid.uuid4()}"}, timeout=5.0)
    assert resp.status_code in (401, 403)


# OWASP API1:2023 Broken Object Level Authorization
@pytest.mark.xfail(strict=True, reason="SG-PT-02: a key for client A can post entries against client B's accounts")
def test_client_cannot_write_to_another_clients_accounts(gateway_base_url, api_key_factory):
    http_a, _ = _tenant(gateway_base_url, api_key_factory)
    _, (b1, b2) = _tenant(gateway_base_url, api_key_factory)

    resp = http_a.post("/ledger/transactions", json={"entries": [
        {"account_id": b1, "direction": "debit", "amount": 100, "reason": "bola"},
        {"account_id": b2, "direction": "credit", "amount": 100, "reason": "bola"},
    ]})
    assert resp.status_code in (403, 404)


# OWASP API1:2023 Broken Object Level Authorization
@pytest.mark.xfail(strict=True, reason="SG-PT-03: /notifications returns every client's rows to any valid key")
def test_client_cannot_read_another_clients_notifications(gateway_base_url, api_key_factory):
    http_a, _ = _tenant(gateway_base_url, api_key_factory)
    http_b, (b1, b2) = _tenant(gateway_base_url, api_key_factory)

    # 20,000,000 is over SETTLEMENT_MISMATCH_THRESHOLD, so B's transaction is
    # held and notification-service records a risk_hold row for it.
    tx_id = http_b.post("/ledger/transactions", json={"entries": [
        {"account_id": b1, "direction": "debit", "amount": 20_000_000, "reason": "tenant-b"},
        {"account_id": b2, "direction": "credit", "amount": 20_000_000, "reason": "tenant-b"},
    ]}).json()[0]["transaction_id"]

    def b_sees_it():
        rows = http_b.get("/notifications/notifications", params={"type": "risk_hold", "limit": 200}).json()
        return rows if any(r["subject_id"] == tx_id for r in rows) else None

    _wait_until(b_sees_it)

    rows_seen_by_a = http_a.get("/notifications/notifications", params={"type": "risk_hold", "limit": 200}).json()
    assert all(r["subject_id"] != tx_id for r in rows_seen_by_a)
