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
    pytest.fail(f"condition not met within {timeout}s")


def _tenant(gateway_base_url, api_key_factory):
    key = api_key_factory()
    http = GatewayClient(gateway_base_url, key.raw_key).http
    accs = []
    for _ in range(2):
        resp = http.post("/accounts/accounts", json={"client_id": key.client_id})
        if resp.status_code != 201:
            pytest.fail(f"setup: account creation returned {resp.status_code}: {resp.text}")
        accs.append(resp.json()["id"])
    return http, accs


# OWASP API8:2023 Security Misconfiguration
@pytest.mark.xfail(strict=True, raises=AssertionError, reason="SG-PT-01: backend ports 8080-8083 accept unauthenticated requests")
def test_backend_ports_require_authentication(accounts_base_url):
    resp = httpx.post(f"{accounts_base_url}/clients", json={"name": f"Direct {uuid.uuid4()}"}, timeout=5.0)
    if resp.status_code >= 500:
        pytest.fail(f"precondition: direct request got {resp.status_code}, not a real answer")
    assert resp.status_code in (401, 403)


# OWASP API1:2023 Broken Object Level Authorization
@pytest.mark.xfail(strict=True, raises=AssertionError, reason="SG-PT-02: a key for client A can post entries against client B's accounts")
def test_client_cannot_write_to_another_clients_accounts(gateway_base_url, api_key_factory):
    http_a, _ = _tenant(gateway_base_url, api_key_factory)
    _, (b1, b2) = _tenant(gateway_base_url, api_key_factory)

    resp = http_a.post("/ledger/transactions", json={"entries": [
        {"account_id": b1, "direction": "debit", "amount": 100, "reason": "bola"},
        {"account_id": b2, "direction": "credit", "amount": 100, "reason": "bola"},
    ]})
    assert resp.status_code in (403, 404)


# OWASP API1:2023 Broken Object Level Authorization
@pytest.mark.xfail(strict=True, raises=AssertionError, reason="SG-PT-03: /notifications returns every client's rows to any valid key")
def test_client_cannot_read_another_clients_notifications(gateway_base_url, api_key_factory):
    http_a, _ = _tenant(gateway_base_url, api_key_factory)
    http_b, (b1, b2) = _tenant(gateway_base_url, api_key_factory)

    # 20,000,000 is over SETTLEMENT_MISMATCH_THRESHOLD, so B's transaction is
    # held and notification-service records a risk_hold row for it.
    post = http_b.post("/ledger/transactions", json={"entries": [
        {"account_id": b1, "direction": "debit", "amount": 20_000_000, "reason": "tenant-b"},
        {"account_id": b2, "direction": "credit", "amount": 20_000_000, "reason": "tenant-b"},
    ]})
    if post.status_code != 201:
        pytest.fail(f"precondition: B's transaction returned {post.status_code}: {post.text}")
    tx_id = post.json()[0]["transaction_id"]

    def b_sees_it():
        rows = http_b.get("/notifications/notifications", params={"type": "risk_hold", "limit": 200}).json()
        return rows if any(r["subject_id"] == tx_id for r in rows) else None

    _wait_until(b_sees_it)

    resp_a = http_a.get("/notifications/notifications", params={"type": "risk_hold", "limit": 200})
    if resp_a.status_code != 200:
        pytest.fail(f"precondition: A's notifications GET returned {resp_a.status_code}: {resp_a.text}")
    rows_seen_by_a = resp_a.json()
    assert all(r["subject_id"] != tx_id for r in rows_seen_by_a)
