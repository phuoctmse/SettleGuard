import time
import uuid

MISMATCH_THRESHOLD_AMOUNT = 20_000_000  # over SETTLEMENT_MISMATCH_THRESHOLD default (10_000_000) -- deterministically triggers a hold
NORMAL_AMOUNT = 1_000


def _wait_until(predicate, timeout=15.0, interval=0.5):
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        result = predicate()
        if result is not None:
            return result
        time.sleep(interval)
    raise AssertionError(f"condition not met within {timeout}s")


def _new_account(accounts):
    client = accounts.create_client(f"E2E {uuid.uuid4()}").json()
    return accounts.create_account(client["id"]).json()["id"]


def test_passing_transaction_flows_to_settlement_and_notification(accounts, ledger, settlement, notifications):
    acc1, acc2 = _new_account(accounts), _new_account(accounts)

    tx = ledger.post_transaction(
        [
            {"account_id": acc1, "direction": "debit", "amount": NORMAL_AMOUNT, "reason": "e2e"},
            {"account_id": acc2, "direction": "credit", "amount": NORMAL_AMOUNT, "reason": "e2e"},
        ]
    ).json()
    # POST /transactions returns a list of entry objects (one per posted
    # entry), each carrying the shared transaction_id -- see
    # services/ledger-service/internal/api/handlers.go writeEntries.
    transaction_id = tx[0]["transaction_id"]

    def scored():
        resp = settlement.get_transaction(transaction_id)
        return resp.json() if resp.status_code == 200 else None

    scored_tx = _wait_until(scored)
    assert scored_tx["status"] == "pending_settlement"

    # settlement-engine batches on a schedule (SETTLEMENT_BATCH_INTERVAL_SECONDS,
    # default 60s) -- run it with SETTLEMENT_BATCH_INTERVAL_SECONDS=5 for this
    # test to complete in reasonable time, see tests/README.md.
    def settled():
        resp = settlement.get_transaction(transaction_id)
        body = resp.json()
        return body if body["status"] == "settled" else None

    _wait_until(settled, timeout=30.0)

    def notified():
        resp = notifications.list_notifications(type_="settlement_finalized")
        matches = [n for n in resp.json() if transaction_id in str(n["payload"])]
        return matches if matches else None

    _wait_until(notified, timeout=15.0)


def test_held_transaction_can_be_approved_and_then_settles(accounts, ledger, settlement, notifications):
    acc1, acc2 = _new_account(accounts), _new_account(accounts)

    tx = ledger.post_transaction(
        [
            {"account_id": acc1, "direction": "debit", "amount": MISMATCH_THRESHOLD_AMOUNT, "reason": "e2e-hold"},
            {"account_id": acc2, "direction": "credit", "amount": MISMATCH_THRESHOLD_AMOUNT, "reason": "e2e-hold"},
        ]
    ).json()
    transaction_id = tx[0]["transaction_id"]

    def held():
        resp = settlement.get_transaction(transaction_id)
        body = resp.json() if resp.status_code == 200 else None
        return body if body and body["status"] == "held" else None

    _wait_until(held)

    def notified_hold():
        resp = notifications.list_notifications(type_="risk_hold")
        matches = [n for n in resp.json() if transaction_id in str(n["payload"])]
        return matches if matches else None

    _wait_until(notified_hold)

    approve_resp = settlement.approve(transaction_id)
    assert approve_resp.status_code == 200
    assert approve_resp.json()["status"] == "pending_settlement"

    def settled():
        resp = settlement.get_transaction(transaction_id)
        body = resp.json()
        return body if body["status"] == "settled" else None

    _wait_until(settled, timeout=30.0)
