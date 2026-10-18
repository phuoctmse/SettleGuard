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


def _new_account(accounts):
    client = accounts.create_client(f"Security {uuid.uuid4()}").json()
    return accounts.create_account(client["id"]).json()["id"]


def test_velocity_limit_holds_transaction_over_threshold(accounts, ledger, settlement):
    # SETTLEMENT-01/02: more than SETTLEMENT_VELOCITY_LIMIT (default 5)
    # transactions for the same account inside the trailing window holds
    # the transaction that crosses the limit.
    acc1, acc2 = _new_account(accounts), _new_account(accounts)

    last_transaction_id = None
    for _ in range(7):
        tx = ledger.post_transaction(
            [
                {"account_id": acc1, "direction": "debit", "amount": 100, "reason": "velocity"},
                {"account_id": acc2, "direction": "credit", "amount": 100, "reason": "velocity"},
            ]
        ).json()
        last_transaction_id = tx[0]["transaction_id"]

    def held():
        resp = settlement.get_transaction(last_transaction_id)
        body = resp.json() if resp.status_code == 200 else None
        return body if body and body["status"] == "held" else None

    result = _wait_until(held)
    assert "velocity_limit" in result["triggered_rules"]


def test_mismatch_threshold_holds_oversized_transaction(accounts, ledger, settlement):
    # SETTLEMENT-01: amount over SETTLEMENT_MISMATCH_THRESHOLD holds, regardless of velocity.
    acc1, acc2 = _new_account(accounts), _new_account(accounts)

    tx = ledger.post_transaction(
        [
            {"account_id": acc1, "direction": "debit", "amount": 20_000_000, "reason": "mismatch"},
            {"account_id": acc2, "direction": "credit", "amount": 20_000_000, "reason": "mismatch"},
        ]
    ).json()

    def held():
        resp = settlement.get_transaction(tx[0]["transaction_id"])
        body = resp.json() if resp.status_code == 200 else None
        return body if body and body["status"] == "held" else None

    result = _wait_until(held)
    assert "mismatch_threshold" in result["triggered_rules"]


def test_blocklist_bypass_holds_transaction_regardless_of_amount_or_velocity(
    accounts, ledger, settlement, seed_blocklist
):
    # SETTLEMENT-01: blocklist is one of the 3 OR'd hold rules -- a
    # blocked account must hold even a single small, low-velocity
    # transaction that would otherwise pass outright.
    acc1, acc2 = _new_account(accounts), _new_account(accounts)
    seed_blocklist(acc1)

    tx = ledger.post_transaction(
        [
            {"account_id": acc1, "direction": "debit", "amount": 100, "reason": "blocklist"},
            {"account_id": acc2, "direction": "credit", "amount": 100, "reason": "blocklist"},
        ]
    ).json()

    def held():
        resp = settlement.get_transaction(tx[0]["transaction_id"])
        body = resp.json() if resp.status_code == 200 else None
        return body if body and body["status"] == "held" else None

    result = _wait_until(held)
    assert "blocklist" in result["triggered_rules"]


def test_double_approve_second_call_returns_409(accounts, ledger, settlement):
    # SETTLEMENT-05: approve/reject only valid from `held`; a second
    # resolve attempt on an already-resolved transaction must not succeed.
    acc1, acc2 = _new_account(accounts), _new_account(accounts)
    tx = ledger.post_transaction(
        [
            {"account_id": acc1, "direction": "debit", "amount": 20_000_000, "reason": "double-approve"},
            {"account_id": acc2, "direction": "credit", "amount": 20_000_000, "reason": "double-approve"},
        ]
    ).json()

    def held():
        resp = settlement.get_transaction(tx[0]["transaction_id"])
        body = resp.json() if resp.status_code == 200 else None
        return body if body and body["status"] == "held" else None

    _wait_until(held)

    first = settlement.approve(tx[0]["transaction_id"])
    assert first.status_code == 200

    second = settlement.approve(tx[0]["transaction_id"])
    assert second.status_code == 409


def test_held_transaction_excluded_from_settlements_until_approved(accounts, ledger, settlement):
    # SETTLEMENT-04: a held transaction must never appear in a settlement
    # batch while still held.
    acc1, acc2 = _new_account(accounts), _new_account(accounts)
    tx = ledger.post_transaction(
        [
            {"account_id": acc1, "direction": "debit", "amount": 20_000_000, "reason": "held-exclusion"},
            {"account_id": acc2, "direction": "credit", "amount": 20_000_000, "reason": "held-exclusion"},
        ]
    ).json()

    def held():
        resp = settlement.get_transaction(tx[0]["transaction_id"])
        body = resp.json() if resp.status_code == 200 else None
        return body if body and body["status"] == "held" else None

    _wait_until(held)

    # give a batch cycle a chance to run and confirm it was skipped
    time.sleep(6)

    for s in settlement.list_settlements().json():
        detail = settlement.get_settlement(s["id"]).json()
        assert tx[0]["transaction_id"] not in detail["transaction_ids"]
