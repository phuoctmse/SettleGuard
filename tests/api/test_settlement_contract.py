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
    assert isinstance(resp.json(), list)
