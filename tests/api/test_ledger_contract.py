import uuid


def _account_id(accounts):
    client = accounts.create_client(f"Ledger Test {uuid.uuid4()}").json()
    return accounts.create_account(client["id"]).json()["id"]


def test_post_transaction_rejects_unbalanced_entries(ledger, accounts):
    acc = _account_id(accounts)

    resp = ledger.post_transaction(
        [{"account_id": acc, "direction": "debit", "amount": 100, "reason": "test"}]
    )

    assert resp.status_code == 422


def test_post_balanced_transaction_and_list_entries(ledger, accounts):
    acc1, acc2 = _account_id(accounts), _account_id(accounts)

    resp = ledger.post_transaction(
        [
            {"account_id": acc1, "direction": "debit", "amount": 500, "reason": "test"},
            {"account_id": acc2, "direction": "credit", "amount": 500, "reason": "test"},
        ]
    )
    assert resp.status_code in (200, 201)

    entries = ledger.list_entries(account_id=acc1)
    assert entries.status_code == 200
    assert len(entries.json()) >= 1
