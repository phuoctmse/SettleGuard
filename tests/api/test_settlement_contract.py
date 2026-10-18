import time
import uuid


def _wait_until(predicate, timeout=15.0, interval=0.5):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        result = predicate()
        if result is not None:
            return result
        time.sleep(interval)
    raise AssertionError(f"condition not met within {timeout}s")


def test_list_transactions_requires_status(settlement):
    resp = settlement.list_transactions("")
    assert resp.status_code == 400


def test_get_transaction_not_found(settlement):
    resp = settlement.get_transaction("00000000-0000-0000-0000-000000000000")
    assert resp.status_code == 404


def test_approve_not_found(settlement):
    resp = settlement.approve("00000000-0000-0000-0000-000000000000")
    assert resp.status_code == 404


def test_reject_not_found(settlement):
    resp = settlement.reject("00000000-0000-0000-0000-000000000000")
    assert resp.status_code == 404

    # The 409-on-not-held path needs a real held transaction to exercise
    # meaningfully -- covered by tests/security/test_fraud_bypass.py
    # (test_double_approve_second_call_returns_409), not duplicated here.


def test_list_settlements(settlement):
    resp = settlement.list_settlements()
    assert resp.status_code == 200
    body = resp.json()
    assert isinstance(body["items"], list)
    assert body["next_cursor"] is None or isinstance(body["next_cursor"], str)


def test_list_held_transactions_pages_with_cursor(accounts, ledger, settlement):
    # Two over-threshold transactions guarantee at least two held rows.
    client_id = accounts.create_client(f"Paging {uuid.uuid4()}").json()["id"]
    debit = accounts.create_account(client_id).json()["id"]
    credit = accounts.create_account(client_id).json()["id"]
    for _ in range(2):
        ledger.post_transaction([
            {"account_id": debit, "direction": "debit", "amount": 20_000_000, "reason": "paging"},
            {"account_id": credit, "direction": "credit", "amount": 20_000_000, "reason": "paging"},
        ])

    def two_held():
        page = settlement.list_transactions("held", limit=1).json()
        return page if page["next_cursor"] else None

    first = _wait_until(two_held)
    second = settlement.list_transactions("held", limit=1, cursor=first["next_cursor"]).json()

    assert len(first["items"]) == 1 and len(second["items"]) == 1
    assert first["items"][0]["id"] != second["items"][0]["id"]
    assert first["items"][0]["scored_at"] >= second["items"][0]["scored_at"]


def test_list_endpoints_reject_bad_paging(settlement):
    for resp in (
        settlement.list_transactions("held", limit=0),
        settlement.list_settlements(limit=0),
    ):
        assert resp.status_code == 400
        assert resp.json() == {"error": "invalid limit"}
    for resp in (
        settlement.list_transactions("held", cursor="garbage!"),
        settlement.list_settlements(cursor="garbage!"),
    ):
        assert resp.status_code == 400
        assert resp.json() == {"error": "invalid cursor"}
