# OWASP API8:2023 Security Misconfiguration / API6:2023 -- hostile input
# through the gateway must be refused cleanly: never 5xx, never internals.
import pytest

from clients.gateway import GatewayClient

LEAK_MARKERS = ["panic", "goroutine", "pq:", "SQLSTATE", "Traceback", ".go:", ".py\""]
SQLI = ["' OR '1'='1", "'; DROP TABLE accounts; --", "\" OR 1=1 --", "1; SELECT pg_sleep(5)"]


@pytest.fixture
def gw(gateway_base_url, api_key_factory):
    key = api_key_factory()
    client = GatewayClient(gateway_base_url, key.raw_key)
    acc1 = client.http.post("/accounts/accounts", json={"client_id": key.client_id}).json()["id"]
    acc2 = client.http.post("/accounts/accounts", json={"client_id": key.client_id}).json()["id"]
    return client.http, key.client_id, acc1, acc2


def _assert_refused_cleanly(resp):
    assert resp.status_code < 500, f"{resp.status_code}: {resp.text[:200]}"
    if resp.status_code >= 400:
        assert set(resp.json()) == {"error"}
    for marker in LEAK_MARKERS:
        assert marker not in resp.text


def _tx(acc1, acc2, amount, reason="pentest"):
    return {"entries": [
        {"account_id": acc1, "direction": "debit", "amount": amount, "reason": reason},
        {"account_id": acc2, "direction": "credit", "amount": amount, "reason": reason},
    ]}


# OWASP API8:2023 Security Misconfiguration (injection via client name)
@pytest.mark.parametrize("payload", SQLI)
def test_sqli_in_client_name_is_stored_verbatim(gw, payload):
    http, *_ = gw
    resp = http.post("/accounts/clients", json={"name": payload})
    _assert_refused_cleanly(resp)
    if resp.status_code == 201:
        assert http.get(f"/accounts/clients/{resp.json()['id']}").json()["name"] == payload


# OWASP API8:2023 Security Misconfiguration (injection via entry reason)
@pytest.mark.parametrize("payload", SQLI)
def test_sqli_in_entry_reason_is_stored_verbatim(gw, payload):
    http, _, acc1, acc2 = gw
    resp = http.post("/ledger/transactions", json=_tx(acc1, acc2, 100, payload))
    _assert_refused_cleanly(resp)
    if resp.status_code == 201:
        assert {e["reason"] for e in resp.json()} == {payload}


# OWASP API6:2023 Unrestricted Access to Sensitive Business Flows (hostile amounts)
@pytest.mark.parametrize(
    "amount",
    [2**63, -1, 0, 1.5, "100", None, 2**64],
    ids=["2^63", "negative", "zero", "float", "string", "null", "2^64"],
)
def test_hostile_amounts_are_refused(gw, amount):
    http, _, acc1, acc2 = gw
    resp = http.post("/ledger/transactions", json=_tx(acc1, acc2, amount))
    _assert_refused_cleanly(resp)
    assert resp.status_code in (400, 422)


# OWASP API6:2023 Unrestricted Access to Sensitive Business Flows (int64 overflow)
def test_entries_summing_past_int64_are_refused(gw):
    # Regression for #24: each entry is valid int64, their sum is not.
    http, _, acc1, acc2 = gw
    big = 2**63 - 1
    body = {"entries": [
        {"account_id": acc1, "direction": "debit", "amount": big, "reason": "overflow"},
        {"account_id": acc1, "direction": "debit", "amount": big, "reason": "overflow"},
        {"account_id": acc2, "direction": "credit", "amount": big, "reason": "overflow"},
        {"account_id": acc2, "direction": "credit", "amount": big, "reason": "overflow"},
    ]}
    resp = http.post("/ledger/transactions", json=body)
    _assert_refused_cleanly(resp)
    assert resp.status_code == 422


# OWASP API6:2023 Unrestricted Access to Sensitive Business Flows (unbalanced entries)
def test_unbalanced_transaction_is_refused(gw):
    http, _, acc1, acc2 = gw
    body = _tx(acc1, acc2, 100)
    body["entries"][1]["amount"] = 99
    resp = http.post("/ledger/transactions", json=body)
    _assert_refused_cleanly(resp)
    assert resp.status_code == 422


# OWASP API8:2023 Security Misconfiguration (malformed bodies)
@pytest.mark.parametrize(
    "raw,content_type",
    [(b"{not json", "application/json"), (b"", "application/json"), (b"name=x", "application/x-www-form-urlencoded"), (b"\xff\xfe\x00", "application/json")],
    ids=["malformed", "empty", "form-encoded", "binary"],
)
def test_malformed_bodies_are_refused(gw, raw, content_type):
    http, *_ = gw
    resp = http.post("/accounts/clients", content=raw, headers={"Content-Type": content_type})
    _assert_refused_cleanly(resp)
    assert resp.status_code == 400


# OWASP API8:2023 Security Misconfiguration (malformed path ids)
@pytest.mark.parametrize("bad_id", ["not-a-uuid", "00000000-0000-0000-0000-000000000000'--", "%00"])
def test_malformed_path_ids_are_refused(gw, bad_id):
    http, *_ = gw
    for path in [f"/accounts/clients/{bad_id}", f"/settlement/transactions/{bad_id}", f"/settlement/settlements/{bad_id}"]:
        resp = http.get(path)
        _assert_refused_cleanly(resp)
        assert resp.status_code in (400, 404)


# OWASP API8:2023 Security Misconfiguration (NUL byte reaches Postgres)
@pytest.mark.xfail(strict=True, reason="SG-PT-06: NUL byte in a TEXT field reaches Postgres and returns 500")
def test_nul_byte_in_text_field_is_refused(gw):
    http, *_ = gw
    _assert_refused_cleanly(http.post("/accounts/clients", json={"name": "a\u0000b"}))


# OWASP API4:2023 Unrestricted Resource Consumption (unvalidated limit)
@pytest.mark.xfail(strict=True, reason="SG-PT-07: negative ?limit= on /notifications is not validated and returns 5xx")
def test_negative_notification_limit_is_refused(gw):
    http, *_ = gw
    resp = http.get("/notifications/notifications", params={"limit": "-1"})
    _assert_refused_cleanly(resp)
    assert resp.status_code == 400


# OWASP API4:2023 Unrestricted Resource Consumption (no body size limit)
@pytest.mark.xfail(strict=True, reason="SG-PT-08: no request body size limit on any service")
def test_oversized_body_is_refused(gw):
    http, *_ = gw
    resp = http.post("/accounts/clients", json={"name": "x" * (5 * 1024 * 1024)})
    assert resp.status_code in (400, 413)
